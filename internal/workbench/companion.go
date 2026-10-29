package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"herdworkbench/internal/companion"
	"herdworkbench/internal/quality"
	"herdworkbench/internal/runner"
	"herdworkbench/internal/storage"
)

type ProjectHealth struct {
	Project   Project        `json:"project"`
	Trusted   bool           `json:"trusted"`
	Checks    []HealthCheck  `json:"checks"`
	Running   int            `json:"running"`
	LastAudit string         `json:"lastAudit"`
	Findings  int            `json:"findings"`
	Coverage  string         `json:"coverage"`
	Composer  PackageSummary `json:"composer"`
	NPM       PackageSummary `json:"npm"`
}
type HealthCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func exists(path string) bool { info, err := os.Stat(path); return err == nil && !info.IsDir() }
func (b *Backend) WorkspaceHealth() []ProjectHealth {
	b.mu.Lock()
	projects := append([]Project{}, b.projects...)
	trusted := map[string]bool{}
	for id, v := range b.trusted {
		trusted[id] = v
	}
	b.mu.Unlock()
	result := []ProjectHealth{}
	snapshot := b.Snapshot()
	for _, p := range projects {
		InspectDependencies(&p)
		h := ProjectHealth{Project: p, Trusted: trusted[p.ID], Checks: []HealthCheck{}, Coverage: "Not run"}
		add := func(name string, ok bool, detail string) {
			status := "attention"
			if ok {
				status = "ready"
			}
			h.Checks = append(h.Checks, HealthCheck{name, status, detail})
		}
		add("Dependencies", p.VendorPresent, p.DependencyState)
		add("PHP executable", exists(p.PHPBinary), p.PHP)
		add("HTTPS", p.Secured, p.URL)
		add("Clockwork package", p.Clockwork != "", p.Clockwork)
		add("Environment file", exists(filepath.Join(p.Path, ".env")), "File presence only; no credentials inspected")
		packages := b.packageHealth(p)
		checked, dateErr := time.Parse(time.RFC3339, packages.CheckedAt)
		stale := packages.CheckedAt != "" && (packages.Revision != packageRevision(p.Path) || dateErr != nil || time.Since(checked) > 24*time.Hour || time.Until(checked) > time.Minute)
		h.Composer = summarizePackages(packages.Composer, packages.CheckedAt != "", stale)
		h.NPM = summarizePackages(packages.NPM, packages.CheckedAt != "", stale)
		if exists(filepath.Join(p.Path, "package.json")) {
			add("Frontend dependencies", exists(filepath.Join(p.Path, "node_modules", ".package-lock.json")) || isDir(filepath.Join(p.Path, "node_modules")), "Node modules directory; build freshness not inferred")
		}
		h.Checks = append(h.Checks, HealthCheck{"Runtime / extensions", "not_checked", "Use Project diagnostics in a trusted local project"}, HealthCheck{"Clockwork endpoint", "not_checked", "Package presence does not prove capture readiness"})
		for _, r := range snapshot.Runs {
			if r.ProjectID == p.ID && r.Running {
				h.Running++
			}
		}
		report, ok := snapshot.AuditReports[p.ID]
		if !ok {
			var saved []quality.AuditReport
			if b.companionStore.Load(p.ID, "audits", &saved) == nil && len(saved) > 0 {
				report = saved[len(saved)-1]
				ok = true
			}
		}
		if ok {
			h.LastAudit = report.FinishedAt
			h.Findings = len(report.Findings)
			complete, total := 0, len(report.Analyzers)
			for _, a := range report.Analyzers {
				if a.Status == "completed" {
					complete++
				}
			}
			h.Coverage = fmt.Sprintf("%d/%d completed", complete, total)
			if report.Running {
				h.Coverage = "Running"
			}
		}
		result = append(result, h)
	}
	return result
}
func isDir(path string) bool { info, err := os.Stat(path); return err == nil && info.IsDir() }

type StartupProfile struct {
	Name      string   `json:"name"`
	Vite      bool     `json:"vite"`
	Queue     bool     `json:"queue"`
	Scheduler bool     `json:"scheduler"`
	Services  []string `json:"services"`
	VitePort  int      `json:"vitePort"`
}
type ProfileSession struct {
	Runs     []string `json:"runs"`
	Services []string `json:"services"`
}
type RepairPlan struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Kind        string    `json:"kind"`
	Command     string    `json:"command"`
	Impact      string    `json:"impact"`
	Fingerprint string    `json:"fingerprint"`
	Created     time.Time `json:"created"`
	spec        runner.Spec
}

func projectRevision(p Project) string {
	h := sha256.New()
	for _, name := range []string{"composer.json", "composer.lock", "package.json", "package-lock.json", ".env", "bootstrap/cache/config.php"} {
		data, _ := os.ReadFile(filepath.Join(p.Path, name))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func npmSpec(p Project, args ...string) (runner.Spec, error) {
	node, err := exec.LookPath("node.exe")
	if err != nil {
		return runner.Spec{}, errors.New("Node is missing from PATH")
	}
	npm, err := exec.LookPath("npm.cmd")
	if err != nil {
		return runner.Spec{}, errors.New("npm is missing from PATH")
	}
	cli := filepath.Join(filepath.Dir(npm), "node_modules", "npm", "bin", "npm-cli.js")
	if !exists(cli) {
		return runner.Spec{}, errors.New("Cannot resolve npm-cli.js")
	}
	return runner.Spec{Executable: node, Directory: p.Path, Args: append([]string{cli}, args...)}, nil
}
func (b *Backend) previewRepair(p Project, kind string) (RepairPlan, error) {
	plan := RepairPlan{ID: fmt.Sprintf("p%d", time.Now().UnixNano()), ProjectID: p.ID, Kind: kind, Fingerprint: projectRevision(p), Created: time.Now()}
	spec := runner.Spec{Executable: p.PHPBinary, Directory: p.Path}
	var err error
	switch kind {
	case "composer-install":
		tool, e := b.dependencies.Resolve("composer")
		if e != nil {
			return plan, e
		}
		spec.Args = []string{tool, "install", "--no-interaction", "--prefer-dist"}
		if strings.HasSuffix(strings.ToLower(tool), ".exe") {
			spec.Executable = tool
			spec.Args = spec.Args[1:]
		}
		plan.Impact = "Installs locked dependencies. Composer plugins and project scripts execute. No composer update."
	case "config-clear":
		spec.Args = []string{"artisan", "config:clear", "--no-interaction"}
		plan.Impact = "Removes the project's cached configuration. Does not edit .env."
	case "npm-install":
		if !exists(filepath.Join(p.Path, "package-lock.json")) {
			return plan, errors.New("npm ci requires package-lock.json; select the project's package manager separately")
		}
		spec, err = npmSpec(p, "ci")
		plan.Impact = "Replaces node_modules from package-lock.json; npm lifecycle scripts execute."
	case "asset-build":
		spec, err = npmSpec(p, "run", "build")
		plan.Impact = "Runs the project's build script and updates built frontend assets."
	case "telescope-setup":
		spec.Args = []string{"artisan", "telescope:install", "--no-interaction"}
		plan.Impact = "Publishes Telescope assets, configuration, provider and migrations. Review local-only provider registration afterwards. Database migration is a separate confirmed action."
	case "pulse-setup":
		spec.Args = []string{"artisan", "vendor:publish", "--provider=Laravel\\Pulse\\PulseServiceProvider", "--no-interaction"}
		plan.Impact = "Publishes Pulse assets, configuration and migrations without --force. Database migration is a separate confirmed action."
	default:
		return plan, errors.New("Unsupported repair; use Settings for Clockwork/package installation")
	}
	if err != nil {
		return plan, err
	}
	plan.spec = spec
	plan.Command = filepath.Base(spec.Executable) + " " + strings.Join(spec.Args, " ")
	b.mu.Lock()
	if len(b.companionPlans) > 100 {
		b.companionPlans = map[string]RepairPlan{}
	}
	b.companionPlans[plan.ID] = plan
	b.mu.Unlock()
	return plan, nil
}
func (b *Backend) persistAudit(p Project, report quality.AuditReport) error {
	if report.Running || report.FinishedAt == "" {
		return nil
	}
	b.companionMu.Lock()
	defer b.companionMu.Unlock()
	var history []quality.AuditReport
	err := b.companionStore.Load(p.ID, "audits", &history)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(history) > 0 && history[len(history)-1].StartedAt == report.StartedAt {
		return nil
	}
	history = append(history, report.Clone())
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	return b.companionStore.Save(p.ID, "audits", history)
}
func auditFindings(report quality.AuditReport) []companion.Finding {
	out := []companion.Finding{}
	for _, f := range report.Findings {
		out = append(out, companion.Finding{Fingerprint: f.Fingerprint, Rule: f.Rule, File: f.File, Line: f.Line, Message: f.Message})
	}
	return out
}
func requestPerformance(r Request) companion.Performance {
	p := companion.Performance{Method: r.Method, URI: r.URI, Duration: r.Duration, DatabaseDuration: r.DatabaseDuration, QueryCount: r.QueryCount, Memory: r.Memory, Queries: []string{}}
	for _, q := range r.Queries {
		p.Queries = append(p.Queries, companion.NormalizeQuery(q.SQL))
	}
	return p
}

// Companion is an explicit allowlist, not a generic shell/PHP execution API.
func (b *Backend) Companion(id, operation string, input map[string]any) (any, error) {
	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	ctx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()
	if input == nil {
		input = map[string]any{}
	}
	switch operation {
	case "migrate", "rollback", "cache-delete", "session-delete", "snapshot-create", "snapshot-restore", "repair-apply", "test-save", "migration-preview", "migrations", "schema-pin", "schema-diff", "cache", "sessions", "telescope", "profile-start", "package-scan":
		if !b.companionMutationMu.TryLock() {
			return nil, errors.New("Another companion write operation is running; wait for completion")
		}
		defer b.companionMutationMu.Unlock()
	}
	text := func(key string) string { v, _ := input[key].(string); return v }
	bridge := func(op string, values map[string]any) (any, error) {
		data, e := companion.Run(ctx, p.PHPBinary, p.Path, op, values, 2*time.Minute)
		if e != nil {
			return nil, e
		}
		var value any
		e = json.Unmarshal(data, &value)
		return value, e
	}
	switch operation {
	case "package-inspect":
		return b.packageHealth(p), nil
	case "package-scan":
		online, _ := input["online"].(bool)
		return b.scanPackages(ctx, p, online)
	case "profile-load":
		var profiles []StartupProfile
		err = b.companionStore.Load(id, "profiles", &profiles)
		if os.IsNotExist(err) {
			return []StartupProfile{}, nil
		}
		return profiles, err
	case "profile-save":
		raw, _ := json.Marshal(input)
		var profile StartupProfile
		if json.Unmarshal(raw, &profile) != nil || strings.TrimSpace(profile.Name) == "" || len(profile.Name) > 80 {
			return nil, errors.New("Profile needs a name of at most 80 characters")
		}
		if profile.VitePort < 1 {
			profile.VitePort = 5173
		}
		if profile.VitePort > 65535 {
			return nil, errors.New("Invalid Vite port")
		}
		if len(profile.Services) > 10 {
			return nil, errors.New("Too many services")
		}
		b.companionMu.Lock()
		defer b.companionMu.Unlock()
		var profiles []StartupProfile
		err = b.companionStore.Load(id, "profiles", &profiles)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		updated := false
		for i, v := range profiles {
			if v.Name == profile.Name {
				profiles[i] = profile
				updated = true
			}
		}
		if !updated {
			profiles = append(profiles, profile)
		}
		return profiles, b.companionStore.Save(id, "profiles", profiles)
	case "profile-start":
		b.companionMu.Lock()
		defer b.companionMu.Unlock()
		if _, ok := b.profileSessions[id]; ok {
			return nil, errors.New("Stop the current profile before starting another")
		}
		var profiles []StartupProfile
		if err = b.companionStore.Load(id, "profiles", &profiles); err != nil {
			return nil, err
		}
		var profile *StartupProfile
		for i := range profiles {
			if profiles[i].Name == text("name") {
				profile = &profiles[i]
			}
		}
		if profile == nil {
			return nil, errors.New("Saved profile not found")
		}
		if profile.Queue || profile.Scheduler {
			if _, e := bridge("runtime", nil); e != nil {
				return nil, e
			}
		}
		session := ProfileSession{Runs: []string{}, Services: []string{}}
		b.profileSessions[id] = session
		fail := func(e error) (any, error) {
			for _, run := range session.Runs {
				_ = b.Stop(run, true)
			}
			for _, service := range session.Services {
				_ = b.StopService(service, true)
			}
			delete(b.profileSessions, id)
			return nil, fmt.Errorf("Profile startup rolled back its newly owned processes: %w", e)
		}
		if profile.Vite {
			var pkg struct {
				Scripts map[string]string `json:"scripts"`
			}
			raw, _ := os.ReadFile(filepath.Join(p.Path, "package.json"))
			_ = json.Unmarshal(raw, &pkg)
			if !regexp.MustCompile(`(^|[ /\\])vite(?:$|[ ])`).MatchString(pkg.Scripts["dev"]) {
				return fail(errors.New("Profile Vite startup requires a direct vite dev script; use saved commands for a custom script"))
			}
			listener, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", profile.VitePort))
			if e != nil {
				return fail(fmt.Errorf("Vite port %d is already occupied", profile.VitePort))
			}
			listener.Close()
		}
		for _, service := range profile.Services {
			already := false
			for _, s := range b.RefreshServices() {
				if s.ID == service && (s.Status == "running" || s.Status == "external") {
					already = true
				}
			}
			if already {
				continue
			}
			if e := b.StartService(service, ""); e != nil {
				return fail(e)
			}
			session.Services = append(session.Services, service)
		}
		for _, item := range []struct {
			enabled bool
			kind    string
		}{{profile.Vite, "vite"}, {profile.Queue, "queue-work"}, {profile.Scheduler, "schedule-work"}} {
			if !item.enabled {
				continue
			}
			r, e := b.startProcess(id, item.kind, profile.VitePort)
			if e != nil {
				return fail(e)
			}
			session.Runs = append(session.Runs, r.ID)
		}
		select {
		case <-ctx.Done():
			return fail(ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
		for _, runID := range session.Runs {
			for _, r := range b.Snapshot().Runs {
				if r.ID == runID && !r.Running {
					return fail(fmt.Errorf("%s exited during startup (exit %d); inspect its process output", r.Name, r.ExitCode))
				}
			}
		}
		b.profileSessions[id] = session
		b.emit()
		return session, nil
	case "profile-stop":
		b.companionMu.Lock()
		defer b.companionMu.Unlock()
		session, ok := b.profileSessions[id]
		if !ok {
			return nil, errors.New("No owned profile session for this project")
		}
		force, _ := input["force"].(bool)
		for _, run := range session.Runs {
			if e := b.Stop(run, force); e != nil {
				return nil, e
			}
		}
		for _, service := range session.Services {
			if e := b.StopService(service, force); e != nil {
				return nil, e
			}
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			running := false
			for _, r := range b.Snapshot().Runs {
				for _, owned := range session.Runs {
					if r.ID == owned && r.Running {
						running = true
					}
				}
			}
			if !running {
				break
			}
			if time.Now().After(deadline) {
				return nil, errors.New("Profile processes are still stopping; wait and retry, or explicitly force stop")
			}
			time.Sleep(50 * time.Millisecond)
		}
		delete(b.profileSessions, id)
		return map[string]any{"message": "Profile processes stopped"}, nil
	case "profile-status":
		b.companionMu.Lock()
		defer b.companionMu.Unlock()
		session, ok := b.profileSessions[id]
		return map[string]any{"active": ok, "session": session}, nil
	case "platform-check":
		tool, e := b.dependencies.Resolve("composer")
		if e != nil {
			return nil, e
		}
		spec := runner.Spec{Executable: p.PHPBinary, Directory: p.Path, Args: []string{tool, "check-platform-reqs", "--format=json", "--no-interaction"}}
		if strings.HasSuffix(strings.ToLower(tool), ".exe") {
			spec.Executable = tool
			spec.Args = spec.Args[1:]
		}
		result, e := runner.CaptureExecContext(ctx, spec, time.Minute)
		if e != nil {
			return nil, e
		}
		var checks any
		if e = json.Unmarshal([]byte(result.Stdout), &checks); e != nil {
			return nil, errors.New("Composer did not return platform requirements JSON; install project dependencies first")
		}
		return map[string]any{"checks": checks, "exitCode": result.ExitCode}, nil
	case "repair-preview":
		return b.previewRepair(p, text("kind"))
	case "repair-apply":
		b.mu.Lock()
		plan, ok := b.companionPlans[text("plan")]
		if ok {
			delete(b.companionPlans, plan.ID)
		}
		b.mu.Unlock()
		if !ok || plan.ProjectID != id || time.Since(plan.Created) > 10*time.Minute {
			return nil, errors.New("Repair preview expired; preview again")
		}
		if text("confirmation") != p.Name {
			return nil, errors.New("Type the project name to confirm")
		}
		if projectRevision(p) != plan.Fingerprint {
			return nil, errors.New("Project configuration changed; preview again")
		}
		if !b.dependencyInstallMu.TryLock() {
			return nil, errors.New("Another package or repair operation is running")
		}
		defer b.dependencyInstallMu.Unlock()
		result, e := runner.CaptureExecContext(ctx, plan.spec, 5*time.Minute)
		if e != nil {
			return nil, errors.New(storage.RedactSensitive(e.Error()))
		}
		if result.ExitCode != 0 {
			return nil, fmt.Errorf("Repair failed (exit %d): %s", result.ExitCode, storage.RedactSensitive(result.Combined))
		}
		b.emit()
		return map[string]any{"output": storage.RedactSensitive(result.Combined), "exitCode": result.ExitCode}, nil
	case "performance-pin", "performance-compare":
		capture := b.clock.State()
		if capture.ProjectID != id {
			return nil, errors.New("Refresh captures for this project first")
		}
		var selected *Request
		for i := range capture.Requests {
			if capture.Requests[i].ID == text("request") {
				selected = &capture.Requests[i]
			}
		}
		if selected == nil {
			return nil, errors.New("Capture not found")
		}
		if operation == "performance-pin" {
			return selected, b.companionStore.Save(id, "performance", requestPerformance(*selected))
		}
		var baseline companion.Performance
		if err = b.companionStore.Load(id, "performance", &baseline); err != nil {
			return nil, errors.New("Pin a baseline first")
		}
		comparison, e := companion.Compare(baseline, requestPerformance(*selected))
		return map[string]any{"baseline": baseline, "current": requestPerformance(*selected), "comparison": comparison, "note": "Observed samples, not a controlled benchmark. Authentication, data, caches and machine load may differ."}, e
	case "performance-baseline":
		var baseline companion.Performance
		err = b.companionStore.Load(id, "performance", &baseline)
		if os.IsNotExist(err) {
			return nil, nil
		}
		return baseline, err
	case "timeline":
		capture := b.clock.State()
		if capture.ProjectID != id {
			return nil, errors.New("Refresh captures for this project first")
		}
		for _, r := range capture.Requests {
			if r.ID == text("request") {
				return map[string]any{"request": r, "note": "Queries and exception count belong to this Clockwork request. No job/mail/event relationship is inferred. Open Telescope batches for additional recorded associations."}, nil
			}
		}
		return nil, errors.New("Capture not found")
	case "audit-history":
		var history []quality.AuditReport
		err = b.companionStore.Load(id, "audits", &history)
		if os.IsNotExist(err) {
			return map[string]any{"reports": []quality.AuditReport{}, "changes": []companion.Change{}}, nil
		}
		if err != nil {
			return nil, err
		}
		changes := []companion.Change{}
		before, after := len(history)-2, len(history)-1
		if n, ok := input["before"].(float64); ok {
			before = int(n)
		}
		if n, ok := input["after"].(float64); ok {
			after = int(n)
		}
		if before >= 0 && after >= 0 && before < len(history) && after < len(history) {
			changes = comparableChanges(history[before], history[after])
		}
		return map[string]any{"reports": history, "changes": changes, "note": "Not observed is only resolved evidence when the same analyzer/configuration completed in both scans."}, nil
	case "schema-pin":
		data, e := bridge("migrations", input)
		if e != nil {
			return nil, e
		}
		return data, b.companionStore.Save(id, "schema", data)
	case "schema-diff":
		data, e := bridge("migrations", input)
		if e != nil {
			return nil, e
		}
		var before map[string]any
		if e = b.companionStore.Load(id, "schema", &before); e != nil {
			return nil, errors.New("Save a schema baseline first")
		}
		now := data.(map[string]any)
		limit := 500
		note := "Schema diff comparing observed tables."
		if before["schemaTruncated"] == true || now["schemaTruncated"] == true {
			note = fmt.Sprintf("Note: Database schema exceeds %d tables. Diff is computed on observed tables; unobserved tables beyond the limit are not compared.", limit)
		}
		a, _ := json.Marshal(before["target"])
		z, _ := json.Marshal(now["target"])
		if string(a) != string(z) {
			return nil, errors.New("Connection changed; schema baseline belongs to a different target")
		}
		old := map[string]string{}
		current := map[string]string{}
		collect := func(v any, dst map[string]string) {
			list, _ := v.([]any)
			for _, item := range list {
				m, _ := item.(map[string]any)
				name, _ := m["name"].(string)
				raw, _ := json.Marshal(m)
				dst[name] = string(raw)
			}
		}
		collect(before["schema"], old)
		collect(now["schema"], current)
		diff := []map[string]string{}
		for name, value := range current {
			status := "added"
			if previous, ok := old[name]; ok {
				if previous == value {
					continue
				}
				status = "changed"
			}
			diff = append(diff, map[string]string{"table": name, "status": status})
		}
		for name := range old {
			if _, ok := current[name]; !ok {
				diff = append(diff, map[string]string{"table": name, "status": "removed"})
			}
		}
		return map[string]any{"changes": diff, "before": before["schema"], "after": now["schema"], "limit": limit, "note": note}, nil
	case "snapshots":
		return b.listSnapshots(id)
	case "snapshot-create", "snapshot-restore":
		return b.snapshotOperation(p, operation, input)
	case "test-draft":
		return generateRequestTest(input)
	case "test-save":
		draft, e := generateRequestTest(input)
		if e != nil {
			return nil, e
		}
		content := text("content")
		if content == "" {
			content = draft
		}
		if len(content) > 65536 || !strings.HasPrefix(strings.TrimSpace(content), "<?php") {
			return nil, errors.New("Test must be PHP source under 64 KiB")
		}
		name := text("name")
		if !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*Test\.php$`).MatchString(name) {
			return nil, errors.New("Use a name such as DashboardTest.php")
		}
		if text("framework") == "phpunit" {
			content = strings.Replace(content, "class RouteCompanionTest extends TestCase", "class "+strings.TrimSuffix(name, ".php")+" extends TestCase", 1)
		}
		dir := filepath.Join(p.Path, "tests", "Feature")
		realDir, e := filepath.EvalSymlinks(dir)
		realProject, ep := filepath.EvalSymlinks(p.Path)
		if ep != nil || realProject == "" {
			realProject = p.Path
		}
		if e != nil || !strings.HasPrefix(strings.ToLower(realDir), strings.ToLower(realProject)+string(filepath.Separator)) {
			return nil, errors.New("tests/Feature must be an existing directory inside this project")
		}
		f, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		_, e = f.WriteString(content)
		closeErr := f.Close()
		if e != nil {
			return nil, e
		}
		return map[string]string{"file": filepath.Join(dir, name)}, closeErr
	case "runtime", "routes", "migrations", "migration-preview", "migrate", "rollback", "cache", "cache-delete", "sessions", "session-delete", "telescope":
		data, e := bridge(operation, input)
		return redactResult(data, text("key") != "" && input["reveal"] == true), e
	case "dashboard-url":
		kind := text("kind")
		if kind != "telescope" && kind != "pulse" {
			return nil, errors.New("Unsupported dashboard")
		}
		data, e := bridge("integration-url", map[string]any{"kind": kind})
		if e != nil {
			return nil, e
		}
		path, _ := data.(map[string]any)["path"].(string)
		if path == "" {
			return nil, errors.New("Dashboard path missing")
		}
		return strings.TrimRight(p.URL, "/") + path, nil
	case "integrations":
		var composer struct {
			Packages []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"packages"`
			Dev []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"packages-dev"`
		}
		raw, _ := os.ReadFile(filepath.Join(p.Path, "composer.lock"))
		_ = json.Unmarshal(raw, &composer)
		items := []map[string]any{}
		for _, name := range []string{"telescope", "pulse"} {
			version := ""
			for _, pkg := range append(composer.Packages, composer.Dev...) {
				if pkg.Name == "laravel/"+name {
					version = pkg.Version
				}
			}
			installed := exists(filepath.Join(p.Path, "vendor", "laravel", name, "composer.json"))
			items = append(items, map[string]any{"id": name, "version": version, "installed": installed, "url": strings.TrimRight(p.URL, "/") + "/" + name, "note": "Package presence does not prove routes, provider configuration or database migrations are ready."})
		}
		return items, nil
	}
	return nil, errors.New("Unsupported companion action")
}
func redactResult(value any, reveal bool) any {
	switch v := value.(type) {
	case string:
		if reveal {
			return storage.RedactSensitive(v)
		}
		return storage.RedactSensitive(v)
	case []any:
		for i, item := range v {
			v[i] = redactResult(item, reveal)
		}
	case map[string]any:
		for key, item := range v {
			v[key] = redactResult(item, reveal)
		}
	}
	return value
}
func generateTest(method, uri, framework, status string, authenticated bool) (string, error) {
	method = strings.ToUpper(method)
	if !regexp.MustCompile(`^(GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD)$`).MatchString(method) {
		return "", errors.New("Choose one HTTP method")
	}
	if !strings.HasPrefix(uri, "/") || strings.ContainsAny(uri, "\r\n\x00") || strings.HasPrefix(uri, "//") {
		return "", errors.New("Test URI must be a local path")
	}
	if !regexp.MustCompile(`^[1-5][0-9]{2}$`).MatchString(status) {
		return "", errors.New("Expected status must be 100–599")
	}
	literal := "'" + strings.ReplaceAll(strings.ReplaceAll(uri, "\\", "\\\\"), "'", "\\'") + "'"
	call := fmt.Sprintf("$this->%s(%s)", strings.ToLower(method), literal)
	if framework == "pest" {
		authLine := ""
		if authenticated {
			authLine = "    $user = \\App\\Models\\User::factory()->create();\n    $this->actingAs($user);\n\n"
		}
		return "<?php\n\n// Replace route placeholders and arrange authentication/data before execution.\nit('responds to the selected route', function () {\n" + authLine + "    " + call + "->assertStatus(" + status + ");\n});\n", nil
	}
	authLine := ""
	if authenticated {
		authLine = "        $user = \\App\\Models\\User::factory()->create();\n        $this->actingAs($user);\n\n"
	}
	return "<?php\n\nnamespace Tests\\Feature;\n\nuse Tests\\TestCase;\n\nclass RouteCompanionTest extends TestCase\n{\n    // Arrange authentication/data and replace route placeholders before execution.\n    public function test_selected_route_responds(): void\n    {\n" + authLine + "        " + call + "->assertStatus(" + status + ");\n    }\n}\n", nil
}

type SnapshotManifest struct {
	ID         string         `json:"id"`
	Created    string         `json:"created"`
	Target     map[string]any `json:"target"`
	TargetHash string         `json:"targetHash"`
	Driver     string         `json:"driver"`
	Bytes      int64          `json:"bytes"`
	SHA256     string         `json:"sha256"`
}

func (b *Backend) listSnapshots(id string) ([]SnapshotManifest, error) {
	dir, err := b.companionStore.Path(id, "index")
	if err != nil {
		return nil, err
	}
	files, err := filepath.Glob(filepath.Join(filepath.Dir(dir), "snapshot-*.json"))
	if err != nil {
		return nil, err
	}
	out := []SnapshotManifest{}
	for _, file := range files {
		raw, e := os.ReadFile(file)
		var m SnapshotManifest
		if e == nil && json.Unmarshal(raw, &m) == nil {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created > out[j].Created })
	return out, nil
}
func (b *Backend) snapshotOperation(p Project, operation string, input map[string]any) (any, error) {
	if operation == "snapshot-restore" {
		for _, run := range b.Snapshot().Runs {
			if run.ProjectID == p.ID && run.Running && (run.Name == "Queue worker" || run.Name == "Scheduler") {
				return nil, errors.New("Stop this project's queue worker and scheduler before restoring")
			}
		}
		if b.actionsCoord.GetWorkerStatus().Running {
			return nil, errors.New("Stop the queue worker in Actions before restoring")
		}
	}

	b.snapshotMu.Lock()
	defer b.snapshotMu.Unlock()
	return b.snapshotUnlocked(p, operation, input)
}
func resolveNativeClient(candidate string) string {
	if path, err := exec.LookPath(candidate); err == nil && path != "" {
		return path
	}
	userProfile := os.Getenv("USERPROFILE")
	localAppData := os.Getenv("LOCALAPPDATA")
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	searchPatterns := []string{
		filepath.Join(userProfile, ".config", "herd", "bin", candidate),
		filepath.Join(localAppData, "Programs", "Herd", "resources", "app.asar.unpacked", "resources", "bin", candidate),
		filepath.Join(programFiles, "Herd", "resources", "app.asar.unpacked", "resources", "bin", candidate),
		filepath.Join(programFiles, "MySQL", "MySQL Server *", "bin", candidate),
		filepath.Join(programFiles, "MariaDB *", "bin", candidate),
		filepath.Join(programFiles, "PostgreSQL", "*", "bin", candidate),
		filepath.Join("C:\\laragon", "bin", "mysql", "*", "bin", candidate),
		filepath.Join("C:\\laragon", "bin", "postgresql", "*", "bin", candidate),
		filepath.Join(programFilesX86, "MySQL", "MySQL Server *", "bin", candidate),
		filepath.Join(programFilesX86, "MariaDB *", "bin", candidate),
		filepath.Join(programFilesX86, "PostgreSQL", "*", "bin", candidate),
	}
	for _, pattern := range searchPatterns {
		if strings.Contains(pattern, "*") {
			matches, _ := filepath.Glob(pattern)
			if len(matches) > 0 {
				sort.Sort(sort.Reverse(sort.StringSlice(matches)))
				for _, m := range matches {
					if info, err := os.Stat(m); err == nil && !info.IsDir() {
						return m
					}
				}
			}
		} else {
			if info, err := os.Stat(pattern); err == nil && !info.IsDir() {
				return pattern
			}
		}
	}
	return ""
}

func (b *Backend) snapshotUnlocked(p Project, operation string, input map[string]any) (any, error) {
	text := func(key string) string { v, _ := input[key].(string); return v }
	id := fmt.Sprintf("snapshot-%d", time.Now().UnixNano())
	if operation == "snapshot-restore" {
		id = text("snapshot")
	}
	path, err := b.companionStore.Path(p.ID, id)
	if err != nil || !strings.HasPrefix(id, "snapshot-") {
		return nil, errors.New("Invalid snapshot")
	}
	dataPath := strings.TrimSuffix(path, ".json") + ".dump"
	values := map[string]any{"path": dataPath, "confirmation": text("confirmation")}
	tool := text("tool")
	if tool != "" {
		absolute, e := filepath.Abs(tool)
		if e != nil {
			return nil, e
		}
		values["tool"] = absolute
	}
	if operation == "snapshot-restore" {
		var m SnapshotManifest
		if err = b.companionStore.Load(p.ID, id, &m); err != nil {
			return nil, err
		}
		checksum, e := snapshotChecksum(dataPath)
		if e != nil {
			return nil, e
		}
		if checksum != m.SHA256 {
			return nil, errors.New("Snapshot checksum mismatch")
		}
		values["targetHash"] = m.TargetHash
		if tool == "" {
			candidate := "mysql.exe"
			if m.Driver == "pgsql" {
				candidate = "pg_restore.exe"
			}
			values["tool"] = resolveNativeClient(candidate)
		}
		// Check connection and confirmation before creating the recovery snapshot.
		raw, e := companion.Run(b.ctx, p.PHPBinary, p.Path, "runtime", nil, time.Minute)
		if e != nil {
			return nil, e
		}
		var runtime struct {
			Database map[string]any `json:"database"`
		}
		if e = json.Unmarshal(raw, &runtime); e != nil {
			return nil, e
		}
		current, _ := json.Marshal(runtime.Database)
		target, _ := json.Marshal(m.Target)
		if string(current) != string(target) {
			return nil, errors.New("Snapshot target differs from the current database")
		}
		label := fmt.Sprint(m.Target["connection"]) + " / " + fmt.Sprint(m.Target["database"])
		if text("confirmation") != label {
			return nil, errors.New("Type the database target to confirm restore")
		}
		backupCandidate := "mysqldump.exe"
		if m.Driver == "pgsql" {
			backupCandidate = "pg_dump.exe"
		}
		backupTool := text("backupTool")
		if backupTool == "" {
			backupTool = resolveNativeClient(backupCandidate)
		}
		recoveryInput := map[string]any{"tool": backupTool}
		recovery, e := b.snapshotUnlocked(p, "snapshot-create", recoveryInput)
		if e != nil {
			return nil, fmt.Errorf("Recovery snapshot failed; restore cancelled: %w", e)
		}
		raw, e = companion.Run(b.ctx, p.PHPBinary, p.Path, operation, values, 10*time.Minute)
		if e != nil {
			return nil, fmt.Errorf("Restore failed; recovery snapshot retained: %w", e)
		}
		var result any
		_ = json.Unmarshal(raw, &result)
		return map[string]any{"result": result, "recovery": recovery}, nil
	}
	// Resolve installed native clients when no custom path was selected.
	if tool == "" {
		runtime, e := companion.Run(b.ctx, p.PHPBinary, p.Path, "runtime", nil, time.Minute)
		if e != nil {
			return nil, e
		}
		var data struct {
			Database struct {
				Driver string `json:"driver"`
			} `json:"database"`
		}
		_ = json.Unmarshal(runtime, &data)
		candidate := "mysqldump.exe"
		if data.Database.Driver == "pgsql" {
			candidate = "pg_dump.exe"
		}
		values["tool"] = resolveNativeClient(candidate)
	}
	raw, err := companion.Run(b.ctx, p.PHPBinary, p.Path, operation, values, 10*time.Minute)
	if err != nil {
		_ = os.Remove(dataPath)
		return nil, err
	}
	var m SnapshotManifest
	if err = json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	m.ID = id
	m.Created = time.Now().UTC().Format(time.RFC3339Nano)
	m.SHA256, err = snapshotChecksum(dataPath)
	if err != nil {
		return nil, err
	}
	if err = b.companionStore.Save(p.ID, id, m); err != nil {
		return nil, err
	}
	return m, nil
}

func snapshotChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func phpLiteral(value string) string {
	return "'" + strings.ReplaceAll(strings.ReplaceAll(value, "\\", "\\\\"), "'", "\\'") + "'"
}
func generateRequestTest(input map[string]any) (string, error) {
	text := func(key string) string { v, _ := input[key].(string); return v }
	authenticated, _ := input["authenticated"].(bool)
	method, uri := strings.ToUpper(text("method")), text("uri")
	draft, err := generateTest(method, uri, text("framework"), text("status"), authenticated)
	if err != nil {
		return "", err
	}
	headers := map[string]any{}
	if raw := text("headers"); raw != "" {
		if json.Unmarshal([]byte(raw), &headers) != nil {
			return "", errors.New("Headers must be a JSON object")
		}
	}
	var body any = map[string]any{}
	if raw := text("body"); raw != "" {
		if json.Unmarshal([]byte(raw), &body) != nil {
			return "", errors.New("Test generation requires a JSON request body; edit the draft for other encodings")
		}
		switch body.(type) {
		case map[string]any, []any:
		default:
			return "", errors.New("Test body must be a JSON object or array")
		}
	}
	var scrub func(any) any
	sensitive := regexp.MustCompile(`(?i)password|token|secret|authorization|cookie|api.?key`)
	scrub = func(value any) any {
		switch v := value.(type) {
		case map[string]any:
			for key, item := range v {
				if sensitive.MatchString(key) {
					v[key] = "[redacted: supply a test fixture]"
				} else {
					v[key] = scrub(item)
				}
			}
		case []any:
			for i, item := range v {
				v[i] = scrub(item)
			}
		}
		return value
	}
	hb, _ := json.Marshal(scrub(headers))
	bb, _ := json.Marshal(scrub(body))
	call := fmt.Sprintf("$this->json(%s, %s, json_decode(%s, true), json_decode(%s, true))", phpLiteral(method), phpLiteral(storage.RedactSensitive(uri)), phpLiteral(string(bb)), phpLiteral(string(hb)))
	old := fmt.Sprintf("$this->%s(%s)", strings.ToLower(method), phpLiteral(uri))
	return strings.Replace(draft, old, call, 1), nil
}

func companionAuditKey(p Project, options quality.AuditOptions, report quality.AuditReport) string {
	h := sha256.New()
	h.Write([]byte("laradeck-audit-comparison-v1"))
	raw, _ := json.Marshal(options)
	h.Write(raw)
	files := []string{"composer.lock", "package-lock.json", "pnpm-lock.yaml", "yarn.lock", "phpstan.neon", "phpstan.neon.dist", "phpstan-baseline.neon", "eslint.config.js", "eslint.config.mjs", "eslint.config.cjs", "eslint.config.ts", ".eslintrc.json", ".eslintrc.cjs", ".gitleaks.toml"}
	for _, a := range report.Analyzers {
		if a.Configuration != "" {
			files = append(files, a.Configuration)
		}
	}
	if options.SemgrepConfig != "" {
		files = append(files, options.SemgrepConfig)
	}
	sort.Strings(files)
	for _, file := range files {
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(p.Path, path)
		}
		h.Write([]byte(file))
		if data, err := os.ReadFile(path); err == nil {
			h.Write(data)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
func comparableChanges(before, after quality.AuditReport) []companion.Change {
	changes := companion.Diff(auditFindings(before), auditFindings(after))
	if before.ComparisonKey == "" || before.ComparisonKey != after.ComparisonKey {
		return changes
	}
	owners := map[string]string{}
	for _, rule := range quality.AuditRules() {
		owners[rule.ID] = rule.Owner
	}
	complete := func(report quality.AuditReport, id string) (quality.AnalyzerResult, bool) {
		for _, a := range report.Analyzers {
			if a.ID == id {
				return a, a.Status == "completed"
			}
		}
		return quality.AnalyzerResult{}, false
	}
	for i, c := range changes {
		if c.Status != "not_observed" {
			continue
		}
		owner := owners[c.Finding.Rule]
		if owner == "" {
			for _, f := range before.Findings {
				if f.Fingerprint != c.Finding.Fingerprint {
					continue
				}
				tool := strings.ToLower(f.Tool)
				for _, id := range []string{"phpstan", "eslint", "gitleaks", "semgrep"} {
					if strings.Contains(tool, id) {
						owner = id
					}
				}
				if strings.Contains(tool, "larastan") {
					owner = "phpstan"
				}
			}
		}
		if owner == "" {
			continue
		}
		a, ok := complete(before, owner)
		z, good := complete(after, owner)
		if ok && good && a.Version == z.Version && a.Configuration == z.Configuration && a.Coverage == z.Coverage {
			changes[i].Status = "resolved"
		}
	}
	return changes
}
