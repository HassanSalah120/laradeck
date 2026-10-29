package apphost

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type mockAPI struct{}

func (m *mockAPI) Add(a, b int) int {
	return a + b
}

func (m *mockAPI) Echo(msg string) string {
	return "echo: " + msg
}

func TestHostLifecycleAndCall(t *testing.T) {
	memFS := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html><body>Dashboard</body></html>")},
	}

	api := &mockAPI{}
	host, err := Start(api, nil, memFS)
	if err != nil {
		t.Fatalf("failed to start apphost: %v", err)
	}
	defer host.Close(context.Background())

	if !strings.HasPrefix(host.URL, "http://127.0.0.1:") {
		t.Errorf("expected URL on loopback, got %s", host.URL)
	}
	launchURL := host.LaunchURL()
	if !strings.Contains(launchURL, "#session="+host.token) {
		t.Errorf("launch URL missing session token: %s", launchURL)
	}

	// 1. Unauthenticated call should fail with 401
	client := &http.Client{Timeout: 3 * time.Second}
	callPayload, _ := json.Marshal(map[string]any{
		"method": "Add",
		"args":   []int{2, 3},
	})

	req, err := http.NewRequest("POST", host.URL+"/api/call", bytes.NewReader(callPayload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated call, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Authenticated call via Bearer token
	req, err = http.NewRequest("POST", host.URL+"/api/call", bytes.NewReader(callPayload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+host.token)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for authenticated call, got %d", resp.StatusCode)
	}
	var res struct {
		Result int `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if res.Result != 5 {
		t.Errorf("expected result 5, got %d", res.Result)
	}

	// 3. Static asset serving
	req, err = http.NewRequest("GET", host.URL+"/index.html", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for static asset, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
