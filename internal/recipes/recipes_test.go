package recipes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecipeCatalogAndPreview(t *testing.T) {
	rc := NewRecipeCatalog()
	list := rc.ListRecipes()
	if len(list) < 4 {
		t.Fatalf("expected at least 4 bundled recipes, got %d", len(list))
	}

	livewire, err := rc.GetRecipe("livewire-3")
	if err != nil {
		t.Fatalf("GetRecipe livewire-3 failed: %v", err)
	}
	if livewire.Category != "package" {
		t.Errorf("expected category 'package', got %s", livewire.Category)
	}

	preview, err := rc.PreviewRecipe("livewire-3")
	if err != nil {
		t.Fatalf("PreviewRecipe failed: %v", err)
	}
	if len(preview.CommandsToRun) != 2 {
		t.Errorf("expected 2 commands in preview, got %d", len(preview.CommandsToRun))
	}
}

func TestRecipeFileAndEnvSteps(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	_ = os.WriteFile(envPath, []byte("APP_NAME=Laravel\n"), 0644)

	customRecipe := Recipe{
		ID:    "test-custom",
		Title: "Custom Test Recipe",
		Steps: []RecipeStep{
			{
				Type:       StepWriteFile,
				Title:      "Create custom config file",
				TargetFile: "config/custom.php",
				Content:    "<?php return ['enabled' => true];",
			},
			{
				Type:     StepEnvSet,
				Title:    "Set custom env key",
				EnvKey:   "CUSTOM_FEATURE",
				EnvValue: "enabled",
			},
			{
				Type:        StepVerify,
				Title:       "Verify custom file exists",
				VerifyCheck: "config/custom.php",
			},
		},
		RollbackInstructions: "Delete config/custom.php",
	}

	rc := NewRecipeCatalog()
	rc.recipes["test-custom"] = customRecipe

	result, err := rc.ExecuteRecipe("test-custom", "php", tmpDir)
	if err != nil {
		t.Fatalf("ExecuteRecipe failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected recipe success, got false")
	}

	// Verify written file
	createdFile := filepath.Join(tmpDir, "config", "custom.php")
	if _, err := os.Stat(createdFile); err != nil {
		t.Errorf("expected file %s to exist", createdFile)
	}

	// Verify .env modification
	envBytes, _ := os.ReadFile(envPath)
	if !stringsContains(string(envBytes), "CUSTOM_FEATURE=enabled") {
		t.Errorf("expected CUSTOM_FEATURE=enabled in .env, got %s", string(envBytes))
	}
}

func stringsContains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && findSubstr(s, substr)))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestRecipeDoesNotHideMissingEnvOrUnknownSteps(t *testing.T) {
	catalog := NewRecipeCatalog()
	catalog.recipes["missing-env"] = Recipe{ID: "missing-env", Steps: []RecipeStep{{Type: StepEnvSet, EnvKey: "FEATURE", EnvValue: "on"}}}
	if result, err := catalog.ExecuteRecipe("missing-env", "php", t.TempDir()); err == nil || (result != nil && result.Success) {
		t.Fatal("missing .env reported success")
	}
	catalog.recipes["unknown"] = Recipe{ID: "unknown", Steps: []RecipeStep{{Type: StepType("unknown")}}}
	if _, err := catalog.ExecuteRecipe("unknown", "php", t.TempDir()); err == nil {
		t.Fatal("unknown step reported success")
	}
	catalog.recipes["escape"] = Recipe{ID: "escape", Steps: []RecipeStep{{Type: StepWriteFile, TargetFile: "../outside.php", Content: "wrong"}}}
	if _, err := catalog.ExecuteRecipe("escape", "php", t.TempDir()); err == nil {
		t.Fatal("recipe escaped project")
	}
}
