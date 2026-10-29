package database

import (
	"encoding/json"
	"testing"
)

func TestExplainValidation(t *testing.T) {
	svc := NewDatabaseService()

	_, err := svc.ExplainQuery("", "C:\\project", "SELECT 1")
	if err == nil {
		t.Error("expected error for empty php binary in ExplainQuery")
	}

	_, err = svc.ExplainQuery("php", "", "SELECT 1")
	if err == nil {
		t.Error("expected error for empty project path in ExplainQuery")
	}

	_, err = svc.ExplainQuery("php", "C:\\project", "")
	if err == nil {
		t.Error("expected error for empty sql in ExplainQuery")
	}
}

func TestExplainResultSerialization(t *testing.T) {
	sample := ExplainResult{
		Query: "SELECT * FROM posts WHERE user_id = 5 ORDER BY created_at DESC",
		Score: 40,
		Bottlenecks: []ExplainBottleneck{
			{
				Type:       "full_table_scan",
				Table:      "posts",
				Details:    "Full table scan on 'posts' (type: ALL). Every row is inspected.",
				Severity:   "critical",
				Suggestion: "Add an index covering user_id",
			},
			{
				Type:       "filesort",
				Table:      "posts",
				Details:    "Query performs a filesort on disk or memory to sort records.",
				Severity:   "warning",
				Suggestion: "Create composite index on (user_id, created_at)",
			},
		},
		Summary: "2 performance bottlenecks detected.",
	}

	data, err := json.Marshal(sample)
	if err != nil {
		t.Fatalf("failed to marshal ExplainResult: %v", err)
	}

	var parsed ExplainResult
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal ExplainResult: %v", err)
	}

	if parsed.Score != 40 {
		t.Errorf("expected score 40, got %d", parsed.Score)
	}
	if len(parsed.Bottlenecks) != 2 {
		t.Errorf("expected 2 bottlenecks, got %d", len(parsed.Bottlenecks))
	}
}
