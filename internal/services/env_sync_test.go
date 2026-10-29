package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyServicesToEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	initialContent := `APP_NAME=LaravelApp
APP_ENV=local
APP_KEY=base64:TestKey123
APP_DEBUG=true

DB_CONNECTION=sqlite
DB_DATABASE=/some/path/db.sqlite

MAIL_MAILER=log
`
	if err := os.WriteFile(envPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}

	opts := EnvSyncOptions{
		DatabaseEngine:       "mysql",
		DatabaseName:         "laravel_app_db",
		ConfigureRedis:       true,
		ConfigureMailpit:     true,
		ConfigureMeilisearch: true,
	}

	result, err := ApplyServicesToEnv(envPath, opts)
	if err != nil {
		t.Fatalf("ApplyServicesToEnv failed: %v", err)
	}

	// 1. Verify backup was created
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Errorf("expected backup file at %s: %v", result.BackupPath, err)
	}

	// 2. Verify new content
	newData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	newStr := string(newData)

	if !strings.Contains(newStr, "DB_CONNECTION=mysql") {
		t.Errorf("missing DB_CONNECTION=mysql")
	}
	if !strings.Contains(newStr, "DB_DATABASE=laravel_app_db") {
		t.Errorf("missing DB_DATABASE=laravel_app_db")
	}
	if !strings.Contains(newStr, "DB_PORT=3306") {
		t.Errorf("missing DB_PORT=3306")
	}
	if !strings.Contains(newStr, "REDIS_HOST=127.0.0.1") {
		t.Errorf("missing REDIS_HOST=127.0.0.1")
	}
	if !strings.Contains(newStr, "MAIL_MAILER=smtp") {
		t.Errorf("missing MAIL_MAILER=smtp")
	}
	if !strings.Contains(newStr, "MAIL_PORT=1025") {
		t.Errorf("missing MAIL_PORT=1025")
	}
	if !strings.Contains(newStr, "SCOUT_DRIVER=meilisearch") {
		t.Errorf("missing SCOUT_DRIVER=meilisearch")
	}
	// Original keys preserved
	if !strings.Contains(newStr, "APP_NAME=LaravelApp") {
		t.Errorf("original APP_NAME was lost")
	}
	if !strings.Contains(newStr, "APP_KEY=base64:TestKey123") {
		t.Errorf("original APP_KEY was lost")
	}
}
