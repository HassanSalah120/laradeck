package quality

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type IncrementalDiffOptions struct {
	ProjectPath string
	BaseRef     string // e.g. "HEAD~1", "origin/main", or empty for uncommitted changes
}

// GetChangedProjectFiles identifies touched PHP and TSX files using git diff/status
func GetChangedProjectFiles(opts IncrementalDiffOptions) ([]string, error) {
	if opts.ProjectPath == "" {
		return nil, errors.New("empty project path")
	}

	gitDir := filepath.Join(opts.ProjectPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return nil, errors.New("not a git repository")
	}

	filesMap := make(map[string]bool)

	// 1. If BaseRef is provided, diff against that ref
	if opts.BaseRef != "" {
		cmd := exec.Command("git", "diff", "--name-only", opts.BaseRef)
		cmd.Dir = opts.ProjectPath
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			for _, line := range strings.Split(out.String(), "\n") {
				trimmed := strings.TrimSpace(line)
				if isAuditableFile(trimmed) {
					filesMap[filepath.Join(opts.ProjectPath, trimmed)] = true
				}
			}
		}
	} else {
		// 2. Diff against HEAD for unstaged/staged modifications
		cmd := exec.Command("git", "diff", "--name-only", "HEAD")
		cmd.Dir = opts.ProjectPath
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil {
			for _, line := range strings.Split(out.String(), "\n") {
				trimmed := strings.TrimSpace(line)
				if isAuditableFile(trimmed) {
					filesMap[filepath.Join(opts.ProjectPath, trimmed)] = true
				}
			}
		}

		// Also check untracked files with status --porcelain
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = opts.ProjectPath
		var statusOut bytes.Buffer
		statusCmd.Stdout = &statusOut
		if err := statusCmd.Run(); err == nil {
			for _, line := range strings.Split(statusOut.String(), "\n") {
				if len(line) > 3 {
					filePath := strings.TrimSpace(line[3:])
					if isAuditableFile(filePath) {
						filesMap[filepath.Join(opts.ProjectPath, filePath)] = true
					}
				}
			}
		}
	}

	var result []string
	for f := range filesMap {
		if _, err := os.Stat(f); err == nil {
			result = append(result, f)
		}
	}

	return result, nil
}

func isAuditableFile(relPath string) bool {
	ext := strings.ToLower(filepath.Ext(relPath))
	switch ext {
	case ".php", ".tsx", ".jsx", ".ts", ".js":
		// Ignore vendor and node_modules
		normalized := filepath.ToSlash(relPath)
		if strings.HasPrefix(normalized, "vendor/") || strings.HasPrefix(normalized, "node_modules/") || strings.HasPrefix(normalized, "dist/") {
			return false
		}
		return true
	default:
		return false
	}
}
