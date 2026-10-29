package tinker

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"herdworkbench/internal/runner"
)

//go:embed scripts/tinker_runner.php
var tinkerRunnerScript []byte

var (
	runnerPathCache string
	runnerMu        sync.Mutex
)

type QueryLog struct {
	SQL        string `json:"sql"`
	TimeMs     float64 `json:"timeMs"`
	Bindings   []any  `json:"bindings,omitempty"`
	Connection string `json:"connection,omitempty"`
}

type EvaluationResult struct {
	Output     string     `json:"output"`
	DurationMs float64    `json:"durationMs"`
	Queries    []QueryLog `json:"queries"`
	Error      string     `json:"error,omitempty"`
}

type TinkerService struct{}

func NewTinkerService() *TinkerService {
	return &TinkerService{}
}

func ensureRunnerScript() (string, error) {
	runnerMu.Lock()
	defer runnerMu.Unlock()

	if runnerPathCache != "" {
		if _, err := os.Stat(runnerPathCache); err == nil {
			return runnerPathCache, nil
		}
	}

	tempDir := filepath.Join(os.TempDir(), "herd-workbench")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	targetPath := filepath.Join(tempDir, "tinker_runner.php")
	if err := os.WriteFile(targetPath, tinkerRunnerScript, 0644); err != nil {
		return "", fmt.Errorf("failed to write tinker runner script: %w", err)
	}

	runnerPathCache = targetPath
	return targetPath, nil
}

func (s *TinkerService) Evaluate(phpBinary, projectPath, code string) (EvaluationResult, error) {
	return s.EvaluateContext(context.Background(), phpBinary, projectPath, code, 30*time.Second)
}

func (s *TinkerService) EvaluateContext(ctx context.Context, phpBinary, projectPath, code string, timeout time.Duration) (EvaluationResult, error) {
	if strings.TrimSpace(code) == "" {
		return EvaluationResult{Output: "=> null", DurationMs: 0, Queries: []QueryLog{}}, nil
	}
	if phpBinary == "" {
		return EvaluationResult{}, errors.New("PHP binary path is required")
	}
	if projectPath == "" {
		return EvaluationResult{}, errors.New("project path is required")
	}

	runnerScript, err := ensureRunnerScript()
	if err != nil {
		return EvaluationResult{}, err
	}

	// Create temp file for user snippet to prevent command-line quoting issues on Windows
	tempDir := filepath.Join(os.TempDir(), "herd-workbench", "tinker-snippets")
	_ = os.MkdirAll(tempDir, 0755)
	codeFile := filepath.Join(tempDir, fmt.Sprintf("snippet-%d.php", time.Now().UnixNano()))
	if err := os.WriteFile(codeFile, []byte(code), 0600); err != nil {
		return EvaluationResult{}, fmt.Errorf("failed to write snippet to temp file: %w", err)
	}
	defer os.Remove(codeFile)

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	spec := runner.Spec{
		Executable: phpBinary,
		Args:       []string{runnerScript, projectPath, codeFile},
		Directory:  projectPath,
	}

	execRes, err := runner.CaptureExecContext(ctx, spec, timeout)
	if err != nil {
		return EvaluationResult{
			Output:     execResCombined(execRes),
			DurationMs: 0,
			Queries:    []QueryLog{},
			Error:      err.Error(),
		}, err
	}

	rawOut := strings.TrimSpace(execRes.Stdout)
	var res EvaluationResult
	if err := json.Unmarshal([]byte(rawOut), &res); err != nil {
		// Attempt to extract JSON if PHP warnings/notices preceded it
		if startIdx := strings.Index(rawOut, "{\"output\":"); startIdx != -1 {
			if endIdx := strings.LastIndex(rawOut, "}"); endIdx > startIdx {
				jsonPart := rawOut[startIdx : endIdx+1]
				if json.Unmarshal([]byte(jsonPart), &res) == nil {
					return res, nil
				}
			}
		}
		// If cannot parse JSON, return raw output
		return EvaluationResult{
			Output:     execResCombined(execRes),
			DurationMs: 0,
			Queries:    []QueryLog{},
			Error:      fmt.Sprintf("Failed to parse Tinker result: %s", rawOut),
		}, nil
	}

	return res, nil
}

func execResCombined(r *runner.ExecutionResult) string {
	if r == nil {
		return ""
	}
	if r.Stdout != "" && r.Stderr != "" {
		return r.Stdout + "\n" + r.Stderr
	}
	if r.Stdout != "" {
		return r.Stdout
	}
	return r.Stderr
}
