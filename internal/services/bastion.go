package services

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type BastionConfig struct {
	Name           string `json:"name"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	User           string `json:"user"`
	KeyPath        string `json:"keyPath"`
	RemoteDBHost   string `json:"remoteDbHost"`
	RemoteDBPort   int    `json:"remoteDbPort"`
	LocalPort      int    `json:"localPort"`
	StrictReadOnly bool   `json:"strictReadOnly"`
}

var destructiveSqlRegex = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|truncate|rename|grant|revoke|create|replace)\b`)
var allowedSqlPrefixes = regexp.MustCompile(`(?i)^\s*(select|explain|describe|show|set|pragma)\b`)

type QueryAuditResult struct {
	Allowed   bool   `json:"allowed"`
	SQL       string `json:"sql"`
	Violation string `json:"violation,omitempty"`
}

// ValidateQuerySafety evaluates whether a SQL statement is safe to execute in an audited read-only enclave
func ValidateQuerySafety(sql string, strictReadOnly bool) QueryAuditResult {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return QueryAuditResult{Allowed: true, SQL: sql}
	}

	if !strictReadOnly {
		return QueryAuditResult{Allowed: true, SQL: sql}
	}

	// Must begin with an allowed read-only verb
	if !allowedSqlPrefixes.MatchString(trimmed) {
		return QueryAuditResult{
			Allowed:   false,
			SQL:       sql,
			Violation: "Query rejected: read-only enclave only allows SELECT, EXPLAIN, DESCRIBE, and SHOW statements.",
		}
	}

	// Must not contain destructive keywords even in subqueries/CTEs
	if matches := destructiveSqlRegex.FindStringSubmatch(trimmed); len(matches) > 0 {
		return QueryAuditResult{
			Allowed:   false,
			SQL:       sql,
			Violation: fmt.Sprintf("Query rejected: detected mutation keyword '%s' inside statement.", matches[0]),
		}
	}

	return QueryAuditResult{
		Allowed: true,
		SQL:     sql,
	}
}

// BuildSSHTunnelArgs generates the OpenSSH port-forwarding command arguments
func BuildSSHTunnelArgs(cfg BastionConfig) ([]string, error) {
	if cfg.Host == "" || cfg.User == "" {
		return nil, errors.New("bastion host and user are required")
	}
	if cfg.LocalPort == 0 || cfg.RemoteDBPort == 0 {
		return nil, errors.New("local port and remote database port must be non-zero")
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}

	remoteHost := cfg.RemoteDBHost
	if remoteHost == "" {
		remoteHost = "127.0.0.1"
	}

	forward := fmt.Sprintf("%d:%s:%d", cfg.LocalPort, remoteHost, cfg.RemoteDBPort)
	target := fmt.Sprintf("%s@%s", cfg.User, cfg.Host)

	args := []string{
		"-N",
		"-L", forward,
		"-p", fmt.Sprintf("%d", port),
		target,
	}

	if cfg.KeyPath != "" {
		args = append([]string{"-i", cfg.KeyPath}, args...)
	}

	return args, nil
}
