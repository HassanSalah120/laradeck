package database

import (
	"encoding/json"
	"testing"
)

func TestERDValidation(t *testing.T) {
	svc := NewDatabaseService()

	_, err := svc.GetErdSchema("", "C:\\project")
	if err == nil {
		t.Error("expected error for empty php binary in GetErdSchema")
	}

	_, err = svc.GetErdSchema("php", "")
	if err == nil {
		t.Error("expected error for empty project path in GetErdSchema")
	}
}

func TestERDSchemaSerialization(t *testing.T) {
	sample := ERDSchemaResult{
		Tables: []ERDTable{
			{
				ModelClass: "App\\Models\\User",
				ModelName:  "User",
				Table:      "users",
				PrimaryKey: "id",
				RowCount:   15,
				Columns: []ERDColumn{
					{Name: "id", Type: "bigint", IsPrimary: true},
					{Name: "name", Type: "varchar"},
					{Name: "email", Type: "varchar"},
				},
				Indexes: []ERDIndex{
					{Name: "PRIMARY", Columns: []string{"id"}, Primary: true, Unique: true},
					{Name: "users_email_unique", Columns: []string{"email"}, Unique: true},
				},
			},
			{
				ModelClass: "App\\Models\\Post",
				ModelName:  "Post",
				Table:      "posts",
				PrimaryKey: "id",
				RowCount:   42,
				Columns: []ERDColumn{
					{Name: "id", Type: "bigint", IsPrimary: true},
					{Name: "user_id", Type: "bigint", IsForeign: true},
					{Name: "title", Type: "varchar"},
				},
			},
		},
		Relations: []ERDRelation{
			{
				Name:        "posts",
				Type:        "HasMany",
				SourceModel: "App\\Models\\User",
				SourceTable: "users",
				TargetModel: "App\\Models\\Post",
				TargetTable: "posts",
				ForeignKey:  "user_id",
				LocalKey:    "id",
			},
		},
		Advisories: []ERDAdvisorItem{
			{
				Table:        "posts",
				Column:       "user_id",
				Issue:        "Unindexed Foreign Key Column",
				Severity:     "warning",
				Suggestion:   "Add index on user_id",
				MigrationSQL: "$table->index('user_id');",
			},
		},
	}

	data, err := json.Marshal(sample)
	if err != nil {
		t.Fatalf("failed to marshal ERDSchemaResult: %v", err)
	}

	var parsed ERDSchemaResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal ERDSchemaResult: %v", err)
	}

	if len(parsed.Tables) != 2 {
		t.Errorf("expected 2 tables, got %d", len(parsed.Tables))
	}
	if len(parsed.Relations) != 1 {
		t.Errorf("expected 1 relation, got %d", len(parsed.Relations))
	}
	if len(parsed.Advisories) != 1 {
		t.Errorf("expected 1 advisory, got %d", len(parsed.Advisories))
	}
}
