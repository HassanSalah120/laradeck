package workbench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"herdworkbench/internal/storage"
)

func TestSaveEnvPathConfinementAndTrust(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "my-project")
	_ = os.MkdirAll(projDir, 0755)
	envPath := filepath.Join(projDir, ".env")
	_ = os.WriteFile(envPath, []byte("APP_NAME=Laravel\n"), 0644)

	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	b.projects = []Project{
		{ID: "proj_trusted", Name: "My Project", Path: projDir, URL: "http://my-project.test", PHPBinary: "php.exe"},
	}
	b.trusted["proj_trusted"] = true

	envFile, err := b.GetEnv("proj_trusted")
	if err != nil {
		t.Fatalf("GetEnv failed: %v", err)
	}

	// 1. Attempt path substitution outside project directory (Finding 2)
	envFile.Path = "C:\\Windows\\System32\\calc.exe"
	err = b.SaveEnv("proj_trusted", envFile, envFile.Fingerprint)
	if err == nil || !strings.Contains(err.Error(), "caller path substitution rejected") {
		t.Errorf("expected caller path substitution to be rejected, got: %v", err)
	}

	// 2. Untrusted project cannot save .env
	b.trusted["proj_trusted"] = false
	envFile.Path = envPath
	err = b.SaveEnv("proj_trusted", envFile, envFile.Fingerprint)
	if err == nil {
		t.Errorf("expected error when saving .env on untrusted project, got nil")
	}
}

func TestIniChangeAllowedDirectivesAndTrust(t *testing.T) {
	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	b.projects = []Project{
		{ID: "proj_trusted", Name: "My Project", Path: "C:\\nonexistent", PHPBinary: "php.exe"},
	}
	b.trusted["proj_trusted"] = true

	// 1. Disallowed arbitrary directive must be rejected (Finding 2)
	err := b.ApplyIniChange("proj_trusted", "allow_url_include", "On")
	if err == nil || !strings.Contains(err.Error(), "not in the approved configurable directives") {
		t.Errorf("expected disallowed directive to be rejected, got: %v", err)
	}

	_, err = b.PreviewIniChange("proj_trusted", "disable_functions", "")
	if err == nil || !strings.Contains(err.Error(), "not in the approved configurable directives") {
		t.Errorf("expected disallowed preview directive to be rejected, got: %v", err)
	}

	// 2. Untrusted project must be rejected
	b.trusted["proj_trusted"] = false
	err = b.ApplyIniChange("proj_trusted", "memory_limit", "512M")
	if err == nil {
		t.Errorf("expected error when modifying ini on untrusted project, got nil")
	}
}

func TestActiveAccountingAndShutdownCoordination(t *testing.T) {
	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)

	// Initially 0 active
	if b.Active() != 0 {
		t.Errorf("expected initial Active=0, got %d", b.Active())
	}

	// Test inFlightOps via beginOperation
	ctx, done, err := b.beginOperation()
	if err != nil {
		t.Fatalf("beginOperation failed: %v", err)
	}
	if b.Active() != 1 {
		t.Errorf("expected Active=1 during in-flight operation, got %d", b.Active())
	}
	done()
	if b.Active() != 0 {
		t.Errorf("expected Active=0 after operation done, got %d", b.Active())
	}

	// Test StopAll sets isShuttingDown and cancels context
	_ = b.StopAll(false)
	if !b.isShuttingDown {
		t.Errorf("expected isShuttingDown=true after StopAll")
	}
	select {
	case <-ctx.Done():
		// expected: context canceled
	default:
		t.Errorf("expected backend context to be canceled on StopAll")
	}

	// All start paths must be rejected during shutdown (Finding 3 & Finding 5)
	_, err = b.Start("any", "serve")
	if err == nil || !strings.Contains(err.Error(), "Shutdown is in progress") {
		t.Errorf("expected Start to be rejected during shutdown, got: %v", err)
	}
	err = b.StartService("redis", "")
	if err == nil || !strings.Contains(err.Error(), "shutting down") {
		t.Errorf("expected StartService to be rejected during shutdown, got: %v", err)
	}
	_, err = b.StartQueueWorker("proj_trusted", "default")
	if err == nil || !strings.Contains(err.Error(), "shutting down") {
		t.Errorf("expected StartQueueWorker to be rejected during shutdown, got: %v", err)
	}
	_, err = b.StartTunnel("cloudflared", "http://localhost:8000")
	if err == nil || !strings.Contains(err.Error(), "shutting down") {
		t.Errorf("expected StartTunnel to be rejected during shutdown, got: %v", err)
	}
	_, _, err = b.beginOperation()
	if err == nil || !strings.Contains(err.Error(), "shutting down") {
		t.Errorf("expected beginOperation to be rejected during shutdown, got: %v", err)
	}

	// CancelShutdown restores ability to start
	b.CancelShutdown()
	if b.isShuttingDown {
		t.Errorf("expected isShuttingDown=false after CancelShutdown")
	}
	_, done2, err := b.beginOperation()
	if err != nil {
		t.Fatalf("expected beginOperation to succeed after CancelShutdown, got: %v", err)
	}
	done2()
}

func TestSaveREADMEBackupAndAtomicReplace(t *testing.T) {
	tmpDir := t.TempDir()
	projDir := filepath.Join(tmpDir, "readme-proj")
	_ = os.MkdirAll(projDir, 0755)

	initialReadme := "# Original README\nLine 1\n"
	readmePath := filepath.Join(projDir, "README.md")
	if err := os.WriteFile(readmePath, []byte(initialReadme), 0644); err != nil {
		t.Fatalf("failed writing initial README: %v", err)
	}

	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	b.projects = []Project{
		{ID: "proj_readme", Name: "Readme Proj", Path: projDir, PHPBinary: "php.exe"},
	}
	b.trusted["proj_readme"] = true

	if err := b.SaveREADME("proj_readme", "unpreviewed"); err == nil {
		t.Fatal("save without preview accepted")
	}
	if _, err := b.MergeREADME("proj_readme", "# generated", nil); err != nil {
		t.Fatal(err)
	}
	newContent := "# Updated README\nGenerated content\n"
	if err := b.SaveREADME("proj_readme", newContent); err != nil {
		t.Fatalf("SaveREADME failed: %v", err)
	}

	// Verify README was updated
	updatedBytes, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("failed reading updated README: %v", err)
	}
	if string(updatedBytes) != newContent {
		t.Errorf("expected %q, got %q", newContent, string(updatedBytes))
	}

	// Verify backup file was created
	files, err := os.ReadDir(projDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	foundBackup := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "README.md.backup.") {
			foundBackup = true
			backupBytes, _ := os.ReadFile(filepath.Join(projDir, f.Name()))
			if string(backupBytes) != initialReadme {
				t.Errorf("backup content mismatch: expected %q, got %q", initialReadme, string(backupBytes))
			}
			break
		}
	}
	if !foundBackup {
		t.Error("expected README backup")
	}
	if err := os.WriteFile(readmePath, []byte("external edit"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := b.SaveREADME("proj_readme", "overwrite"); err == nil {
		t.Fatal("external README edit overwritten")
	}
}

func TestGetRawLogsIdempotentIndexing(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "storage", "logs")
	_ = os.MkdirAll(logDir, 0755)

	logContent := "[2026-10-16 01:00:00] local.ERROR: Database connection timed out at App\\Models\\User.php:45\n"
	_ = os.WriteFile(filepath.Join(logDir, "laravel.log"), []byte(logContent), 0644)

	dbPath := filepath.Join(tmpDir, "diag.db")
	db, err := storage.OpenDiagnosticsDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDiagnosticsDB failed: %v", err)
	}
	defer db.Close()

	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	_ = b.diagDB.Close()
	b.diagDB = db
	b.projects = []Project{
		{ID: "proj_log_test", Name: "Log Test", Path: tmpDir, PHPBinary: "php.exe"},
	}

	// Read logs 5 times consecutively
	for i := 0; i < 5; i++ {
		entries, err := b.GetRawLogs("proj_log_test")
		if err != nil {
			t.Fatalf("GetRawLogs failed: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(entries))
		}
	}

	// Occurrences in database must remain 1 (Finding 16)
	errorsList, err := b.GetErrors("proj_log_test", "", 10)
	if err != nil || len(errorsList) != 1 {
		t.Fatalf("GetErrors failed: %v, count: %d", err, len(errorsList))
	}
	if errorsList[0].Occurrences != 1 {
		t.Errorf("expected occurrences to stay 1 across repeated log reads, but got %d", errorsList[0].Occurrences)
	}
}

func TestClockworkAutoWatchOnSelectAndTrust(t *testing.T) {
	tmpDir := t.TempDir()
	clockDir := filepath.Join(tmpDir, "storage", "clockwork")
	_ = os.MkdirAll(clockDir, 0755)

	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)

	b.projects = []Project{
		{ID: "proj_1", Name: "Proj 1", Path: tmpDir, URL: "http://proj1.test", Clockwork: "v5.3.5"},
		{ID: "proj_2", Name: "Proj 2", Path: tmpDir, URL: "http://proj2.test", Clockwork: "v5.3.5"},
	}

	// 1. Select untrusted project -> Watch should NOT be active
	if err := b.Select("proj_1"); err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if b.clock.State().Watching {
		t.Errorf("expected watching=false for untrusted project, got true")
	}

	// 2. Trust the currently selected project -> Watch should become active
	if err := b.Trust("proj_1"); err != nil {
		t.Fatalf("Trust failed: %v", err)
	}
	if !b.clock.State().Watching {
		t.Errorf("expected watching=true after trusting selected project, got false")
	}

	// 3. Mark proj_2 as trusted beforehand, then Select proj_2 -> Watch should become active
	b.trusted["proj_2"] = true
	if err := b.Select("proj_2"); err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if !b.clock.State().Watching {
		t.Errorf("expected watching=true when selecting pre-trusted project, got false")
	}

	// 4. StopAll should stop watch
	_ = b.StopAll(false)
	if b.clock.State().Watching {
		t.Errorf("expected watching=false after StopAll, got true")
	}
}
