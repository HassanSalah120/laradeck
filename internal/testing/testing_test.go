package testing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestExecuteAPIRequestAndAssertions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom-Header", "HerdWorkbench")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","message":"pong"}`))
	}))
	defer server.Close()

	tc := NewTestCoordinator()
	ctx := context.Background()

	spec := APIRequestSpec{
		ID:     "req1",
		Name:   "Ping Check",
		Method: "GET",
		URL:    server.URL,
		Assertions: []APIAssertion{
			{Type: "status_code", Value: "200"},
			{Type: "body_contains", Value: "pong"},
			{Type: "header_equals", Target: "X-Custom-Header", Value: "HerdWorkbench"},
			{Type: "max_latency", Value: "5000"},
		},
	}

	res, err := tc.ExecuteAPIRequest(ctx, spec)
	if err != nil {
		t.Fatalf("ExecuteAPIRequest failed: %v", err)
	}
	if !res.Passed {
		t.Fatalf("expected all assertions to pass, got %+v", res.AssertionResults)
	}
	if res.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", res.StatusCode)
	}

	// Test boundary case generation
	boundaryCases := tc.GenerateBoundaryCases(spec)
	if len(boundaryCases) < 4 {
		t.Errorf("expected at least 4 boundary cases, got %d", len(boundaryCases))
	}
}

func TestUnknownAssertionFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	tc := NewTestCoordinator()
	ctx := context.Background()

	spec := APIRequestSpec{
		ID:     "req_unknown",
		Method: "GET",
		URL:    server.URL,
		Assertions: []APIAssertion{
			{Type: "unknown_assertion_type_typo", Value: "whatever"},
		},
	}

	res, err := tc.ExecuteAPIRequest(ctx, spec)
	if err != nil {
		t.Fatalf("ExecuteAPIRequest error: %v", err)
	}
	if res.Passed {
		t.Errorf("expected unknown assertion type to cause Passed=false, but got true")
	}
	if len(res.AssertionResults) != 1 || res.AssertionResults[0].Passed {
		t.Errorf("expected assertion result to fail for unknown type")
	}
}

func TestMutationLoadTestGuard(t *testing.T) {
	tc := NewTestCoordinator()
	ctx := context.Background()

	// Mutating request without allowMutation must fail
	spec := LoadTestSpec{
		TargetURL:       "http://127.0.0.1:8000/api/users",
		Method:          "POST",
		VirtualUsers:    2,
		DurationSeconds: 1,
		AllowMutation:   false,
	}

	_, err := tc.RunLoadTest(ctx, spec)
	if err == nil {
		t.Fatalf("expected error for mutating load test without allowMutation, got nil")
	}

	// With allowMutation=true and test server with POST body
	var receivedBody atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if string(b) == `{"name":"test"}` {
			receivedBody.Store(true)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	spec.TargetURL = server.URL
	spec.AllowMutation = true
	spec.Body = `{"name":"test"}`
	spec.Headers = map[string]string{"Content-Type": "application/json"}

	res, err := tc.RunLoadTest(ctx, spec)
	if err != nil {
		t.Fatalf("RunLoadTest with allowMutation failed: %v", err)
	}
	if !receivedBody.Load() {
		t.Errorf("expected server to receive POST body")
	}
	if res.Successful == 0 {
		t.Errorf("expected successful requests for StatusCreated")
	}
}

func TestLoadTestFailureRateAttribution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("error"))
	}))
	defer server.Close()

	tc := NewTestCoordinator()
	ctx := context.Background()

	spec := LoadTestSpec{
		TargetURL:       server.URL,
		Method:          "GET",
		VirtualUsers:    2,
		DurationSeconds: 1,
	}

	res, err := tc.RunLoadTest(ctx, spec)
	if err != nil {
		t.Fatalf("RunLoadTest failed: %v", err)
	}
	if res.Failed == 0 {
		t.Errorf("expected failures for 500 InternalServerError, got 0")
	}
	if res.Successful != 0 {
		t.Errorf("expected 0 successful requests for 500 error, got %d", res.Successful)
	}
}

func TestLoadTestCancellation(t *testing.T) {
	tc := NewTestCoordinator()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	spec := LoadTestSpec{
		TargetURL:       "http://127.0.0.1:9999",
		Method:          "GET",
		VirtualUsers:    2,
		DurationSeconds: 5,
		Engine:          "native",
	}

	_, err := tc.RunLoadTest(ctx, spec)
	if err == nil {
		t.Fatalf("expected error on cancelled context, got nil")
	}
}

func TestLoadTestExplicitK6Missing(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	tc := NewTestCoordinator()
	spec := LoadTestSpec{
		TargetURL:       "http://127.0.0.1:9999",
		Method:          "GET",
		VirtualUsers:    2,
		DurationSeconds: 1,
		Engine:          "k6",
	}
	_, err := tc.RunLoadTest(context.Background(), spec)
	if err != nil && !strings.Contains(err.Error(), "k6") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDeleteBodyAndCaseInsensitiveHeaderAssertion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != "DELETE" || string(body) != "payload" {
			t.Error("DELETE request body lost")
		}
		w.Header().Set("X-Probe", "yes")
		w.Write([]byte("ok"))
	}))
	defer server.Close()
	result, err := NewTestCoordinator().ExecuteAPIRequest(context.Background(), APIRequestSpec{Method: "DELETE", URL: server.URL, Body: "payload", Assertions: []APIAssertion{{Type: "header_equals", Target: "x-probe", Value: "yes"}}})
	if err != nil || !result.Passed {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
func TestTruncatedAPIResponseCannotPass(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(strings.Repeat("x", 600*1024))) }))
	defer server.Close()
	result, err := NewTestCoordinator().ExecuteAPIRequest(context.Background(), APIRequestSpec{URL: server.URL})
	if err != nil || result.Passed || !result.Truncated {
		t.Fatalf("result truncated=%v passed=%v err=%v", result.Truncated, result.Passed, err)
	}
}

func TestLiveK6Contract(t *testing.T) {
	executable := os.Getenv("WORKBENCH_K6")
	if executable == "" {
		t.Skip("set WORKBENCH_K6 and WORKBENCH_RUNNER to verify actual k6")
	}
	t.Setenv("PATH", filepath.Dir(executable)+string(os.PathListSeparator)+os.Getenv("PATH"))
	var received atomic.Int64
	body := `{"quote":"single' and double\""}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || string(data) != body || r.Header.Get("X-Probe") != "fixture" {
			t.Error("k6 changed request contract")
		}
		received.Add(1)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("ok"))
	}))
	defer server.Close()
	result, err := NewTestCoordinator().RunLoadTest(context.Background(), LoadTestSpec{Engine: "k6", TargetURL: strings.Replace(server.URL, "127.0.0.1", "workbench-load-fixture.test", 1) + "/probe?quote='", Method: "POST", Body: body, Headers: map[string]string{"X-Probe": "fixture"}, AllowMutation: true, VirtualUsers: 1, DurationSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Engine != "k6" || result.TotalRequests == 0 || result.TotalRequests != int(received.Load()) || result.Failed != 0 || result.Successful != result.TotalRequests || result.P99LatencyMs <= 0 {
		t.Fatalf("unexpected k6 metrics: %+v", result)
	}
	t.Logf("Actual k6: %d POSTs, intact quoted URL/body/headers, p99=%.3fms", result.TotalRequests, result.P99LatencyMs)
}
