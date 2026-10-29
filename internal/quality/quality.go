package quality

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"herdworkbench/internal/runner"
)

type QualityFinding struct {
	Tool       string `json:"tool"`
	Rule       string `json:"rule"`
	Severity   string `json:"severity"` // "error", "warning", "info"
	File       string `json:"file"`
	Line       int    `json:"line"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type NPlusOneCandidate struct {
	Fingerprint string  `json:"fingerprint"`
	Count       int     `json:"count"`
	TotalTime   float64 `json:"totalTimeMs"`
	SampleQuery string  `json:"sampleQuery"`
	IsNPlusOne  bool    `json:"isNPlusOne"` // count >= 3
}

type ConfigAuditItem struct {
	Category    string `json:"category"` // "performance", "security", "environment"
	Key         string `json:"key"`
	Value       string `json:"value"`
	Status      string `json:"status"` // "good", "warning", "info"
	Message     string `json:"message"`
	FixGuidance string `json:"fixGuidance"`
}

type QualityReport struct {
	Tool        string           `json:"tool"`
	Findings    []QualityFinding `json:"findings"`
	TotalErrors int              `json:"totalErrors"`
	TotalWarns  int              `json:"totalWarns"`
	RawOutput   string           `json:"rawOutput,omitempty"`
	Duration    float64          `json:"durationSeconds"`
}

var sqlParamRegex = regexp.MustCompile(`'(?:''|[^'])*'|\b\d+\b`)

// NormalizeSQL replaces literals and numbers with ? to identify query fingerprints.
func NormalizeSQL(sql string) string {
	normalized := sqlParamRegex.ReplaceAllString(sql, "?")
	return strings.Join(strings.Fields(normalized), " ")
}

// DetectNPlusOne scans a list of queries for repetition patterns.
func DetectNPlusOne(queries []struct {
	SQL      string
	Duration float64
}) []NPlusOneCandidate {
	groups := make(map[string]*NPlusOneCandidate)

	for _, q := range queries {
		fp := NormalizeSQL(q.SQL)
		if strings.HasPrefix(strings.ToLower(fp), "select") {
			cand, exists := groups[fp]
			if !exists {
				cand = &NPlusOneCandidate{
					Fingerprint: fp,
					SampleQuery: q.SQL,
				}
				groups[fp] = cand
			}
			cand.Count++
			cand.TotalTime += q.Duration
		}
	}

	var candidates []NPlusOneCandidate
	for _, cand := range groups {
		if cand.Count > 1 {
			cand.IsNPlusOne = cand.Count >= 3
			candidates = append(candidates, *cand)
		}
	}
	return candidates
}

// RunPint executes Laravel Pint on the project and parses issues.
func RunPint(phpBinary, projectPath string, testOnly bool) (*QualityReport, error) {
	return RunPintContext(context.Background(), phpBinary, projectPath, testOnly)
}

func RunPintContext(ctx context.Context, phpBinary, projectPath string, testOnly bool) (*QualityReport, error) {
	pintBin := filepath.Join(projectPath, "vendor", "bin", "pint")
	if _, err := os.Stat(pintBin); err != nil {
		pintBin = filepath.Join(projectPath, "vendor", "bin", "pint.bat")
	}
	if _, err := os.Stat(pintBin); err != nil {
		return nil, fmt.Errorf("Pint binary not installed in project vendor/bin")
	}

	args := []string{pintBin, "--format=json"}
	if testOnly {
		args = append(args, "--test")
	}

	start := time.Now()
	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       args,
		Directory:  projectPath,
	}, 60*time.Second)

	dur := time.Since(start).Seconds()
	if err != nil {
		return nil, err
	}

	rawOutput := ""
	var data []byte
	if res != nil {
		rawOutput = res.Combined
		data = []byte(res.Stdout)
		if len(data) == 0 {
			data = []byte(res.Combined)
		}
	}

	var pintResult struct {
		Tool   string `json:"tool"`
		Result string `json:"result"`
		Files  []struct {
			Path   string   `json:"path"`
			Fixers []string `json:"fixers"`
		} `json:"files"`
		Summary struct {
			Target  string `json:"target"`
			Checked int    `json:"checked"`
		} `json:"summary"`
		PendingFiles []struct {
			Name   string `json:"name"`
			Issues []struct {
				Rule        string `json:"rule"`
				Description string `json:"description"`
			} `json:"issues"`
		} `json:"pendingFiles"`
	}

	var findings []QualityFinding
	if err := json.Unmarshal(data, &pintResult); err != nil {
		return nil, fmt.Errorf("invalid Pint JSON: %w", err)
	}
	if pintResult.Tool != "pint" && pintResult.Summary.Checked == 0 && pintResult.PendingFiles == nil {
		return nil, errors.New("unrecognized Pint output format")
	}
	for _, file := range pintResult.Files {
		for _, fixer := range file.Fixers {
			findings = append(findings, QualityFinding{Tool: "Pint", Rule: fixer, Severity: "warning", File: file.Path, Line: 1, Message: "Formatting rule: " + fixer, Suggestion: "Run Pint fix to apply formatting"})
		}
	}
	{
		for _, pf := range pintResult.PendingFiles {
			for _, iss := range pf.Issues {
				findings = append(findings, QualityFinding{
					Tool:       "Pint",
					Rule:       iss.Rule,
					Severity:   "warning",
					File:       pf.Name,
					Line:       1,
					Message:    iss.Description,
					Suggestion: fmt.Sprintf("Run Pint fix to apply %s formatting", iss.Rule),
				})
			}
		}
	}

	totalWarns := len(findings)
	if (err != nil || (res != nil && res.ExitCode != 0)) && len(findings) == 0 {
		return &QualityReport{
			Tool:      "Pint",
			RawOutput: rawOutput,
			Duration:  dur,
		}, fmt.Errorf("Pint exited with code %d: %s", res.ExitCode, rawOutput)
	}

	return &QualityReport{
		Tool:       "Pint",
		Findings:   findings,
		TotalWarns: totalWarns,
		RawOutput:  rawOutput,
		Duration:   dur,
	}, nil
}

// RunPHPStan executes PHPStan / Larastan and parses JSON output.
func RunPHPStan(phpBinary, projectPath string) (*QualityReport, error) {
	return RunPHPStanContext(context.Background(), phpBinary, projectPath)
}

func RunPHPStanContext(ctx context.Context, phpBinary, projectPath string) (*QualityReport, error) {
	stanBin := filepath.Join(projectPath, "vendor", "bin", "phpstan")
	if _, err := os.Stat(stanBin); err != nil {
		stanBin = filepath.Join(projectPath, "vendor", "bin", "phpstan.bat")
	}
	if _, err := os.Stat(stanBin); err != nil {
		return nil, fmt.Errorf("PHPStan / Larastan binary not installed in project vendor/bin")
	}

	start := time.Now()
	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{stanBin, "analyse", "--memory-limit=1G", "--error-format=json", "--no-interaction", "--no-ansi"},
		Directory:  projectPath,
	}, 90*time.Second)

	dur := time.Since(start).Seconds()
	if err != nil {
		return nil, err
	}

	rawOutput := ""
	var data []byte
	if res != nil {
		rawOutput = res.Combined
		data = []byte(res.Stdout)
		if len(data) == 0 {
			data = []byte(res.Combined)
		}
	}

	var stanOutput struct {
		Totals struct {
			Errors     int `json:"errors"`
			FileErrors int `json:"file_errors"`
		} `json:"totals"`
		Files map[string]struct {
			Errors   int `json:"errors"`
			Messages []struct {
				Message    string `json:"message"`
				Line       int    `json:"line"`
				Ignorable  bool   `json:"ignorable"`
				Tip        string `json:"tip"`
				Identifier string `json:"identifier"`
			} `json:"messages"`
		} `json:"files"`
	}

	var findings []QualityFinding
	if err := json.Unmarshal(data, &stanOutput); err != nil {
		return nil, fmt.Errorf("invalid PHPStan JSON: %w", err)
	}
	{
		for fPath, fData := range stanOutput.Files {
			relPath := fPath
			if strings.HasPrefix(fPath, projectPath) {
				relPath = strings.TrimPrefix(strings.TrimPrefix(fPath, projectPath), "\\")
				relPath = strings.TrimPrefix(relPath, "/")
			}
			for _, m := range fData.Messages {
				findings = append(findings, QualityFinding{
					Tool:       "PHPStan",
					Rule:       m.Identifier,
					Severity:   "error",
					File:       relPath,
					Line:       m.Line,
					Message:    m.Message,
					Suggestion: m.Tip,
				})
			}
		}
	}

	if (err != nil || (res != nil && res.ExitCode != 0)) && len(findings) == 0 {
		return &QualityReport{
			Tool:      "PHPStan",
			RawOutput: rawOutput,
			Duration:  dur,
		}, fmt.Errorf("PHPStan exited with code %d: %s", res.ExitCode, rawOutput)
	}

	return &QualityReport{
		Tool:        "PHPStan",
		Findings:    findings,
		TotalErrors: len(findings),
		RawOutput:   string(data),
		Duration:    dur,
	}, nil
}

// AuditConfiguration checks configuration for development vs deployment best practices.
func AuditConfiguration(envValues map[string]string, phpSettings map[string]string) []ConfigAuditItem {
	var items []ConfigAuditItem

	// APP_DEBUG
	appDebug := strings.ToLower(envValues["APP_DEBUG"])
	if appDebug == "true" {
		items = append(items, ConfigAuditItem{
			Category:    "security",
			Key:         "APP_DEBUG",
			Value:       "true",
			Status:      "info",
			Message:     "Debug mode is enabled (safe for local development, must be false in production).",
			FixGuidance: "Set APP_DEBUG=false before deploying to production.",
		})
	} else {
		items = append(items, ConfigAuditItem{
			Category:    "security",
			Key:         "APP_DEBUG",
			Value:       "false",
			Status:      "good",
			Message:     "Debug mode is disabled.",
			FixGuidance: "",
		})
	}

	// APP_KEY
	appKey := envValues["APP_KEY"]
	if appKey == "" {
		items = append(items, ConfigAuditItem{
			Category:    "security",
			Key:         "APP_KEY",
			Value:       "(missing)",
			Status:      "warning",
			Message:     "Application encryption key is missing. Sessions and encryption will fail.",
			FixGuidance: "Run 'php artisan key:generate' to create an encryption key.",
		})
	} else {
		items = append(items, ConfigAuditItem{
			Category:    "security",
			Key:         "APP_KEY",
			Value:       "Configured",
			Status:      "good",
			Message:     "Application encryption key is set.",
			FixGuidance: "",
		})
	}

	// CACHE_DRIVER / CACHE_STORE
	cacheDriver := envValues["CACHE_STORE"]
	if cacheDriver == "" {
		cacheDriver = envValues["CACHE_DRIVER"]
	}
	if cacheDriver == "file" || cacheDriver == "" {
		items = append(items, ConfigAuditItem{
			Category:    "performance",
			Key:         "CACHE_STORE",
			Value:       "file",
			Status:      "info",
			Message:     "File cache driver is used. For higher concurrency, consider Redis or Memcached.",
			FixGuidance: "Set CACHE_STORE=redis when Redis service is running.",
		})
	} else if cacheDriver == "redis" {
		items = append(items, ConfigAuditItem{
			Category:    "performance",
			Key:         "CACHE_STORE",
			Value:       "redis",
			Status:      "good",
			Message:     "In-memory Redis cache store is configured.",
			FixGuidance: "",
		})
	}

	// OPcache
	if opVal, ok := phpSettings["opcache.enable"]; ok {
		if opVal == "1" || strings.ToLower(opVal) == "on" {
			items = append(items, ConfigAuditItem{
				Category:    "performance",
				Key:         "opcache.enable",
				Value:       opVal,
				Status:      "good",
				Message:     "PHP OPcache bytecode caching is enabled.",
				FixGuidance: "",
			})
		} else {
			items = append(items, ConfigAuditItem{
				Category:    "performance",
				Key:         "opcache.enable",
				Value:       opVal,
				Status:      "warning",
				Message:     "PHP OPcache is disabled. Enabling it significantly improves PHP response times.",
				FixGuidance: "Enable opcache.enable=1 in php.ini.",
			})
		}
	}

	// memory_limit
	if memLimit, ok := phpSettings["memory_limit"]; ok {
		items = append(items, ConfigAuditItem{
			Category:    "environment",
			Key:         "memory_limit",
			Value:       memLimit,
			Status:      "good",
			Message:     fmt.Sprintf("PHP CLI memory limit is set to %s.", memLimit),
			FixGuidance: "",
		})
	}

	return items
}

//go:embed scripts/analyze.php
var analyzeScriptFS []byte

type CodeBestPracticeFinding struct {
	Fingerprint string   `json:"fingerprint,omitempty"`
	Tool        string   `json:"tool"`
	Tools       []string `json:"tools"`
	Kind        string   `json:"kind"`
	Certainty   string   `json:"certainty"`
	Category    string   `json:"category"`
	Rule        string   `json:"rule"`
	Severity    string   `json:"severity"` // "error", "warning", "info"
	File        string   `json:"file"`
	Line        int      `json:"line"`
	Message     string   `json:"message"`
	Snippet     string   `json:"snippet,omitempty"`
	FixGuidance string   `json:"fixGuidance,omitempty"`
	GithubRef   string   `json:"githubRef,omitempty"`
}

type ProjectContext struct {
	ProjectPath           string `json:"projectPath"`
	HasInertiaV2          bool   `json:"hasInertiaV2"`
	UsesOctaneOrWorkerman bool   `json:"usesOctaneOrWorkerman"`
	HasEchoOrReverb       bool   `json:"hasEchoOrReverb"`
}

type PHPAnalyzePayload struct {
	Files        []string       `json:"files"`
	ContextFiles []string       `json:"contextFiles,omitempty"`
	Context      ProjectContext `json:"context"`
}

func DetectProjectContext(projectPath string) ProjectContext {
	pCtx := ProjectContext{
		ProjectPath: projectPath,
	}

	// 1. Inspect composer.lock
	lockPath := filepath.Join(projectPath, "composer.lock")
	if data, err := os.ReadFile(lockPath); err == nil {
		var lockData struct {
			Packages []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"packages"`
			PackagesDev []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"packages-dev"`
		}
		if json.Unmarshal(data, &lockData) == nil {
			allPackages := append(lockData.Packages, lockData.PackagesDev...)
			for _, pkg := range allPackages {
				name := strings.ToLower(pkg.Name)
				if name == "inertiajs/inertia-laravel" {
					ver := strings.TrimPrefix(pkg.Version, "v")
					if strings.HasPrefix(ver, "2.") || strings.HasPrefix(ver, "^2.") {
						pCtx.HasInertiaV2 = true
					}
				}
				if name == "laravel/octane" || strings.Contains(name, "workerman") {
					pCtx.UsesOctaneOrWorkerman = true
				}
				if name == "laravel/reverb" || strings.Contains(name, "pusher") {
					pCtx.HasEchoOrReverb = true
				}
			}
		}
	}

	// 2. Check composer.json fallback
	composerJsonPath := filepath.Join(projectPath, "composer.json")
	if data, err := os.ReadFile(composerJsonPath); err == nil {
		content := string(data)
		if strings.Contains(content, `"inertiajs/inertia-laravel": "^2.`) || strings.Contains(content, `"inertiajs/inertia-laravel": "2.`) {
			pCtx.HasInertiaV2 = true
		}
		if strings.Contains(content, "laravel/octane") || strings.Contains(content, "workerman") {
			pCtx.UsesOctaneOrWorkerman = true
		}
		if strings.Contains(content, "laravel/reverb") {
			pCtx.HasEchoOrReverb = true
		}
	}

	// 3. Check package.json for Echo / Reverb / Inertia
	pkgPath := filepath.Join(projectPath, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		content := string(data)
		if strings.Contains(content, "laravel-echo") || strings.Contains(content, "pusher-js") || strings.Contains(content, "@laravel/reverb") {
			pCtx.HasEchoOrReverb = true
		}
		if strings.Contains(content, `"@inertiajs/react": "^2.`) || strings.Contains(content, `"@inertiajs/vue3": "^2.`) {
			pCtx.HasInertiaV2 = true
		}
	}

	return pCtx
}

func collectPHPFiles(projectPath string) ([]string, error) {
	var files []string
	searchDirs := []string{"app", "config", "routes", "bootstrap"}
	for _, sub := range searchDirs {
		dir := filepath.Join(projectPath, sub)
		if _, err := os.Stat(dir); err != nil {
			if !os.IsNotExist(err) {
				return files, err
			}
			continue
		}
		walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.HasSuffix(info.Name(), ".php") {
				files = append(files, path)
			}
			return nil
		})
		if walkErr != nil {
			return files, walkErr
		}
	}
	return files, nil
}

func normalizeCategory(cat string) string {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "inertia", "inertia.js":
		return "Inertia.js"
	case "react", "react & ui", "ui":
		return "React & UI"
	case "realtime", "real-time", "real-time & sockets", "real-time & websockets", "sockets", "websockets":
		return "Real-Time & WebSockets"
	default:
		return "Laravel & PHP"
	}
}

func runSinglePHPBatch(ctx context.Context, phpBinary, scriptPath string, phpFiles, contextFiles []string, pCtx ProjectContext) ([]CodeBestPracticeFinding, error) {
	if len(phpFiles) == 0 {
		return nil, nil
	}

	payload := PHPAnalyzePayload{
		Files:        phpFiles,
		ContextFiles: contextFiles,
		Context:      pCtx,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize payload: %w", err)
	}

	payloadFile, err := os.CreateTemp("", "hwb-payload-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(payloadFile.Name())
	if _, err = payloadFile.Write(payloadBytes); err != nil {
		_ = payloadFile.Close()
		return nil, err
	}
	_ = payloadFile.Close()

	result, err := runner.CaptureExecContext(ctx, runner.Spec{Executable: phpBinary, Args: []string{scriptPath, payloadFile.Name()}, Directory: pCtx.ProjectPath}, 120*time.Second)
	if err != nil {
		return nil, fmt.Errorf("PHP analyzer did not finish: %w", err)
	}
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("PHP analyzer failed: %s", SanitizeAuditText(result.Stderr))
	}

	var findings []CodeBestPracticeFinding
	if err := json.Unmarshal([]byte(result.Stdout), &findings); err != nil {
		return nil, fmt.Errorf("failed to parse php analyzer output: %w", err)
	}

	return findings, nil
}

func runPHPBatchAnalyzer(ctx context.Context, phpBinary string, phpFiles []string, pCtx ProjectContext) ([]CodeBestPracticeFinding, error) {
	if len(phpFiles) == 0 {
		return nil, nil
	}
	if phpBinary == "" {
		phpBinary = "php"
	}

	tmpFile, err := os.CreateTemp("", "hwb-analyze-*.php")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp php script: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(analyzeScriptFS); err != nil {
		_ = tmpFile.Close()
		return nil, fmt.Errorf("failed to write embedded php script: %w", err)
	}
	_ = tmpFile.Close()

	if len(phpFiles) <= 100 {
		return runSinglePHPBatch(ctx, phpBinary, tmpPath, phpFiles, phpFiles, pCtx)
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > 6 {
		numWorkers = 6
	}
	if numWorkers < 2 {
		numWorkers = 2
	}

	chunkSize := (len(phpFiles) + numWorkers - 1) / numWorkers
	if chunkSize < 50 {
		chunkSize = 50
	}

	var chunks [][]string
	for i := 0; i < len(phpFiles); i += chunkSize {
		end := i + chunkSize
		if end > len(phpFiles) {
			end = len(phpFiles)
		}
		chunks = append(chunks, phpFiles[i:end])
	}

	type chunkResult struct {
		findings []CodeBestPracticeFinding
		err      error
	}
	results := make([]chunkResult, len(chunks))
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, files []string) {
			defer wg.Done()
			f, err := runSinglePHPBatch(ctx, phpBinary, tmpPath, files, phpFiles, pCtx)
			results[idx] = chunkResult{findings: f, err: err}
		}(i, chunk)
	}

	wg.Wait()

	var allFindings []CodeBestPracticeFinding
	var errs []error
	for _, res := range results {
		if res.err != nil {
			errs = append(errs, res.err)
		} else {
			allFindings = append(allFindings, res.findings...)
		}
	}

	return allFindings, errors.Join(errs...)
}

func scanBuiltinsRaw(ctx context.Context, phpBinary string, pCtx ProjectContext) ([]CodeBestPracticeFinding, error) {
	phpFiles, collectErr := collectPHPFiles(pCtx.ProjectPath)
	if collectErr != nil {
		return nil, collectErr
	}

	var (
		phpFindings   []CodeBestPracticeFinding
		bladeFindings []CodeBestPracticeFinding
		tsxFindings   []CodeBestPracticeFinding
		phpErr        error
		wg            sync.WaitGroup
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		phpFindings, phpErr = runPHPBatchAnalyzer(ctx, phpBinary, phpFiles, pCtx)
	}()

	go func() {
		defer wg.Done()
		bladeFindings, _ = ScanBladeAndStructure(pCtx.ProjectPath)
	}()

	go func() {
		defer wg.Done()
		tsxFindings, _ = ScanTSXAndReact(pCtx.ProjectPath)
	}()

	wg.Wait()

	if phpErr != nil && len(phpFiles) > 0 && len(bladeFindings) == 0 && len(tsxFindings) == 0 {
		return nil, phpErr
	}

	var all []CodeBestPracticeFinding
	all = append(all, phpFindings...)
	all = append(all, bladeFindings...)
	all = append(all, tsxFindings...)

	seen := make(map[string]bool)
	var deduped []CodeBestPracticeFinding

	for _, f := range all {
		f.Category = normalizeCategory(f.Category)
		f.File = filepath.ToSlash(f.File)
		key := fmt.Sprintf("%s:%d:%s", f.File, f.Line, f.Rule)
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, f)
		}
	}

	severityOrder := map[string]int{
		"error":   0,
		"warning": 1,
		"info":    2,
	}

	sort.SliceStable(deduped, func(i, j int) bool {
		si := severityOrder[deduped[i].Severity]
		sj := severityOrder[deduped[j].Severity]
		if si != sj {
			return si < sj
		}
		if deduped[i].File != deduped[j].File {
			return deduped[i].File < deduped[j].File
		}
		return deduped[i].Line < deduped[j].Line
	})

	if deduped == nil {
		deduped = make([]CodeBestPracticeFinding, 0)
	}

	return deduped, nil
}
