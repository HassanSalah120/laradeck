package health

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHeartbeat_NonExistentDir(t *testing.T) {
	ctx := context.Background()
	rep := ProbeHeartbeat(ctx, "", filepath.Join(os.TempDir(), "non_existent_laradeck_dir_12345"))

	if rep.HealthScore >= 100 {
		t.Fatalf("expected degraded or failing score for non-existent dir, got %d", rep.HealthScore)
	}
	if len(rep.Components) == 0 {
		t.Fatalf("expected components in report")
	}
}

func TestHeartbeat_WritableDir(t *testing.T) {
	tmp := t.TempDir()
	storage := filepath.Join(tmp, "storage")
	cache := filepath.Join(tmp, "bootstrap", "cache")
	if err := os.MkdirAll(storage, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cache, 0755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	rep := ProbeHeartbeat(ctx, "", tmp)

	var storageComp *ComponentHealth
	for i := range rep.Components {
		if rep.Components[i].Component == "storage" {
			storageComp = &rep.Components[i]
			break
		}
	}

	if storageComp == nil {
		t.Fatalf("expected storage component")
	}
	if storageComp.Status != "healthy" {
		t.Fatalf("expected healthy storage component, got %s", storageComp.Status)
	}
}
