package quality

import (
	"testing"
)

func TestNormalizeSQLAndNPlusOne(t *testing.T) {
	q1 := "SELECT * FROM users WHERE id = 1"
	q2 := "SELECT * FROM users WHERE id = 2"
	q3 := "SELECT * FROM users WHERE id = 3"
	qOther := "SELECT * FROM orders WHERE status = 'pending'"

	fp1 := NormalizeSQL(q1)
	fp2 := NormalizeSQL(q2)
	if fp1 != fp2 {
		t.Errorf("expected fingerprints to match, got %q vs %q", fp1, fp2)
	}

	queries := []struct {
		SQL      string
		Duration float64
	}{
		{SQL: q1, Duration: 1.2},
		{SQL: q2, Duration: 1.1},
		{SQL: q3, Duration: 1.4},
		{SQL: qOther, Duration: 2.5},
	}

	cands := DetectNPlusOne(queries)
	if len(cands) != 1 {
		t.Fatalf("expected 1 N+1 candidate group, got %d", len(cands))
	}
	if cands[0].Count != 3 {
		t.Errorf("expected count 3, got %d", cands[0].Count)
	}
	if !cands[0].IsNPlusOne {
		t.Errorf("expected IsNPlusOne to be true for count >= 3")
	}
}

func TestAuditConfiguration(t *testing.T) {
	envVals := map[string]string{
		"APP_DEBUG":   "true",
		"APP_KEY":     "",
		"CACHE_STORE": "redis",
	}
	phpVals := map[string]string{
		"opcache.enable": "0",
		"memory_limit":   "512M",
	}

	items := AuditConfiguration(envVals, phpVals)
	if len(items) != 5 {
		t.Fatalf("expected 5 audit items, got %d", len(items))
	}

	// APP_KEY missing should be warning
	var appKeyItem *ConfigAuditItem
	for _, it := range items {
		if it.Key == "APP_KEY" {
			appKeyItem = &it
		}
	}
	if appKeyItem == nil || appKeyItem.Status != "warning" {
		t.Errorf("expected APP_KEY to have warning status, got %+v", appKeyItem)
	}
}
