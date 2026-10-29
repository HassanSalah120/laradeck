package quality

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"herdworkbench/internal/companion"
	"herdworkbench/internal/storage"
)

type RulePolicy struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Owner    string `json:"owner"`
	Optional bool   `json:"optional"`
}

// This registry owns the policy of every built-in rule. Analyzer-owned rules
// have no heuristic fallback when the real analyzer is unavailable.
func AuditRules() []RulePolicy {
	var rules []RulePolicy
	add := func(kind, owner string, optional bool, ids ...string) {
		for _, id := range ids {
			rules = append(rules, RulePolicy{id, kind, owner, optional})
		}
	}
	add("correctness", "eslint", false, "RulesOfHooks")
	add("correctness", "phpstan", false, "NoEnvOutsideConfig", "LongLivedProcessStateLeak")
	add("correctness", "builtin", false, "InertiaFilePutSpoofing", "VersionAwareLazyProps")
	add("security", "builtin", false, "AvoidRawSqlInjection", "NoDebugCallsInApp", "ChannelAuthGuards", "BroadcastPayloadSanitization", "SensitiveDataLogging", "ModelExposedSensitiveAttributes", "UnsafeFileUploadValidation", "UnsafeExternalRedirect", "AuthRouteRateLimiting", "UnsafeDynamicExecution", "UnsafeMassAssignmentRequestAll")
	add("correctness", "builtin", false, "AbortEffectFetches", "StaleClosureInSocketListener", "EchoCleanupUnmount", "UnthrottledClientWhisper", "JobRetriesTimeout", "QueuedBroadcasts", "MultiWriteTransactionRequired", "ReactEffectCleanupRequired")
	add("performance", "builtin", true, "NoQueriesInBlade", "LightSharedProps", "WrapInertiaPropsInClosures", "PreventLazyLoadingGuard", "UnboundedQueryInJobs", "QueueJobWithoutRelations")
	add("convention", "builtin", true, "ModelNamingSingular", "ControllerNamingSingular", "FormRequestValidation", "NoMutationsInRoutes", "ExtractDomainLogic", "TypedInertiaPageProps", "SafeTargetBlankRel", "RegistrationRegisteredEvent", "StableListKeys", "EmptyGuardedRisk")
	return rules
}

type AuditOptions struct {
	IncludeConventions       bool   `json:"includeConventions"`
	IncludePerformanceAdvice bool   `json:"includePerformanceAdvice"`
	GitHistory               bool   `json:"gitHistory"`
	GitleaksBaseline         string `json:"gitleaksBaseline"`
	SemgrepConfig            string `json:"semgrepConfig"`
	SemgrepWSL               string `json:"semgrepWsl"`
}
type AnalyzerResult struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Version       string  `json:"version"`
	Configuration string  `json:"configuration"`
	Status        string  `json:"status"`
	Coverage      string  `json:"coverage"`
	Message       string  `json:"message"`
	Duration      float64 `json:"durationSeconds"`
}
type AuditReport struct {
	ComparisonKey string                    `json:"comparisonKey,omitempty"`
	ProjectID     string                    `json:"projectId"`
	Running       bool                      `json:"running"`
	StartedAt     string                    `json:"startedAt"`
	FinishedAt    string                    `json:"finishedAt"`
	Findings      []CodeBestPracticeFinding `json:"findings"`
	Analyzers     []AnalyzerResult          `json:"analyzers"`
}

func (r AuditReport) Clone() AuditReport {
	r.Findings = append([]CodeBestPracticeFinding{}, r.Findings...)
	for i := range r.Findings {
		r.Findings[i].Tools = append([]string{}, r.Findings[i].Tools...)
	}
	r.Analyzers = append([]AnalyzerResult{}, r.Analyzers...)
	return r
}

var secretPattern = regexp.MustCompile(`(?i)(?:sk_(?:live|test)_[a-z0-9_]{12,}|gh[pousr]_[a-z0-9_]{20,}|github_pat_[a-z0-9_]{20,}|AKIA[A-Z0-9]{16})`)
var privateKeyPattern = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)`)

func SanitizeAuditText(value string) string {
	value = privateKeyPattern.ReplaceAllString(value, "[REDACTED PRIVATE KEY]")
	value = secretPattern.ReplaceAllString(value, "[REDACTED SECRET]")
	value = storage.RedactSensitive(value)
	if len(value) > 4000 {
		value = value[:4000] + " ..."
	}
	return value
}
func projectRelative(root, path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "[outside project]"
	}
	return filepath.ToSlash(rel)
}
func normalizeAuditFinding(root string, f CodeBestPracticeFinding) CodeBestPracticeFinding {
	f.File = projectRelative(root, f.File)
	f.Fingerprint = companion.Fingerprint(root, f.File, f.Rule, f.Message, f.Line)
	f.Message = SanitizeAuditText(f.Message)
	f.FixGuidance = SanitizeAuditText(f.FixGuidance)
	// Source and DOM snippets may contain unrelated credentials or clinical data.
	f.Snippet = ""
	if f.Certainty == "" {
		f.Certainty = "needs_review"
	}
	if f.Kind == "" {
		f.Kind = "convention"
	}
	if f.Severity != "error" && f.Severity != "warning" {
		f.Severity = "info"
	}
	if len(f.Tools) == 0 {
		f.Tools = []string{f.Tool}
	}
	if !strings.HasPrefix(f.GithubRef, "https://") {
		f.GithubRef = ""
	}
	return f
}
func builtinPolicy(findings []CodeBestPracticeFinding, options AuditOptions) []CodeBestPracticeFinding {
	policies := map[string]RulePolicy{}
	for _, p := range AuditRules() {
		policies[p.ID] = p
	}
	result := []CodeBestPracticeFinding{}
	for _, f := range findings {
		p, ok := policies[f.Rule]
		if !ok || p.Owner != "builtin" {
			continue
		}
		if p.Kind == "convention" && !options.IncludeConventions {
			continue
		}
		if p.Kind == "performance" && !options.IncludePerformanceAdvice {
			continue
		}
		f.Tool = "Workbench heuristic"
		f.Kind = p.Kind
		f.Certainty = "needs_review"
		f.Message = "Review candidate: " + f.Message
		if f.Severity == "error" {
			f.Severity = "warning"
		}
		if p.Optional {
			f.Severity = "info"
		}
		result = append(result, f)
	}
	return result
}
func mergeAuditFindings(root string, findings []CodeBestPracticeFinding) []CodeBestPracticeFinding {
	result := []CodeBestPracticeFinding{}
	seen := map[string]int{}
	canonical := map[string]string{"react-hooks/rules-of-hooks": "RulesOfHooks", "larastan.noEnvCallsOutsideOfConfig": "NoEnvOutsideConfig"}
	for _, f := range findings {
		f = normalizeAuditFinding(root, f)
		rule := f.Rule
		if alias, ok := canonical[rule]; ok {
			rule = alias
		}
		key := fmt.Sprintf("%s:%d:%s:%s", f.File, f.Line, rule, f.Message)
		if i, ok := seen[key]; ok {
			for _, tool := range f.Tools {
				if !strings.Contains("|"+strings.Join(result[i].Tools, "|")+"|", "|"+tool+"|") {
					result[i].Tools = append(result[i].Tools, tool)
				}
			}
			continue
		}
		seen[key] = len(result)
		result = append(result, f)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].File != result[j].File {
			return result[i].File < result[j].File
		}
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		return result[i].Rule < result[j].Rule
	})
	return result
}

// A failed engine always remains in the report, even when another engine finds
// issues. Completed means the declared coverage ran, not that the app is safe.
func RunAudit(ctx context.Context, php string, pCtx ProjectContext, options AuditOptions, notify func(AuditReport)) AuditReport {
	report := AuditReport{Running: true, StartedAt: time.Now().UTC().Format(time.RFC3339), Findings: []CodeBestPracticeFinding{}, Analyzers: []AnalyzerResult{}}
	emit := func() {
		if notify != nil {
			notify(report.Clone())
		}
	}
	tasks := []struct {
		id, name string
		run      func() (AnalyzerResult, []CodeBestPracticeFinding)
	}{
		{"builtin", "Workbench source heuristics", func() (AnalyzerResult, []CodeBestPracticeFinding) {
			files, collectErr := collectPHPFiles(pCtx.ProjectPath)
			if collectErr != nil {
				return AnalyzerResult{Status: "failed", Message: SanitizeAuditText(collectErr.Error())}, nil
			}

			var (
				phpFindings   []CodeBestPracticeFinding
				bladeFindings []CodeBestPracticeFinding
				tsxFindings   []CodeBestPracticeFinding
				phpErr        error
				bladeErr      error
				tsxErr        error
				wg            sync.WaitGroup
			)

			wg.Add(3)
			go func() {
				defer wg.Done()
				phpFindings, phpErr = runPHPBatchAnalyzer(ctx, php, files, pCtx)
			}()
			go func() {
				defer wg.Done()
				bladeFindings, bladeErr = ScanBladeAndStructure(pCtx.ProjectPath)
			}()
			go func() {
				defer wg.Done()
				tsxFindings, tsxErr = ScanTSXAndReact(pCtx.ProjectPath)
			}()
			wg.Wait()

			f := append([]CodeBestPracticeFinding{}, phpFindings...)
			f = append(f, bladeFindings...)
			f = append(f, tsxFindings...)
			err := errors.Join(phpErr, bladeErr, tsxErr)
			status := "completed"
			message := "Heuristics require review; analyzer-owned rules have no heuristic fallback."
			if err != nil {
				status = "failed"
				message = SanitizeAuditText(err.Error())
			}
			return AnalyzerResult{Status: status, Coverage: fmt.Sprintf("PHP candidates in app/config/routes/bootstrap (%d files); Blade in resources/views and JSX in resources/js. Heuristic coverage only; optional advice selected separately", len(files)), Message: message}, builtinPolicy(f, options)
		}},
		{"phpstan", "PHPStan / Larastan", func() (AnalyzerResult, []CodeBestPracticeFinding) { return auditPHPStan(ctx, php, pCtx.ProjectPath) }},
		{"eslint", "ESLint", func() (AnalyzerResult, []CodeBestPracticeFinding) { return auditESLint(ctx, pCtx.ProjectPath) }},
		{"gitleaks", "Gitleaks", func() (AnalyzerResult, []CodeBestPracticeFinding) {
			return auditGitleaks(ctx, pCtx.ProjectPath, options.GitHistory, options.GitleaksBaseline)
		}},
		{"semgrep", "Semgrep (optional)", func() (AnalyzerResult, []CodeBestPracticeFinding) {
			return auditSemgrep(ctx, pCtx.ProjectPath, options)
		}},
	}
	for _, task := range tasks {
		if ctx.Err() != nil {
			report.Analyzers = append(report.Analyzers, AnalyzerResult{ID: task.id, Name: task.name, Status: "cancelled", Message: "Audit cancelled before this check"})
			continue
		}
		report.Analyzers = append(report.Analyzers, AnalyzerResult{ID: task.id, Name: task.name, Status: "running"})
		emit()
		start := time.Now()
		result, findings := task.run()
		result.ID = task.id
		result.Name = task.name
		result.Duration = time.Since(start).Seconds()
		if ctx.Err() != nil {
			result.Status = "cancelled"
			result.Message = "Audit cancelled"
		}
		report.Analyzers[len(report.Analyzers)-1] = result
		selected := []CodeBestPracticeFinding{}
		for _, finding := range findings {
			if finding.Kind == "convention" && !options.IncludeConventions {
				continue
			}
			if finding.Kind == "performance" && !options.IncludePerformanceAdvice {
				continue
			}
			selected = append(selected, finding)
		}
		report.Findings = mergeAuditFindings(pCtx.ProjectPath, append(report.Findings, selected...))
		emit()
	}
	report.Analyzers = append(report.Analyzers, AnalyzerResult{ID: "axe", Name: "axe-core browser accessibility", Status: "skipped", Coverage: "No rendered page checked in this static audit", Message: "Open a temporary browser and explicitly audit a selected page"})
	report.Running = false
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	emit()
	return report
}
