package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWebviewProfilePreservesReviewedLegacyStorageAcrossBuildNames(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"laradeck-fixed.exe", "laradeck-ui.exe"} {
		dir := filepath.Join(root, name, "EBWebView", "Default", "Local Storage", "leveldb")
		os.MkdirAll(dir, 0700)
		value := "other-local-state"
		if name == "laradeck-fixed.exe" {
			value = "laradeck_audit_fp_fixture"
		}
		os.WriteFile(filepath.Join(dir, "00001.log"), []byte(value), 0600)
	}
	selected, err := WebviewProfile(root)
	if err != nil || selected != filepath.Join(root, "laradeck-fixed.exe") {
		t.Fatalf("reviewed profile lost: %q %v", selected, err)
	}
	// A later-named release/profile must not change the pinned choice.
	os.MkdirAll(filepath.Join(root, "laradeck-new.exe"), 0700)
	again, err := WebviewProfile(root)
	if err != nil || again != selected {
		t.Fatalf("profile changed across release: %q %v", again, err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "laradeck-fixed.exe", "EBWebView", "Default", "Local Storage", "leveldb", "00001.log"))
	if string(raw) != "laradeck_audit_fp_fixture" {
		t.Fatal("legacy profile modified")
	}
}
func TestWebviewProfileNewInstallAndConfinement(t *testing.T) {
	root := t.TempDir()
	selected, err := WebviewProfile(root)
	if err != nil || selected != filepath.Join(root, "HerdWorkbench", "WebView") {
		t.Fatalf("new profile: %s %v", selected, err)
	}
	os.WriteFile(filepath.Join(root, "HerdWorkbench", "webview-profile.json"), []byte(`{"folder":"../outside"}`), 0600)
	if _, err = WebviewProfile(root); err == nil {
		t.Fatal("profile selection escaped user configuration")
	}
}
