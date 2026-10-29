package storage

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSettingsManager(t *testing.T) {
	tmpDir := t.TempDir()
	settingsPath := filepath.Join(tmpDir, "settings.json")

	sm, err := NewSettingsManager(settingsPath)
	if err != nil {
		t.Fatalf("NewSettingsManager failed: %v", err)
	}

	initial := sm.Get()
	if initial.Version != 1 {
		t.Errorf("expected version 1, got %d", initial.Version)
	}

	err = sm.Update(func(s *Settings) {
		s.Trusted["proj123"] = true
		s.Theme = "dark"
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Reload from disk to verify atomic persistence
	sm2, err := NewSettingsManager(settingsPath)
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}
	loaded := sm2.Get()
	if !loaded.Trusted["proj123"] {
		t.Errorf("expected proj123 to be trusted")
	}
	if loaded.Theme != "dark" {
		t.Errorf("expected dark theme, got %s", loaded.Theme)
	}
}

func TestDiagnosticsDB(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test-diag.db")

	db, err := OpenDiagnosticsDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDiagnosticsDB failed: %v", err)
	}
	defer db.Close()

	rec := DiagnosticRecord{
		ID:        "rec1",
		ProjectID: "proj_a",
		Source:    "clockwork",
		Type:      "request",
		Timestamp: "2026-10-16T01:00:00Z",
		Status:    200,
		URI:       "/api/users",
		Duration:  45.2,
		Summary:   "GET /api/users 200 OK",
		Details:   "{}",
	}
	if err := db.SaveRecord(rec); err != nil {
		t.Fatalf("SaveRecord failed: %v", err)
	}

	records, err := db.QueryRecords("proj_a", "users", 10, 0)
	if err != nil {
		t.Fatalf("QueryRecords failed: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].URI != "/api/users" {
		t.Errorf("unexpected uri: %s", records[0].URI)
	}

	// Test error logging and grouping
	err = db.RecordError("proj_a", "fp_null_pointer", "ERROR", "Call to a member function on null", "app/Models/User.php", 42, "trace here")
	if err != nil {
		t.Fatalf("RecordError failed: %v", err)
	}
	// Second occurrence
	err = db.RecordError("proj_a", "fp_null_pointer", "ERROR", "Call to a member function on null", "app/Models/User.php", 42, "trace here")
	if err != nil {
		t.Fatalf("RecordError second occurrence failed: %v", err)
	}

	errorsList, err := db.QueryErrors("proj_a", "", 10)
	if err != nil {
		t.Fatalf("QueryErrors failed: %v", err)
	}
	if len(errorsList) != 1 {
		t.Fatalf("expected 1 error group, got %d", len(errorsList))
	}
	if errorsList[0].Occurrences != 2 {
		t.Errorf("expected 2 occurrences, got %d", errorsList[0].Occurrences)
	}

	// Test dump
	err = db.SaveDump("proj_a", "app/Http/Controllers/Test.php", 15, `{"foo":"bar"}`)
	if err != nil {
		t.Fatalf("SaveDump failed: %v", err)
	}
	dumps, err := db.QueryDumps("proj_a", 10)
	if err != nil {
		t.Fatalf("QueryDumps failed: %v", err)
	}
	if len(dumps) != 1 {
		t.Errorf("expected 1 dump, got %d", len(dumps))
	}

	// Clear project history
	if err := db.ClearProjectHistory("proj_a"); err != nil {
		t.Fatalf("ClearProjectHistory failed: %v", err)
	}
	recordsAfter, _ := db.QueryRecords("proj_a", "", 10, 0)
	if len(recordsAfter) != 0 {
		t.Errorf("expected 0 records after clear, got %d", len(recordsAfter))
	}
}

func TestRedactionBeforePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "redact-test.db")

	db, err := OpenDiagnosticsDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDiagnosticsDB failed: %v", err)
	}
	defer db.Close()

	// Sensitive markers that must NOT survive into SQLite
	secretMsg := "Failed connection to redis://admin:SecretPassword123@10.0.0.5:6379"
	secretStack := "Stack trace: at login(password='mySuperSecretPassword') with Bearer eyJhbGciOi..."
	mockGhp := "ghp_" + strings.Repeat("A1b2", 9)
	mockSk := "sk_" + "live_" + strings.Repeat("C3d4", 6)
	secretDump := `{"user":"admin","token":"` + mockGhp + `","api_key":"` + mockSk + `"}`

	_ = db.RecordError("proj_redact", "fp_secret", "ERROR", secretMsg, "LoginController.php", 10, secretStack)
	_ = db.SaveDump("proj_redact", "LoginController.php", 12, secretDump)

	// Read back and verify all sensitive markers were sanitized
	errorsList, err := db.QueryErrors("proj_redact", "", 10)
	if err != nil || len(errorsList) == 0 {
		t.Fatalf("QueryErrors failed: %v", err)
	}
	savedErr := errorsList[0]
	if strings.Contains(savedErr.Message, "SecretPassword123") {
		t.Errorf("database contains unredacted password in message: %s", savedErr.Message)
	}
	if strings.Contains(savedErr.Stack, "mySuperSecretPassword") {
		t.Errorf("database contains unredacted password in stack: %s", savedErr.Stack)
	}

	dumpsList, err := db.QueryDumps("proj_redact", 10)
	if err != nil || len(dumpsList) == 0 {
		t.Fatalf("QueryDumps failed: %v", err)
	}
	savedDump := dumpsList[0]
	if strings.Contains(savedDump.Payload, mockGhp) {
		t.Errorf("database contains unredacted GitHub token in dump: %s", savedDump.Payload)
	}
	if strings.Contains(savedDump.Payload, mockSk) {
		t.Errorf("database contains unredacted Stripe secret key in dump: %s", savedDump.Payload)
	}
}

func TestIdempotentErrorRecording(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "idempotent-test.db")

	db, err := OpenDiagnosticsDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDiagnosticsDB failed: %v", err)
	}
	defer db.Close()

	// Simulate repeated reads of the exact same log entry
	logKey := "proj_b:log_fp123_20261016010000"
	fp := "fp123"

	// First read
	newRecord, err := db.RecordErrorIdempotent("proj_b", logKey, fp, "ERROR", "Something broke", "App.php", 1, "stack")
	if err != nil {
		t.Fatalf("first RecordErrorIdempotent failed: %v", err)
	}
	if !newRecord {
		t.Errorf("expected first read to report newRecord=true")
	}

	// Repeated 9 reads of the exact same line
	for i := 0; i < 9; i++ {
		newRecord, err = db.RecordErrorIdempotent("proj_b", logKey, fp, "ERROR", "Something broke", "App.php", 1, "stack")
		if err != nil {
			t.Fatalf("repeat RecordErrorIdempotent failed: %v", err)
		}
		if newRecord {
			t.Errorf("repeat read %d should report newRecord=false", i+1)
		}
	}

	// Occurrences must remain exactly 1!
	errorsList, err := db.QueryErrors("proj_b", "", 10)
	if err != nil || len(errorsList) != 1 {
		t.Fatalf("QueryErrors failed: %v", err)
	}
	if errorsList[0].Occurrences != 1 {
		t.Errorf("expected occurrences to stay 1, but got %d", errorsList[0].Occurrences)
	}

	// Now a second distinct line occurs
	logKey2 := "proj_b:log_fp123_20261016010500"
	newRecord, err = db.RecordErrorIdempotent("proj_b", logKey2, fp, "ERROR", "Something broke again", "App.php", 1, "stack")
	if err != nil || !newRecord {
		t.Fatalf("expected newRecord=true for distinct log entry")
	}

	errorsList, _ = db.QueryErrors("proj_b", "", 10)
	if errorsList[0].Occurrences != 2 {
		t.Errorf("expected occurrences to increment to 2, got %d", errorsList[0].Occurrences)
	}
}

func TestRetentionPruning(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "retention-test.db")

	db, err := OpenDiagnosticsDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDiagnosticsDB failed: %v", err)
	}
	defer db.Close()

	// Insert old records
	oldDate := time.Now().UTC().AddDate(0, 0, -35).Format(time.RFC3339)
	_, _ = db.db.Exec(`INSERT INTO error_logs (id, project_id, fingerprint, level, message, file, line, stack, occurrences, first_seen, last_seen)
		VALUES ('old_err', 'proj_ret', 'fp_old', 'ERROR', 'Old error', 'App.php', 1, '', 1, ?, ?)`, oldDate, oldDate)

	err = db.PruneRetention(30)
	if err != nil {
		t.Fatalf("PruneRetention failed: %v", err)
	}

	errorsList, err := db.QueryErrors("proj_ret", "", 10)
	if err != nil {
		t.Fatalf("QueryErrors failed: %v", err)
	}
	if len(errorsList) != 0 {
		t.Errorf("expected old record to be pruned, got %d records", len(errorsList))
	}
}

func TestSettingsSnapshotDoesNotShareMaps(t *testing.T) {
	manager, err := NewSettingsManager(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := manager.Get()
	snapshot.Trusted["injected"] = true
	if manager.Get().Trusted["injected"] {
		t.Fatal("snapshot mutated stored trust")
	}
	if err := manager.Update(func(s *Settings) { s.Trusted = nil; s.SavedCommands = nil; s.ServiceConfigs = nil }); err != nil {
		t.Fatal(err)
	}
	normalized := manager.Get()
	if normalized.Trusted == nil || normalized.SavedCommands == nil || normalized.ServiceConfigs == nil {
		t.Fatal("nil settings maps accepted")
	}
}

func TestRedactionCoversEnvironmentStyleKeys(t *testing.T) {
	payload := `APP_KEY=fixture-app-value DB_PASSWORD=fixture-db-value {"STRIPE_SECRET":"fixture-payment-value","authorization":"fixture-auth-value"}`
	result := RedactSensitive(payload)
	for _, value := range []string{"fixture-app-value", "fixture-db-value", "fixture-payment-value", "fixture-auth-value"} {
		if strings.Contains(result, value) {
			t.Errorf("secret-style value was not redacted")
		}
	}
}
