package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDumpServer(t *testing.T) {
	received := make(chan DumpPayload, 1)
	server, err := NewDumpServer(0, 10, func(d DumpPayload) {
		received <- d
	})
	if err != nil {
		t.Fatalf("failed to create dump server: %v", err)
	}
	defer server.Close()

	payload := DumpPayload{
		ProjectID: "test-project",
		File:      "app/Http/Controllers/Test.php",
		Line:      42,
		Payload:   `{"foo":"bar"}`,
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d", server.Port()), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to post dump: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	select {
	case got := <-received:
		if got.ProjectID != "test-project" || got.Line != 42 {
			t.Fatalf("unexpected dump payload: %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dump callback")
	}

	recent := server.GetRecentDumps("test-project")
	if len(recent) != 1 {
		t.Fatalf("expected 1 recent dump, got %d", len(recent))
	}

	server.Clear("test-project")
	if len(server.GetRecentDumps("test-project")) != 0 {
		t.Fatal("expected dumps to be cleared")
	}
}

func TestXdebugHelpers(t *testing.T) {
	url := BuildXdebugTriggerURL("http://myapp.test/dashboard?tab=1", "MYKEY")
	if url != "http://myapp.test/dashboard?XDEBUG_SESSION=MYKEY&tab=1" && url != "http://myapp.test/dashboard?tab=1&XDEBUG_SESSION=MYKEY" {
		t.Fatalf("unexpected trigger url: %s", url)
	}

	tmp := t.TempDir()
	if err := GenerateVSCodeConfig(tmp); err != nil {
		t.Fatalf("failed to generate vscode config: %v", err)
	}

	launchFile := filepath.Join(tmp, ".vscode", "launch.json")
	if _, err := os.Stat(launchFile); err != nil {
		t.Fatal("expected launch.json to exist")
	}

	status, err := CheckXdebugStatus(context.Background(), "", "http://test.test")
	if err != nil {
		t.Fatalf("CheckXdebugStatus failed: %v", err)
	}
	if status.Port != 9003 {
		t.Fatalf("expected default port 9003, got %d", status.Port)
	}
}

func TestAgentAttachmentAndTelemetry(t *testing.T) {
	tmpDir := t.TempDir()
	publicDir := filepath.Join(tmpDir, "public")
	if err := os.MkdirAll(publicDir, 0755); err != nil {
		t.Fatalf("failed to create public dir: %v", err)
	}

	// 1. Initially not attached
	if IsDebuggerAttached(tmpDir) {
		t.Fatal("expected debugger to NOT be attached initially")
	}

	// 2. Attach debugger
	if err := AttachDebugger(tmpDir); err != nil {
		t.Fatalf("AttachDebugger failed: %v", err)
	}
	if !IsDebuggerAttached(tmpDir) {
		t.Fatal("expected debugger to BE attached after AttachDebugger")
	}

	// 3. Detach debugger
	if err := DetachDebugger(tmpDir); err != nil {
		t.Fatalf("DetachDebugger failed: %v", err)
	}
	if IsDebuggerAttached(tmpDir) {
		t.Fatal("expected debugger to NOT be attached after DetachDebugger")
	}

	// 4. Test Telemetry Endpoint
	server, err := NewDumpServer(0, 10, nil)
	if err != nil {
		t.Fatalf("failed to create dump server: %v", err)
	}
	defer server.Close()

	frame := RequestTelemetryFrame{
		ProjectID:  "my-app",
		Method:     "POST",
		URL:        "/api/checkout",
		Status:     200,
		DurationMs: 45.2,
		Queries: []TelemetryQuery{
			{SQL: "SELECT * FROM users WHERE id = 1", TimeMs: 2.1, Connection: "mysql"},
		},
		Logs: []TelemetryLog{
			{Level: "info", Message: "Order created successfully"},
		},
	}

	body, _ := json.Marshal(frame)
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/telemetry", server.Port()), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to post telemetry: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	frames := server.GetRecentFrames("my-app")
	if len(frames) != 1 {
		t.Fatalf("expected 1 telemetry frame, got %d", len(frames))
	}
	if frames[0].URL != "/api/checkout" || len(frames[0].Queries) != 1 {
		t.Fatalf("unexpected frame content: %+v", frames[0])
	}

	server.SetDumpsInHtml(true)
	if !server.GetDumpsInHtml() {
		t.Fatal("expected dumpsInHtml to be true")
	}
}
