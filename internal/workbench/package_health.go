package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"herdworkbench/internal/runner"
)

type PackageEntry struct {
	Name      string `json:"name"`
	Group     string `json:"group"`
	Requested string `json:"requested"`
	Locked    string `json:"locked"`
	Installed string `json:"installed"`
}
type PackageIssue struct {
	Name     string `json:"name"`
	Current  string `json:"current"`
	Wanted   string `json:"wanted"`
	Latest   string `json:"latest"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
}
type PackageCheck struct {
	Name   string         `json:"name"`
	Status string         `json:"status"`
	Detail string         `json:"detail"`
	Issues []PackageIssue `json:"issues"`
}
type PackageManagerHealth struct {
	Present  bool           `json:"present"`
	Checks   []PackageCheck `json:"checks"`
	Packages []PackageEntry `json:"packages"`
}
type PackageHealth struct {
	CheckedAt string               `json:"checkedAt"`
	Revision  string               `json:"revision"`
	Online    bool                 `json:"online"`
	Composer  PackageManagerHealth `json:"composer"`
	NPM       PackageManagerHealth `json:"npm"`
}
type PackageSummary struct {
	Status string `json:"status"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

func packageRevision(root string) string {
	h := sha256.New()
	for _, name := range []string{"composer.json", "composer.lock", "vendor/composer/installed.json", "package.json", "package-lock.json", "npm-shrinkwrap.json", "node_modules/.package-lock.json", "yarn.lock", "pnpm-lock.yaml", "bun.lock", ".npmrc"} {
		h.Write([]byte(name))
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			h.Write([]byte("missing"))
		} else {
			h.Write(data)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
func readPackageJSON(root, name string, target any) error {
	data, err := os.ReadFile(filepath.Join(root, name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func packageCheck(name, status, detail string) PackageCheck {
	return PackageCheck{Name: name, Status: status, Detail: detail, Issues: []PackageIssue{}}
}
func displayConstraint(value string) string {
	if strings.Contains(value, "://") || strings.HasPrefix(value, "git@") || strings.HasPrefix(value, "file:") || strings.HasPrefix(value, "link:") {
		return "External source (details hidden)"
	}
	return value
}
func (b *Backend) packageHealth(p Project) PackageHealth {
	current := inspectPackages(p.Path)
	var saved PackageHealth
	if b.companionStore.Load(p.ID, "packages", &saved) == nil && saved.Revision == current.Revision {
		return saved
	}
	return current
}

var npmPackageName = regexp.MustCompile(`^(?:@[a-zA-Z0-9._-]+/)?[a-zA-Z0-9._-]+$`)

// Static inventory never boots project PHP or executes package scripts.
func inspectPackages(root string) PackageHealth {
	r := PackageHealth{Revision: packageRevision(root)}
	r.Composer = PackageManagerHealth{Present: exists(filepath.Join(root, "composer.json")), Checks: []PackageCheck{}, Packages: []PackageEntry{}}
	r.NPM = PackageManagerHealth{Present: exists(filepath.Join(root, "package.json")), Checks: []PackageCheck{}, Packages: []PackageEntry{}}
	if r.Composer.Present {
		var manifest struct {
			Require map[string]string `json:"require"`
			Dev     map[string]string `json:"require-dev"`
		}
		var lock struct {
			Packages []struct {
				Name    string
				Version string
			}
			Dev []struct {
				Name    string
				Version string
			} `json:"packages-dev"`
		}
		if readPackageJSON(root, "composer.json", &manifest) != nil || readPackageJSON(root, "composer.lock", &lock) != nil {
			r.Composer.Checks = append(r.Composer.Checks, packageCheck("Manifest / lockfile", "attention", "composer.json or composer.lock is missing or invalid JSON."))
		} else {
			r.Composer.Checks = append(r.Composer.Checks, packageCheck("Manifest / lockfile", "not_checked", "JSON parsed; run local checks to validate the Composer schema and lock hash."))
			var installed struct {
				Packages []struct {
					Name    string
					Version string
				}
			}
			_ = readPackageJSON(root, "vendor/composer/installed.json", &installed)
			versions := map[string]string{}
			for _, p := range installed.Packages {
				versions[p.Name] = p.Version
			}
			missing := 0
			for _, p := range append(lock.Packages, lock.Dev...) {
				group, requested := "Transitive", ""
				if v, ok := manifest.Require[p.Name]; ok {
					group, requested = "Runtime", v
				}
				if v, ok := manifest.Dev[p.Name]; ok {
					group, requested = "Dev", v
				}
				r.Composer.Packages = append(r.Composer.Packages, PackageEntry{p.Name, group, displayConstraint(requested), p.Version, versions[p.Name]})
				if strings.TrimPrefix(versions[p.Name], "v") != strings.TrimPrefix(p.Version, "v") {
					missing++
				}
			}
			status, detail := "ready", "Installed package metadata matches locked versions (runtime and dev). File integrity is not checked."
			if missing > 0 {
				status, detail = "attention", fmt.Sprintf("%d locked packages are missing or differ in vendor metadata; this includes dev packages, which may be intentionally omitted.", missing)
			}
			r.Composer.Checks = append(r.Composer.Checks, packageCheck("Installed packages", status, detail))
		}
	}
	if r.NPM.Present {
		var manifest struct {
			Dependencies map[string]string
			Dev          map[string]string `json:"devDependencies"`
			Optional     map[string]string `json:"optionalDependencies"`
			Peer         map[string]string `json:"peerDependencies"`
			Manager      string            `json:"packageManager"`
		}
		var lock struct {
			Version  int `json:"lockfileVersion"`
			Packages map[string]struct {
				Version      string
				Dependencies map[string]string
				Dev          map[string]string `json:"devDependencies"`
				Optional     map[string]string `json:"optionalDependencies"`
				Peer         map[string]string `json:"peerDependencies"`
			}
		}
		lockName := "package-lock.json"
		if exists(filepath.Join(root, "npm-shrinkwrap.json")) {
			lockName = "npm-shrinkwrap.json"
		}
		if readPackageJSON(root, "package.json", &manifest) != nil {
			r.NPM.Checks = append(r.NPM.Checks, packageCheck("Manifest / lockfile", "attention", "package.json is invalid JSON."))
		} else {
			lockErr := readPackageJSON(root, lockName, &lock)
			status, detail := "not_checked", "No supported npm lockfile. Yarn, pnpm and Bun projects are not checked with npm."
			if lockErr == nil && lock.Version >= 2 {
				if entry, ok := lock.Packages[""]; ok {
					status, detail = "ready", "Root dependency declarations match the npm lockfile; nested workspace declarations and full resolution are not validated."
					for _, pair := range [][2]map[string]string{{manifest.Dependencies, entry.Dependencies}, {manifest.Dev, entry.Dev}, {manifest.Optional, entry.Optional}, {manifest.Peer, entry.Peer}} {
						if len(pair[0]) != 0 || len(pair[1]) != 0 {
							if !reflect.DeepEqual(pair[0], pair[1]) {
								status, detail = "attention", "package.json declarations differ from the npm lockfile."
							}
						}
					}
				}
			} else if lockErr != nil && exists(filepath.Join(root, lockName)) {
				status, detail = "attention", "npm lockfile is invalid or unreadable."
			}
			r.NPM.Checks = append(r.NPM.Checks, packageCheck("Manifest / lockfile", status, detail))
			seen := map[string]bool{}
			for _, group := range []struct {
				name string
				deps map[string]string
			}{{"Runtime", manifest.Dependencies}, {"Dev", manifest.Dev}, {"Optional", manifest.Optional}, {"Peer", manifest.Peer}} {
				for name, requested := range group.deps {
					if seen[name] {
						continue
					}
					seen[name] = true
					installed := ""
					if npmPackageName.MatchString(name) && name != ".." && name != "." {
						var p struct{ Version string }
						if readPackageJSON(root, filepath.Join("node_modules", filepath.FromSlash(name), "package.json"), &p) == nil {
							installed = p.Version
						}
					}
					r.NPM.Packages = append(r.NPM.Packages, PackageEntry{name, group.name, displayConstraint(requested), lock.Packages["node_modules/"+name].Version, installed})
				}
			}
			if manifest.Manager != "" && !strings.HasPrefix(manifest.Manager, "npm@") {
				r.NPM.Checks = append(r.NPM.Checks, packageCheck("Package manager", "not_checked", "Declared manager is not npm; npm commands are skipped."))
			}
		}
	}
	for _, m := range []*PackageManagerHealth{&r.Composer, &r.NPM} {
		sort.Slice(m.Packages, func(i, j int) bool { return m.Packages[i].Name < m.Packages[j].Name })
	}
	return r
}

func summarizePackages(m PackageManagerHealth, scanned, stale bool) PackageSummary {
	if !m.Present {
		return PackageSummary{"not_checked", "Not used", "No package manifest found."}
	}
	if stale {
		return PackageSummary{"not_checked", "Recheck", "Manifest, lockfile or installation metadata changed since the last check."}
	}
	attention, unknown := 0, 0
	for _, c := range m.Checks {
		for _, issue := range c.Issues {
			if issue.Severity == "high" || issue.Severity == "critical" {
				return PackageSummary{"danger", "Security review", "High or critical advisories were reported. Review Packages before deciding on remediation."}
			}
		}
		if c.Status == "attention" {
			attention++
		}
		if c.Status == "not_checked" || c.Status == "error" {
			unknown++
		}
	}
	if attention > 0 {
		return PackageSummary{"attention", fmt.Sprintf("%d attention", attention), "Review package checks and affected packages in Companion → Packages."}
	}
	if !scanned || unknown > 0 {
		return PackageSummary{"not_checked", "Incomplete", "Package checks have not all completed. Open Companion → Packages."}
	}
	return PackageSummary{"ready", "Checks passed", "Last requested package checks passed. This is a point-in-time result, not a security certification."}
}

func (b *Backend) scanPackages(ctx context.Context, p Project, online bool) (PackageHealth, error) {
	r := inspectPackages(p.Path)
	r.Online = online
	for _, manager := range []string{"composer", "npm"} {
		m := &r.Composer
		if manager == "npm" {
			m = &r.NPM
		}
		if !m.Present {
			continue
		}
		if manager == "npm" {
			skip := !exists(filepath.Join(p.Path, "package-lock.json")) && !exists(filepath.Join(p.Path, "npm-shrinkwrap.json"))
			for _, c := range m.Checks {
				if c.Name == "Package manager" {
					skip = true
				}
			}
			if skip {
				m.Checks = append(m.Checks, packageCheck("npm checks", "not_checked", "Requires an npm project with package-lock.json or npm-shrinkwrap.json. No other package manager is invoked."))
				continue
			}
		}
		checks := []string{"validate", "platform"}
		if manager == "npm" {
			checks = []string{"installed"}
		}
		if online {
			checks = append(checks, "outdated", "audit")
		} else {
			m.Checks = append(m.Checks, packageCheck("Registry checks", "not_checked", "Updates and advisories require an explicit registry check."))
		}
		for _, kind := range checks {
			if ctx.Err() != nil {
				return r, ctx.Err()
			}
			var spec runner.Spec
			var err error
			if manager == "composer" {
				args := map[string][]string{"validate": {"validate", "--strict", "--no-check-publish"}, "platform": {"check-platform-reqs", "--lock", "--format=json"}, "outdated": {"outdated", "--locked", "--format=json"}, "audit": {"audit", "--locked", "--format=json"}}[kind]
				tool, e := b.dependencies.Resolve("composer")
				err = e
				args = append([]string{"--no-plugins", "--no-scripts", "--no-interaction", "--no-ansi"}, args...)
				spec = runner.Spec{Executable: p.PHPBinary, Directory: p.Path, Args: append([]string{tool}, args...)}
				if strings.HasSuffix(strings.ToLower(tool), ".exe") {
					spec.Executable = tool
					spec.Args = args
				}
				if !online {
					spec.Env = []string{"COMPOSER_DISABLE_NETWORK=1"}
				}
			} else {
				args := map[string][]string{"installed": {"ls", "--depth=0", "--json"}, "outdated": {"outdated", "--json"}, "audit": {"audit", "--package-lock-only", "--json"}}[kind]
				args = append(args, "--ignore-scripts", "--no-progress", "--fetch-retries=0", "--fetch-timeout=20000")
				if !online {
					args = append(args, "--offline")
				}
				spec, err = npmSpec(p, args...)
			}
			if err != nil {
				m.Checks = append(m.Checks, packageCheck(kind, "error", "Package-manager executable is unavailable. Check Settings and PATH."))
				continue
			}
			res, err := runner.CaptureExecContext(ctx, spec, 45*time.Second)
			if err != nil {
				m.Checks = append(m.Checks, packageCheck(kind, "error", "Command failed, timed out, was cancelled or exceeded the output limit; no successful result inferred."))
				continue
			}
			check := parsePackageCheck(manager, kind, res.Stdout, res.ExitCode)
			if manager == "composer" && (kind == "outdated" || kind == "audit") {
				stderr := strings.ToLower(res.Stderr)
				for _, marker := range []string{"could not be fully loaded", "may be out of date", "could not be fetched", "repositories were unreachable"} {
					if strings.Contains(stderr, marker) {
						check.Status = "error"
						check.Detail = "Registry data was incomplete or stale; any listed findings remain relevant, but this check did not establish complete coverage."
						break
					}
				}
			}
			if manager == "composer" && kind == "validate" && len(m.Checks) > 0 && m.Checks[0].Status == "not_checked" {
				check.Name = "Manifest / lockfile"
				m.Checks[0] = check
			} else {
				m.Checks = append(m.Checks, check)
			}
		}
	}
	if packageRevision(p.Path) != r.Revision {
		return r, errors.New("Package files changed during the check; refresh and run again")
	}
	r.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	return r, b.companionStore.Save(p.ID, "packages", r)
}

func parsePackageCheck(manager, kind, output string, exit int) PackageCheck {
	c := packageCheck(kind, "error", "Command returned an unsupported or failed response; check package-manager compatibility and registry access. Raw command output is not retained.")
	if manager == "composer" && kind == "outdated" && exit == 0 && strings.TrimSpace(output) == "[]" {
		return packageCheck(kind, "ready", "No available updates reported for locked Composer packages.")
	}
	if manager == "composer" && kind == "validate" {
		if exit == 0 {
			c.Status = "ready"
			c.Detail = "Composer schema and lockfile validation passed (publish checks excluded)."
		} else {
			c.Status = "attention"
			c.Detail = "Composer validation reported warnings or errors. Review composer.json and composer.lock with composer validate --strict --no-check-publish."
		}
		return c
	}
	var data map[string]json.RawMessage
	if kind == "platform" {
		var requirements []struct {
			Name   string
			Status string
		}
		if json.Unmarshal([]byte(output), &requirements) != nil || requirements == nil {
			return c
		}
		c.Status = "ready"
		c.Detail = "Locked PHP and extension requirements passed."
		for _, p := range requirements {
			if p.Status != "success" {
				c.Issues = append(c.Issues, PackageIssue{Name: p.Name, Title: p.Status})
			}
		}
		if len(c.Issues) > 0 || exit != 0 {
			c.Status = "attention"
			c.Detail = "Some locked platform requirements are not satisfied."
		}
		return c
	}
	if json.Unmarshal([]byte(output), &data) != nil || data == nil {
		return c
	}
	for _, raw := range data {
		if string(raw) == "null" {
			return c
		}
	}
	if kind == "installed" {
		var problems []string
		if data["dependencies"] == nil && data["name"] == nil {
			return c
		}
		if raw := data["problems"]; raw != nil && json.Unmarshal(raw, &problems) != nil {
			return c
		}
		c.Status = "ready"
		c.Detail = "npm direct installed dependency tree passed. Transitive/workspace validation is not exhaustive."
		if exit != 0 || len(problems) > 0 {
			c.Status = "attention"
			c.Detail = "npm reported missing, invalid or extraneous installed dependencies. Run npm ls --depth=0 to review."
		}
		return c
	}
	if data["error"] != nil {
		return c
	}
	if kind == "outdated" {
		if manager == "composer" {
			var packages []struct {
				Name    string
				Version string
				Latest  string
			}
			if raw := data["locked"]; raw != nil {
				if json.Unmarshal(raw, &packages) != nil {
					return c
				}
			} else if raw := data["installed"]; raw != nil {
				if json.Unmarshal(raw, &packages) != nil {
					return c
				}
			} else {
				return c
			}
			for _, p := range packages {
				c.Issues = append(c.Issues, PackageIssue{Name: p.Name, Current: p.Version, Latest: p.Latest})
			}
		} else {
			for name, raw := range data {
				var p struct {
					Current string
					Wanted  string
					Latest  string
				}
				if json.Unmarshal(raw, &p) != nil || p.Latest == "" {
					return c
				}
				c.Issues = append(c.Issues, PackageIssue{Name: name, Current: p.Current, Wanted: p.Wanted, Latest: p.Latest})
			}
		}
		if exit != 0 && !(exit == 1 && len(c.Issues) > 0) {
			return packageCheck(kind, "error", c.Detail)
		}
		c.Status = "ready"
		c.Detail = "No available updates reported in the checked scope."
		if len(c.Issues) > 0 {
			c.Status = "attention"
			c.Detail = "Available updates found. Composer compares locked packages; npm checks direct and configured workspace dependencies against registry tags. Review compatibility before upgrading."
		}
	} else if kind == "audit" {
		if manager == "composer" {
			var advisories map[string][]struct {
				Title    string
				Severity string
				Affected string `json:"affectedVersions"`
			}
			if data["advisories"] == nil {
				return c
			}
			// Composer uses [] rather than {} for an empty advisory collection.
			if string(data["advisories"]) != "[]" && json.Unmarshal(data["advisories"], &advisories) != nil {
				return c
			}
			for name, list := range advisories {
				for _, a := range list {
					c.Issues = append(c.Issues, PackageIssue{Name: name, Severity: a.Severity, Title: a.Title, Current: a.Affected})
				}
			}
			var abandoned map[string]any
			if raw := data["abandoned"]; raw != nil && string(raw) != "[]" {
				if json.Unmarshal(raw, &abandoned) != nil {
					return c
				}
			}
			for name := range abandoned {
				c.Issues = append(c.Issues, PackageIssue{Name: name, Severity: "warning", Title: "Package is abandoned; review a maintained replacement."})
			}
			for _, key := range []string{"filter", "cooldown"} {
				if raw := data[key]; raw != nil && string(raw) != "[]" && string(raw) != "{}" {
					var matches map[string]json.RawMessage
					if json.Unmarshal(raw, &matches) != nil {
						return c
					}
					for name := range matches {
						c.Issues = append(c.Issues, PackageIssue{Name: name, Severity: "warning", Title: "Matched Composer " + key + " policy; inspect composer audit for the policy details."})
					}
				}
			}
		} else {
			var vulnerabilities map[string]struct {
				Severity string
				Range    string
				Via      []json.RawMessage
			}
			if data["vulnerabilities"] == nil || json.Unmarshal(data["vulnerabilities"], &vulnerabilities) != nil {
				return c
			}
			for name, v := range vulnerabilities {
				c.Issues = append(c.Issues, PackageIssue{Name: name, Severity: v.Severity, Current: v.Range, Title: "Affected by a reported vulnerability; inspect npm audit for the advisory chain and remediation."})
			}
		}
		if exit != 0 && len(c.Issues) == 0 {
			return c
		}
		c.Status = "ready"
		c.Detail = "No advisories or abandoned packages reported under the project package-manager policy. Ignored advisories and unreported vulnerabilities are outside this result."
		if len(c.Issues) > 0 {
			c.Status = "attention"
			c.Detail = "Advisories or abandoned packages reported. Review affected versions and upgrade guidance; no fix command was run."
		}
		if raw := data["unreachable-repositories"]; raw != nil && string(raw) != "[]" && string(raw) != "{}" {
			c.Status = "error"
			c.Detail = "Some advisory repositories were unreachable. Findings are partial; registry coverage is incomplete."
		}
	}
	sort.Slice(c.Issues, func(i, j int) bool { return c.Issues[i].Name < c.Issues[j].Name })
	return c
}
