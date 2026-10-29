package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Exercises the production GUI-subsystem binary in its real headless modes.
func TestPackagedHeadlessModes(t *testing.T) {
	binary := os.Getenv("WORKBENCH_PACKAGED")
	if binary == "" {
		t.Skip("set WORKBENCH_PACKAGED to verify compiled executable")
	}
	t.Setenv("APPDATA", t.TempDir())
	run := func(input string, args ...string) []byte {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
		cmd.Stdin = strings.NewReader(input)
		data, err := cmd.Output()
		if err != nil {
			t.Fatalf("packaged mode %v failed: %v", args, err)
		}
		return data
	}
	if data := run("", "--cli", "version"); !strings.Contains(string(data), "Herd Workbench") {
		t.Fatal("CLI version missing")
	}
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"workbench_list_projects","arguments":{}}}
`
	data := run(input, "--mcp")
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("MCP output should contain exactly three protocol responses, got %d", len(lines))
	}
	for _, line := range lines {
		var response struct {
			JSONRPC string          `json:"jsonrpc"`
			Error   any             `json:"error"`
			Result  json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &response); err != nil || response.JSONRPC != "2.0" || response.Error != nil {
			t.Fatalf("invalid MCP response, parse=%v error=%v", err, response.Error)
		}
	}
	fixture := os.Getenv("WORKBENCH_LIVE_FIXTURE")
	if fixture != "" {
		var listing struct {
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(lines[2]), &listing); err != nil || len(listing.Result.Content) == 0 {
			t.Fatal("project listing is not MCP content")
		}
		var projects []Project
		if err := json.Unmarshal([]byte(listing.Result.Content[0].Text), &projects); err != nil {
			t.Fatal(err)
		}
		var fixtureID string
		for _, project := range projects {
			if strings.EqualFold(filepath.Clean(project.Path), filepath.Clean(fixture)) {
				fixtureID = project.ID
			}
		}
		if fixtureID == "" {
			t.Fatal("production discovery omitted fixture")
		}
		call := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"workbench_get_routes","arguments":{"project_id":%q}}}`+"\n", fixtureID)
		denied := run(call, "--mcp")
		if !strings.Contains(string(denied), "Trust this project") {
			t.Fatal("MCP route execution did not enforce desktop trust")
		}
		profile := filepath.Join(os.Getenv("APPDATA"), "HerdWorkbench", "settings.json")
		trusted, _ := json.Marshal(map[string]any{"version": 1, "trusted": map[string]bool{fixtureID: true}})
		if err := os.WriteFile(profile, trusted, 0600); err != nil {
			t.Fatal(err)
		}
		trustedResponse := run(call, "--mcp")
		var routeResult struct {
			Error  any `json:"error"`
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal(trustedResponse, &routeResult); err != nil || routeResult.Error != nil || len(routeResult.Result.Content) == 0 {
			t.Fatalf("trusted MCP route call failed: parse=%v error=%v response=%s", err, routeResult.Error, Redact(string(trustedResponse)))
		}
		var routes []Route
		if err := json.Unmarshal([]byte(routeResult.Result.Content[0].Text), &routes); err != nil || len(routes) < 4 {
			t.Fatal("trusted MCP route response is not real route data")
		}
		t.Logf("Production MCP: discovered project ID round-trip, untrusted denial and %d trusted routes verified", len(routes))
		audit := run("", "--cli", "audit", "--project", fixture, "--json")
		var records []map[string]any
		if err := json.Unmarshal(audit, &records); err != nil || len(records) == 0 {
			t.Fatal("packaged audit JSON invalid")
		}
		report := run("", "--cli", "report", "--project", fixture)
		if !strings.Contains(string(report), "## Requirements") || strings.Contains(string(report), "Standard VPS") {
			t.Fatal("packaged report missing or contains invented sizing")
		}
	}
	t.Log("Production executable: CLI version/audit/report and MCP initialize/notifications/tools/discovery verified")
}
