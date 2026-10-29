package quality

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditPoliciesAndRedaction(t *testing.T) {
	rules := AuditRules()
	if len(rules) != 40 {
		t.Fatalf("rules: %d", len(rules))
	}
	seen := map[string]bool{}
	raw := []CodeBestPracticeFinding{}
	for _, p := range rules {
		if seen[p.ID] {
			t.Fatal("duplicate", p.ID)
		}
		seen[p.ID] = true
		raw = append(raw, CodeBestPracticeFinding{Rule: p.ID, Severity: "error"})
	}
	for _, f := range builtinPolicy(raw, AuditOptions{}) {
		if f.Kind == "convention" || f.Kind == "performance" || f.Rule == "RulesOfHooks" || f.Rule == "NoEnvOutsideConfig" || f.Rule == "LongLivedProcessStateLeak" || f.Certainty != "needs_review" || f.Severity == "error" {
			t.Fatalf("bad policy: %+v", f)
		}
	}
	root := t.TempDir()
	token := "ghp_" + strings.Repeat("a", 30)
	findings := mergeAuditFindings(root, []CodeBestPracticeFinding{{Tool: "A", Rule: "RulesOfHooks", File: "src/test.tsx", Line: 2, Snippet: token, Message: token}, {Tool: "B", Message: token, Rule: "react-hooks/rules-of-hooks", File: filepath.Join(root, "src/test.tsx"), Line: 2}})
	if len(findings) != 1 || len(findings[0].Tools) != 2 || findings[0].Snippet != "" || strings.Contains(findings[0].Message, token) {
		t.Fatalf("unsafe merge: %+v", findings)
	}
	if projectRelative(root, filepath.Join(root, "..", "secret.env")) != "[outside project]" {
		t.Fatal("outside path escaped")
	}
}
func TestAnalyzerMalformedAndPartialReports(t *testing.T) {
	for _, data := range []string{"null", "{}", "garbage"} {
		if _, _, err := parsePHPStan([]byte(data)); err == nil {
			t.Fatal("accepted phpstan", data)
		}
	}
	findings, failures, err := parsePHPStan([]byte(`{"totals":{"errors":1},"errors":["bootstrap failure"],"files":{"app/X.php":{"messages":[{"message":"Undefined method","line":2,"identifier":"method.notFound"}]}}}`))
	if err != nil || len(findings) != 1 || len(failures) != 1 {
		t.Fatalf("partial PHPStan report: %v", err)
	}
	findings, fatal, err := parseESLint([]byte(`[{"filePath":"src/a.tsx","messages":[{"ruleId":null,"fatal":true,"severity":2,"message":"Parsing error","line":1}]}]`))
	if err != nil || !fatal || len(findings) != 1 {
		t.Fatal("parser error hidden")
	}
	if _, _, err := parseESLint([]byte(" null \n")); err == nil {
		t.Fatal("null lint accepted")
	}
	if _, _, err := parseSemgrep([]byte(`{}`)); err == nil {
		t.Fatal("missing semgrep result accepted")
	}
	_, partial, err := parseSemgrep([]byte(`{"results":[],"errors":[{"message":"failed"}]}`))
	if err != nil || !partial {
		t.Fatal("semgrep partial hidden")
	}
	secrets, err := parseGitleaks([]byte(`[{"RuleID":"test","Description":"Token","File":"test.env","StartLine":2,"Secret":"never-export-this","Match":"never-export-this"}]`))
	data, _ := json.Marshal(secrets)
	if err != nil || strings.Contains(string(data), "never-export-this") {
		t.Fatal("secret exported")
	}
}
func TestCancelledAuditRetainsCoverage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report := RunAudit(ctx, "missing", ProjectContext{ProjectPath: t.TempDir()}, AuditOptions{}, nil)
	if report.Running || report.FinishedAt == "" || len(report.Analyzers) != 6 {
		t.Fatalf("bad report %+v", report)
	}
	for _, a := range report.Analyzers {
		if a.ID != "axe" && a.Status != "cancelled" {
			t.Fatal("cancelled check disappeared", a)
		}
	}
}
func TestSetupConflictAndNoOverwrite(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "eslint.config.cjs")
	os.WriteFile(file, []byte("existing config"), 0600)
	plan := AuditSetupPlan{Fingerprints: map[string]string{file: fileFingerprint(file)}, Files: []SetupFile{}}
	os.WriteFile(file, []byte("user edited config"), 0600)
	if err := ApplyAuditSetup(context.Background(), plan, nil); err == nil {
		t.Fatal("stale preview accepted")
	}
	plan = AuditSetupPlan{Fingerprints: map[string]string{}, Files: []SetupFile{{file, "replacement"}}}
	if err := ApplyAuditSetup(context.Background(), plan, nil); err == nil {
		t.Fatal("config overwritten")
	}
	data, _ := os.ReadFile(file)
	if string(data) != "user edited config" {
		t.Fatal("user work changed")
	}
	os.WriteFile(filepath.Join(root, "package-lock.json"), []byte("{}"), 0600)
	os.WriteFile(filepath.Join(root, "yarn.lock"), nil, 0600)
	os.WriteFile(filepath.Join(root, "package.json"), []byte("{}"), 0600)
	if _, err := PreviewAuditSetup(root, "php", "eslint", ""); err == nil {
		t.Fatal("ambiguous manager accepted")
	}
}
func TestBrowserOriginBoundary(t *testing.T) {
	for _, address := range []string{"https://example.com/", "file:///etc/passwd", "https://user:password@local.test/", "http://project.test.evil.com/"} {
		if _, err := LocalAuditURL(address, ""); err == nil {
			t.Fatal("accepted", address)
		}
	}
	if _, err := LocalAuditURL("http://other.test/", "http://project.test"); err == nil {
		t.Fatal("other origin")
	}
	if _, err := LocalAuditURL("http://project.test/dashboard", "http://project.test"); err != nil {
		t.Fatal(err)
	}
}
func TestBuiltinReadFailureRetainsPartialReport(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "resources/views"), 0700)
	os.WriteFile(filepath.Join(root, "resources/views/long.blade.php"), []byte(strings.Repeat("x", 128*1024)), 0600)
	report := RunAudit(context.Background(), "php.exe", ProjectContext{ProjectPath: root}, AuditOptions{}, nil)
	if report.Analyzers[0].Status != "failed" {
		t.Fatalf("truncated source marked complete: %+v", report.Analyzers[0])
	}
}
func TestESLintSetupPreservesConfigurationAndVersions(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "eslint.config.cjs")
	os.WriteFile(config, []byte("custom rules"), 0600)
	os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"devDependencies":{"eslint":"^10.0.0","eslint-plugin-react-hooks":"^7.1.1"}}`), 0600)
	plan, err := PreviewAuditSetup(root, "php.exe", "eslint", "npm")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Files) != 0 || plan.Fingerprints[config] == "" {
		t.Fatalf("existing configuration not preserved: %+v", plan)
	}
	args := strings.Join(plan.Commands[0].Args, " ")
	if !strings.Contains(args, "eslint@^10.0.0") || strings.Contains(args, "eslint@^9") {
		t.Fatal("existing version replaced:", args)
	}
	data, _ := os.ReadFile(config)
	if string(data) != "custom rules" {
		t.Fatal("preview wrote configuration")
	}
}
