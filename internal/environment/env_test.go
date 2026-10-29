package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvParsingAndPreservation(t *testing.T) {
	sample := "# Application config\r\nAPP_NAME=\"Laravel Workbench\"\r\nAPP_ENV=local\r\n\r\n# Database settings\r\nDB_CONNECTION=sqlite\r\nDB_PASSWORD=secret123 # inline comment\r\n"
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(sample), 0644); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	envFile, err := ParseEnv(envPath)
	if err != nil {
		t.Fatalf("ParseEnv failed: %v", err)
	}

	if envFile.LineEnding != "\r\n" {
		t.Errorf("expected line ending \\r\\n, got %q", envFile.LineEnding)
	}

	if len(envFile.Entries) != 7 {
		t.Fatalf("expected 7 entries, got %d", len(envFile.Entries))
	}

	// Check sensitive masking
	foundPassword := false
	for _, e := range envFile.Entries {
		if e.Key == "DB_PASSWORD" {
			foundPassword = true
			if !e.IsSensitive {
				t.Errorf("DB_PASSWORD should be sensitive")
			}
			if e.MaskedValue != "••••••••" {
				t.Errorf("expected masked password, got %s", e.MaskedValue)
			}
			if e.Value != "secret123" {
				t.Errorf("expected value secret123, got %s", e.Value)
			}
		}
	}
	if !foundPassword {
		t.Errorf("DB_PASSWORD entry not found")
	}

	// Modify one value and test Diff
	modEntries := append([]EnvEntry{}, envFile.Entries...)
	for i, e := range modEntries {
		if e.Key == "APP_ENV" {
			modEntries[i].Value = "production"
		}
	}
	modFile := &EnvFile{Path: envPath, Entries: modEntries}
	diffs := Diff(envFile, modFile)
	if len(diffs) != 1 || diffs[0].Key != "APP_ENV" || diffs[0].NewValue != "production" {
		t.Errorf("unexpected diff: %+v", diffs)
	}

	// Test SaveEnv
	envFile.Entries = modEntries
	if err := SaveEnv(envFile, envFile.Fingerprint); err != nil {
		t.Fatalf("SaveEnv failed: %v", err)
	}

	// Read back and verify formatting and inline comment preserved
	savedBytes, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read back: %v", err)
	}
	savedStr := string(savedBytes)
	if !strings.Contains(savedStr, "APP_ENV=production") {
		t.Errorf("expected APP_ENV=production in %s", savedStr)
	}
	if !strings.Contains(savedStr, "# inline comment") {
		t.Errorf("expected inline comment to be preserved in %s", savedStr)
	}
	if !strings.Contains(savedStr, "\r\n") {
		t.Errorf("expected CRLF line endings to be preserved")
	}

	// Test concurrent conflict detection
	_ = os.WriteFile(envPath, []byte("EXTERNAL=change\r\n"), 0644)
	err = SaveEnv(envFile, envFile.Fingerprint)
	if err == nil {
		t.Fatalf("expected concurrent modification error, got nil")
	}
}

func TestEnvUntouchedPreservationAndTrailingNewline(t *testing.T) {
	// 1. Synthetic file containing unusual whitespace and no trailing newline
	sample := "  KEEP = value  \nOTHER=original"
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(sample), 0644); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	envFile, err := ParseEnv(envPath)
	if err != nil {
		t.Fatalf("ParseEnv failed: %v", err)
	}

	if envFile.HasTrailingNewline {
		t.Errorf("expected HasTrailingNewline to be false")
	}

	// Modify unrelated key OTHER
	for i, e := range envFile.Entries {
		if e.Key == "OTHER" {
			envFile.Entries[i].Value = "modified"
		}
	}

	// Save with fingerprint
	if err := SaveEnv(envFile, envFile.Fingerprint); err != nil {
		t.Fatalf("SaveEnv failed: %v", err)
	}

	// Verify that untouched line "  KEEP = value  " is preserved exactly
	savedBytes, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	savedStr := string(savedBytes)

	if !strings.Contains(savedStr, "  KEEP = value  ") {
		t.Errorf("expected untouched line '  KEEP = value  ' to be preserved exactly, got:\n%s", savedStr)
	}
	if strings.HasSuffix(savedStr, "\n") {
		t.Errorf("expected no trailing newline, but found trailing newline")
	}

	// 2. Empty expected fingerprint must be rejected
	err = SaveEnv(envFile, "")
	if err == nil {
		t.Errorf("expected error when saving without fingerprint")
	}

	// 3. Verify backup file was created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	hasBackup := false
	for _, f := range files {
		if strings.HasPrefix(f.Name(), ".env.backup.") {
			hasBackup = true
			break
		}
	}
	if !hasBackup {
		t.Errorf("expected backup file to be created in directory")
	}
}

func TestCompareDrift(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	examplePath := filepath.Join(tmpDir, ".env.example")

	envContent := "APP_KEY=base64:xyz\nDB_DATABASE=db.sqlite\nCUSTOM_VAR=hello\n"
	exampleContent := "APP_KEY=\nDB_DATABASE=\nMAIL_HOST=127.0.0.1\n"

	_ = os.WriteFile(envPath, []byte(envContent), 0644)
	_ = os.WriteFile(examplePath, []byte(exampleContent), 0644)

	drift, err := CompareDrift(envPath, examplePath)
	if err != nil {
		t.Fatalf("CompareDrift failed: %v", err)
	}

	if len(drift.MissingKeys) != 1 || drift.MissingKeys[0] != "MAIL_HOST" {
		t.Errorf("expected MAIL_HOST to be missing, got %+v", drift.MissingKeys)
	}
	if len(drift.ExtraKeys) != 1 || drift.ExtraKeys[0] != "CUSTOM_VAR" {
		t.Errorf("expected CUSTOM_VAR to be extra, got %+v", drift.ExtraKeys)
	}
	if drift.Matching != 2 {
		t.Errorf("expected 2 matching keys, got %d", drift.Matching)
	}
}

func TestValidateIniDirective_RejectsInjection(t *testing.T) {
	// Rejects newline / carriage return injection
	err := ValidateIniDirective("memory_limit", "256M\r\nauto_prepend_file = review-fixture.php")
	if err == nil || !strings.Contains(err.Error(), "newline") {
		t.Fatalf("expected error rejecting newline injection, got: %v", err)
	}

	// Rejects invalid size format
	err = ValidateIniDirective("memory_limit", "invalid_size")
	if err == nil {
		t.Fatalf("expected error rejecting invalid size format, got nil")
	}

	// Accepts valid sizes
	if err := ValidateIniDirective("memory_limit", "512M"); err != nil {
		t.Errorf("unexpected error on valid size 512M: %v", err)
	}
	if err := ValidateIniDirective("memory_limit", "-1"); err != nil {
		t.Errorf("unexpected error on valid size -1: %v", err)
	}

	// Rejects unapproved directives
	if err := ValidateIniDirective("auto_prepend_file", "malicious.php"); err == nil {
		t.Errorf("expected error rejecting unapproved directive auto_prepend_file, got nil")
	}
}

func TestApplyIniChange_RejectsNewlineInjection(t *testing.T) {
	tmpDir := t.TempDir()
	iniPath := filepath.Join(tmpDir, "php.ini")
	_ = os.WriteFile(iniPath, []byte("memory_limit = 128M\nmax_execution_time = 30\n"), 0644)

	err := ApplyIniChange(iniPath, "memory_limit", "256M\r\nauto_prepend_file = evil.php")
	if err == nil {
		t.Fatalf("expected ApplyIniChange to reject newline injection, but succeeded")
	}

	content, _ := os.ReadFile(iniPath)
	if strings.Contains(string(content), "auto_prepend_file") {
		t.Fatalf("disallowed directive was written to php.ini: %s", string(content))
	}
}

func TestQuotedCommentsAndCallerMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(path, []byte("NAME=\"old # value\" # keep\nOTHER=unchanged\n"), 0600)
	env, err := ParseEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	if env.Entries[0].Value != "old # value" || env.Entries[0].Comment != "# keep" {
		t.Fatal("quoted inline comment parsed incorrectly")
	}
	env.Entries[0].Value = "new"
	env.Entries[0].InitialValue = "new"
	env.Entries[0].Raw = "INJECTED=wrong"
	if err := SaveEnv(env, env.Fingerprint); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "NAME=\"new\" # keep") || strings.Contains(string(data), "INJECTED") {
		t.Fatal("caller formatting metadata was trusted")
	}
}

func TestIniEffectiveDuplicateAndConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "php.ini")
	os.WriteFile(path, []byte("memory_limit = 128M\r\nmemory_limit = 256M\r\n"), 0600)
	preview, err := PreviewIniChange(path, "memory_limit", "512M")
	if err != nil {
		t.Fatal(err)
	}
	if preview.OldValue != "256M" {
		t.Fatal("preview did not use last effective directive")
	}
	if err := ApplyIniChangeChecked(path, "memory_limit", "512M", preview.Fingerprint); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "memory_limit = 128M\r\nmemory_limit = 512M\r\n") {
		t.Fatal("wrong directive or line ending changed")
	}
	if err := ApplyIniChangeChecked(path, "memory_limit", "1G", preview.Fingerprint); err == nil {
		t.Fatal("stale preview accepted")
	}
	if err := ValidateIniDirective("error_reporting", "E_ALL & ~E_DEPRECATED"); err != nil {
		t.Fatal(err)
	}
}

func TestMultilineEnvParsingAndSecretScanning(t *testing.T) {
	mockStripe := "sk_" + "live_" + strings.Repeat("x123", 6)
	multilineSample := "APP_NAME=\"LaraDeck\"\nRSA_PRIVATE_KEY=\"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0Y3\n-----END RSA PRIVATE KEY-----\"\nSTRIPE_SECRET=\"" + mockStripe + "\"\nAWS_KEY=AKIAIOSFODNN7EXAMPLE\n"
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(multilineSample), 0644); err != nil {
		t.Fatalf("failed to write test .env: %v", err)
	}

	envFile, err := ParseEnv(envPath)
	if err != nil {
		t.Fatalf("ParseEnv failed on multiline: %v", err)
	}

	if len(envFile.Entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(envFile.Entries))
	}

	rsaEntry := envFile.Entries[1]
	if rsaEntry.Key != "RSA_PRIVATE_KEY" {
		t.Errorf("expected RSA_PRIVATE_KEY, got %s", rsaEntry.Key)
	}
	if !strings.Contains(rsaEntry.Value, "BEGIN RSA PRIVATE KEY") || !strings.Contains(rsaEntry.Value, "END RSA PRIVATE KEY") {
		t.Errorf("multiline value not preserved, got: %s", rsaEntry.Value)
	}

	// Test Secret Scanning
	findings := ScanSecrets(envFile)
	if len(findings) < 2 {
		t.Fatalf("expected at least 2 secret findings (Stripe, AWS), got %d", len(findings))
	}

	hasStripe := false
	hasAWS := false
	for _, f := range findings {
		if f.Kind == "stripe_live" && f.Severity == "danger" {
			hasStripe = true
		}
		if f.Kind == "aws_access_key" && f.Severity == "danger" {
			hasAWS = true
		}
	}
	if !hasStripe {
		t.Errorf("expected stripe_live danger finding")
	}
	if !hasAWS {
		t.Errorf("expected aws_access_key danger finding")
	}
}

