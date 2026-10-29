package workbench

import (
	"context"
	"encoding/json"
	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/runner"
	"herdworkbench/internal/services"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProjectDependencyInstallRequiresTrust(t *testing.T) {
	b := NewBackend(nil)
	defer b.Shutdown()
	root := t.TempDir()
	b.projects = []Project{{ID: "fixture", Path: root, PHPBinary: "php.exe"}}
	if _, err := b.InstallDependency("clockwork", "fixture"); err == nil || !strings.Contains(err.Error(), "Trust") {
		t.Fatal("untrusted project installation accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "composer.json")); err == nil {
		t.Fatal("untrusted project was modified")
	}
}
func TestLivePortableDependencyInstall(t *testing.T) {
	if os.Getenv("WORKBENCH_RUN_PORTABLE_INSTALL") != "1" {
		t.Skip("opt in to download and install a private Mailpit fixture")
	}
	b := NewBackend(nil)
	defer b.Shutdown()
	result, err := b.InstallDependency("mailpit", "")
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "installed" {
		t.Fatal(result.State)
	}
	output, err := runner.Capture(runner.Spec{Executable: result.Path, Args: []string{"version"}}, 30*time.Second)
	if err != nil || !strings.Contains(strings.ToLower(string(output)), "mailpit") {
		t.Fatalf("downloaded executable did not run: err=%v output=%s", err, Redact(string(output)))
	}
	t.Logf("Verified official Mailpit download and executable: %s", result.Version)
	if services.IsPortInUse(1025) || services.IsPortInUse(8025) {
		t.Log("default mail ports occupied; did not start or stop external service")
		return
	}
	if err := b.StartService("mailpit", ""); err != nil {
		t.Fatal("service did not resolve newly installed private binary:", err)
	}
	if !services.WaitForPort(8025, 5*time.Second) {
		t.Fatal("installed Mailpit did not start")
	}
	if err := b.StopService("mailpit", false); err != nil {
		t.Fatal(err)
	}
	t.Log("Settings install -> automatic service resolution -> Mailpit start/stop verified")
}
func TestLiveProjectClockworkInstall(t *testing.T) {
	root := os.Getenv("WORKBENCH_SETUP_PROJECT")
	if root == "" {
		t.Skip("set WORKBENCH_SETUP_PROJECT to a disposable wizard fixture")
	}
	if filepath.Base(filepath.Dir(root)) != "wizard-fixtures" || !strings.HasPrefix(filepath.Base(root), "wizard-") {
		t.Fatal("only the disposable wizard fixture may be modified")
	}
	herd, err := FindHerd()
	if err != nil {
		t.Fatal(err)
	}
	b := NewBackend(nil)
	defer b.Shutdown()
	p := Project{ID: "setup-fixture", Path: root, PHPBinary: herd.PHP}
	InspectDependencies(&p)
	if p.Clockwork != "not installed" {
		t.Skip("fixture already has Clockwork")
	}
	b.projects = []Project{p}
	b.trusted[p.ID] = true
	b.selected = p.ID
	b.clock.Select(p)
	result, err := b.InstallDependency("clockwork", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "installed" || result.Version == "" {
		t.Fatal("installation was not verified from vendor metadata")
	}
	backups, _ := filepath.Glob(filepath.Join(root, "composer.json.workbench-backup.*"))
	if len(backups) == 0 {
		t.Fatal("Composer manifest backup missing")
	}
	updated, err := b.project(p.ID, true)
	if err != nil || updated.Clockwork == "not installed" {
		t.Fatal("project status was not refreshed")
	}
	data, err := runner.CaptureContext(context.Background(), runner.Spec{Executable: p.PHPBinary, Args: []string{"artisan", "route:list", "--json"}, Directory: root}, 20*time.Second)
	if err != nil || !strings.Contains(string(data), "__clockwork") {
		t.Fatal("installed Clockwork routes were not registered")
	}
	t.Logf("Settings Composer install -> vendor verification -> project status refresh -> registered Clockwork routes: %s", result.Version)
}

func TestComposerInstallReportsProgressAndCanCancel(t *testing.T) {
	if os.Getenv("WORKBENCH_RUNNER") == "" {
		helper := filepath.Join(t.TempDir(), "workbench-runner.exe")
		if data, err := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go.exe"), "build", "-o", helper, "../../cmd/runner").CombinedOutput(); err != nil {
			t.Fatalf("build test helper: %v %s", err, Redact(string(data)))
		}
		t.Setenv("WORKBENCH_RUNNER", helper)
	}

	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	composer := filepath.Join(root, "composer.exe")
	if err := os.WriteFile(composer, bytes, 0700); err != nil {
		t.Fatal(err)
	}
	vendor := filepath.Join(root, "vendor", "composer")
	os.MkdirAll(vendor, 0700)
	os.WriteFile(filepath.Join(vendor, "installed.json"), []byte(`{"packages":[{"name":"itsgoingd/clockwork","version":"v5.3.5"}]}`), 0600)
	os.WriteFile(filepath.Join(root, "composer.json"), []byte(`{}`), 0600)
	t.Setenv("WORKBENCH_COMPOSER_PROBE", "1")
	b := NewBackend(nil)
	defer b.Shutdown()
	b.dependencies = dependencies.NewManager(t.TempDir(), nil)
	if err := b.SetDependencyBinary("composer", composer); err != nil {
		t.Fatal(err)
	}
	// ComposerSpec uses the same private root as the app's dependency resolver.
	defaultManager := dependencies.NewManager("", nil)
	if err := defaultManager.SetBinary("composer", composer); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(filepath.Join(os.Getenv("APPDATA"), "HerdWorkbench", "tools", "composer"))
	b.projects = []Project{{ID: "fixture", Path: root, PHPBinary: "php.exe"}}
	b.trusted["fixture"] = true
	b.selected = "fixture"
	finished := make(chan error, 1)
	go func() { _, err := b.InstallDependency("clockwork", "fixture"); finished <- err }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := json.Marshal(b.Snapshot())
		var state struct {
			Installation struct {
				Running bool
				Output  []string
			}
		}
		json.Unmarshal(data, &state)
		if state.Installation.Running && strings.Contains(strings.Join(state.Installation.Output, "\n"), "Optimizing autoload") {
			if strings.Contains(string(data), "probe-secret") {
				t.Fatal("progress leaked output secret")
			}
			canceler, ok := any(b).(interface{ CancelDependencyInstall() error })
			if !ok {
				t.Fatal("no cancellation available")
			}
			if err := canceler.CancelDependencyInstall(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-finished:
				if err == nil {
					t.Fatal("cancelled install reported success")
				}
			case <-time.After(4 * time.Second):
				t.Fatal("cancel did not stop Composer")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("Clockwork is present, but installation has no visible progress or cancellation while Composer is still running")
}

func TestLiveClockworkSetupRepair(t *testing.T) {
	root := os.Getenv("WORKBENCH_SETUP_PROJECT")
	if root == "" {
		t.Skip("set disposable wizard fixture to verify setup repair")
	}
	if filepath.Base(filepath.Dir(root)) != "wizard-fixtures" || !strings.HasPrefix(filepath.Base(root), "wizard-") {
		t.Fatal("only disposable wizard fixture may change")
	}
	herd, err := FindHerd()
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, "bootstrap", "cache", "packages.php")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer os.WriteFile(manifest, original, 0600)
	if err := os.WriteFile(manifest, []byte("<?php return [];"), 0600); err != nil {
		t.Fatal(err)
	}
	b := NewBackend(nil)
	defer b.Shutdown()
	p := Project{ID: "repair-fixture", Path: root, PHPBinary: herd.PHP}
	InspectDependencies(&p)
	b.projects = []Project{p}
	b.trusted[p.ID] = true
	b.selected = p.ID
	result, err := b.InstallDependency("clockwork", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.State != "installed" {
		t.Fatalf("repair state=%s", result.State)
	}
	data, err := runner.Capture(runner.Spec{Executable: herd.PHP, Directory: root, Args: []string{"artisan", "route:list", "--path=__clockwork", "--json"}}, 20*time.Second)
	if err != nil || !strings.Contains(string(data), "__clockwork") {
		t.Fatal("Clockwork routes absent after repair", err)
	}
	t.Log("Existing package repaired: autoloader, Laravel package discovery, registered metadata routes")
}
