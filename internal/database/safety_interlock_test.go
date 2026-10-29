package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAssessBranchSafety(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. Feature branch: destructive operations allowed
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature/awesome-ui\n"), 0644); err != nil {
		t.Fatal(err)
	}
	status := AssessBranchSafety(tmpDir, "migrate:fresh")
	if !status.OperationPermitted {
		t.Errorf("expected operation to be permitted on feature branch, got blocked: %s", status.Warning)
	}
	if status.IsProtected {
		t.Errorf("feature branch should not be protected")
	}

	// 2. Main branch: destructive operations blocked
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	statusMain := AssessBranchSafety(tmpDir, "migrate:fresh")
	if statusMain.OperationPermitted {
		t.Errorf("expected migrate:fresh to be blocked on main branch")
	}
	if !statusMain.IsProtected {
		t.Errorf("main branch must be flagged as protected")
	}

	// 3. Non-destructive operations allowed on main
	statusRead := AssessBranchSafety(tmpDir, "migrate:status")
	if !statusRead.OperationPermitted {
		t.Errorf("read/status operation should be permitted even on main")
	}
}
