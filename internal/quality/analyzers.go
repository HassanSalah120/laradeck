package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/runner"
)

func existing(root string, names ...string) string {
	for _, name := range names {
		path := filepath.Join(root, name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}
func PHPStanConfiguration(root string) string {
	return existing(root, "phpstan.neon", "phpstan.neon.dist", "phpstan.dist.neon")
}
func ESLintConfiguration(root string) string {
	if p := existing(root, "eslint.config.js", "eslint.config.mjs", "eslint.config.cjs", "eslint.config.ts", "eslint.config.mts", "eslint.config.cts", ".eslintrc", ".eslintrc.json", ".eslintrc.js", ".eslintrc.cjs", ".eslintrc.yml", ".eslintrc.yaml"); p != "" {
		return p
	}
	var pkg map[string]json.RawMessage
	if data, err := os.ReadFile(filepath.Join(root, "package.json")); err == nil && json.Unmarshal(data, &pkg) == nil && len(pkg["eslintConfig"]) > 0 {
		return filepath.Join(root, "package.json")
	}
	return ""
}
func NodeBinary() (string, error) { return dependencies.NewManager("", nil).Resolve("node") }
func toolVersion(ctx context.Context, spec runner.Spec) string {
	result, err := runner.CaptureExecContext(ctx, spec, 8*time.Second)
	if err != nil || result.ExitCode != 0 {
		return "unknown"
	}
	text := strings.TrimSpace(result.Stdout)
	if len(text) > 180 {
		text = text[:180]
	}
	return SanitizeAuditText(text)
}
func failedResult(result AnalyzerResult, err error, execution *runner.ExecutionResult) AnalyzerResult {
	result.Status = "failed"
	if err != nil {
		result.Message = SanitizeAuditText(err.Error())
	} else if execution != nil {
		result.Message = SanitizeAuditText(execution.Stderr)
		if result.Message == "" {
			result.Message = fmt.Sprintf("Analyzer exited with code %d", execution.ExitCode)
		}
	}
	return result
}
func parsePHPStan(data []byte) ([]CodeBestPracticeFinding, []string, error) {
	var output struct {
		Totals *struct {
			Errors int `json:"errors"`
		} `json:"totals"`
		Errors []string `json:"errors"`
		Files  map[string]struct {
			Messages []struct {
				Message    string `json:"message"`
				Line       int    `json:"line"`
				Identifier string `json:"identifier"`
				Tip        string `json:"tip"`
			} `json:"messages"`
		} `json:"files"`
	}
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, nil, err
	}
	if output.Totals == nil {
		return nil, nil, fmt.Errorf("PHPStan output lacks analysis totals")
	}
	findings := []CodeBestPracticeFinding{}
	for file, entry := range output.Files {
		for _, m := range entry.Messages {
			id := m.Identifier
			if id == "" {
				id = "phpstan.project-rule"
			}
			kind := "correctness"
			certainty := "needs_review"
			for _, prefix := range []string{"method.", "property.", "argument.", "return.", "parameter.", "offsetAccess.", "variable.", "function.", "class.", "syntax.", "larastan.relation", "larastan.noEnv", "larastan.octane", "larastan.checkDispatch"} {
				if strings.HasPrefix(id, prefix) {
					kind = "correctness"
					certainty = "detected"
				}
			}
			if strings.Contains(strings.ToLower(id), "unnecessary") || strings.Contains(strings.ToLower(id), "unusedview") {
				kind = "convention"
				certainty = "needs_review"
			}
			findings = append(findings, CodeBestPracticeFinding{Tool: "PHPStan / Larastan", Rule: id, Category: "Laravel & PHP", Kind: kind, Certainty: certainty, Severity: "error", File: file, Line: m.Line, Message: m.Message, FixGuidance: m.Tip})
		}
	}
	if output.Totals.Errors > 0 && len(output.Errors) == 0 {
		output.Errors = append(output.Errors, "PHPStan reported global analysis errors without details")
	}
	return findings, output.Errors, nil
}
func auditPHPStan(ctx context.Context, php, root string) (AnalyzerResult, []CodeBestPracticeFinding) {
	result := AnalyzerResult{Status: "missing", Coverage: "Project PHPStan configuration and baseline; Laravel-specific coverage depends on configured extensions"}
	binary := existing(root, "vendor/bin/phpstan")
	if binary == "" {
		result.Message = "Install PHPStan/Larastan from Audit setup"
		return result, nil
	}
	config := PHPStanConfiguration(root)
	if config == "" {
		result.Status = "skipped"
		result.Message = "No PHPStan configuration. Preview starter setup; no default level was silently selected."
		return result, nil
	}
	result.Configuration = projectRelative(root, config)
	result.Version = toolVersion(ctx, runner.Spec{Executable: php, Directory: root, Args: []string{binary, "--version"}})
	run, err := runner.CaptureExecContext(ctx, runner.Spec{Executable: php, Directory: root, Args: []string{binary, "analyse", "--configuration=" + config, "--memory-limit=2G", "--error-format=json", "--no-interaction", "--no-ansi", "--no-progress"}}, 10*time.Minute)
	if err != nil {
		return failedResult(result, err, run), nil
	}
	findings, errors, parseErr := parsePHPStan([]byte(run.Stdout))
	if parseErr != nil {
		return failedResult(result, fmt.Errorf("Invalid PHPStan JSON: %w", parseErr), run), nil
	}
	if len(errors) > 0 {
		return failedResult(result, fmt.Errorf("PHPStan analysis errors: %s", strings.Join(errors, "; ")), run), findings
	}
	if run.ExitCode != 0 && run.ExitCode != 1 {
		return failedResult(result, nil, run), findings
	}
	result.Status = "completed"
	result.Message = "Project rules completed; this is not a security certification"
	return result, findings
}

func parseESLint(data []byte) ([]CodeBestPracticeFinding, bool, error) {
	var outputs []struct {
		File     string `json:"filePath"`
		Messages []struct {
			Rule     *string `json:"ruleId"`
			Severity int     `json:"severity"`
			Fatal    bool    `json:"fatal"`
			Message  string  `json:"message"`
			Line     int     `json:"line"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &outputs); err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(string(data)) == "null" {
		return nil, false, fmt.Errorf("null ESLint output")
	}
	findings := []CodeBestPracticeFinding{}
	fatal := false
	for _, file := range outputs {
		for _, m := range file.Messages {
			if m.Fatal || m.Rule == nil {
				fatal = true
			}
			rule := "eslint.parser"
			if m.Rule != nil {
				rule = *m.Rule
			}
			kind := "correctness"
			certainty := "needs_review"
			if rule == "react-hooks/rules-of-hooks" || rule == "@typescript-eslint/no-floating-promises" || rule == "@typescript-eslint/no-misused-promises" || rule == "no-unreachable" || rule == "no-undef" || rule == "no-dupe-keys" {
				kind = "correctness"
				certainty = "detected"
			}
			if strings.HasPrefix(rule, "prettier/") || strings.HasPrefix(rule, "@stylistic/") || rule == "quotes" || rule == "semi" || rule == "indent" || rule == "@typescript-eslint/naming-convention" {
				kind = "convention"
				certainty = "needs_review"
			}
			if strings.HasPrefix(rule, "jsx-a11y/") {
				kind = "correctness"
				certainty = "needs_review"
			}
			severity := "warning"
			if m.Severity == 2 {
				severity = "error"
			}
			findings = append(findings, CodeBestPracticeFinding{Tool: "ESLint", Rule: rule, Kind: kind, Certainty: certainty, Category: "React & UI", Severity: severity, File: file.File, Line: m.Line, Message: m.Message})
		}
	}
	return findings, fatal, nil
}
func auditESLint(ctx context.Context, root string) (AnalyzerResult, []CodeBestPracticeFinding) {
	result := AnalyzerResult{Status: "missing", Coverage: "Project ESLint configuration, overrides and ignores; plugin enablement is defined by that configuration"}
	binary := existing(root, "node_modules/eslint/bin/eslint.js")
	node, err := NodeBinary()
	if binary == "" || err != nil {
		result.Message = "Project-local ESLint or Node is missing. Use Audit setup."
		return result, nil
	}
	config := ESLintConfiguration(root)
	if config == "" {
		result.Status = "skipped"
		result.Message = "No ESLint configuration. Preview starter setup; existing configuration is never replaced."
		return result, nil
	}
	result.Configuration = projectRelative(root, config)
	result.Version = toolVersion(ctx, runner.Spec{Executable: node, Directory: root, Args: []string{binary, "--version"}})
	targets := []string{}
	for _, dir := range []string{"resources/js", "src"} {
		if info, err := os.Stat(filepath.Join(root, dir)); err == nil && info.IsDir() {
			targets = append(targets, dir)
		}
	}
	if len(targets) == 0 {
		result.Status = "skipped"
		result.Message = "No resources/js or src directory; no frontend coverage"
		return result, nil
	}
	args := append([]string{binary}, targets...)
	args = append(args, "--format", "json")
	spec := runner.Spec{Executable: node, Directory: root, Args: args}
	if !strings.HasPrefix(filepath.Base(config), "eslint.config.") {
		spec.Env = []string{"ESLINT_USE_FLAT_CONFIG=false"}
	}
	run, err := runner.CaptureExecContext(ctx, spec, 3*time.Minute)
	if err != nil {
		return failedResult(result, err, run), nil
	}
	findings, fatal, parseErr := parseESLint([]byte(run.Stdout))
	if parseErr != nil {
		return failedResult(result, fmt.Errorf("Invalid ESLint JSON: %w", parseErr), run), nil
	}
	if fatal || run.ExitCode > 1 {
		return failedResult(result, fmt.Errorf("ESLint could not analyze all selected files; review configuration/parser diagnostics"), run), findings
	}
	result.Status = "completed"
	return result, findings
}

func parseGitleaks(data []byte) ([]CodeBestPracticeFinding, error) {
	var entries []struct {
		Rule        string `json:"RuleID"`
		Description string `json:"Description"`
		File        string `json:"File"`
		Line        int    `json:"StartLine"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "null" {
		return nil, fmt.Errorf("Missing Gitleaks results")
	}
	result := []CodeBestPracticeFinding{}
	for _, e := range entries {
		result = append(result, CodeBestPracticeFinding{Tool: "Gitleaks", Rule: e.Rule, Kind: "security", Certainty: "needs_review", Category: "Security", Severity: "warning", File: e.File, Line: e.Line, Message: "Suspected credential: " + e.Description, FixGuidance: "Review locally. A pattern match does not prove a credential is active. No credential value is included in this report."})
	}
	return result, nil
}
func auditGitleaks(ctx context.Context, root string, history bool, baseline string) (AnalyzerResult, []CodeBestPracticeFinding) {
	result := AnalyzerResult{Status: "missing", Coverage: "Current project files; provider patterns identify suspected secrets, not verified live credentials"}
	binary, err := dependencies.NewManager("", nil).Resolve("gitleaks")
	if err != nil {
		result.Message = "Install the private Gitleaks binary from Settings"
		return result, nil
	}
	if history {
		result.Coverage = "Git history, including historical credentials; project baselines/exclusions apply"
	}
	result.Version = toolVersion(ctx, runner.Spec{Executable: binary, Directory: root, Args: []string{"version"}})
	dir, err := os.MkdirTemp("", "hwb-gitleaks-")
	if err != nil {
		return failedResult(result, err, nil), nil
	}
	defer os.RemoveAll(dir)
	report := filepath.Join(dir, "report.json")
	mode := "dir"
	if history {
		mode = "git"
	}
	args := []string{mode, root, "--redact=100", "--report-format=json", "--report-path=" + report, "--exit-code=1", "--no-banner"}
	config := existing(root, ".gitleaks.toml", "gitleaks.toml")
	result.Configuration = projectRelative(root, config)
	if config != "" {
		args = append(args, "--config="+config)
	}
	if baseline != "" {
		path, err := localConfig(root, baseline)
		if err != nil {
			return failedResult(result, fmt.Errorf("Gitleaks baseline: %w", err), nil), nil
		}
		args = append(args, "--baseline-path="+path)
		result.Configuration += " baseline: " + projectRelative(root, path)
	}
	run, err := runner.CaptureExecContext(ctx, runner.Spec{Executable: binary, Args: args, Directory: root}, 3*time.Minute)
	if err != nil {
		return failedResult(result, err, run), nil
	}
	if run.ExitCode != 0 && run.ExitCode != 1 {
		return failedResult(result, nil, run), nil
	}
	reportFile, err := os.Open(report)
	if err != nil {
		return failedResult(result, fmt.Errorf("Gitleaks did not produce a report"), run), nil
	}
	defer reportFile.Close()
	data, err := io.ReadAll(io.LimitReader(reportFile, 16*1024*1024+1))
	if err != nil {
		return failedResult(result, err, run), nil
	}
	if len(data) > 16*1024*1024 {
		return failedResult(result, fmt.Errorf("Gitleaks report exceeds 16 MiB"), run), nil
	}
	findings, err := parseGitleaks(data)
	if err != nil {
		return failedResult(result, fmt.Errorf("Invalid Gitleaks report"), run), nil
	}
	result.Status = "completed"
	return result, findings
}

func localConfig(root, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("Select a local Semgrep rule configuration; registry rules are not downloaded automatically")
	}
	path := name
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	base, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if projectRelative(base, path) == "[outside project]" {
		return "", fmt.Errorf("Configuration must be inside the selected project")
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("Select an existing local configuration file")
	}
	return path, nil
}
func parseSemgrep(data []byte) ([]CodeBestPracticeFinding, bool, error) {
	var output struct {
		Results []struct {
			Rule  string `json:"check_id"`
			Path  string `json:"path"`
			Start struct {
				Line int `json:"line"`
			} `json:"start"`
			Extra struct {
				Message  string `json:"message"`
				Severity string `json:"severity"`
			} `json:"extra"`
		} `json:"results"`
		Errors []json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, false, err
	}
	var shape map[string]json.RawMessage
	json.Unmarshal(data, &shape)
	if _, ok := shape["results"]; !ok {
		return nil, false, fmt.Errorf("Missing Semgrep results")
	}
	findings := []CodeBestPracticeFinding{}
	for _, r := range output.Results {
		findings = append(findings, CodeBestPracticeFinding{Tool: "Semgrep", Rule: r.Rule, Category: "Security", Kind: "security", Certainty: "needs_review", Severity: "warning", File: r.Path, Line: r.Start.Line, Message: r.Extra.Message})
	}
	return findings, len(output.Errors) > 0, nil
}
func auditSemgrep(ctx context.Context, root string, options AuditOptions) (AnalyzerResult, []CodeBestPracticeFinding) {
	result := AnalyzerResult{Status: "skipped", Coverage: "Only the explicitly selected local Semgrep rules; no registry or source uploads"}
	if options.SemgrepConfig == "" {
		result.Message = "Optional: select a local rule file and an existing native or WSL installation"
		return result, nil
	}
	config, err := localConfig(root, options.SemgrepConfig)
	if err != nil {
		return failedResult(result, err, nil), nil
	}
	result.Configuration = projectRelative(root, config)
	binary, err := exec.LookPath("semgrep.exe")
	prefix := []string{}
	if options.SemgrepWSL != "" {
		binary, err = exec.LookPath("wsl.exe")
		if err == nil {
			convert := func(path string) (string, error) {
				r, e := runner.CaptureExecContext(ctx, runner.Spec{Executable: binary, Args: []string{"--distribution", options.SemgrepWSL, "--exec", "wslpath", "-a", "-u", path}}, 10*time.Second)
				if e != nil {
					return "", e
				}
				if r.ExitCode != 0 {
					return "", fmt.Errorf("WSL path conversion failed")
				}
				return strings.TrimSpace(r.Stdout), nil
			}
			convertedRoot, e := convert(root)
			if e != nil {
				return failedResult(result, e, nil), nil
			}
			config, e = convert(config)
			if e != nil {
				return failedResult(result, e, nil), nil
			}
			prefix = []string{"--distribution", options.SemgrepWSL, "--cd", convertedRoot, "--exec", "timeout", "--signal=TERM", "--kill-after=5s", "170s", "semgrep"}
			result.Coverage += "; WSL requires GNU timeout. Cancelling closes the Windows client; the guest command is bounded to 175 seconds."
		}
	}
	if err != nil {
		result.Status = "missing"
		result.Message = "Semgrep is not installed. Use your own installation; Workbench does not provision WSL or rule packs."
		return result, nil
	}
	result.Version = toolVersion(ctx, runner.Spec{Executable: binary, Directory: root, Args: append(append([]string{}, prefix...), "--version")})
	args := append(append([]string{}, prefix...), "scan", "--json", "--metrics=off", "--disable-version-check", "--config", config, ".")
	run, err := runner.CaptureExecContext(ctx, runner.Spec{Executable: binary, Directory: root, Args: args}, 3*time.Minute)
	if err != nil {
		return failedResult(result, err, run), nil
	}
	findings, partial, err := parseSemgrep([]byte(run.Stdout))
	if err != nil {
		return failedResult(result, err, run), nil
	}
	if partial || run.ExitCode > 1 {
		return failedResult(result, fmt.Errorf("Semgrep reported analysis errors or incomplete coverage"), run), findings
	}
	result.Status = "completed"
	return result, findings
}
