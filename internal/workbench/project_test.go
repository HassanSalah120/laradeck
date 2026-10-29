package workbench

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDependencyMismatchAndMissingVendor(t *testing.T) {
	p := Project{Path: t.TempDir()}
	os.WriteFile(filepath.Join(p.Path, "composer.lock"), []byte(`{"packages":[{"name":"laravel/framework","version":"v12.70.0"}],"packages-dev":[{"name":"itsgoingd/clockwork","version":"v5.3.5"}]}`), 0600)
	InspectDependencies(&p)
	if p.Framework != "v12.70.0" || p.VendorPresent || p.Clockwork != "v5.3.5 (locked)" {
		t.Fatalf("lock-only state: %+v", p)
	}
	os.MkdirAll(filepath.Join(p.Path, "vendor", "composer"), 0700)
	os.WriteFile(filepath.Join(p.Path, "vendor", "autoload.php"), []byte("<?php"), 0600)
	os.WriteFile(filepath.Join(p.Path, "vendor", "composer", "installed.json"), []byte(`{"packages":[{"name":"laravel/framework","version":"v12.69.0"}]}`), 0600)
	InspectDependencies(&p)
	if p.DependencyState != "Lockfile/vendor mismatch" || p.Clockwork != "not installed" {
		t.Fatalf("mismatch missed: %+v", p)
	}
}
