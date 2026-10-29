package quality

import (
	"os"
	"strings"
	"testing"
)

func TestPestArchitectureGenerator(t *testing.T) {
	cfg := ArchRuleConfig{
		EnforceThinControllers:      true,
		EnforceSinglePurposeActions: true,
		EnforcePureModels:           true,
		EnforceStrictTypes:          true,
	}

	suite := GeneratePestArchitectureSuite(cfg)
	if !strings.Contains(suite, "declare(strict_types=1);") {
		t.Errorf("missing strict types declaration")
	}
	if !strings.Contains(suite, "App\\Http\\Controllers") || !strings.Contains(suite, "Illuminate\\Database\\Eloquent\\Model") {
		t.Errorf("missing thin controller rule")
	}
	if !strings.Contains(suite, "App\\Actions") || !strings.Contains(suite, "handle") {
		t.Errorf("missing action handle method rule")
	}

	tmpDir := t.TempDir()
	path, err := WritePestArchitectureFile(tmpDir, cfg, false)
	if err != nil {
		t.Fatalf("failed to write architecture file: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("written file not found on disk")
	}

	// Should fail if overwriting without flag
	if _, err := WritePestArchitectureFile(tmpDir, cfg, false); err == nil {
		t.Errorf("expected error when file already exists without overwrite flag")
	}
}

func TestIsAuditableFile(t *testing.T) {
	if !isAuditableFile("app/Http/Controllers/OrderController.php") {
		t.Errorf("controller php should be auditable")
	}
	if !isAuditableFile("resources/js/Pages/Dashboard.tsx") {
		t.Errorf("tsx file should be auditable")
	}
	if isAuditableFile("vendor/laravel/framework/src/Illuminate/Foundation/Application.php") {
		t.Errorf("vendor files must not be auditable")
	}
	if isAuditableFile("node_modules/react/index.js") {
		t.Errorf("node_modules files must not be auditable")
	}
}
