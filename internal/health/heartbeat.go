package health

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"herdworkbench/internal/runner"
)

type ComponentHealth struct {
	Component string  `json:"component"`
	Status    string  `json:"status"` // "healthy", "degraded", "failing"
	LatencyMs float64 `json:"latencyMs,omitempty"`
	Message   string  `json:"message"`
}

type HeartbeatReport struct {
	ProjectID   string            `json:"projectId"`
	Timestamp   time.Time         `json:"timestamp"`
	HealthScore int               `json:"healthScore"`
	Overall     string            `json:"overall"`
	Components  []ComponentHealth `json:"components"`
}

func ProbeHeartbeat(ctx context.Context, phpBinary, projectPath string) HeartbeatReport {
	components := make([]ComponentHealth, 0)
	score := 100

	// 1. Storage & Cache Write Permissions
	storageDir := filepath.Join(projectPath, "storage")
	cacheDir := filepath.Join(projectPath, "bootstrap", "cache")

	storageOk := isWritable(storageDir)
	cacheOk := isWritable(cacheDir)

	if storageOk && cacheOk {
		components = append(components, ComponentHealth{
			Component: "storage",
			Status:    "healthy",
			Message:   "Storage directory and bootstrap cache are fully writable.",
		})
	} else {
		score -= 25
		components = append(components, ComponentHealth{
			Component: "storage",
			Status:    "failing",
			Message:   "Storage or cache permissions failure. Run chmod -R 775 storage.",
		})
	}

	// 2. Database & Framework Kernel Boot
	if phpBinary != "" && projectPath != "" {
		start := time.Now()
		probePHP := `
try {
    $autoload = './vendor/autoload.php';
    $bootstrap = './bootstrap/app.php';
    if (!file_exists($autoload) || !file_exists($bootstrap)) {
        echo json_encode(['error' => 'not_laravel']);
        exit;
    }
    require_once $autoload;
    $app = require_once $bootstrap;
    if (is_object($app) && method_exists($app, 'make')) {
        $kernel = $app->make(\Illuminate\Contracts\Console\Kernel::class);
        $kernel->bootstrap();
        $dbStatus = 'connected';
        try {
            \Illuminate\Support\Facades\DB::connection()->getPdo();
        } catch (\Throwable $t) {
            $dbStatus = 'error: ' . $t->getMessage();
        }

        $queueStatus = 'unconfigured';
        try {
            if (class_exists(\Illuminate\Support\Facades\Queue::class)) {
                $qConn = \Illuminate\Support\Facades\Queue::connection();
                $size = $qConn->size();
                $queueStatus = 'connected (pending: ' . $size . ')';
            }
        } catch (\Throwable $t) {
            $queueStatus = 'error: ' . $t->getMessage();
        }

        echo json_encode(['db' => $dbStatus, 'queue' => $queueStatus]);
        exit;
    }
} catch (\Throwable $e) {
    echo json_encode(['error' => $e->getMessage()]);
}
`
		res, err := runner.CaptureExecContext(ctx, runner.Spec{
			Executable: phpBinary,
			Args:       []string{"-r", probePHP},
			Directory:  projectPath,
		}, 10*time.Second)

		dur := float64(time.Since(start).Milliseconds())

		if err != nil {
			score -= 35
			components = append(components, ComponentHealth{
				Component: "database",
				Status:    "failing",
				LatencyMs: dur,
				Message:   fmt.Sprintf("Failed to boot kernel: %v", err),
			})
			components = append(components, ComponentHealth{
				Component: "queue",
				Status:    "unknown",
				Message:   "Kernel boot failed; queue unprobed.",
			})
		} else {
			var parsed struct {
				DB    string `json:"db"`
				Queue string `json:"queue"`
				Error string `json:"error"`
			}
			_ = json.Unmarshal([]byte(strings.TrimSpace(res.Stdout)), &parsed)

			// Database assessment
			if parsed.DB == "connected" {
				status := "healthy"
				if dur > 500 {
					status = "degraded"
					score -= 10
				}
				components = append(components, ComponentHealth{
					Component: "database",
					Status:    status,
					LatencyMs: dur,
					Message:   fmt.Sprintf("Database PDO connected in %.1fms.", dur),
				})
			} else if strings.HasPrefix(parsed.DB, "error:") {
				score -= 25
				components = append(components, ComponentHealth{
					Component: "database",
					Status:    "failing",
					LatencyMs: dur,
					Message:   fmt.Sprintf("Database connection failed: %s", strings.TrimPrefix(parsed.DB, "error: ")),
				})
			} else {
				score -= 20
				components = append(components, ComponentHealth{
					Component: "database",
					Status:    "degraded",
					LatencyMs: dur,
					Message:   fmt.Sprintf("Database check: %s", strings.TrimSpace(res.Stdout)),
				})
			}

			// Queue assessment
			if strings.HasPrefix(parsed.Queue, "connected") {
				components = append(components, ComponentHealth{
					Component: "queue",
					Status:    "healthy",
					Message:   fmt.Sprintf("Queue connection responsive (%s).", parsed.Queue),
				})
			} else if strings.HasPrefix(parsed.Queue, "error:") {
				score -= 15
				components = append(components, ComponentHealth{
					Component: "queue",
					Status:    "failing",
					Message:   fmt.Sprintf("Queue probe error: %s", strings.TrimPrefix(parsed.Queue, "error: ")),
				})
			} else if parsed.Queue == "unconfigured" {
				score -= 5
				components = append(components, ComponentHealth{
					Component: "queue",
					Status:    "degraded",
					Message:   "Queue facade unconfigured or default connection unavailable.",
				})
			} else {
				score -= 10
				components = append(components, ComponentHealth{
					Component: "queue",
					Status:    "unknown",
					Message:   "Queue status unverified.",
				})
			}
		}
	} else {
		components = append(components, ComponentHealth{
			Component: "database",
			Status:    "unknown",
			Message:   "PHP binary or project path not provided",
		})
		components = append(components, ComponentHealth{
			Component: "queue",
			Status:    "unknown",
			Message:   "PHP binary or project path not provided",
		})
	}

	if score < 0 {
		score = 0
	}

	overall := "healthy"
	if score < 60 {
		overall = "failing"
	} else if score < 85 {
		overall = "degraded"
	}

	return HeartbeatReport{
		Timestamp:   time.Now(),
		HealthScore: score,
		Overall:     overall,
		Components:  components,
	}
}

func isWritable(dir string) bool {
	if _, err := os.Stat(dir); err != nil {
		return false
	}
	testFile := filepath.Join(dir, fmt.Sprintf(".perm_test_%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte("ok"), 0644); err != nil {
		return false
	}
	_ = os.Remove(testFile)
	return true
}
