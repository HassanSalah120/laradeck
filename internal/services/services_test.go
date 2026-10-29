package services

import (
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestServiceManagerPortCheckAndDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewServiceManager(filepath.Join(tmpDir, "srv"))

	// Ensure defaults are registered
	insts := sm.RefreshStatuses()
	if len(insts) < 5 {
		t.Fatalf("expected at least 5 default service profiles, got %d", len(insts))
	}

	mailpit, err := sm.GetInstance("mailpit")
	if err != nil {
		t.Fatalf("GetInstance mailpit failed: %v", err)
	}
	if mailpit.Port != 1025 || mailpit.AdminPort != 8025 {
		t.Errorf("unexpected mailpit ports: %+v", mailpit)
	}

	// Start a mock external listener on a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on test port: %v", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	if !IsPortInUse(port) {
		t.Errorf("expected port %d to be in use", port)
	}
	if IsPortInUse(port + 12345) {
		t.Errorf("expected port %d to not be in use", port+12345)
	}

	// Simulate external service detection on postgres port by setting port
	sm.mu.Lock()
	sm.instances["postgres"].Port = port
	sm.mu.Unlock()

	refreshed := sm.RefreshStatuses()
	var pgStatus ServiceStatus
	for _, inst := range refreshed {
		if inst.ID == "postgres" {
			pgStatus = inst.Status
		}
	}
	if pgStatus != StatusExternal {
		t.Errorf("expected postgres to be marked external, got %s", pgStatus)
	}
}

func TestDockerDetection(t *testing.T) {
	status := DetectDocker()
	// Should not crash and should return well-formed message
	if status.Message == "" {
		t.Errorf("expected non-empty Docker status message")
	}
}

func TestLiveOwnedMailpitService(t *testing.T) {
	binary := os.Getenv("WORKBENCH_MAILPIT")
	if binary == "" || os.Getenv("WORKBENCH_RUNNER") == "" {
		t.Skip("set WORKBENCH_MAILPIT and WORKBENCH_RUNNER for native service test")
	}
	freePort := func() int {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		return l.Addr().(*net.TCPAddr).Port
	}
	manager := NewServiceManager(t.TempDir())
	manager.instances["mailpit"].Port = freePort()
	manager.instances["mailpit"].AdminPort = freePort()
	if err := manager.StartService("mailpit", binary); err != nil {
		t.Fatal(err)
	}
	defer manager.StopAll(true)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		inst, _ := manager.GetInstance("mailpit")
		if inst.Status == StatusRunning {
			break
		}
		if inst.Status == StatusFailed {
			t.Fatalf("startup failed: %s", inst.Error)
		}
		time.Sleep(50 * time.Millisecond)
	}
	inst, _ := manager.GetInstance("mailpit")
	if inst.Status != StatusRunning || !inst.IsOwned {
		t.Fatal("owned service did not become ready")
	}
	if err := manager.StopService("mailpit", false); err != nil {
		t.Fatal(err)
	}
	if manager.ActiveOwnedCount() != 0 || IsPortInUse(inst.Port) {
		t.Fatal("service stop left process or port active")
	}
	t.Log("ServiceManager: actual Mailpit launch, port readiness, ownership and graceful stop verified")
}
