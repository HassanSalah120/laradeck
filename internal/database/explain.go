package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"herdworkbench/internal/runner"
)

type ExplainBottleneck struct {
	Type       string `json:"type"`
	Table      string `json:"table"`
	Details    string `json:"details"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion"`
}

type ExplainResult struct {
	Query       string              `json:"query"`
	Raw         []map[string]any    `json:"raw"`
	Bottlenecks []ExplainBottleneck `json:"bottlenecks"`
	Score       int                 `json:"score"`
	Summary     string              `json:"summary"`
	Error       string              `json:"error,omitempty"`
}

func (s *DatabaseService) ExplainQuery(phpBinary, projectPath, sql string) (ExplainResult, error) {
	return s.ExplainQueryContext(context.Background(), phpBinary, projectPath, sql)
}

func (s *DatabaseService) ExplainQueryContext(ctx context.Context, phpBinary, projectPath, sql string) (ExplainResult, error) {
	if phpBinary == "" || projectPath == "" || strings.TrimSpace(sql) == "" {
		return ExplainResult{Error: "PHP binary, project path, and query are required"}, errors.New("PHP binary, project path, and query are required")
	}

	script, err := ensureExplorerScript()
	if err != nil {
		return ExplainResult{Error: err.Error()}, err
	}

	payload := map[string]any{
		"query": sql,
	}

	payloadBytes, _ := json.Marshal(payload)
	tempFile := filepath.Join(os.TempDir(), "herd-workbench", fmt.Sprintf("explain-%d.json", time.Now().UnixNano()))
	_ = os.WriteFile(tempFile, payloadBytes, 0600)
	defer os.Remove(tempFile)

	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{script, projectPath, "explain_query", tempFile},
		Directory:  projectPath,
	}, 20*time.Second)

	if err != nil {
		return ExplainResult{Error: err.Error()}, err
	}

	out := strings.TrimSpace(res.Stdout)
	var parsed ExplainResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return ExplainResult{Error: fmt.Sprintf("Failed to parse explain result: %s", out)}, nil
	}

	if parsed.Error != "" {
		return parsed, errors.New(parsed.Error)
	}

	return parsed, nil
}
