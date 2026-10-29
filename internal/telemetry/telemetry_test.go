package telemetry

import (
	"testing"
)

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "SELECT * FROM `users` WHERE `id` = 42",
			expected: "SELECT * FROM users WHERE id = ?",
		},
		{
			input:    "SELECT * FROM `posts` WHERE `user_id` = 10 AND status = 'published'",
			expected: "SELECT * FROM posts WHERE user_id = ? AND status = ?",
		},
		{
			input:    "SELECT * FROM tags WHERE id IN (1, 2, 3, 4)",
			expected: "SELECT * FROM tags WHERE id IN (?)",
		},
	}

	for _, tt := range tests {
		got := NormalizeQuery(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeQuery(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDetectNPlusOne(t *testing.T) {
	queries := []RawQueryItem{
		{Query: "SELECT * FROM users WHERE id = 1", Duration: 2.0},
		{Query: "SELECT * FROM posts WHERE user_id = 1", Duration: 5.0},
		{Query: "SELECT * FROM posts WHERE user_id = 2", Duration: 4.5},
		{Query: "SELECT * FROM posts WHERE user_id = 3", Duration: 6.0},
		{Query: "SELECT * FROM posts WHERE user_id = 4", Duration: 5.5},
	}

	report := DetectNPlusOne("req-123", "/users/posts", queries)

	if report.TotalLoops != 1 {
		t.Fatalf("expected 1 loop, got %d", report.TotalLoops)
	}

	issue := report.Issues[0]
	if issue.Count != 4 {
		t.Errorf("expected count 4, got %d", issue.Count)
	}
	if issue.Table != "posts" {
		t.Errorf("expected table 'posts', got %q", issue.Table)
	}
	if issue.Severity != "warning" {
		t.Errorf("expected severity 'warning', got %q", issue.Severity)
	}
	if report.WastedTime <= 0 {
		t.Errorf("expected positive wasted time, got %.2f", report.WastedTime)
	}
}

func TestBuildWaterfall(t *testing.T) {
	events := []RawTimelineEvent{
		{Description: "Booting framework", Start: 0, End: 10, Duration: 10},
		{Description: "App\\Http\\Controllers\\UserController@index", Start: 10, End: 35, Duration: 25},
	}
	queries := []RawQueryItem{
		{Query: "SELECT * FROM users", Duration: 8.0},
		{Query: "SELECT * FROM roles WHERE user_id = 1", Duration: 3.0},
	}

	waterfall := BuildWaterfall("req-456", 50.0, events, queries)

	if waterfall.RequestID != "req-456" {
		t.Errorf("expected req-456, got %s", waterfall.RequestID)
	}
	if len(waterfall.Segments) < 4 {
		t.Errorf("expected at least 4 segments, got %d", len(waterfall.Segments))
	}
	if waterfall.Breakdown["boot"] != 10 {
		t.Errorf("expected boot breakdown 10, got %.2f", waterfall.Breakdown["boot"])
	}
	if waterfall.Breakdown["database"] != 11.0 {
		t.Errorf("expected database breakdown 11.0, got %.2f", waterfall.Breakdown["database"])
	}
}
