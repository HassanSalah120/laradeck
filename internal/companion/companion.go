// Package companion owns local Laravel inspection and companion artifacts.
package companion

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"herdworkbench/internal/runner"
	"herdworkbench/internal/storage"
)

//go:embed bridge.php
var bridge []byte

type Store struct {
	mu   sync.Mutex
	Root string
}

func NewStore(root string) *Store {
	if root == "" {
		root, _ = os.UserConfigDir()
		root = filepath.Join(root, "HerdWorkbench", "companion")
	}
	return &Store{Root: root}
}

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

func (s *Store) Path(project, name string) (string, error) {
	if !safeID.MatchString(project) || !safeID.MatchString(name) {
		return "", errors.New("Invalid artifact identity")
	}
	dir := filepath.Join(s.Root, project)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}
func (s *Store) Save(project, name string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.Path(project, name)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".artifact-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_ = f.Chmod(0600)
	if _, err = f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func (s *Store) Load(project, name string, target any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	path, err := s.Path(project, name)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// Run starts only the embedded bridge, inside the existing owned process runner.
func Run(ctx context.Context, php, project, operation string, input map[string]any, timeout time.Duration) (json.RawMessage, error) {
	dir, err := os.MkdirTemp("", "laradeck-companion-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	script := filepath.Join(dir, "bridge.php")
	if err = os.WriteFile(script, bridge, 0600); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	request := filepath.Join(dir, "request.json")
	if err = os.WriteFile(request, payload, 0600); err != nil {
		return nil, err
	}
	res, err := runner.CaptureExecContext(ctx, runner.Spec{Executable: php, Directory: project, Args: []string{script, project, operation, request}}, timeout)
	if err != nil {
		return nil, fmt.Errorf("Companion operation failed: %s", storage.RedactSensitive(err.Error()))
	}
	if res.Truncated {
		return nil, errors.New("Result exceeds capture limit; narrow the selection")
	}
	var result struct {
		Data  json.RawMessage `json:"data"`
		Error string          `json:"error"`
	}
	if json.Unmarshal([]byte(strings.TrimSpace(res.Stdout)), &result) != nil {
		return nil, errors.New("Laravel bridge returned invalid output; inspect the project's bootstrap/logs")
	}
	if result.Error != "" {
		return nil, errors.New(storage.RedactSensitive(result.Error))
	}
	return result.Data, nil
}

type Finding struct {
	Fingerprint string `json:"fingerprint"`
	Rule        string `json:"rule"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Message     string `json:"message"`
}

// Identity ignores line numbers but includes surrounding source. Changed source
// requires review again; moving unchanged source keeps its identity.
func Fingerprint(root, file, rule, message string, line int) string {
	anchor := message
	clean := filepath.Clean(filepath.FromSlash(file))
	if !filepath.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		if data, err := os.ReadFile(filepath.Join(root, clean)); err == nil {
			lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
			if line > 0 && line <= len(lines) {
				lo, hi := max(0, line-3), min(len(lines), line+2)
				anchor = strings.Join(strings.Fields(strings.Join(lines[lo:hi], "\n")), " ")
			}
		}
	}
	hash := sha256.Sum256([]byte(filepath.ToSlash(clean) + "\x00" + rule + "\x00" + message + "\x00" + anchor))
	return hex.EncodeToString(hash[:16])
}

type Change struct {
	Status  string  `json:"status"`
	Finding Finding `json:"finding"`
}

func Diff(before, after []Finding) []Change {
	old := map[string]Finding{}
	now := map[string]Finding{}
	for _, f := range before {
		old[f.Fingerprint] = f
	}
	for _, f := range after {
		now[f.Fingerprint] = f
	}
	result := []Change{}
	for key, f := range now {
		status := "new"
		if _, ok := old[key]; ok {
			status = "recurring"
		}
		result = append(result, Change{status, f})
	}
	for key, f := range old {
		if _, ok := now[key]; !ok {
			result = append(result, Change{"not_observed", f})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Finding.File+result[i].Finding.Rule < result[j].Finding.File+result[j].Finding.Rule
	})
	return result
}

type Performance struct {
	Method           string   `json:"method"`
	URI              string   `json:"uri"`
	Duration         float64  `json:"duration"`
	DatabaseDuration float64  `json:"databaseDuration"`
	QueryCount       int      `json:"queryCount"`
	Memory           *float64 `json:"memory"`
	Queries          []string `json:"queries"`
}

var quoted = regexp.MustCompile(`'(?:''|[^'])*'|\b\d+(?:\.\d+)?\b`)

func NormalizeQuery(sql string) string {
	return strings.Join(strings.Fields(quoted.ReplaceAllString(sql, "?")), " ")
}

type Comparison struct {
	DurationDelta float64  `json:"durationDelta"`
	DatabaseDelta float64  `json:"databaseDelta"`
	QueryDelta    int      `json:"queryDelta"`
	MemoryDelta   *float64 `json:"memoryDelta"`
	Added         []string `json:"added"`
	Removed       []string `json:"removed"`
}

func Compare(a, b Performance) (Comparison, error) {
	if a.Method != b.Method || a.URI != b.URI {
		return Comparison{}, errors.New("Select the same method and URI as the baseline")
	}
	result := Comparison{DurationDelta: b.Duration - a.Duration, DatabaseDelta: b.DatabaseDuration - a.DatabaseDuration, QueryDelta: b.QueryCount - a.QueryCount, Added: []string{}, Removed: []string{}}
	if a.Memory != nil && b.Memory != nil {
		d := *b.Memory - *a.Memory
		result.MemoryDelta = &d
	}
	counts := map[string]int{}
	for _, q := range a.Queries {
		counts[NormalizeQuery(q)]--
	}
	for _, q := range b.Queries {
		counts[NormalizeQuery(q)]++
	}
	for q, n := range counts {
		if n > 0 {
			result.Added = append(result.Added, fmt.Sprintf("%dx %s", n, q))
		}
		if n < 0 {
			result.Removed = append(result.Removed, fmt.Sprintf("%dx %s", -n, q))
		}
	}
	sort.Strings(result.Added)
	sort.Strings(result.Removed)
	return result, nil
}
