package performance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectCheckup(t *testing.T) {
	tmp := t.TempDir()

	// Simulate bootstrap/cache/config.php
	cfgDir := filepath.Join(tmp, "bootstrap", "cache")
	_ = os.MkdirAll(cfgDir, 0755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.php"), []byte("<?php return [];"), 0644)

	rep := RunProjectCheckup(context.Background(), tmp, "")
	if len(rep.Items) == 0 {
		t.Fatal("expected checkup items")
	}

	foundConfig := false
	for _, item := range rep.Items {
		if item.ID == "config_cache" {
			foundConfig = true
			if item.Status != "ok" {
				t.Fatalf("expected config cache status ok, got %s", item.Status)
			}
		}
	}
	if !foundConfig {
		t.Fatal("config_cache check not found in report")
	}
}

func TestRunLoadTest(t *testing.T) {
	count := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	res, err := RunLoadTest(context.Background(), LoadTestSpec{
		URL:         ts.URL,
		Concurrency: 2,
		DurationSec: 1,
		MaxRequests: 10,
	})
	if err != nil {
		t.Fatalf("load test failed: %v", err)
	}

	if res.TotalRequests == 0 {
		t.Fatal("expected at least 1 request")
	}
	if res.Successful == 0 {
		t.Fatal("expected successful requests")
	}
	if res.RPS <= 0 {
		t.Fatalf("expected positive RPS, got %f", res.RPS)
	}
}
