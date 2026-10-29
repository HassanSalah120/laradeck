package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCLIHandlerHelpAndVersion(t *testing.T) {
	c := NewCLIHandler()
	if code := c.Run([]string{"version"}); code != 0 {
		t.Errorf("expected exit code 0 for version, got %d", code)
	}
	if code := c.Run([]string{"help"}); code != 0 {
		t.Errorf("expected exit code 0 for help, got %d", code)
	}
	if code := c.Run([]string{}); code != 1 {
		t.Errorf("expected exit code 1 for empty args, got %d", code)
	}
}

func TestCLINonexistentAndEmptyProjectRejection(t *testing.T) {
	c := NewCLIHandler()

	// 1. Nonexistent directory
	if code := c.Run([]string{"report", "--project", "C:\\nonexistent_dir_12345"}); code != 1 {
		t.Errorf("expected report on nonexistent directory to exit with 1, got %d", code)
	}
	if code := c.Run([]string{"audit", "--project", "C:\\nonexistent_dir_12345"}); code != 1 {
		t.Errorf("expected audit on nonexistent directory to exit with 1, got %d", code)
	}

	// 2. Empty directory (no artisan or composer.json)
	tmpDir := t.TempDir()
	if code := c.Run([]string{"report", "--project", tmpDir}); code != 1 {
		t.Errorf("expected report on empty directory to exit with 1, got %d", code)
	}

	// 3. Valid directory with composer.json
	composerContent := `{"name":"acme/test-app","require":{"php":"^8.3","laravel/framework":"^12.0"}}`
	_ = os.WriteFile(filepath.Join(tmpDir, "composer.json"), []byte(composerContent), 0644)
	outReport := filepath.Join(tmpDir, "README.md")
	if code := c.Run([]string{"report", "--project", tmpDir, "--out", outReport}); code != 0 {
		t.Errorf("expected report on valid directory to exit with 0, got %d", code)
	}

	// Verify report contains real metadata
	bytes, err := os.ReadFile(outReport)
	if err != nil {
		t.Fatalf("failed to read generated report: %v", err)
	}
	reportStr := string(bytes)
	if !containsStr(reportStr, "acme/test-app") {
		t.Errorf("expected report to contain acme/test-app, got:\n%s", reportStr)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || (len(s) > 0 && len(sub) > 0 && stringContains(s, sub)))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
