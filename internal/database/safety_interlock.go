package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var protectedBranchRegex = regexp.MustCompile(`^(main|master|production|prod|release/.*)$`)

type BranchSafetyStatus struct {
	CurrentBranch     string `json:"currentBranch"`
	IsProtected       bool   `json:"isProtected"`
	OperationPermitted bool   `json:"operationPermitted"`
	Warning           string `json:"warning,omitempty"`
}

// GetCurrentBranch inspects .git/HEAD to resolve the branch name without executing git CLI
func GetCurrentBranch(projectPath string) string {
	headFile := filepath.Join(projectPath, ".git", "HEAD")
	data, err := os.ReadFile(headFile)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: refs/heads/") {
		return strings.TrimPrefix(content, "ref: refs/heads/")
	}
	if len(content) >= 7 {
		return content[:7] // Detached HEAD commit hash
	}
	return content
}

// AssessBranchSafety evaluates whether a destructive operation (wipe, fresh, restore) is safe on the current branch
func AssessBranchSafety(projectPath string, operation string) BranchSafetyStatus {
	branch := GetCurrentBranch(projectPath)
	if branch == "" {
		return BranchSafetyStatus{
			CurrentBranch:      "unknown",
			IsProtected:        false,
			OperationPermitted: true,
		}
	}

	isProtected := protectedBranchRegex.MatchString(branch)
	isDestructive := isDestructiveOperation(operation)

	if isProtected && isDestructive {
		return BranchSafetyStatus{
			CurrentBranch:      branch,
			IsProtected:        true,
			OperationPermitted: false,
			Warning:            fmt.Sprintf("Branch '%s' is marked protected. Destructive operation '%s' was blocked to prevent data loss.", branch, operation),
		}
	}

	return BranchSafetyStatus{
		CurrentBranch:      branch,
		IsProtected:        isProtected,
		OperationPermitted: true,
	}
}

func isDestructiveOperation(op string) bool {
	op = strings.ToLower(strings.TrimSpace(op))
	switch op {
	case "migrate:fresh", "fresh", "db:wipe", "wipe", "restore", "snapshot-restore", "reset":
		return true
	default:
		return false
	}
}
