package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"herdworkbench/internal/runner"
)

type ERDColumn struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Nullable  bool   `json:"nullable"`
	Default   string `json:"default,omitempty"`
	IsPrimary bool   `json:"isPrimary"`
	IsForeign bool   `json:"isForeign"`
}

type ERDIndex struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
}

type ERDRelation struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	SourceModel string `json:"sourceModel"`
	SourceTable string `json:"sourceTable"`
	TargetModel string `json:"targetModel"`
	TargetTable string `json:"targetTable"`
	ForeignKey  string `json:"foreignKey"`
	LocalKey    string `json:"localKey"`
	PivotTable  string `json:"pivotTable,omitempty"`
}

type ERDTable struct {
	ModelClass string        `json:"modelClass"`
	ModelName  string        `json:"modelName"`
	Table      string        `json:"table"`
	PrimaryKey string        `json:"primaryKey"`
	Columns    []ERDColumn   `json:"columns"`
	Indexes    []ERDIndex    `json:"indexes"`
	RowCount   int           `json:"rowCount"`
	Relations  []ERDRelation `json:"relations"`
}

type ERDAdvisorItem struct {
	Table        string `json:"table"`
	Column       string `json:"column"`
	Issue        string `json:"issue"`
	Severity     string `json:"severity"`
	Suggestion   string `json:"suggestion"`
	MigrationSQL string `json:"migrationSql,omitempty"`
}

type ERDSchemaResult struct {
	Tables     []ERDTable       `json:"tables"`
	Relations  []ERDRelation    `json:"relations"`
	Advisories []ERDAdvisorItem `json:"advisories"`
	Error      string           `json:"error,omitempty"`
}

func (s *DatabaseService) GetErdSchema(phpBinary, projectPath string) (ERDSchemaResult, error) {
	return s.GetErdSchemaContext(context.Background(), phpBinary, projectPath)
}

func (s *DatabaseService) GetErdSchemaContext(ctx context.Context, phpBinary, projectPath string) (ERDSchemaResult, error) {
	if phpBinary == "" || projectPath == "" {
		return ERDSchemaResult{Error: "PHP binary and project path are required"}, errors.New("PHP binary and project path are required")
	}

	script, err := ensureExplorerScript()
	if err != nil {
		return ERDSchemaResult{Error: err.Error()}, err
	}

	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{script, projectPath, "get_erd_schema"},
		Directory:  projectPath,
	}, 30*time.Second)

	if err != nil {
		return ERDSchemaResult{Error: err.Error()}, err
	}

	out := strings.TrimSpace(res.Stdout)
	var parsed ERDSchemaResult
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return ERDSchemaResult{Error: fmt.Sprintf("Failed to parse ERD schema: %s", out)}, nil
	}

	if parsed.Error != "" {
		return parsed, errors.New(parsed.Error)
	}

	return parsed, nil
}
