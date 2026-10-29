package database

import (
	"os"
	"testing"
)

func TestEnsureExplorerScript(t *testing.T) {
	scriptPath, err := ensureExplorerScript()
	if err != nil {
		t.Fatalf("ensureExplorerScript failed: %v", err)
	}
	if _, err := os.Stat(scriptPath); err != nil {
		t.Fatalf("script file does not exist at %s: %v", scriptPath, err)
	}
}

func TestDatabaseServiceValidation(t *testing.T) {
	svc := NewDatabaseService()

	_, err := svc.ListModels("", "C:\\project")
	if err == nil {
		t.Error("expected error for empty php binary in ListModels")
	}

	_, err = svc.GetModelRecords("php", "", "App\\Models\\User", 1, 25, "", "", "")
	if err == nil {
		t.Error("expected error for empty project path in GetModelRecords")
	}

	_, err = svc.UpdateModelRecord("php", "C:\\project", "", 1, map[string]any{"name": "test"})
	if err == nil {
		t.Error("expected error for empty model class in UpdateModelRecord")
	}

	_, err = svc.DeleteModelRecord("php", "C:\\project", "App\\Models\\User", nil)
	if err == nil {
		t.Error("expected error for nil record ID in DeleteModelRecord")
	}
}
