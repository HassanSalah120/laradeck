package workbench

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMetadataRetryAndPrivacy(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Companion-Internal") != "1" {
			t.Error("internal marker absent")
		}
		if calls.Add(1) == 1 {
			w.Write([]byte(`{"id":`))
			return
		}
		w.Write([]byte(`{"id":"capture-1","uri":"/orders?token=private","responseStatus":200,"databaseQueries":[{"query":"select * from users where email = 'private@example.test'","duration":1},{"query":"select * from users where email = 'second@example.test'","duration":2}],"headers":{"authorization":"private"},"log":[{"exception":{"message":"private"}}]}`))
	}))
	defer server.Close()
	var raw rawRequest
	if err := fetchMetadata(context.Background(), server.URL, "latest", &raw); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
	safe := cleanRequest(raw)
	data, _ := json.Marshal(safe)
	if strings.Contains(string(data), "private") || strings.Contains(string(data), "second@") {
		t.Fatalf("private field imported: %s", data)
	}
	if safe.URI != "/orders" || safe.Exceptions != 1 || !safe.Queries[0].Repeated {
		t.Fatalf("wrong summary: %+v", safe)
	}
}

func TestMetadataRejectsRemoteAndRedirect(t *testing.T) {
	for _, base := range []string{"https://example.com", "file:///tmp/data", "http://user:pass@localhost"} {
		if _, err := localBase(base); err == nil {
			t.Errorf("accepted %s", base)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://example.com", 302) }))
	defer server.Close()
	var result rawRequest
	if err := fetchMetadata(context.Background(), server.URL, "latest", &result); err == nil {
		t.Fatal("redirect accepted")
	}
}

func TestDirectoryWatchCoalescesAndIgnoresIndexMaintenance(t *testing.T) {
	var reads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		if strings.Contains(r.URL.Path, "previous") {
			w.Write([]byte(`[]`))
			return
		}
		w.Write([]byte(`{"id":"capture-1","uri":"/real-request","time":1}`))
	}))
	defer server.Close()
	p := Project{Path: t.TempDir(), URL: server.URL}
	dir := filepath.Join(p.Path, "storage", "clockwork")
	os.MkdirAll(dir, 0700)
	c := NewClockwork(nil)
	c.Select(p)
	if err := c.Watch(p); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	// Establish the initial snapshot before deliberately producing a burst.
	time.Sleep(100 * time.Millisecond)
	for i := 0; i < 8; i++ {
		os.WriteFile(filepath.Join(dir, "capture-1.json"), []byte(`{}`), 0600)
	}
	deadline := time.Now().Add(3 * time.Second)
	for reads.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if reads.Load() != 2 {
		t.Fatalf("expected latest + history, got %d reads", reads.Load())
	}
	for i := 0; i < 4; i++ {
		os.WriteFile(filepath.Join(dir, "index"), []byte("changed"), 0600)
		os.Remove(filepath.Join(dir, "index"))
	}
	time.Sleep(650 * time.Millisecond)
	if reads.Load() != 2 {
		t.Fatalf("index maintenance retriggered PHP: %d reads", reads.Load())
	}
	if len(c.State().Requests) != 1 {
		t.Fatal("capture not retained")
	}
}

func TestMissingClockworkIsSetupStateWithoutHTTP(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); http.NotFound(w, r) }))
	defer server.Close()
	clock := NewClockwork(nil)
	clock.Select(Project{ID: "missing", Path: t.TempDir(), URL: server.URL, Clockwork: "not installed", DependencyState: "Installed metadata matches", VendorPresent: true})
	capture, err := clock.Refresh(context.Background())
	if err != nil {
		t.Fatalf("missing optional package should be a setup state, got: %v", err)
	}
	if calls.Load() != 0 || capture.Status != "not_installed" || !strings.Contains(capture.SetupCommand, "composer require") {
		t.Fatalf("missing package was queried or installation guidance missing: calls=%d message=%s", calls.Load(), capture.Message)
	}
}

func TestClockworkFailureStates(t *testing.T) {
	for _, test := range []struct {
		name, body, status string
		code               int
		wantError          bool
	}{
		{"no captures", "null", "no_captures", 200, false},
		{"endpoint disabled", "", "endpoint_unavailable", 404, false},
		{"authentication", "", "access_denied", 403, true},
		{"html fallback", "<html>Application</html>", "invalid_response", 200, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(test.code); w.Write([]byte(test.body)) }))
			defer server.Close()
			clock := NewClockwork(nil)
			clock.Select(Project{ID: "fixture", Path: t.TempDir(), URL: server.URL, Clockwork: "v5.3.5"})
			capture, err := clock.Refresh(context.Background())
			if (err != nil) != test.wantError || capture.Status != test.status {
				t.Fatalf("status=%s err=%v", capture.Status, err)
			}
			if strings.Contains(capture.Message, "unavailable or metadata incomplete") {
				t.Fatal("ambiguous catch-all error remains")
			}
		})
	}
}
func TestRealProjectsWithoutClockworkShowSetup(t *testing.T) {
	if os.Getenv("WORKBENCH_RUN_CLOCKWORK_DISCOVERY") != "1" {
		t.Skip("opt in to local Herd discovery")
	}
	herd, err := FindHerd()
	if err != nil {
		t.Fatal(err)
	}
	projects, err := herd.Discover()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, project := range projects {
		if project.Clockwork != "not installed" || project.DependencyState != "Installed metadata matches" {
			continue
		}
		clock := NewClockwork(nil)
		clock.Select(project)
		capture, err := clock.Refresh(context.Background())
		if err != nil || capture.Status != "not_installed" {
			t.Fatalf("project %s: status=%s err=%v", project.Name, capture.Status, err)
		}
		checked++
		t.Logf("%s: missing Clockwork correctly shown as setup", project.Name)
	}
	if checked == 0 {
		t.Fatal("no missing-package projects were exercised")
	}
}
