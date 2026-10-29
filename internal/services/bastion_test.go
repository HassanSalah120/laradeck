package services

import (
	"strings"
	"testing"
)

func TestValidateQuerySafety(t *testing.T) {
	// Safe queries
	safeQueries := []string{
		"SELECT * FROM users WHERE id = 1",
		"EXPLAIN ANALYZE SELECT count(*) FROM orders",
		"SHOW TABLES",
		"DESCRIBE products",
	}

	for _, q := range safeQueries {
		res := ValidateQuerySafety(q, true)
		if !res.Allowed {
			t.Errorf("expected query to be allowed: %s (violation: %s)", q, res.Violation)
		}
	}

	// Destructive or mutation queries
	unsafeQueries := []string{
		"DELETE FROM users WHERE id = 1",
		"UPDATE users SET role = 'admin' WHERE id = 2",
		"DROP TABLE audit_logs",
		"ALTER TABLE users ADD COLUMN is_admin boolean",
		"TRUNCATE sessions",
		"INSERT INTO settings (key, value) VALUES ('env', 'prod')",
		"SELECT * FROM users; DROP TABLE orders;",
	}

	for _, q := range unsafeQueries {
		res := ValidateQuerySafety(q, true)
		if res.Allowed {
			t.Errorf("expected unsafe query to be rejected: %s", q)
		}
	}

	// When strictReadOnly is false, all are allowed
	for _, q := range unsafeQueries {
		res := ValidateQuerySafety(q, false)
		if !res.Allowed {
			t.Errorf("expected query allowed when strictReadOnly is false: %s", q)
		}
	}
}

func TestBuildSSHTunnelArgs(t *testing.T) {
	cfg := BastionConfig{
		Host:         "bastion.example.com",
		User:         "ubuntu",
		Port:         2222,
		RemoteDBHost: "rds.internal.example.com",
		RemoteDBPort: 5432,
		LocalPort:    15432,
		KeyPath:      `C:\keys\staging.pem`,
	}

	args, err := BuildSSHTunnelArgs(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-L 15432:rds.internal.example.com:5432") {
		t.Errorf("missing port forwarding arg: %s", joined)
	}
	if !strings.Contains(joined, "ubuntu@bastion.example.com") {
		t.Errorf("missing user/host target: %s", joined)
	}
	if !strings.Contains(joined, "-i C:\\keys\\staging.pem") {
		t.Errorf("missing keypath arg: %s", joined)
	}
}
