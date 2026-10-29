package performance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type CheckupItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Category  string `json:"category"` // "opcache", "cache", "composer", "env"
	Status    string `json:"status"`   // "ok", "warning", "critical"
	Message   string `json:"message"`
	FixAction string `json:"fixAction,omitempty"`
}

type CheckupReport struct {
	Items     []CheckupItem `json:"items"`
	Score     int           `json:"score"`
	ScannedAt string        `json:"scannedAt"`
}

// RunProjectCheckup inspects the project environment for performance bottlenecks
func RunProjectCheckup(ctx context.Context, projectPath, phpBinary string) CheckupReport {
	if phpBinary == "" {
		phpBinary = "php"
	}

	items := make([]CheckupItem, 0)
	score := 100

	// 1. Config cache check
	cfgCache := filepath.Join(projectPath, "bootstrap", "cache", "config.php")
	if _, err := os.Stat(cfgCache); err == nil {
		items = append(items, CheckupItem{
			ID:       "config_cache",
			Title:    "Configuration Cache",
			Category: "cache",
			Status:   "ok",
			Message:  "Config cache is warmed and active",
		})
	} else {
		score -= 10
		items = append(items, CheckupItem{
			ID:        "config_cache",
			Title:     "Configuration Cache",
			Category:  "cache",
			Status:    "warning",
			Message:   "Config files are parsed on every request. Warm config cache to accelerate boot times.",
			FixAction: "artisan config:cache",
		})
	}

	// 2. Route cache check
	routeCache := filepath.Join(projectPath, "bootstrap", "cache", "routes-v7.php")
	if _, err := os.Stat(routeCache); err == nil {
		items = append(items, CheckupItem{
			ID:       "route_cache",
			Title:    "Route Cache",
			Category: "cache",
			Status:   "ok",
			Message:  "Routes are compiled into cache file",
		})
	} else {
		score -= 10
		items = append(items, CheckupItem{
			ID:        "route_cache",
			Title:     "Route Cache",
			Category:  "cache",
			Status:    "warning",
			Message:   "Route definitions are evaluated dynamically. Warm route cache for high traffic.",
			FixAction: "artisan route:cache",
		})
	}

	// 3. View cache check
	viewDir := filepath.Join(projectPath, "storage", "framework", "views")
	if entries, err := os.ReadDir(viewDir); err == nil && len(entries) > 2 {
		items = append(items, CheckupItem{
			ID:       "view_cache",
			Title:    "Blade View Cache",
			Category: "cache",
			Status:   "ok",
			Message:  fmt.Sprintf("%d compiled Blade templates present in storage", len(entries)),
		})
	} else {
		score -= 5
		items = append(items, CheckupItem{
			ID:        "view_cache",
			Title:     "Blade View Cache",
			Category:  "cache",
			Status:    "warning",
			Message:   "Blade views are compiled just-in-time on request. Pre-render views for faster initial response.",
			FixAction: "artisan view:cache",
		})
	}

	// 4. Composer Classmap Optimization
	classmapFile := filepath.Join(projectPath, "vendor", "composer", "autoload_classmap.php")
	if data, err := os.ReadFile(classmapFile); err == nil && len(data) > 500 {
		items = append(items, CheckupItem{
			ID:       "composer_classmap",
			Title:    "Composer Autoloader Classmap",
			Category: "composer",
			Status:   "ok",
			Message:  "Optimized classmap is generated with rapid lookup",
		})
	} else {
		score -= 10
		items = append(items, CheckupItem{
			ID:        "composer_classmap",
			Title:     "Composer Autoloader Classmap",
			Category:  "composer",
			Status:    "warning",
			Message:   "Composer is using PSR-4 filesystem discovery. Optimize classmap with -o flag.",
			FixAction: "composer dump-autoload -o",
		})
	}

	// 5. OPcache Probe
	cmd := exec.CommandContext(ctx, phpBinary, "-r", "echo function_exists('opcache_get_status') && is_array(opcache_get_status(false)) ? 'OPCACHE_ENABLED' : 'OPCACHE_DISABLED';")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if out, err := cmd.CombinedOutput(); err == nil {
		if strings.Contains(string(out), "OPCACHE_ENABLED") {
			items = append(items, CheckupItem{
				ID:       "opcache",
				Title:    "Zend OPcache",
				Category: "opcache",
				Status:   "ok",
				Message:  "OPcache opcode caching engine is active",
			})
		} else {
			score -= 15
			items = append(items, CheckupItem{
				ID:        "opcache",
				Title:     "Zend OPcache",
				Category:  "opcache",
				Status:    "warning",
				Message:   "OPcache is disabled. Enable zend_extension=opcache and opcache.enable=1 in php.ini",
				FixAction: "enable_opcache",
			})
		}
	}

	// 6. Event cache check
	eventCache := filepath.Join(projectPath, "bootstrap", "cache", "events.php")
	if _, err := os.Stat(eventCache); err == nil {
		items = append(items, CheckupItem{
			ID:       "event_cache",
			Title:    "Event Discovery Cache",
			Category: "cache",
			Status:   "ok",
			Message:  "Event listener mappings are cached",
		})
	} else {
		score -= 5
		items = append(items, CheckupItem{
			ID:        "event_cache",
			Title:     "Event Discovery Cache",
			Category:  "cache",
			Status:    "warning",
			Message:   "Application scans listeners dynamically on every boot. Cache events for production throughput.",
			FixAction: "artisan event:cache",
		})
	}

	// 7. Debugger & Profiler Overhead Check (.env inspection)
	envFile := filepath.Join(projectPath, ".env")
	if envBytes, err := os.ReadFile(envFile); err == nil {
		envStr := string(envBytes)
		if strings.Contains(envStr, "DEBUGBAR_ENABLED=true") || strings.Contains(envStr, "TELESCOPE_ENABLED=true") {
			score -= 10
			items = append(items, CheckupItem{
				ID:        "profiler_overhead",
				Title:     "Profiler Overhead (Telescope / Debugbar)",
				Category:  "env",
				Status:    "warning",
				Message:   "Active debuggers intercept all queries and events, adding 20-40% latency overhead during benchmarks.",
				FixAction: "disable_debuggers",
			})
		}

		if strings.Contains(envStr, "QUEUE_CONNECTION=sync") {
			score -= 5
			items = append(items, CheckupItem{
				ID:       "queue_driver",
				Title:    "Queue Driver Configuration",
				Category: "env",
				Status:   "warning",
				Message:  "QUEUE_CONNECTION=sync blocks user requests while running jobs. Use redis or database for background jobs.",
			})
		}
	}

	if score < 0 {
		score = 0
	}

	return CheckupReport{
		Items:     items,
		Score:     score,
		ScannedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

// ExecuteCheckupFix runs the 1-click remediation command for an optimization check
func ExecuteCheckupFix(ctx context.Context, projectPath, phpBinary, fixID string) error {
	if phpBinary == "" {
		phpBinary = "php"
	}

	var args []string
	switch fixID {
	case "config_cache":
		args = []string{"artisan", "config:cache"}
	case "route_cache":
		args = []string{"artisan", "route:cache"}
	case "view_cache":
		args = []string{"artisan", "view:cache"}
	case "event_cache":
		args = []string{"artisan", "event:cache"}
	case "composer_classmap":
		args = []string{"composer", "dump-autoload", "-o"}
	case "clear_all_cache":
		args = []string{"artisan", "optimize:clear"}
	default:
		return fmt.Errorf("unknown fix action: %s", fixID)
	}

	cmd := exec.CommandContext(ctx, phpBinary, args...)
	if args[0] == "composer" {
		cmd = exec.CommandContext(ctx, "composer", args[1:]...)
	}
	cmd.Dir = projectPath
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fix %s failed: %w (output: %s)", fixID, err, string(out))
	}
	return nil
}
