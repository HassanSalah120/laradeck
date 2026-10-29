package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseLaravelLog(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pail-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sampleLog := `[2026-10-19 12:00:00] local.INFO: Application booted successfully.
[2026-10-19 12:01:10] local.ERROR: Illuminate\Database\QueryException: SQLSTATE[42S02]: Table 'users' not found (SQL: select * from users where id = 42)
#0 /var/www/vendor/laravel/framework/src/Illuminate/Database/Connection.php(712): Illuminate\Database\Connection->runQueryCallback()
#1 /var/www/app/Http/Controllers/UserController.php(25): Illuminate\Database\Eloquent\Builder->get()
#2 /var/www/vendor/laravel/framework/src/Illuminate/Routing/Controller.php(54): App\Http\Controllers\UserController->index()
[2026-10-19 12:02:15] local.ERROR: Illuminate\Database\QueryException: SQLSTATE[42S02]: Table 'users' not found (SQL: select * from users where id = 99)
#0 /var/www/vendor/laravel/framework/src/Illuminate/Database/Connection.php(712): Illuminate\Database\Connection->runQueryCallback()
#1 /var/www/app/Http/Controllers/UserController.php(25): Illuminate\Database\Eloquent\Builder->get()
[2026-10-19 12:03:00] local.WARNING: Rate limit approached for IP 127.0.0.1
`
	logPath := filepath.Join(tmpDir, "laravel.log")
	if err := os.WriteFile(logPath, []byte(sampleLog), 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	report, err := ParseLaravelLog(logPath, 50)
	if err != nil {
		t.Fatalf("failed to parse log: %v", err)
	}

	if report.TotalEntries != 4 {
		t.Errorf("expected 4 entries, got %d", report.TotalEntries)
	}
	if report.ErrorCount != 2 {
		t.Errorf("expected 2 errors, got %d", report.ErrorCount)
	}
	if report.WarnCount != 1 {
		t.Errorf("expected 1 warning, got %d", report.WarnCount)
	}

	// Verify deduplicated incidents
	if len(report.Incidents) != 1 {
		t.Fatalf("expected 1 deduplicated incident, got %d", len(report.Incidents))
	}

	inc := report.Incidents[0]
	if inc.Count != 2 {
		t.Errorf("expected incident count 2, got %d", inc.Count)
	}
	if inc.Exception != "Illuminate\\Database\\QueryException" {
		t.Errorf("unexpected exception class: %s", inc.Exception)
	}
	if inc.AIPrompt == "" {
		t.Error("expected AI prompt to be generated")
	}
}
