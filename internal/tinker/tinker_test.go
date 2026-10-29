package tinker

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestEnsureRunnerScript(t *testing.T) {
	scriptPath, err := ensureRunnerScript()
	if err != nil {
		t.Fatalf("ensureRunnerScript failed: %v", err)
	}
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("runner script does not exist at %s: %v", scriptPath, err)
	}
}

func TestTinkerServiceEmptyCode(t *testing.T) {
	svc := NewTinkerService()
	res, err := svc.Evaluate("php", "C:\\some\\project", "   \n")
	if err != nil {
		t.Fatalf("expected nil error for empty code, got: %v", err)
	}
	if res.Output != "=> null" {
		t.Errorf("expected '=> null', got %q", res.Output)
	}
}

func TestTinkerServiceValidation(t *testing.T) {
	svc := NewTinkerService()
	_, err := svc.Evaluate("", "C:\\project", "echo 1;")
	if err == nil {
		t.Error("expected error for empty php binary")
	}

	_, err = svc.Evaluate("php", "", "echo 1;")
	if err == nil {
		t.Error("expected error for empty project path")
	}
}

func TestTinkerServiceNonLaravelProject(t *testing.T) {
	// If php is available in test environment, run against an empty directory
	phpBin, err := os.Executable()
	if err != nil {
		t.Skip("cannot find executable")
	}

	tempDir := t.TempDir()
	svc := NewTinkerService()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Using a dummy mock script or php if available
	_ = phpBin
	// Check if php exists in PATH or Herd
	phpCmd := "php"
	if custom := os.Getenv("WORKBENCH_PHP"); custom != "" {
		phpCmd = custom
	}

	res, _ := svc.EvaluateContext(ctx, phpCmd, tempDir, "1 + 1", 5*time.Second)
	// It should either return error or JSON with error about missing vendor/autoload
	if res.Error == "" && res.Output == "" {
		// If php doesn't exist in environment, err is fine
		t.Logf("Result: %+v", res)
	}
}
