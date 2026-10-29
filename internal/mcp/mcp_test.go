package mcp

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMCPServerToolsList(t *testing.T) {
	srv := NewServer()
	t.Cleanup(srv.Close)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	resp := srv.HandleRequest(req)
	if resp.Error != nil {
		t.Fatalf("tools/list failed: %+v", resp.Error)
	}

	resMap, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	tools, ok := resMap["tools"].([]MCPTool)
	if !ok {
		t.Fatalf("expected []MCPTool, got %T", resMap["tools"])
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
	}

	expected := []string{
		"workbench_list_projects",
		"workbench_get_snapshot",
		"workbench_get_env_drift",
		"workbench_audit_project",
		"workbench_get_routes",
		"workbench_get_services",
		"workbench_get_errors",
	}
	for _, exp := range expected {
		if !names[exp] {
			t.Errorf("tool %q missing from tools/list", exp)
		}
	}
}

func TestMCPTool_InvalidProjectFails(t *testing.T) {
	srv := NewServer()
	t.Cleanup(srv.Close)

	// Calling workbench_get_routes for a nonexistent project must fail with an error, not status: ok
	callArgs, _ := json.Marshal(map[string]any{
		"name": "workbench_get_routes",
		"arguments": map[string]any{
			"project_id": "does-not-exist",
		},
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  callArgs,
	}

	resp := srv.HandleRequest(req)
	if resp.Error == nil {
		t.Fatalf("expected error when calling workbench_get_routes on nonexistent project, got success: %+v", resp.Result)
	}

	if strings.TrimSpace(resp.Error.Message) == "" {
		t.Error("tool failure lacks an actionable error")
	}
}

func TestMCPTool_AuditAndServices(t *testing.T) {
	srv := NewServer()
	t.Cleanup(srv.Close)

	// Services tool call
	servArgs, _ := json.Marshal(map[string]any{
		"name":      "workbench_get_services",
		"arguments": map[string]any{},
	})
	resp := srv.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  servArgs,
	})
	if resp.Error != nil {
		t.Fatalf("workbench_get_services failed: %+v", resp.Error)
	}

	// Audit tool call on invalid path
	auditArgs, _ := json.Marshal(map[string]any{
		"name": "workbench_audit_project",
		"arguments": map[string]any{
			"project_id": "nonexistent-dir",
		},
	})
	auditResp := srv.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  auditArgs,
	})
	if auditResp.Error == nil {
		t.Fatalf("expected error on nonexistent project audit, got success")
	}
}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "workbench-mcp-test-")
	if err != nil {
		panic(err)
	}
	helper := filepath.Join(dir, "workbench-runner.exe")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go.exe"), "build", "-o", helper, "../../cmd/runner")
	if out, err := cmd.CombinedOutput(); err != nil {
		panic(string(out))
	}
	os.Setenv("WORKBENCH_RUNNER", helper)
	os.Setenv("APPDATA", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
func TestHandshakeAndNotifications(t *testing.T) {
	srv := NewServer()
	defer srv.Close()
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"ping"}
`
	var out bytes.Buffer
	if err := srv.Serve(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected two responses, got %s", out.String())
	}
	var response JSONRPCResponse
	if err := json.Unmarshal([]byte(lines[0]), &response); err != nil || response.Error != nil {
		t.Fatalf("handshake: %s", lines[0])
	}
	result := response.Result.(map[string]any)
	if result["protocolVersion"] != "2025-11-25" || result["capabilities"] == nil {
		t.Fatal(result)
	}
}
