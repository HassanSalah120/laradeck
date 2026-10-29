package quality

import (
	"bytes"
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"herdworkbench/internal/runner"
	"herdworkbench/internal/storage"
)

//go:embed scripts/browser.cjs
var browserScript []byte

type BrowserSessionStatus struct {
	Running bool   `json:"running"`
	Message string `json:"message"`
}
type BrowserSession struct {
	process                *runner.Process
	directory              string
	address, token, origin string
	mu                     sync.Mutex
	closeOnce              sync.Once
	cleanupError           string
	done                   chan struct{}
}

func LocalAuditURL(value, origin string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || u.User != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("Select an HTTP(S) local project URL without embedded credentials")
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" && !strings.HasSuffix(host, ".test") && !strings.HasSuffix(host, ".localhost") && !strings.HasSuffix(host, ".local") {
		return nil, fmt.Errorf("Browser auditing is limited to local project domains")
	}
	if origin != "" && u.Scheme+"://"+u.Host != origin {
		return nil, fmt.Errorf("Select a page on the project origin")
	}
	return u, nil
}
func StartBrowserSession(ctx context.Context, base string, headless bool) (*BrowserSession, error) {
	u, err := LocalAuditURL(base, "")
	if err != nil {
		return nil, err
	}
	runtime := BrowserRuntimeDir()
	if existing(runtime, "node_modules/playwright/cli.js") == "" || existing(runtime, "node_modules/axe-core/axe.min.js") == "" {
		return nil, fmt.Errorf("Browser runtime is missing; preview and install Browser accessibility setup in Settings")
	}
	node, err := NodeBinary()
	if err != nil {
		return nil, err
	}
	parent := filepath.Join(storage.AppDataDir(), "audit-sessions")
	if err := os.MkdirAll(parent, 0700); err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp(parent, "session-")
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			os.RemoveAll(directory)
		}
	}()
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(secret[:])
	config := map[string]any{"runtime": runtime, "url": u.String(), "origin": u.Scheme + "://" + u.Host, "token": token, "profile": filepath.Join(directory, "profile"), "headless": headless}
	data, _ := json.Marshal(config)
	if err := os.WriteFile(filepath.Join(directory, "config.json"), data, 0600); err != nil {
		return nil, err
	}
	script := filepath.Join(directory, "browser.cjs")
	if err := os.WriteFile(script, browserScript, 0600); err != nil {
		return nil, err
	}
	p, err := runner.Start(runner.Spec{Executable: node, Directory: directory, Args: []string{script, filepath.Join(directory, "config.json")}, Env: []string{"PLAYWRIGHT_BROWSERS_PATH=" + filepath.Join(runtime, "browsers")}})
	if err != nil {
		return nil, err
	}
	s := &BrowserSession{process: p, directory: directory, token: token, origin: u.Scheme + "://" + u.Host, done: make(chan struct{})}
	go func() {
		<-p.Done()
		p.Close()
		var cleanupErr error
		for attempt := 0; attempt < 5; attempt++ {
			cleanupErr = os.RemoveAll(directory)
			if cleanupErr == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if cleanupErr != nil {
			s.cleanupError = "Browser closed; temporary profile cleanup failed. Close any remaining browser processes and remove " + directory
		}
		close(s.done)
	}()
	deadline := time.NewTimer(35 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.Close()
			return nil, ctx.Err()
		case <-deadline.C:
			s.Close()
			return nil, fmt.Errorf("Browser startup timed out; recheck the private runtime")
		case <-s.done:
			return nil, fmt.Errorf("Browser runtime exited before becoming ready; recheck setup")
		case <-ticker.C:
			for _, event := range p.Events() {
				if event.Kind != "stdout" {
					continue
				}
				var ready struct {
					Event string `json:"event"`
					Port  int    `json:"port"`
				}
				if json.Unmarshal([]byte(event.Text), &ready) == nil && ready.Event == "ready" && ready.Port > 0 && ready.Port <= 65535 {
					s.address = net.JoinHostPort("127.0.0.1", fmt.Sprint(ready.Port))
					cleanup = false
					go func() {
						select {
						case <-ctx.Done():
							s.Close()
						case <-s.done:
						}
					}()
					return s, nil
				}
			}
		}
	}
}
func (s *BrowserSession) Done() <-chan struct{} { return s.done }

func (s *BrowserSession) Status() BrowserSessionStatus {
	if s == nil {
		return BrowserSessionStatus{Message: "Browser not opened"}
	}
	select {
	case <-s.done:
		if s.cleanupError != "" {
			return BrowserSessionStatus{Message: s.cleanupError}
		}
		return BrowserSessionStatus{Message: "Browser closed; temporary profile removed"}
	default:
		return BrowserSessionStatus{Running: true, Message: "Temporary browser session. Sign in manually, then explicitly audit a selected local page."}
	}
}
func (s *BrowserSession) post(ctx context.Context, path string, value any) ([]byte, error) {
	data, _ := json.Marshal(value)
	req, err := http.NewRequestWithContext(ctx, "POST", "http://"+s.address+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Workbench-Token", s.token)
	req.Header.Set("Content-Type", "application/json")
	client := http.Client{Timeout: 45 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Browser controller unavailable or timed out")
	}
	defer resp.Body.Close()
	output, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(output) > 4*1024*1024 {
		return nil, fmt.Errorf("Browser report exceeds limit")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Browser check failed (controller HTTP %d). Verify the page and session.", resp.StatusCode)
	}
	return output, nil
}
func (s *BrowserSession) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.address != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			s.post(ctx, "/close", map[string]any{})
			cancel()
		}
		if !s.process.Wait(3 * time.Second) {
			s.process.ForceStop()
			s.process.Wait(3 * time.Second)
		}
		select {
		case <-s.done:
		case <-time.After(2 * time.Second):
		}
	})
}
func (s *BrowserSession) Audit(ctx context.Context, address string) (AnalyzerResult, []CodeBestPracticeFinding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := AnalyzerResult{ID: "axe", Name: "axe-core browser accessibility", Version: BrowserAxeVersion, Coverage: "Explicitly selected rendered page only; manual accessibility review is still required"}
	u, err := LocalAuditURL(address, s.origin)
	if err != nil {
		return failedResult(result, err, nil), nil
	}
	start := time.Now()
	data, err := s.post(ctx, "/audit", map[string]string{"url": u.String()})
	result.Duration = time.Since(start).Seconds()
	if err != nil {
		return failedResult(result, err, nil), nil
	}
	var report struct {
		Status   string `json:"status"`
		Message  string `json:"message"`
		URL      string `json:"finalUrl"`
		Findings []struct {
			Rule      string `json:"rule"`
			Message   string `json:"message"`
			Reference string `json:"reference"`
			Impact    string `json:"impact"`
			Count     int    `json:"count"`
			Certainty string `json:"certainty"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return failedResult(result, fmt.Errorf("Invalid browser report"), nil), nil
	}
	result.Status = report.Status
	result.Message = SanitizeAuditText(report.Message)
	result.Configuration = report.URL
	findings := []CodeBestPracticeFinding{}
	for _, f := range report.Findings {
		severity := "warning"
		if f.Impact == "critical" || f.Impact == "serious" {
			severity = "error"
		}
		findings = append(findings, CodeBestPracticeFinding{Tool: "axe-core", Rule: f.Rule, Category: "Accessibility", Kind: "correctness", Certainty: f.Certainty, Severity: severity, Message: fmt.Sprintf("%s (%d affected elements)", f.Message, f.Count), FixGuidance: "Inspect the page in the temporary browser. Source HTML, form values and cookies are not included in this report.", GithubRef: f.Reference})
	}
	return result, findings
}
