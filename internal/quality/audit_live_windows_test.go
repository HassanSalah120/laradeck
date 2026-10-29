package quality

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/recipes"
	"herdworkbench/internal/runner"
)

// Explicit opt-in: installs real tools only into a disposable fixture/profile.
func TestLiveAuditIntegration(t *testing.T) {
	root := os.Getenv("WORKBENCH_LIVE_AUDIT_ROOT")
	if root == "" {
		t.Skip("set WORKBENCH_LIVE_AUDIT_ROOT to a disposable directory")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	root = absolute
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPDATA", filepath.Join(root, "profile"))
	project := filepath.Join(root, "project")
	os.MkdirAll(filepath.Join(project, "resources/js"), 0700)
	os.MkdirAll(filepath.Join(project, "app"), 0700)
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(project, path), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"name":"disposable-workbench-audit-fixture","private":true,"type":"module"}`)
	write("resources/js/Bad.tsx", `import { useState } from 'react'; export function Bad({enabled}:{enabled:boolean}) { if(enabled) { useState(0); } return <img src="test.png"/>; }`)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	progress := func(s string) { t.Log(s) }
	t.Run("ESLint", func(t *testing.T) {
		plan, err := PreviewAuditSetup(project, "php.exe", "eslint", "npm")
		if err != nil {
			t.Fatal(err)
		}
		if err := ApplyAuditSetup(ctx, plan, progress); err != nil {
			t.Fatal(err)
		}
		result, findings := auditESLint(ctx, project)
		t.Logf("ESLint: %+v", result)
		if result.Status != "completed" {
			t.Fatal(result)
		}
		found := false
		for _, f := range findings {
			if f.Rule == "react-hooks/rules-of-hooks" {
				found = true
			}
		}
		if !found {
			t.Fatalf("real hooks violation missing: %+v", findings)
		}
		write("resources/js/Bad.tsx", `export function Good(){ return <main><h1>Accessible</h1></main>; }`)
		result, findings = auditESLint(ctx, project)
		if result.Status != "completed" || len(findings) != 0 {
			t.Fatalf("safe control: %+v %+v", result, findings)
		}
	})
	t.Run("PHPStan", func(t *testing.T) {
		php, err := exec.LookPath("php.exe")
		if override := os.Getenv("WORKBENCH_AUDIT_PHP"); override != "" {
			php = override
			err = nil
		}
		if err != nil {
			t.Fatal(err)
		}
		write("composer.json", `{"name":"test/workbench-audit-fixture","require-dev":{"phpstan/phpstan":"^2"},"config":{"allow-plugins":false}}`)
		spec, err := recipes.ComposerSpec(php, project, []string{"install", "--no-scripts", "--no-interaction", "--prefer-dist"})
		if err != nil {
			t.Fatal(err)
		}
		execution, err := runner.CaptureExecContext(ctx, spec, 4*time.Minute)
		if err != nil || execution.ExitCode != 0 {
			t.Fatalf("composer: %v %+v", err, execution)
		}
		write("phpstan.neon", "parameters:\n    level: 5\n    paths:\n        - app\n")
		write("app/Test.php", "<?php function auditFailure(): int { return 'wrong'; }\n")
		result, findings := auditPHPStan(ctx, php, project)
		t.Logf("PHPStan: %+v", result)
		if result.Status != "completed" || len(findings) == 0 {
			t.Fatalf("bad control: %+v %+v", result, findings)
		}
		write("app/Test.php", "<?php function auditSuccess(): int { return 1; }\n")
		result, findings = auditPHPStan(ctx, php, project)
		if result.Status != "completed" || len(findings) != 0 {
			t.Fatalf("safe control: %+v %+v", result, findings)
		}
	})
	t.Run("Gitleaks", func(t *testing.T) {
		manager := dependencies.NewManager("", nil)
		if _, err := manager.Resolve("gitleaks"); err != nil {
			if _, err := manager.Install(ctx, "gitleaks"); err != nil {
				t.Fatal(err)
			}
		}
		// Synthetic value; no credential is created or contacted.
		token := "ghp_" + strings.Repeat("aB3cD4", 6)
		write("secret-fixture.txt", "github_token = \""+token+"\"\n")
		result, findings := auditGitleaks(ctx, project, false, "")
		t.Logf("Gitleaks: %+v", result)
		if result.Status != "completed" || len(findings) == 0 {
			t.Fatalf("bad control %+v %+v", result, findings)
		}
		for _, f := range findings {
			if strings.Contains(f.Message, token) || strings.Contains(f.Snippet, token) {
				t.Fatal("secret escaped")
			}
		}
		binary, err := manager.Resolve("gitleaks")
		if err != nil {
			t.Fatal(err)
		}
		baseline := filepath.Join(project, "workbench-baseline.json")
		execution, err := runner.CaptureExecContext(ctx, runner.Spec{Executable: binary, Directory: project, Args: []string{"dir", project, "--redact=100", "--report-format=json", "--report-path=" + baseline, "--exit-code=1", "--no-banner"}}, time.Minute)
		if err != nil || execution.ExitCode > 1 {
			t.Fatalf("baseline generation failed: %v", err)
		}
		result, findings = auditGitleaks(ctx, project, false, "workbench-baseline.json")
		if result.Status != "completed" || len(findings) != 0 {
			t.Fatalf("baseline not respected: %+v (%d findings)", result, len(findings))
		}
		os.Remove(baseline)
		os.Remove(filepath.Join(project, "secret-fixture.txt"))
	})
	t.Run("Browser", func(t *testing.T) {
		plan, err := PreviewAuditSetup(project, "php.exe", "browser", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := ApplyAuditSetup(ctx, plan, progress); err != nil {
			t.Fatal(err)
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/protected" {
				if _, err := r.Cookie("fixture_session"); err != nil {
					http.Redirect(w, r, "/login", 302)
					return
				}
			}
			w.Header().Set("Content-Type", "text/html")
			if r.URL.Path == "/login" {
				w.Write([]byte(`<!doctype html><html lang="en"><title>Fixture login</title><main><h1>Sign in</h1><a href="/login-complete">Fixture sign in</a></main></html>`))
				return
			}
			if r.URL.Path == "/login-complete" {
				http.SetCookie(w, &http.Cookie{Name: "fixture_session", Value: "synthetic", Path: "/", HttpOnly: true})
				http.Redirect(w, r, "/protected", 302)
				return
			}
			if r.URL.Path == "/slow" {
				time.Sleep(4 * time.Second)
			}
			w.Write([]byte(`<!doctype html><html lang="en"><title>Audit fixture</title><main><h1>Fixture</h1><img src="missing.png"><label>Account<input name="account"></label></main></html>`))
		}))
		defer server.Close()
		session, err := StartBrowserSession(ctx, server.URL, true)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		result, findings := session.Audit(ctx, server.URL+"/")
		t.Logf("Browser: %+v", result)
		found := false
		for _, f := range findings {
			if f.Rule == "image-alt" {
				found = true
			}
		}
		if result.Status != "completed" || !found {
			t.Fatalf("rendered violation missing %+v %+v", result, findings)
		}
		result, _ = session.Audit(ctx, server.URL+"/protected")
		if result.Status != "skipped" {
			t.Fatalf("protected page silently audited %+v", result)
		}
		// Simulate a user's explicit login navigation in the isolated fixture browser.
		session.Audit(ctx, server.URL+"/login-complete")
		result, _ = session.Audit(ctx, server.URL+"/protected")
		if result.Status != "completed" {
			t.Fatalf("session not retained %+v", result)
		}
		session.Close()
		if session.Status().Running {
			t.Fatal("session left running")
		}
		if _, err := os.Stat(session.directory); !os.IsNotExist(err) {
			t.Fatalf("profile not removed: %v", err)
		}
		second, err := StartBrowserSession(ctx, server.URL, true)
		if err != nil {
			t.Fatal(err)
		}
		defer second.Close()
		result, _ = second.Audit(ctx, server.URL+"/protected")
		if result.Status != "skipped" {
			t.Fatal("login persisted across sessions")
		}
		auditCtx, stop := context.WithCancel(ctx)
		time.AfterFunc(250*time.Millisecond, stop)
		done := context.AfterFunc(auditCtx, second.Close)
		second.Audit(auditCtx, server.URL+"/slow")
		done()
		second.Close()
		if second.Status().Running {
			t.Fatal("cancel left browser running")
		}
	})
	t.Run("CombinedReport", func(t *testing.T) {
		php := os.Getenv("WORKBENCH_AUDIT_PHP")
		if php == "" {
			php = "php.exe"
		}
		report := RunAudit(ctx, php, DetectProjectContext(project), AuditOptions{}, nil)
		if report.Running || report.FinishedAt == "" || len(report.Analyzers) != 6 {
			t.Fatalf("incomplete combined report: %+v", report)
		}
		for _, engine := range report.Analyzers {
			expected := "completed"
			if engine.ID == "semgrep" || engine.ID == "axe" {
				expected = "skipped"
			}
			if engine.Status != expected {
				t.Fatalf("combined engine: %+v", engine)
			}
		}
	})
}
func TestLiveLarastanSetup(t *testing.T) {
	root := os.Getenv("WORKBENCH_LARASTAN_FIXTURE")
	if root == "" {
		t.Skip("set WORKBENCH_LARASTAN_FIXTURE to a disposable Laravel project")
	}
	php := os.Getenv("WORKBENCH_AUDIT_PHP")
	if php == "" {
		t.Fatal("select actual Herd PHP explicitly")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	plan, err := PreviewAuditSetup(root, php, "larastan", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyAuditSetup(ctx, plan, func(s string) { t.Log(s) }); err != nil {
		t.Fatal(err)
	}
	result, findings := auditPHPStan(ctx, php, root)
	t.Logf("Larastan: %+v; %d project findings", result, len(findings))
	if result.Status != "completed" {
		t.Fatal(result)
	}
}
