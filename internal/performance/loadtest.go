package performance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os/exec"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type LoadTestSpec struct {
	URL         string            `json:"url"`
	Method      string            `json:"method"`
	Concurrency int               `json:"concurrency"` // 1 - 50
	DurationSec int               `json:"durationSec"` // 1 - 60
	MaxRequests int               `json:"maxRequests"`
	Headers     map[string]string `json:"headers"`
	Cookies     string            `json:"cookies"`
	Body        string            `json:"body"`
}

type LoadTestPoint struct {
	ElapsedSec    float64 `json:"elapsedSec"`
	RequestsCount int     `json:"requestsCount"`
	RPS           float64 `json:"rps"`
	AvgMs         float64 `json:"avgMs"`
}

type AuthCredentials struct {
	Type  string         `json:"type"` // "bearer" or "cookie"
	Token string         `json:"token,omitempty"`
	Name  string         `json:"name,omitempty"`
	Value string         `json:"value,omitempty"`
	User  map[string]any `json:"user,omitempty"`
}

type LoadTestResult struct {
	TotalRequests   int             `json:"totalRequests"`
	Successful      int             `json:"successful"`
	Failed          int             `json:"failed"`
	DurationSec     float64         `json:"durationSec"`
	RPS             float64         `json:"rps"`
	MinMs           float64         `json:"minMs"`
	MaxMs           float64         `json:"maxMs"`
	AvgMs           float64         `json:"avgMs"`
	P50Ms           float64         `json:"p50Ms"`
	P90Ms           float64         `json:"p90Ms"`
	P99Ms           float64         `json:"p99Ms"`
	StatusCodes     map[int]int     `json:"statusCodes"`
	TimeSeries      []LoadTestPoint `json:"timeSeries"`
	RedirectWarning bool            `json:"redirectWarning"`
	Errors          []string        `json:"errors,omitempty"`
}

// RunLoadTest executes a high-performance concurrent HTTP load benchmark
func RunLoadTest(ctx context.Context, spec LoadTestSpec) (*LoadTestResult, error) {
	if spec.URL == "" {
		return nil, errors.New("target URL is required")
	}
	if spec.Concurrency <= 0 {
		spec.Concurrency = 5
	}
	if spec.Concurrency > 50 {
		spec.Concurrency = 50
	}
	if spec.DurationSec <= 0 {
		spec.DurationSec = 5
	}
	if spec.DurationSec > 60 {
		spec.DurationSec = 60
	}
	if spec.Method == "" {
		spec.Method = http.MethodGet
	}

	testCtx, cancel := context.WithTimeout(ctx, time.Duration(spec.DurationSec)*time.Second)
	defer cancel()

	var (
		totalReqs  int64
		successful int64
		failed     int64
		latencies  []float64
		latMu      sync.Mutex
		statusMap  = make(map[int]int)
		statusMu   sync.Mutex
		errList    []string
		errMu      sync.Mutex
	)

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: spec.Concurrency,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	start := time.Now()
	var wg sync.WaitGroup

	// Time-series collector (every 500ms)
	var timeSeries []LoadTestPoint
	var tsMu sync.Mutex
	ticker := time.NewTicker(500 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		var lastReqs int64
		for {
			select {
			case <-testCtx.Done():
				return
			case t := <-ticker.C:
				currentReqs := atomic.LoadInt64(&totalReqs)
				delta := currentReqs - lastReqs
				lastReqs = currentReqs
				elapsed := t.Sub(start).Seconds()

				latMu.Lock()
				var curAvg float64
				if len(latencies) > 0 {
					var s float64
					recentSlice := latencies
					if len(recentSlice) > 50 {
						recentSlice = recentSlice[len(recentSlice)-50:]
					}
					for _, v := range recentSlice {
						s += v
					}
					curAvg = math.Round((s/float64(len(recentSlice)))*10) / 10
				}
				latMu.Unlock()

				tsMu.Lock()
				timeSeries = append(timeSeries, LoadTestPoint{
					ElapsedSec:    math.Round(elapsed*10) / 10,
					RequestsCount: int(currentReqs),
					RPS:           math.Round(float64(delta)*2*10) / 10,
					AvgMs:         curAvg,
				})
				tsMu.Unlock()
			}
		}
	}()

	for i := 0; i < spec.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-testCtx.Done():
					return
				default:
					if spec.MaxRequests > 0 && atomic.LoadInt64(&totalReqs) >= int64(spec.MaxRequests) {
						return
					}

					req, err := http.NewRequestWithContext(testCtx, spec.Method, spec.URL, bytes.NewBufferString(spec.Body))
					if err != nil {
						atomic.AddInt64(&failed, 1)
						return
					}

					for k, v := range spec.Headers {
						req.Header.Set(k, v)
					}
					if spec.Cookies != "" {
						req.Header.Set("Cookie", spec.Cookies)
					}
					req.Header.Set("User-Agent", "LaraDeck-LoadTest/1.0")

					reqStart := time.Now()
					resp, err := client.Do(req)
					reqDur := float64(time.Since(reqStart).Microseconds()) / 1000.0

					atomic.AddInt64(&totalReqs, 1)

					if err != nil {
						atomic.AddInt64(&failed, 1)
						errMu.Lock()
						if len(errList) < 5 {
							errList = append(errList, err.Error())
						}
						errMu.Unlock()
						continue
					}

					statusMu.Lock()
					statusMap[resp.StatusCode]++
					statusMu.Unlock()

					if resp.StatusCode >= 200 && resp.StatusCode < 400 {
						atomic.AddInt64(&successful, 1)
					} else {
						atomic.AddInt64(&failed, 1)
					}

					_ = resp.Body.Close()

					latMu.Lock()
					latencies = append(latencies, reqDur)
					latMu.Unlock()
				}
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(start).Seconds()
	if totalDuration <= 0 {
		totalDuration = 0.001
	}

	// Detect if majority of responses were redirects (indicating unauthenticated route redirect)
	redirects := statusMap[301] + statusMap[302] + statusMap[307] + statusMap[308]
	redirectWarning := false
	if totalReqs > 0 && float64(redirects)/float64(totalReqs) >= 0.5 {
		redirectWarning = true
		errList = append(errList, "Warning: Over 50% of responses were redirects (3xx). The test likely measured the login redirect rather than your page. Run again as a signed-in user.")
	}

	tsMu.Lock()
	collectedTS := timeSeries
	tsMu.Unlock()

	res := &LoadTestResult{
		TotalRequests:   int(totalReqs),
		Successful:      int(successful),
		Failed:          int(failed),
		DurationSec:     math.Round(totalDuration*100) / 100,
		RPS:             math.Round((float64(totalReqs)/totalDuration)*10) / 10,
		StatusCodes:     statusMap,
		TimeSeries:      collectedTS,
		RedirectWarning: redirectWarning,
		Errors:          errList,
	}

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		var sum float64
		for _, l := range latencies {
			sum += l
		}
		res.MinMs = math.Round(latencies[0]*100) / 100
		res.MaxMs = math.Round(latencies[len(latencies)-1]*100) / 100
		res.AvgMs = math.Round((sum/float64(len(latencies)))*100) / 100
		res.P50Ms = math.Round(latencies[int(float64(len(latencies))*0.50)]*100) / 100
		res.P90Ms = math.Round(latencies[int(float64(len(latencies))*0.90)]*100) / 100
		p99Idx := int(float64(len(latencies)) * 0.99)
		if p99Idx >= len(latencies) {
			p99Idx = len(latencies) - 1
		}
		res.P99Ms = math.Round(latencies[p99Idx]*100) / 100
	}

	return res, nil
}

// GenerateAuthCredentials issues a session cookie or Sanctum bearer token for a selected test user
func GenerateAuthCredentials(ctx context.Context, projectPath, phpBinary, identifier string) (*AuthCredentials, error) {
	if phpBinary == "" {
		phpBinary = "php"
	}
	if identifier == "" {
		identifier = "1"
	}

	phpCode := `
require_once __DIR__ . '/vendor/autoload.php';
$app = require_once __DIR__ . '/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();

$id = $argv[1];
$user = null;

if (class_exists(\App\Models\User::class)) {
    if (is_numeric($id)) {
        $user = \App\Models\User::find($id);
    } elseif (str_contains($id, '@')) {
        $user = \App\Models\User::where('email', $id)->first();
    } elseif (class_exists(\Spatie\Permission\Models\Role::class)) {
        $user = \App\Models\User::role($id)->first();
    }
    if (!$user) {
        $user = \App\Models\User::first();
    }
}

if (!$user) {
    echo json_encode(['error' => 'No matching User model found in database']);
    exit(1);
}

// Prefer Sanctum if installed
if (method_exists($user, 'createToken')) {
    $token = $user->createToken('laradeck_load_test')->plainTextToken;
    echo json_encode([
        'type' => 'bearer',
        'token' => $token,
        'user' => [
            'id' => $user->id ?? $user->getKey(),
            'email' => $user->email ?? 'unknown',
        ]
    ]);
    exit(0);
}

// Fallback to session cookie
if ($app->bound('session')) {
    $session = $app['session']->driver();
    $session->start();
    $guard = $app['auth']->guard()->getName();
    $session->put($guard, $user->getAuthIdentifier());
    $session->save();
    $cookieName = config('session.cookie', 'laravel_session');
    $val = $session->getId();
    if ($app->bound('encrypter')) {
        $val = $app['encrypter']->encrypt($val, false);
    }
    echo json_encode([
        'type' => 'cookie',
        'name' => $cookieName,
        'value' => $val,
        'user' => [
            'id' => $user->id ?? $user->getKey(),
            'email' => $user->email ?? 'unknown',
        ]
    ]);
    exit(0);
}

echo json_encode(['error' => 'Neither Sanctum nor Session guard available']);
exit(1);
`

	cmd := exec.CommandContext(ctx, phpBinary, "-r", phpCode, identifier)
	cmd.Dir = projectPath
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to generate auth credentials: %w (output: %s)", err, string(out))
	}

	var creds AuthCredentials
	if err := json.Unmarshal(out, &creds); err != nil {
		return nil, fmt.Errorf("failed to parse auth credentials JSON: %w (output: %s)", err, string(out))
	}

	return &creds, nil
}
