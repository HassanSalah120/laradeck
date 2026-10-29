package workbench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadLaravelLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logDir := filepath.Join(tmpDir, "storage", "logs")
	_ = os.MkdirAll(logDir, 0755)

	sampleLog := `[2026-10-16 01:23:45] local.ERROR: Call to a member function get() on null at C:\projects\app\Services\UserService.php:42
[stacktrace]
#0 C:\projects\app\Http\Controllers\UserController.php(25): App\Services\UserService->get()
#1 {main}
[2026-10-16 01:24:10] local.INFO: User logged in {"id":1}
`
	logFile := filepath.Join(logDir, "laravel.log")
	if err := os.WriteFile(logFile, []byte(sampleLog), 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	entries, err := ReadLaravelLogs(tmpDir, 0)
	if err != nil {
		t.Fatalf("ReadLaravelLogs failed: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}

	// Reversed order: INFO first, then ERROR
	if entries[0].Level != "INFO" {
		t.Errorf("expected latest entry to be INFO, got %s", entries[0].Level)
	}
	if entries[1].Level != "ERROR" {
		t.Errorf("expected earlier entry to be ERROR, got %s", entries[1].Level)
	}
	if entries[1].Line != 42 || !stringsContains(entries[1].File, "UserService.php") {
		t.Errorf("expected file UserService.php:42, got %s:%d", entries[1].File, entries[1].Line)
	}
}

func stringsContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSameSecondErrorsHaveDistinctStableIDs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "storage", "logs")
	os.MkdirAll(dir, 0755)
	line := "[2026-10-16 01:23:45] local.ERROR: Repeated failure\r\n"
	os.WriteFile(filepath.Join(dir, "laravel.log"), []byte(line+line), 0600)
	entries, err := ReadLaravelLogs(root, 0)
	if err != nil || len(entries) != 2 {
		t.Fatal(err)
	}
	if entries[0].ID == entries[1].ID {
		t.Fatal("two distinct errors collapsed into one")
	}
	repeated, err := ReadLaravelLogs(root, 0)
	if err != nil || repeated[0].ID != entries[0].ID {
		t.Fatal("IDs changed on reread")
	}
}
