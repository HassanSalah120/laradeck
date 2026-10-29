package wizard

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWizardSpecValidation(t *testing.T) {
	ctx := context.Background()

	// Missing dir / name
	_, err := CreateProject(ctx, WizardSpec{})
	if err == nil {
		t.Fatal("expected error for empty spec")
	}

	// Invalid name
	_, err = CreateProject(ctx, WizardSpec{
		Directory: t.TempDir(),
		Name:      "invalid name with spaces",
	})
	if err == nil {
		t.Fatal("expected error for invalid project name")
	}

	// Non-empty target dir
	tmp := t.TempDir()
	target := filepath.Join(tmp, "existing_project")
	_ = os.MkdirAll(target, 0755)
	_ = os.WriteFile(filepath.Join(target, "file.txt"), []byte("content"), 0644)

	_, err = CreateProject(ctx, WizardSpec{
		Directory: tmp,
		Name:      "existing_project",
	})
	if err == nil {
		t.Fatal("expected error for non-empty existing directory")
	}
}
