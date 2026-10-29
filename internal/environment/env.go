package environment

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var sensitiveKeyRegex = regexp.MustCompile(`(?i)(secret|password|token|key|auth|credential|api|private)`)

type EnvEntry struct {
	LineNumber   int    `json:"lineNumber"`
	Type         string `json:"type"` // "comment", "blank", "keyvalue"
	Raw          string `json:"raw"`
	Key          string `json:"key"`
	Value        string `json:"value"`
	MaskedValue  string `json:"maskedValue"`
	Quote        string `json:"quote"` // "'", `"`, or ""
	Comment      string `json:"comment"`
	IsSensitive  bool   `json:"isSensitive"`
	InitialKey   string `json:"initialKey,omitempty"`
	InitialValue string `json:"initialValue,omitempty"`
	InitialQuote string `json:"initialQuote,omitempty"`
}

type EnvFile struct {
	Path               string     `json:"path"`
	Fingerprint        string     `json:"fingerprint"`
	LineEnding         string     `json:"lineEnding"`
	HasTrailingNewline bool       `json:"hasTrailingNewline"`
	Entries            []EnvEntry `json:"entries"`
}

type EnvDiffItem struct {
	Key      string `json:"key"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
	Status   string `json:"status"` // "added", "modified", "deleted"
}

type DriftReport struct {
	MissingKeys []string `json:"missingKeys"` // Present in .env.example but not in .env
	ExtraKeys   []string `json:"extraKeys"`   // Present in .env but not in .env.example
	Matching    int      `json:"matching"`
}

// ParseEnv parses a .env file into tokenized entries preserving line endings and comments.
func ParseEnv(path string) (*EnvFile, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(bytes)
	fingerprint := hex.EncodeToString(hash[:])

	content := string(bytes)
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}
	hasTrailingNewline := strings.HasSuffix(content, "\n")

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" && hasTrailingNewline {
		lines = lines[:len(lines)-1]
	}

	entries := make([]EnvEntry, 0, len(lines))
	for idx := 0; idx < len(lines); idx++ {
		line := lines[idx]
		startLineNumber := idx + 1
		trimmed := strings.TrimSpace(line)
		entry := EnvEntry{
			LineNumber: startLineNumber,
			Raw:        line,
		}

		if trimmed == "" {
			entry.Type = "blank"
		} else if strings.HasPrefix(trimmed, "#") {
			entry.Type = "comment"
			entry.Comment = trimmed
		} else if strings.Contains(line, "=") {
			entry.Type = "keyvalue"
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			val := parts[1]

			quote := ""
			trimmedVal := strings.TrimSpace(val)
			if len(trimmedVal) > 0 && (trimmedVal[0] == '"' || trimmedVal[0] == '\'') {
				q := trimmedVal[0]
				end := -1
				escaped := false
				for i := 1; i < len(trimmedVal); i++ {
					if escaped {
						escaped = false
						continue
					}
					if trimmedVal[i] == '\\' && q == '"' {
						escaped = true
						continue
					}
					if trimmedVal[i] == q {
						end = i
						break
					}
				}
				if end < 0 {
					// Multiline quoted value
					var multilineParts []string
					multilineParts = append(multilineParts, trimmedVal[1:])
					closingIdx := -1
					var closingComment string

					for nextIdx := idx + 1; nextIdx < len(lines); nextIdx++ {
						nextLine := lines[nextIdx]
						cEnd := -1
						escapedNext := false
						for ci := 0; ci < len(nextLine); ci++ {
							if escapedNext {
								escapedNext = false
								continue
							}
							if nextLine[ci] == '\\' && q == '"' {
								escapedNext = true
								continue
							}
							if nextLine[ci] == q {
								cEnd = ci
								break
							}
						}
						if cEnd >= 0 {
							multilineParts = append(multilineParts, nextLine[:cEnd])
							closingIdx = nextIdx
							remainder := strings.TrimSpace(nextLine[cEnd+1:])
							if remainder != "" && !strings.HasPrefix(remainder, "#") {
								return nil, fmt.Errorf("invalid quoted .env value on line %d", nextIdx+1)
							}
							closingComment = remainder
							break
						} else {
							multilineParts = append(multilineParts, nextLine)
						}
					}

					if closingIdx < 0 {
						return nil, fmt.Errorf("unterminated multiline quoted .env value on line %d", startLineNumber)
					}

					quote = string(q)
					entry.Comment = closingComment
					contentVal := strings.Join(multilineParts, "\n")
					if q == '"' {
						contentVal = strings.ReplaceAll(strings.ReplaceAll(contentVal, `\"`, `"`), `\\`, `\`)
					}
					trimmedVal = contentVal
					entry.Raw = strings.Join(lines[idx:closingIdx+1], lineEnding)
					idx = closingIdx // Advance loop
				} else {
					remainder := strings.TrimSpace(trimmedVal[end+1:])
					if remainder != "" && !strings.HasPrefix(remainder, "#") {
						return nil, fmt.Errorf("invalid quoted .env value on line %d", startLineNumber)
					}
					entry.Comment = remainder
					quote = string(q)
					trimmedVal = trimmedVal[1:end]
					if q == '"' {
						trimmedVal = strings.ReplaceAll(strings.ReplaceAll(trimmedVal, `\"`, `"`), `\\`, `\`)
					}
				}
			} else if cIdx := strings.Index(trimmedVal, " #"); cIdx >= 0 {
				entry.Comment = strings.TrimSpace(trimmedVal[cIdx+1:])
				trimmedVal = strings.TrimSpace(trimmedVal[:cIdx])
			}

			entry.Key = key
			entry.Value = trimmedVal
			entry.Quote = quote
			entry.InitialKey = key
			entry.InitialValue = trimmedVal
			entry.InitialQuote = quote
			entry.IsSensitive = sensitiveKeyRegex.MatchString(key)
			if entry.IsSensitive && len(entry.Value) > 0 {
				entry.MaskedValue = "••••••••"
			} else {
				entry.MaskedValue = entry.Value
			}
		} else {
			entry.Type = "comment"
			entry.Comment = line
		}

		entries = append(entries, entry)
	}

	return &EnvFile{
		Path:               path,
		Fingerprint:        fingerprint,
		LineEnding:         lineEnding,
		HasTrailingNewline: hasTrailingNewline,
		Entries:            entries,
	}, nil
}

// Diff compares original and modified EnvFiles.
func Diff(orig, mod *EnvFile) []EnvDiffItem {
	origMap := make(map[string]string)
	for _, e := range orig.Entries {
		if e.Type == "keyvalue" {
			origMap[e.Key] = e.Value
		}
	}

	modMap := make(map[string]string)
	for _, e := range mod.Entries {
		if e.Type == "keyvalue" {
			modMap[e.Key] = e.Value
		}
	}

	var diffs []EnvDiffItem
	for k, newV := range modMap {
		oldV, exists := origMap[k]
		if !exists {
			diffs = append(diffs, EnvDiffItem{Key: k, OldValue: "", NewValue: newV, Status: "added"})
		} else if oldV != newV {
			diffs = append(diffs, EnvDiffItem{Key: k, OldValue: oldV, NewValue: newV, Status: "modified"})
		}
	}
	for k, oldV := range origMap {
		if _, exists := modMap[k]; !exists {
			diffs = append(diffs, EnvDiffItem{Key: k, OldValue: oldV, NewValue: "", Status: "deleted"})
		}
	}
	return diffs
}

// formatKeyValue safely formats an edited key-value pair with correct escaping and quoting.
func formatKeyValue(e EnvEntry) string {
	quote := e.Quote
	val := e.Value

	// Determine if quoting is required if unquoted
	if quote == "" {
		if strings.ContainsAny(val, " \t#'\"\\") || strings.Contains(val, "\n") || strings.Contains(val, "\r") || strings.HasPrefix(val, " ") || strings.HasSuffix(val, " ") {
			quote = `"`
		}
	}

	var valStr string
	if quote == `"` {
		escaped := strings.ReplaceAll(val, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		escaped = strings.ReplaceAll(escaped, "\n", `\n`)
		escaped = strings.ReplaceAll(escaped, "\r", `\r`)
		valStr = fmt.Sprintf(`"%s"`, escaped)
	} else if quote == `'` {
		if strings.Contains(val, `'`) || strings.Contains(val, "\n") {
			// Single quotes cannot escape single quotes in dotenv, fall back to double quotes
			escaped := strings.ReplaceAll(val, `\`, `\\`)
			escaped = strings.ReplaceAll(escaped, `"`, `\"`)
			escaped = strings.ReplaceAll(escaped, "\n", `\n`)
			escaped = strings.ReplaceAll(escaped, "\r", `\r`)
			valStr = fmt.Sprintf(`"%s"`, escaped)
		} else {
			valStr = fmt.Sprintf(`'%s'`, val)
		}
	} else {
		valStr = val
	}

	line := fmt.Sprintf("%s=%s", e.Key, valStr)
	if e.Comment != "" {
		line += " " + e.Comment
	}
	return line
}

// SaveEnv writes changes to disk after checking for concurrent modifications, creates a backup, and replaces atomically.
// It preserves untouched lines byte-for-byte and retains the file's original trailing-newline state.
var envSaveMu sync.Mutex
var envKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func SaveEnv(env *EnvFile, expectedFingerprint string) error {
	envSaveMu.Lock()
	defer envSaveMu.Unlock()
	if env == nil {
		return errors.New("env file is required")
	}
	if env.LineEnding != "\n" && env.LineEnding != "\r\n" {
		return errors.New("invalid line ending")
	}
	for _, entry := range env.Entries {
		if entry.Type == "keyvalue" && !envKey.MatchString(entry.Key) {
			return errors.New("invalid environment key")
		}
	}

	currentBytes, err := os.ReadFile(env.Path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		currentHash := sha256.Sum256(currentBytes)
		currentFingerprint := hex.EncodeToString(currentHash[:])
		if expectedFingerprint == "" {
			return errors.New("expected fingerprint must be provided to save .env safely")
		}
		if currentFingerprint != expectedFingerprint {
			return errors.New("concurrent modification detected: .env was changed externally. Reload before saving.")
		}

		original, parseErr := ParseEnv(env.Path)
		if parseErr != nil {
			return parseErr
		}
		for i := range env.Entries {
			e := &env.Entries[i]
			if e.LineNumber > 0 && e.LineNumber <= len(original.Entries) {
				orig := original.Entries[e.LineNumber-1]
				if e.Type != orig.Type {
					return errors.New("existing .env line type cannot be changed")
				}
				e.Raw, e.InitialKey, e.InitialValue, e.InitialQuote = orig.Raw, orig.Key, orig.Value, orig.Quote
				e.Comment = orig.Comment
			} else {
				e.Raw = ""
				e.InitialKey = ""
			}
		}
		env.LineEnding, env.HasTrailingNewline = original.LineEnding, original.HasTrailingNewline
		// Create backup and verify it succeeded
		dir := filepath.Dir(env.Path)
		backupName := fmt.Sprintf(".env.backup.%s", time.Now().Format("20060102150405.000000000"))
		backupPath := filepath.Join(dir, backupName)
		if err := os.WriteFile(backupPath, currentBytes, 0600); err != nil {
			return fmt.Errorf("failed to create backup before saving: %w", err)
		}
	}

	// Rebuild content preserving untouched lines verbatim
	var lines []string
	for _, e := range env.Entries {
		switch e.Type {
		case "blank":
			if e.Raw != "" {
				lines = append(lines, e.Raw)
			} else {
				lines = append(lines, "")
			}
		case "comment":
			lines = append(lines, e.Raw)
		case "keyvalue":
			// If untouched, preserve raw bytes exactly
			if e.Raw != "" && e.Key == e.InitialKey && e.Value == e.InitialValue && e.Quote == e.InitialQuote {
				lines = append(lines, e.Raw)
			} else {
				lines = append(lines, formatKeyValue(e))
			}
		default:
			if e.Raw != "" {
				lines = append(lines, e.Raw)
			}
		}
	}

	newContent := strings.Join(lines, env.LineEnding)
	if env.HasTrailingNewline {
		newContent += env.LineEnding
	}

	tmpPath := fmt.Sprintf("%s.tmp.%d", env.Path, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, []byte(newContent), 0600); err != nil {
		return err
	}

	defer os.Remove(tmpPath)
	if err := ReplaceFileAtomic(tmpPath, env.Path); err != nil {
		return err
	}

	// Update fingerprint in memory
	newHash := sha256.Sum256([]byte(newContent))
	env.Fingerprint = hex.EncodeToString(newHash[:])
	return nil
}

// CompareDrift compares .env against .env.example to find missing or extra keys without revealing secrets.
func CompareDrift(envPath, examplePath string) (DriftReport, error) {
	envFile, err := ParseEnv(envPath)
	if err != nil {
		return DriftReport{}, err
	}
	exampleFile, err := ParseEnv(examplePath)
	if err != nil {
		return DriftReport{}, err
	}

	envKeys := make(map[string]bool)
	for _, e := range envFile.Entries {
		if e.Type == "keyvalue" {
			envKeys[e.Key] = true
		}
	}

	exampleKeys := make(map[string]bool)
	for _, e := range exampleFile.Entries {
		if e.Type == "keyvalue" {
			exampleKeys[e.Key] = true
		}
	}

	var missing []string
	for k := range exampleKeys {
		if !envKeys[k] {
			missing = append(missing, k)
		}
	}

	var extra []string
	matching := 0
	for k := range envKeys {
		if !exampleKeys[k] {
			extra = append(extra, k)
		} else {
			matching++
		}
	}

	return DriftReport{
		MissingKeys: missing,
		ExtraKeys:   extra,
		Matching:    matching,
	}, nil
}

type SecretFinding struct {
	Key         string `json:"key"`
	LineNumber  int    `json:"lineNumber"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

var (
	stripeLiveRegex = regexp.MustCompile(`(sk|rk)_live_[0-9a-zA-Z]{24,}`)
	awsKeyRegex     = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	githubPatRegex  = regexp.MustCompile(`ghp_[0-9a-zA-Z]{36}`)
	slackHookRegex  = regexp.MustCompile(`https://hooks\.slack\.com/services/T[a-zA-Z0-9_]+/B[a-zA-Z0-9_]+/[a-zA-Z0-9_]+`)
	googleKeyRegex  = regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`)
)

// ScanSecrets identifies production secrets, cloud keys, and high-entropy credentials accidentally placed in local .env
func ScanSecrets(env *EnvFile) []SecretFinding {
	if env == nil {
		return nil
	}
	var findings []SecretFinding
	for _, e := range env.Entries {
		if e.Type != "keyvalue" {
			continue
		}
		val := e.Value

		if stripeLiveRegex.MatchString(val) {
			findings = append(findings, SecretFinding{
				Key:         e.Key,
				LineNumber:  e.LineNumber,
				Kind:        "stripe_live",
				Description: "Live Stripe API secret key detected in local environment configuration.",
				Severity:    "danger",
			})
		}
		if awsKeyRegex.MatchString(val) {
			findings = append(findings, SecretFinding{
				Key:         e.Key,
				LineNumber:  e.LineNumber,
				Kind:        "aws_access_key",
				Description: "Production AWS Access Key ID detected.",
				Severity:    "danger",
			})
		}
		if githubPatRegex.MatchString(val) {
			findings = append(findings, SecretFinding{
				Key:         e.Key,
				LineNumber:  e.LineNumber,
				Kind:        "github_token",
				Description: "Personal GitHub access token detected in environment.",
				Severity:    "danger",
			})
		}
		if slackHookRegex.MatchString(val) {
			findings = append(findings, SecretFinding{
				Key:         e.Key,
				LineNumber:  e.LineNumber,
				Kind:        "slack_webhook",
				Description: "Live Slack incoming webhook URL detected.",
				Severity:    "warning",
			})
		}
		if googleKeyRegex.MatchString(val) {
			findings = append(findings, SecretFinding{
				Key:         e.Key,
				LineNumber:  e.LineNumber,
				Kind:        "google_api_key",
				Description: "Live Google API credential detected.",
				Severity:    "warning",
			})
		}
	}
	return findings
}

