package workbench

import (
	"context"
	"encoding/json"
	"herdworkbench/internal/companion"
	"herdworkbench/internal/runner"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPackageResponsesAndIncompleteCoverage(t *testing.T) {
	cases := []struct {
		manager, kind, body string
		code                int
		status              string
		issues              int
	}{
		{"npm", "outdated", `{"vite":{"current":"6.0.0","wanted":"6.4.0","latest":"7.0.0"}}`, 1, "attention", 1},
		{"npm", "outdated", `{}`, 0, "ready", 0},
		{"npm", "outdated", `{}`, 1, "error", 0},
		{"npm", "audit", `{"error":{"code":"ENOAUDIT"}}`, 1, "error", 0},
		{"npm", "audit", `{"vulnerabilities":{"example":{"severity":"high","range":"<2"}}}`, 1, "attention", 1},
		{"npm", "audit", `{"vulnerabilities":{}}`, 0, "ready", 0},
		{"npm", "audit", `{"vulnerabilities":null}`, 0, "error", 0},
		{"npm", "installed", `{"name":"fixture","problems":["missing"],"error":{"code":"ELSPROBLEMS"}}`, 1, "attention", 0},
		{"composer", "outdated", `{"locked":[{"name":"a/b","version":"v1.0.0","latest":"v2.0.0"}]}`, 0, "attention", 1},
		{"composer", "outdated", `[]`, 0, "ready", 0},
		{"composer", "audit", `{"advisories":[],"abandoned":[]}`, 0, "ready", 0},
		{"composer", "audit", `{"advisories":[],"unreachable-repositories":["https://example.invalid"]}`, 0, "error", 0},
		{"composer", "audit", `{"advisories":[],"filter":{"a/b":[{}]},"cooldown":[]}`, 0, "attention", 1},
		{"composer", "audit", `{"advisories":{},"abandoned":{"a/b":null}}`, 2, "attention", 1},
		{"composer", "audit", `{"advisories":{"a/b":[{"title":"Fixture advisory","severity":"critical","affectedVersions":"<2"}]},"abandoned":[]}`, 1, "attention", 1},
		{"composer", "platform", `[{"name":"ext-missing","status":"missing"}]`, 1, "attention", 1},
		{"composer", "audit", `{"unexpected":"schema"}`, 0, "error", 0},
		{"npm", "outdated", `{truncated`, 0, "error", 0},
	}
	for _, v := range cases {
		t.Run(v.manager+"/"+v.kind+"/"+v.body, func(t *testing.T) {
			r := parsePackageCheck(v.manager, v.kind, v.body, v.code)
			if r.Status != v.status || len(r.Issues) != v.issues {
				t.Fatalf("got %s/%d want %s/%d", r.Status, len(r.Issues), v.status, v.issues)
			}
		})
	}
	m := PackageManagerHealth{Present: true, Checks: []PackageCheck{packageCheck("registry", "error", "offline")}}
	if summarizePackages(m, true, false).Status == "ready" {
		t.Fatal("offline check claimed healthy")
	}
	m.Checks = []PackageCheck{packageCheck("audit", "ready", "passed")}
	if summarizePackages(m, true, true).Status == "ready" {
		t.Fatal("stale report claimed healthy")
	}
	m.Checks[0].Issues = []PackageIssue{{Severity: "critical"}}
	if summarizePackages(m, true, false).Status != "danger" {
		t.Fatal("critical advisory lost")
	}
}

func TestPackageInventoryAndFreshness(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(path), 0700)
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("composer.json", `{"require":{"a/b":"^1"},"require-dev":{"c/d":"^2"}}`)
	write("composer.lock", `{"packages":[{"name":"a/b","version":"v1.2.0"}],"packages-dev":[{"name":"c/d","version":"2.0.0"}]}`)
	write("vendor/composer/installed.json", `{"packages":[{"name":"a/b","version":"v1.2.0"}]}`)
	write("package.json", `{"name":"fixture","dependencies":{"@scope/demo":"^1.0.0"}}`)
	write("package-lock.json", `{"lockfileVersion":3,"packages":{"":{"dependencies":{"@scope/demo":"^1.0.0"}},"node_modules/@scope/demo":{"version":"1.2.0"}}}`)
	write("node_modules/@scope/demo/package.json", `{"version":"1.2.0"}`)
	r := inspectPackages(root)
	if len(r.Composer.Packages) != 2 || r.Composer.Checks[1].Status != "attention" {
		t.Fatal("missing dev dependency was hidden")
	}
	if r.NPM.Checks[0].Status != "ready" || r.NPM.Packages[0].Installed != "1.2.0" {
		t.Fatal("scoped package inventory/lock failed")
	}
	b := NewBackend(nil)
	t.Cleanup(b.Shutdown)
	b.companionStore = companion.NewStore(t.TempDir())
	p := Project{ID: "packages", Path: root}
	b.projects = []Project{p}
	if _, err := b.Companion(p.ID, "package-scan", nil); err == nil {
		t.Fatal("untrusted package process execution")
	}
	r.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	r.Online = true
	if err := b.companionStore.Save(p.ID, "packages", r); err != nil {
		t.Fatal(err)
	}
	if b.packageHealth(p).CheckedAt == "" {
		t.Fatal("matching report not retained")
	}
	write("package.json", `{"name":"fixture","dependencies":{"@scope/demo":"^2.0.0"}}`)
	updated := b.packageHealth(p)
	if updated.CheckedAt != "" || updated.NPM.Checks[0].Status != "attention" {
		t.Fatal("changed manifest reused successful report")
	}
	if displayConstraint("https://example.invalid/token@repo") != "External source (details hidden)" {
		t.Fatal("source URL exposed")
	}
}

// Uses real CLIs with an empty disposable dependency tree, never a user's project.
func TestLivePackageLocalChecks(t *testing.T) {
	if os.Getenv("WORKBENCH_RUNNER") == "" {
		t.Skip("set WORKBENCH_RUNNER for real Windows process checks")
	}
	php, err := exec.LookPath("php.exe")
	if err != nil {
		t.Skip("PHP unavailable")
	}
	if herd, e := FindHerd(); e == nil && exists(herd.PHP) {
		php = herd.PHP
	}
	b := NewBackend(nil)
	t.Cleanup(b.Shutdown)
	b.companionStore = companion.NewStore(t.TempDir())
	tool, err := b.dependencies.Resolve("composer")
	if err != nil {
		t.Skip("Composer unavailable")
	}
	root := t.TempDir()
	manifest := `{"name":"fixture/packages","description":"Disposable dependency health fixture","license":"MIT","repositories":[{"type":"package","package":{"name":"fixture/metapackage","version":"1.0.0","type":"metapackage"}},{"packagist.org":false}],"require":{"php":">=8.0","fixture/metapackage":"^1.0"},"scripts":{"post-update-cmd":"@php -r \"file_put_contents('script-ran','unsafe');\""}}`
	os.WriteFile(filepath.Join(root, "composer.json"), []byte(manifest), 0600)
	spec := runner.Spec{Executable: php, Directory: root, Env: []string{"COMPOSER_DISABLE_NETWORK=1"}, Args: []string{tool, "--no-plugins", "--no-scripts", "--no-interaction", "update", "--no-install", "--no-audit"}}
	if res, e := runner.CaptureExecContext(context.Background(), spec, 30*time.Second); e != nil || res.ExitCode != 0 {
		t.Fatalf("fixture lock generation failed: %v %s", e, Redact(res.Combined))
	}
	os.MkdirAll(filepath.Join(root, "vendor/composer"), 0700)
	os.WriteFile(filepath.Join(root, "vendor/composer/installed.json"), []byte(`{"packages":[{"name":"fixture/metapackage","version":"1.0.0"}]}`), 0600)
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"packages-fixture","version":"1.0.0","dependencies":{},"scripts":{"preinstall":"node -e \"require('fs').writeFileSync('script-ran','unsafe')\""}}`), 0600)
	os.WriteFile(filepath.Join(root, "package-lock.json"), []byte(`{"name":"packages-fixture","version":"1.0.0","lockfileVersion":3,"packages":{"":{"name":"packages-fixture","version":"1.0.0","dependencies":{}}}}`), 0600)
	p := Project{ID: "live-packages", Path: root, PHPBinary: php}
	b.projects = []Project{p}
	b.trusted[p.ID] = true
	before := packageRevision(root)
	value, err := b.Companion(p.ID, "package-scan", map[string]any{"online": false})
	if err != nil {
		t.Fatal(err)
	}
	r := value.(PackageHealth)
	for name, m := range map[string]PackageManagerHealth{"Composer": r.Composer, "npm": r.NPM} {
		for _, c := range m.Checks {
			if c.Name != "Registry checks" && c.Status != "ready" {
				t.Fatalf("%s %s: %s %s", name, c.Name, c.Status, c.Detail)
			}
		}
	}
	if exists(filepath.Join(root, "script-ran")) || packageRevision(root) != before {
		t.Fatal("local checks mutated project or executed scripts")
	}
	if os.Getenv("WORKBENCH_PACKAGE_ONLINE") == "1" {
		value, e := b.Companion(p.ID, "package-scan", map[string]any{"online": true})
		if e != nil {
			t.Fatal(e)
		}
		online := value.(PackageHealth)
		for name, m := range map[string]PackageManagerHealth{"Composer": online.Composer, "npm": online.NPM} {
			for _, c := range m.Checks {
				if c.Status != "ready" {
					if name == "Composer" {
						debug := runner.Spec{Executable: php, Directory: root, Args: []string{tool, "--no-plugins", "--no-scripts", "--no-interaction", c.Name, "--locked", "--format=json"}}
						if res, e := runner.CaptureExecContext(context.Background(), debug, 30*time.Second); e == nil {
							t.Logf("Disposable fixture response: %s", Redact(res.Combined))
						}
					}
					t.Fatalf("real registry %s/%s: %s (%s)", name, c.Name, c.Status, c.Detail)
				}
			}
		}
		if exists(filepath.Join(root, "script-ran")) || packageRevision(root) != before {
			t.Fatal("registry checks mutated project or executed scripts")
		}
		t.Log("Actual disposable Composer/npm outdated + audit responses parsed successfully")
	}
	data, _ := json.Marshal(r)
	if !json.Valid(data) {
		t.Fatal("public report invalid")
	}
	t.Log("Real Composer validation/platform checks + npm ls passed; manifests/locks unchanged, lifecycle scripts not executed, registry checks explicitly incomplete")
}
