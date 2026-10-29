package database

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

//go:embed scripts/model_explorer.php
var modelExplorerScript []byte

var (
	explorerPathCache string
	explorerMu        sync.Mutex
)

type ModelInfo struct {
	Class      string            `json:"class"`
	Name       string            `json:"name"`
	Table      string            `json:"table"`
	PrimaryKey string            `json:"primaryKey"`
	Fillable   []string          `json:"fillable"`
	Hidden     []string          `json:"hidden"`
	Casts      map[string]string `json:"casts"`
	Count      int               `json:"count"`
}

type RecordQueryResult struct {
	Records []map[string]any `json:"records"`
	Total   int              `json:"total"`
	Page    int              `json:"page"`
	PerPage int              `json:"perPage"`
	Columns []string         `json:"columns"`
	Error   string           `json:"error,omitempty"`
}

type ModelRecordUpdateResult struct {
	Success bool           `json:"success"`
	Record  map[string]any `json:"record,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type DatabaseService struct{}

func NewDatabaseService() *DatabaseService {
	return &DatabaseService{}
}

func ensureExplorerScript() (string, error) {
	explorerMu.Lock()
	defer explorerMu.Unlock()

	if explorerPathCache != "" {
		if _, err := os.Stat(explorerPathCache); err == nil {
			return explorerPathCache, nil
		}
	}

	tempDir := filepath.Join(os.TempDir(), "herd-workbench")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}

	targetPath := filepath.Join(tempDir, "model_explorer.php")
	if err := os.WriteFile(targetPath, modelExplorerScript, 0644); err != nil {
		return "", fmt.Errorf("failed to write model explorer script: %w", err)
	}

	explorerPathCache = targetPath
	return targetPath, nil
}

func (s *DatabaseService) ListModels(phpBinary, projectPath string) ([]ModelInfo, error) {
	return s.ListModelsContext(context.Background(), phpBinary, projectPath)
}

func (s *DatabaseService) ListModelsContext(ctx context.Context, phpBinary, projectPath string) ([]ModelInfo, error) {
	if phpBinary == "" || projectPath == "" {
		return nil, errors.New("PHP binary and project path are required")
	}

	script, err := ensureExplorerScript()
	if err != nil {
		return nil, err
	}

	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{script, projectPath, "list_models"},
		Directory:  projectPath,
	}, 20*time.Second)

	if err != nil {
		return nil, err
	}

	var parsed struct {
		Models []ModelInfo `json:"models"`
		Error  string      `json:"error"`
	}

	out := strings.TrimSpace(res.Stdout)
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse models: %s", out)
	}
	if parsed.Error != "" {
		return nil, errors.New(parsed.Error)
	}

	return parsed.Models, nil
}

func (s *DatabaseService) GetModelRecords(phpBinary, projectPath, modelClass string, page, perPage int, search, sortColumn, sortDirection string) (RecordQueryResult, error) {
	return s.GetModelRecordsContext(context.Background(), phpBinary, projectPath, modelClass, page, perPage, search, sortColumn, sortDirection)
}

func (s *DatabaseService) GetModelRecordsContext(ctx context.Context, phpBinary, projectPath, modelClass string, page, perPage int, search, sortColumn, sortDirection string) (RecordQueryResult, error) {
	if phpBinary == "" || projectPath == "" || modelClass == "" {
		return RecordQueryResult{}, errors.New("PHP binary, project path, and model class are required")
	}

	script, err := ensureExplorerScript()
	if err != nil {
		return RecordQueryResult{}, err
	}

	payload := map[string]any{
		"class":         modelClass,
		"page":          page,
		"perPage":       perPage,
		"search":        search,
		"sortColumn":    sortColumn,
		"sortDirection": sortDirection,
	}

	payloadBytes, _ := json.Marshal(payload)
	tempFile := filepath.Join(os.TempDir(), "herd-workbench", fmt.Sprintf("req-%d.json", time.Now().UnixNano()))
	_ = os.WriteFile(tempFile, payloadBytes, 0600)
	defer os.Remove(tempFile)

	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{script, projectPath, "get_records", tempFile},
		Directory:  projectPath,
	}, 20*time.Second)

	if err != nil {
		return RecordQueryResult{Error: err.Error()}, err
	}

	var parsed RecordQueryResult
	out := strings.TrimSpace(res.Stdout)
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return RecordQueryResult{Error: fmt.Sprintf("Failed to parse records: %s", out)}, nil
	}

	return parsed, nil
}

func (s *DatabaseService) UpdateModelRecord(phpBinary, projectPath, modelClass string, id any, attributes map[string]any) (ModelRecordUpdateResult, error) {
	return s.UpdateModelRecordContext(context.Background(), phpBinary, projectPath, modelClass, id, attributes)
}

func (s *DatabaseService) UpdateModelRecordContext(ctx context.Context, phpBinary, projectPath, modelClass string, id any, attributes map[string]any) (ModelRecordUpdateResult, error) {
	if phpBinary == "" || projectPath == "" || modelClass == "" || id == nil {
		return ModelRecordUpdateResult{}, errors.New("PHP binary, project path, model class, and record ID are required")
	}

	script, err := ensureExplorerScript()
	if err != nil {
		return ModelRecordUpdateResult{}, err
	}

	payload := map[string]any{
		"class":      modelClass,
		"id":         id,
		"attributes": attributes,
	}

	payloadBytes, _ := json.Marshal(payload)
	tempFile := filepath.Join(os.TempDir(), "herd-workbench", fmt.Sprintf("req-%d.json", time.Now().UnixNano()))
	_ = os.WriteFile(tempFile, payloadBytes, 0600)
	defer os.Remove(tempFile)

	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{script, projectPath, "update_record", tempFile},
		Directory:  projectPath,
	}, 20*time.Second)

	if err != nil {
		return ModelRecordUpdateResult{Success: false, Error: err.Error()}, err
	}

	var parsed ModelRecordUpdateResult
	out := strings.TrimSpace(res.Stdout)
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return ModelRecordUpdateResult{Success: false, Error: fmt.Sprintf("Failed to parse update result: %s", out)}, nil
	}

	return parsed, nil
}

func (s *DatabaseService) DeleteModelRecord(phpBinary, projectPath, modelClass string, id any) (bool, error) {
	return s.DeleteModelRecordContext(context.Background(), phpBinary, projectPath, modelClass, id)
}

func (s *DatabaseService) DeleteModelRecordContext(ctx context.Context, phpBinary, projectPath, modelClass string, id any) (bool, error) {
	if phpBinary == "" || projectPath == "" || modelClass == "" || id == nil {
		return false, errors.New("PHP binary, project path, model class, and record ID are required")
	}

	script, err := ensureExplorerScript()
	if err != nil {
		return false, err
	}

	payload := map[string]any{
		"class": modelClass,
		"id":    id,
	}

	payloadBytes, _ := json.Marshal(payload)
	tempFile := filepath.Join(os.TempDir(), "herd-workbench", fmt.Sprintf("req-%d.json", time.Now().UnixNano()))
	_ = os.WriteFile(tempFile, payloadBytes, 0600)
	defer os.Remove(tempFile)

	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{script, projectPath, "delete_record", tempFile},
		Directory:  projectPath,
	}, 20*time.Second)

	if err != nil {
		return false, err
	}

	var parsed struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	out := strings.TrimSpace(res.Stdout)
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return false, fmt.Errorf("failed to parse delete response: %s", out)
	}
	if parsed.Error != "" {
		return false, errors.New(parsed.Error)
	}

	return parsed.Success, nil
}
