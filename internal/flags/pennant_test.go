package flags

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanFlags(t *testing.T) {
	tmp := t.TempDir()

	// 1. Write composer.json with pennant
	composer := `{
		"require": {
			"laravel/pennant": "^1.0"
		}
	}`
	if err := os.WriteFile(filepath.Join(tmp, "composer.json"), []byte(composer), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Write app/Providers/AppServiceProvider.php with Feature::define
	appDir := filepath.Join(tmp, "app", "Providers")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatal(err)
	}
	providerCode := `<?php
namespace App\Providers;
use Laravel\Pennant\Feature;

class AppServiceProvider {
    public function boot() {
        Feature::define('beta-billing', fn () => true);
        Feature::define('dark-mode', fn () => false);
    }
}
`
	if err := os.WriteFile(filepath.Join(appDir, "AppServiceProvider.php"), []byte(providerCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Write resources/views/welcome.blade.php with @feature
	viewsDir := filepath.Join(tmp, "resources", "views")
	if err := os.MkdirAll(viewsDir, 0755); err != nil {
		t.Fatal(err)
	}
	bladeCode := `@feature('beta-billing')
    <div>Beta Billing Active</div>
@endfeature
`
	if err := os.WriteFile(filepath.Join(viewsDir, "welcome.blade.php"), []byte(bladeCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Run Scan
	ctx := context.Background()
	rep := ScanFlags(ctx, tmp)

	if !rep.HasPennant {
		t.Errorf("expected HasPennant to be true")
	}

	if rep.TotalFlags != 2 {
		t.Fatalf("expected 2 flags, got %d", rep.TotalFlags)
	}

	foundBeta := false
	for _, fl := range rep.Flags {
		if fl.Name == "beta-billing" {
			foundBeta = true
			if fl.DefinedIn != "app/Providers/AppServiceProvider.php" {
				t.Errorf("unexpected DefinedIn: %s", fl.DefinedIn)
			}
			if len(fl.Locations) != 1 || fl.Locations[0] != "resources/views/welcome.blade.php" {
				t.Errorf("unexpected Locations: %v", fl.Locations)
			}
		}
	}

	if !foundBeta {
		t.Errorf("flag beta-billing was not detected")
	}
}
