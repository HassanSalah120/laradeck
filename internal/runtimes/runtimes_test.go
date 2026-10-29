package runtimes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerCatalogAndDirs(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	if mgr.BaseDir() != tmpDir {
		t.Errorf("expected base dir %s, got %s", tmpDir, mgr.BaseDir())
	}

	runtimes := mgr.ListStandaloneRuntimes()
	if len(runtimes) == 0 {
		t.Fatalf("expected catalog entries in ListStandaloneRuntimes")
	}

	for _, r := range runtimes {
		if r.Installed {
			t.Errorf("expected %s to be not installed in empty temp dir", r.ID)
		}
		if r.Type != "standalone" {
			t.Errorf("expected type standalone, got %s", r.Type)
		}
	}
}

func TestManagerIniInitializationAndToggle(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	verDir := filepath.Join(mgr.PHPDir(), "8.3")
	if err := os.MkdirAll(verDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create fake ext directory with some dummy dlls
	extDir := filepath.Join(verDir, "ext")
	_ = os.MkdirAll(extDir, 0755)
	_ = os.WriteFile(filepath.Join(extDir, "php_pdo_mysql.dll"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(extDir, "php_curl.dll"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(extDir, "php_redis.dll"), []byte(""), 0644)

	// Create fake php.ini-development
	devContent := `[PHP]
memory_limit = 128M
;extension_dir = "ext"
;extension=curl
;extension=pdo_mysql
;extension=redis
;zend_extension=opcache
`
	_ = os.WriteFile(filepath.Join(verDir, "php.ini-development"), []byte(devContent), 0644)

	// Test INI initialization
	if err := mgr.initializePhpIni(verDir); err != nil {
		t.Fatalf("initializePhpIni failed: %v", err)
	}

	iniPath := filepath.Join(verDir, "php.ini")
	data, err := os.ReadFile(iniPath)
	if err != nil {
		t.Fatal(err)
	}
	iniStr := string(data)

	if !strings.Contains(iniStr, "extension=curl") || strings.Contains(iniStr, ";extension=curl") {
		t.Errorf("expected extension=curl to be enabled in php.ini")
	}
	if !strings.Contains(iniStr, "extension=pdo_mysql") || strings.Contains(iniStr, ";extension=pdo_mysql") {
		t.Errorf("expected extension=pdo_mysql to be enabled in php.ini")
	}
	if !strings.Contains(iniStr, "memory_limit = 512M") {
		t.Errorf("expected memory_limit = 512M in php.ini")
	}

	// Test ToggleExtension: disable curl
	if err := mgr.ToggleExtension(iniPath, "curl", false); err != nil {
		t.Fatalf("ToggleExtension disable curl failed: %v", err)
	}
	data, _ = os.ReadFile(iniPath)
	if strings.Contains(string(data), "\nextension=curl") {
		t.Errorf("expected curl to be disabled")
	}

	// Test ToggleExtension: enable redis
	if err := mgr.ToggleExtension(iniPath, "redis", true); err != nil {
		t.Fatalf("ToggleExtension enable redis failed: %v", err)
	}
	data, _ = os.ReadFile(iniPath)
	if !strings.Contains(string(data), "extension=redis") {
		t.Errorf("expected redis to be enabled")
	}
}

func TestManagerCliShims(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)

	if err := mgr.generateCliShims(); err != nil {
		t.Fatalf("generateCliShims failed: %v", err)
	}

	phpBat := filepath.Join(mgr.BinDir(), "php.bat")
	composerBat := filepath.Join(mgr.BinDir(), "composer.bat")

	if _, err := os.Stat(phpBat); err != nil {
		t.Errorf("expected php.bat to exist: %v", err)
	}
	if _, err := os.Stat(composerBat); err != nil {
		t.Errorf("expected composer.bat to exist: %v", err)
	}
}
