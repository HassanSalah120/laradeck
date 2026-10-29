package testing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/runner"
)

type APIAssertion struct {
	Type   string `json:"type"` // "status_code", "body_contains", "header_equals", "max_latency"
	Target string `json:"target"`
	Value  string `json:"value"`
}

type AssertionResult struct {
	Type    string `json:"type"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

type APIRequestSpec struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Method     string            `json:"method"`
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	Assertions []APIAssertion    `json:"assertions"`
}

type APIResponseResult struct {
	StatusCode       int               `json:"statusCode"`
	DurationMs       float64           `json:"durationMs"`
	Headers          map[string]string `json:"headers"`
	Body             string            `json:"body"`
	Size             int               `json:"size"`
	AssertionResults []AssertionResult `json:"assertionResults"`
	Passed           bool              `json:"passed"`
	Error            string            `json:"error,omitempty"`
	Truncated        bool              `json:"truncated"`
}

type LoadTestSpec struct {
	TargetURL       string            `json:"targetUrl"`
	Method          string            `json:"method"`
	VirtualUsers    int               `json:"virtualUsers"`
	DurationSeconds int               `json:"durationSeconds"`
	Headers         map[string]string `json:"headers"`
	Body            string            `json:"body"`
	AllowMutation   bool              `json:"allowMutation"`
	Engine          string            `json:"engine,omitempty"` // "native", "k6", or "auto"
}

type LoadTestResult struct {
	Engine             string      `json:"engine"`
	TotalRequests      int         `json:"totalRequests"`
	Successful         int         `json:"successful"`
	Failed             int         `json:"failed"`
	RequestsPerSecond  float64     `json:"requestsPerSecond"`
	MinLatencyMs       float64     `json:"minLatencyMs"`
	MaxLatencyMs       float64     `json:"maxLatencyMs"`
	AvgLatencyMs       float64     `json:"avgLatencyMs"`
	P50LatencyMs       float64     `json:"p50LatencyMs"`
	P90LatencyMs       float64     `json:"p90LatencyMs"`
	P99LatencyMs       float64     `json:"p99LatencyMs"`
	StatusDistribution map[int]int `json:"statusDistribution"`
	ErrorSample        []string    `json:"errorSample"`
	CompletedAt        string      `json:"completedAt"`
}

type ProjectTestResult struct {
	Suite    string   `json:"suite"`
	Total    int      `json:"total"`
	Passed   int      `json:"passed"`
	Failed   int      `json:"failed"`
	Skipped  int      `json:"skipped"`
	Duration float64  `json:"durationSeconds"`
	Output   []string `json:"output"`
	Success  bool     `json:"success"`
}

type TestCoordinator struct {
	httpClient *http.Client
}

func NewTestCoordinator() *TestCoordinator {
	return &TestCoordinator{
		httpClient: &http.Client{
			Timeout:   15 * time.Second,
			Transport: LocalTransport(),
		},
	}
}

// ExecuteAPIRequest sends an HTTP request and validates assertions.
func (tc *TestCoordinator) ExecuteAPIRequest(ctx context.Context, spec APIRequestSpec) (*APIResponseResult, error) {
	if spec.Method == "" {
		spec.Method = "GET"
	}
	u, err := url.Parse(spec.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	var reqBody io.Reader
	if spec.Body != "" {
		reqBody = bytes.NewBufferString(spec.Body)
	}

	req, err := http.NewRequestWithContext(ctx, spec.Method, u.String(), reqBody)
	if err != nil {
		return nil, err
	}

	for k, v := range spec.Headers {
		if strings.EqualFold(k, "Host") {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "HerdWorkbench-APIClient/1.0")
	}

	start := time.Now()
	resp, err := tc.httpClient.Do(req)
	duration := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return &APIResponseResult{
			DurationMs: duration,
			Passed:     false,
			Error:      err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024+1)) // Detect truncation rather than validating an incomplete response.
	if err != nil {
		return nil, err
	}

	duration = float64(time.Since(start).Microseconds()) / 1000.0
	truncated := len(bodyBytes) > 512*1024
	if truncated {
		bodyBytes = bodyBytes[:512*1024]
	}
	hdrs := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			hdrs[k] = v[0]
		}
	}

	result := &APIResponseResult{
		StatusCode: resp.StatusCode,
		DurationMs: duration,
		Headers:    hdrs,
		Body:       string(bodyBytes),
		Size:       len(bodyBytes),
		Passed:     !truncated,
		Truncated:  truncated,
	}

	if truncated {
		result.Error = "Response exceeded 512 KiB; assertions cannot establish a complete response"
	}
	// Evaluate assertions
	for _, a := range spec.Assertions {
		ar := evaluateAssertion(a, result)
		result.AssertionResults = append(result.AssertionResults, ar)
		if !ar.Passed {
			result.Passed = false
		}
	}

	return result, nil
}

func evaluateAssertion(a APIAssertion, res *APIResponseResult) AssertionResult {
	switch a.Type {
	case "status_code":
		expected := a.Value
		actual := fmt.Sprintf("%d", res.StatusCode)
		passed := expected == actual
		msg := fmt.Sprintf("Status code: expected %s, got %s", expected, actual)
		return AssertionResult{Type: a.Type, Passed: passed, Message: msg}

	case "body_contains":
		passed := strings.Contains(res.Body, a.Value)
		msg := fmt.Sprintf("Body contains %q: %v", a.Value, passed)
		return AssertionResult{Type: a.Type, Passed: passed, Message: msg}

	case "header_equals":
		actual := res.Headers[http.CanonicalHeaderKey(a.Target)]
		passed := actual == a.Value
		msg := fmt.Sprintf("Header %s: expected %q, got %q", a.Target, a.Value, actual)
		return AssertionResult{Type: a.Type, Passed: passed, Message: msg}

	case "max_latency":
		maxLat, err := strconv.ParseFloat(a.Value, 64)
		if err != nil || math.IsNaN(maxLat) || math.IsInf(maxLat, 0) || maxLat < 0 {
			return AssertionResult{Type: a.Type, Passed: false, Message: "max_latency requires a nonnegative finite number"}
		}
		passed := res.DurationMs <= maxLat
		msg := fmt.Sprintf("Latency %.1fms <= %.1fms: %v", res.DurationMs, maxLat, passed)
		return AssertionResult{Type: a.Type, Passed: passed, Message: msg}

	default:
		// Explicit failure for unknown assertion types (Finding 13)
		return AssertionResult{
			Type:    a.Type,
			Passed:  false,
			Message: fmt.Sprintf("Unknown assertion type %q; supported types are status_code, body_contains, header_equals, max_latency", a.Type),
		}
	}
}

// GenerateBoundaryCases creates automated edge-case specifications from a base request.
func (tc *TestCoordinator) GenerateBoundaryCases(base APIRequestSpec) []APIRequestSpec {
	var cases []APIRequestSpec

	// Case 1: Empty body
	emptyCase := base
	emptyCase.ID = base.ID + "_empty"
	emptyCase.Name = base.Name + " · Edge: Empty Payload"
	emptyCase.Body = "{}"
	cases = append(cases, emptyCase)

	// Case 2: Excessive payload (10KB string)
	largeCase := base
	largeCase.ID = base.ID + "_large"
	largeCase.Name = base.Name + " · Edge: Overflow String"
	largeCase.Body = fmt.Sprintf(`{"overflow": "%s"}`, strings.Repeat("A", 10000))
	cases = append(cases, largeCase)

	// Case 3: SQL Probe string
	sqlCase := base
	sqlCase.ID = base.ID + "_sql"
	sqlCase.Name = base.Name + " · Edge: SQL Boundary Probe"
	sqlCase.Body = `{"input": "' OR 1=1 --", "username": "admin' --"}`
	cases = append(cases, sqlCase)

	// Case 4: Null values
	nullCase := base
	nullCase.ID = base.ID + "_nulls"
	nullCase.Name = base.Name + " · Edge: Null Attributes"
	nullCase.Body = `{"name": null, "email": null, "id": null}`
	cases = append(cases, nullCase)

	return cases
}

// RunLoadTest executes a concurrent load test using native goroutines or k6 if available.
func (tc *TestCoordinator) RunLoadTest(ctx context.Context, spec LoadTestSpec) (*LoadTestResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if spec.Engine != "" && spec.Engine != "native" && spec.Engine != "auto" && spec.Engine != "k6" {
		return nil, errors.New("unknown load engine")
	}
	parsed, err := url.Parse(spec.TargetURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return nil, errors.New("load target must be an HTTP(S) URL")
	}
	if spec.DurationSeconds > 300 {
		return nil, errors.New("local load tests are limited to 300 seconds")
	}
	if spec.VirtualUsers <= 0 {
		spec.VirtualUsers = 5
	}
	if spec.DurationSeconds <= 0 {
		spec.DurationSeconds = 5
	}
	if spec.VirtualUsers > 100 {
		spec.VirtualUsers = 100 // Safe local concurrency limit
	}
	if spec.Method == "" {
		spec.Method = "GET"
	}

	// Safety check for mutation methods (Finding 10)
	upperMethod := strings.ToUpper(spec.Method)
	if upperMethod != "GET" && upperMethod != "HEAD" && upperMethod != "OPTIONS" {
		if !spec.AllowMutation {
			return nil, errors.New("mutating load tests (POST, PUT, PATCH, DELETE) require explicit confirmation (allowMutation: true) to prevent unintended side effects or database bloat")
		}
	}

	// Explicit or auto engine selection
	if strings.ToLower(spec.Engine) == "k6" {
		k6Path, err := dependencies.NewManager("", nil).Resolve("k6")
		if err != nil || k6Path == "" {
			return nil, errors.New("k6 engine requested but k6 executable was not found in PATH")
		}
		return runK6(ctx, k6Path, spec)
	}

	if strings.ToLower(spec.Engine) == "auto" {
		if k6Path, err := dependencies.NewManager("", nil).Resolve("k6"); err == nil && k6Path != "" {
			res, k6Err := runK6(ctx, k6Path, spec)
			if k6Err != nil {
				return nil, fmt.Errorf("k6 execution failed: %w (retry with explicit native engine)", k6Err)
			}
			return res, nil
		}
	}

	// Built-in concurrent worker load engine
	return runNativeLoadTest(ctx, spec)
}

func runNativeLoadTest(ctx context.Context, spec LoadTestSpec) (*LoadTestResult, error) {
	var totalReqs int64
	var successCount int64
	var failCount int64

	statusDist := make(map[int]int)
	var distMu sync.Mutex

	var latencies []float64
	var latMu sync.Mutex

	var errorSamples []string
	var errMu sync.Mutex

	started := time.Now()
	deadline := started.Add(time.Duration(spec.DurationSeconds) * time.Second)
	runCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second, Transport: LocalTransport()}
	defer client.CloseIdleConnections()

	var wg sync.WaitGroup
	for i := 0; i < spec.VirtualUsers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}

				var body io.Reader
				if spec.Body != "" {
					body = strings.NewReader(spec.Body)
				}
				req, err := http.NewRequestWithContext(runCtx, spec.Method, spec.TargetURL, body)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					atomic.AddInt64(&totalReqs, 1)
					return
				}
				for k, v := range spec.Headers {
					if strings.EqualFold(k, "Host") {
						req.Host = v
					} else {
						req.Header.Set(k, v)
					}
				}

				start := time.Now()
				resp, err := client.Do(req)
				if err == nil {
					_, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, 16*1024*1024+1))
					_ = resp.Body.Close()
					if readErr != nil {
						err = readErr
					}
				}
				dur := float64(time.Since(start).Microseconds()) / 1000.0

				atomic.AddInt64(&totalReqs, 1)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					errMu.Lock()
					if len(errorSamples) < 5 {
						errorSamples = append(errorSamples, err.Error())
					}
					errMu.Unlock()
					continue
				}

				_ = resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 400 {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}

				distMu.Lock()
				statusDist[resp.StatusCode]++
				distMu.Unlock()

				latMu.Lock()
				latencies = append(latencies, dur)
				latMu.Unlock()

				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("load test cancelled: %w", ctx.Err())
	}

	total := int(atomic.LoadInt64(&totalReqs))
	succ := int(atomic.LoadInt64(&successCount))
	fail := int(atomic.LoadInt64(&failCount))

	if total == 0 {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("load test cancelled: %w", ctx.Err())
		}
		return &LoadTestResult{
			Engine:             "native",
			StatusDistribution: statusDist,
			CompletedAt:        time.Now().UTC().Format(time.RFC3339),
		}, nil
	}

	sort.Float64s(latencies)
	minLat := 0.0
	maxLat := 0.0
	avgLat := 0.0
	p50 := 0.0
	p90 := 0.0
	p99 := 0.0

	if len(latencies) > 0 {
		minLat = latencies[0]
		maxLat = latencies[len(latencies)-1]
		sum := 0.0
		for _, l := range latencies {
			sum += l
		}
		avgLat = sum / float64(len(latencies))
		p50 = latencies[len(latencies)*50/100]
		p90 = latencies[len(latencies)*90/100]
		p99 = latencies[len(latencies)*99/100]
	}

	rps := float64(total) / time.Since(started).Seconds()

	return &LoadTestResult{
		Engine:             "native",
		TotalRequests:      total,
		Successful:         succ,
		Failed:             fail,
		RequestsPerSecond:  rps,
		MinLatencyMs:       minLat,
		MaxLatencyMs:       maxLat,
		AvgLatencyMs:       avgLat,
		P50LatencyMs:       p50,
		P90LatencyMs:       p90,
		P99LatencyMs:       p99,
		StatusDistribution: statusDist,
		ErrorSample:        errorSamples,
		CompletedAt:        time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func runK6(ctx context.Context, k6Path string, spec LoadTestSpec) (*LoadTestResult, error) {
	hosts := map[string]string{}
	if parsed, err := url.Parse(spec.TargetURL); err == nil {
		host := strings.ToLower(parsed.Hostname())
		if strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".localhost") {
			hosts[host] = "127.0.0.1"
		}
	}
	hostsJSON, _ := json.Marshal(hosts)
	headersJSON, _ := json.Marshal(spec.Headers)
	bodyJSON, _ := json.Marshal(spec.Body)
	urlJSON, _ := json.Marshal(spec.TargetURL)
	methodJSON, _ := json.Marshal(strings.ToUpper(spec.Method))

	script := fmt.Sprintf(`
import http from 'k6/http';
export const options = {
  vus: %d,
  duration: '%ds',
  hosts: %s,
  summaryTrendStats: ['avg','min','med','max','p(90)','p(95)','p(99)'],
};
const headers = %s;
const payload = %s;
export default function () {
  const params = { headers: headers };
  http.request(%s, %s, payload, params);
}
`, spec.VirtualUsers, spec.DurationSeconds, string(hostsJSON), string(headersJSON), string(bodyJSON), string(methodJSON), string(urlJSON))

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("k6-script-%d.js", time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, []byte(script), 0600); err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile)

	summaryFile := tmpFile + ".json"
	defer os.Remove(summaryFile)
	res, err := runner.CaptureExecContext(ctx, runner.Spec{Executable: k6Path, Args: []string{"run", "--no-usage-report", "--summary-export", summaryFile, tmpFile}}, time.Duration(spec.DurationSeconds+30)*time.Second)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("k6 exited with code %d: %s", res.ExitCode, res.Combined)
	}

	data, err := os.ReadFile(summaryFile)
	if err != nil {
		return nil, err
	}

	var k6Summary struct {
		Metrics struct {
			Iterations struct {
				Count int `json:"count"`
			} `json:"iterations"`
			HTTPReqs struct {
				Count int `json:"count"`
			} `json:"http_reqs"`
			HTTPReqFailed struct {
				Passes int     `json:"passes"` // in k6, "passes" on http_req_failed means failed requests
				Fails  int     `json:"fails"`  // successful requests
				Value  float64 `json:"value"`
			} `json:"http_req_failed"`
			HTTPReqDuration struct {
				Avg float64 `json:"avg"`
				Min float64 `json:"min"`
				Med float64 `json:"med"`
				Max float64 `json:"max"`
				P90 float64 `json:"p(90)"`
				P95 float64 `json:"p(95)"`
				P99 float64 `json:"p(99)"`
			} `json:"http_req_duration"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(data, &k6Summary); err != nil {
		return nil, err
	}

	total := k6Summary.Metrics.HTTPReqs.Count
	if total == 0 {
		total = k6Summary.Metrics.Iterations.Count
	}
	failed := k6Summary.Metrics.HTTPReqFailed.Passes
	succ := k6Summary.Metrics.HTTPReqFailed.Fails
	if succ == 0 && failed == 0 {
		succ = total
	}

	minLat := k6Summary.Metrics.HTTPReqDuration.Min
	maxLat := k6Summary.Metrics.HTTPReqDuration.Max
	avgLat := k6Summary.Metrics.HTTPReqDuration.Avg
	p50 := k6Summary.Metrics.HTTPReqDuration.Med
	p90 := k6Summary.Metrics.HTTPReqDuration.P90
	p99 := k6Summary.Metrics.HTTPReqDuration.P99
	rps := float64(total) / float64(spec.DurationSeconds)

	return &LoadTestResult{
		Engine:            "k6",
		TotalRequests:     total,
		Successful:        succ,
		Failed:            failed,
		RequestsPerSecond: rps,
		MinLatencyMs:      minLat,
		MaxLatencyMs:      maxLat,
		AvgLatencyMs:      avgLat,
		P50LatencyMs:      p50,
		P90LatencyMs:      p90,
		P99LatencyMs:      p99,
		CompletedAt:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// RunProjectTests executes php artisan test or phpunit in the project.
func (tc *TestCoordinator) RunProjectTests(phpBinary, projectPath, filter string) (*ProjectTestResult, error) {
	return tc.RunProjectTestsContext(context.Background(), phpBinary, projectPath, filter)
}

func (tc *TestCoordinator) RunProjectTestsContext(ctx context.Context, phpBinary, projectPath, filter string) (*ProjectTestResult, error) {
	args := []string{"artisan", "test", "--without-tty"}
	if filter != "" {
		args = append(args, "--filter="+filter)
	}

	start := time.Now()
	resExec, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       args,
		Directory:  projectPath,
	}, 120*time.Second)

	dur := time.Since(start).Seconds()
	if err != nil {
		return nil, err
	}

	outputStr := ""
	if resExec != nil {
		outputStr = resExec.Stdout
		if outputStr == "" {
			outputStr = resExec.Combined
		}
	}
	lines := strings.Split(outputStr, "\n")

	res := &ProjectTestResult{
		Suite:    "PHPUnit / Pest",
		Duration: dur,
		Output:   lines,
		Success:  resExec != nil && resExec.ExitCode == 0,
	}

	// Regex for test summary line
	summaryRegex := regexp.MustCompile(`(?i)(\d+)\s+passed`)
	if m := summaryRegex.FindStringSubmatch(outputStr); len(m) > 1 {
		_, _ = fmt.Sscanf(m[1], "%d", &res.Passed)
	}

	failedRegex := regexp.MustCompile(`(?i)(\d+)\s+failed`)
	if m := failedRegex.FindStringSubmatch(outputStr); len(m) > 1 {
		_, _ = fmt.Sscanf(m[1], "%d", &res.Failed)
	}

	skippedRegex := regexp.MustCompile(`(?i)(\d+)\s+skipped`)
	if m := skippedRegex.FindStringSubmatch(outputStr); len(m) > 1 {
		_, _ = fmt.Sscanf(m[1], "%d", &res.Skipped)
	}

	res.Total = res.Passed + res.Failed + res.Skipped
	if res.Failed > 0 || (resExec != nil && resExec.ExitCode != 0) {
		res.Success = false

	}

	return res, nil
}

// Herd *.test hosts resolve locally without changing Host or TLS verification.
func LocalTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err == nil && (strings.HasSuffix(strings.ToLower(host), ".test") || strings.HasSuffix(strings.ToLower(host), ".localhost")) {
			address = net.JoinHostPort("127.0.0.1", port)
		}
		return dialer.DialContext(ctx, network, address)
	}
	proxy := transport.Proxy
	transport.Proxy = func(r *http.Request) (*url.URL, error) {
		if strings.HasSuffix(strings.ToLower(r.URL.Hostname()), ".test") || strings.HasSuffix(strings.ToLower(r.URL.Hostname()), ".localhost") {
			return nil, nil
		}
		return proxy(r)
	}
	return transport
}
