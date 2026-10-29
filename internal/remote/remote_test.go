package remote

import (
	"os"
	"testing"
)

func TestValidateProfile(t *testing.T) {
	valid := RemoteProfile{
		Name:        "Production API",
		Host:        "api.example.com",
		Port:        22,
		User:        "forge",
		RemotePath:  "/home/forge/api.example.com",
		Environment: "production",
		ReadOnly:    true,
	}

	if err := ValidateProfile(valid); err != nil {
		t.Errorf("expected valid profile, got %v", err)
	}

	invalid := valid
	invalid.Name = ""
	if err := ValidateProfile(invalid); err == nil {
		t.Error("expected error for empty name")
	}

	invalid = valid
	invalid.Port = 99999
	if err := ValidateProfile(invalid); err == nil {
		t.Error("expected error for invalid port")
	}
}

func TestAssessCommandSafety(t *testing.T) {
	readOnlyProfile := RemoteProfile{
		Name:        "Staging Server",
		Environment: "staging",
		ReadOnly:    true,
	}

	res := AssessCommandSafety(readOnlyProfile, "php artisan migrate:fresh", "")
	if res.Allowed {
		t.Error("expected migrate:fresh to be blocked on ReadOnly profile")
	}

	res = AssessCommandSafety(readOnlyProfile, "php artisan route:list", "")
	if !res.Allowed {
		t.Errorf("expected route:list to be allowed, got: %s", res.Reason)
	}

	prodProfile := RemoteProfile{
		Name:        "Production Cluster",
		Environment: "production",
		ReadOnly:    false,
	}

	res = AssessCommandSafety(prodProfile, "php artisan db:wipe", "")
	if res.Allowed {
		t.Error("expected db:wipe without token to be blocked on production")
	}
	if !res.RequiresToken {
		t.Error("expected RequiresToken to be true for dangerous command on production")
	}

	res = AssessCommandSafety(prodProfile, "php artisan db:wipe", "CONFIRM-PRODUCTION-MUTATION")
	if !res.Allowed {
		t.Errorf("expected db:wipe with valid token to be allowed, got: %s", res.Reason)
	}
}

func TestProfileManagerCRUD(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "remote-profile-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	pm, err := NewProfileManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to init profile manager: %v", err)
	}

	p := RemoteProfile{
		Name:        "Test VPS",
		Host:        "192.168.1.50",
		Port:        22,
		User:        "deploy",
		RemotePath:  "/var/www/test",
		Environment: "development",
	}

	saved, err := pm.SaveProfile(p)
	if err != nil {
		t.Fatalf("failed to save profile: %v", err)
	}
	if saved.ID == "" {
		t.Error("expected profile ID to be generated")
	}

	list := pm.ListProfiles()
	if len(list) != 1 {
		t.Fatalf("expected 1 profile in list, got %d", len(list))
	}

	// Verify persistence by reloading from disk
	pm2, err := NewProfileManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to reload profile manager: %v", err)
	}
	if len(pm2.ListProfiles()) != 1 {
		t.Errorf("expected 1 profile after disk reload, got %d", len(pm2.ListProfiles()))
	}

	err = pm.DeleteProfile(saved.ID)
	if err != nil {
		t.Errorf("failed to delete profile: %v", err)
	}
	if len(pm.ListProfiles()) != 0 {
		t.Errorf("expected 0 profiles after delete, got %d", len(pm.ListProfiles()))
	}
}
