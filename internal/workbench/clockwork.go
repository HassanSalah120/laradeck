package workbench

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/storage"
)

type Query struct {
	SQL        string  `json:"sql"`
	Duration   float64 `json:"duration"`
	Connection string  `json:"connection"`
	Repeated   bool    `json:"repeated"`
	Time       float64 `json:"time,omitempty"`
}
type TimelineEvent struct {
	Description string  `json:"description"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Duration    float64 `json:"duration"`
}
type Request struct {
	Memory           *float64        `json:"memory"`
	ID               string          `json:"id"`
	Method           string          `json:"method"`
	URI              string          `json:"uri"`
	Status           int             `json:"status"`
	Time             float64         `json:"time"`
	Duration         float64         `json:"duration"`
	DatabaseDuration float64         `json:"databaseDuration"`
	Queries          []Query         `json:"queries"`
	QueryCount       int             `json:"queryCount"`
	Timeline         []TimelineEvent `json:"timeline,omitempty"`
	Truncated        bool            `json:"truncated"`
	Exceptions       int             `json:"exceptions"`
}
type Capture struct {
	ProjectID    string    `json:"projectId"`
	Status       string    `json:"status"`
	SetupCommand string    `json:"setupCommand,omitempty"`
	Requests     []Request `json:"requests"`
	Watching     bool      `json:"watching"`
	Message      string    `json:"message"`
	Updated      string    `json:"updated"`
}
type rawRequest struct {
	Memory           *float64 `json:"memoryUsage"`
	ID               string   `json:"id"`
	Method           string   `json:"method"`
	URI              string   `json:"uri"`
	ResponseStatus   int      `json:"responseStatus"`
	Time             float64  `json:"time"`
	Duration         float64  `json:"responseDuration"`
	DatabaseDuration float64  `json:"databaseDuration"`
	Queries          []struct {
		Query      string  `json:"query"`
		Duration   float64 `json:"duration"`
		Connection string  `json:"connection"`
		Time       float64 `json:"time"`
	} `json:"databaseQueries"`
	Timeline []struct {
		Description string  `json:"description"`
		Start       float64 `json:"start"`
		End         float64 `json:"end"`
		Duration    float64 `json:"duration"`
	} `json:"timelineData"`
	Log []struct {
		Exception json.RawMessage `json:"exception"`
	} `json:"log"`
}

var literal = regexp.MustCompile(`'(?:''|[^'])*'`)
var credential = regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/=-]+|(password|token|secret|api[_-]?key|authorization|cookie)(\s*[:=]\s*)[^\s,;]+`)
var validCaptureID = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func Redact(s string) string {
	s = storage.RedactSensitive(s)
	s = credential.ReplaceAllString(s, "[redacted]")
	if len(s) > 16000 {
		s = s[:16000] + " …"
	}
	return s
}
func cleanRequest(r rawRequest) Request {
	uri := r.URI
	if parsed, err := url.Parse(uri); err == nil {
		uri = parsed.EscapedPath()
	}
	q := Request{
		Memory:           r.Memory,
		ID:               r.ID,
		Method:           r.Method,
		URI:              uri,
		Status:           r.ResponseStatus,
		Time:             r.Time,
		Duration:         r.Duration,
		DatabaseDuration: r.DatabaseDuration,
		QueryCount:       len(r.Queries),
		Queries:          []Query{},
		Timeline:         make([]TimelineEvent, 0, len(r.Timeline)),
	}
	for _, ev := range r.Timeline {
		q.Timeline = append(q.Timeline, TimelineEvent{
			Description: ev.Description,
			Start:       ev.Start,
			End:         ev.End,
			Duration:    ev.Duration,
		})
	}
	for _, entry := range r.Log {
		if len(entry.Exception) > 0 && string(entry.Exception) != "null" {
			q.Exceptions++
		}
	}
	counts := map[string]int{}
	for _, v := range r.Queries {
		key := strings.Join(strings.Fields(literal.ReplaceAllString(v.Query, "?")), " ")
		counts[key]++
	}
	for i, v := range r.Queries {
		if i >= 200 {
			q.Truncated = true
			break
		}
		sql := literal.ReplaceAllString(v.Query, "'[value]'")
		key := strings.Join(strings.Fields(literal.ReplaceAllString(v.Query, "?")), " ")
		q.Queries = append(q.Queries, Query{
			SQL:        Redact(sql),
			Duration:   v.Duration,
			Connection: v.Connection,
			Repeated:   counts[key] > 1,
			Time:       v.Time,
		})
	}
	return q
}

func localBase(base string) (*url.URL, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("Invalid local site URL")
	}
	host := strings.ToLower(u.Hostname())
	if host != "localhost" && host != "127.0.0.1" && host != "::1" && !strings.HasSuffix(host, ".test") && !strings.HasSuffix(host, ".localhost") && !strings.HasSuffix(host, ".local") {
		return nil, errors.New("Capture is limited to local project domains")
	}
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

type Clockwork struct {
	mu      sync.Mutex
	fetchMu sync.Mutex
	state   Capture
	base    string
	project Project
	cancel  context.CancelFunc
	notify  func(Capture)
}

func NewClockwork(notify func(Capture)) *Clockwork {
	return &Clockwork{notify: notify, state: Capture{Requests: []Request{}, Message: "Select a project to inspect requests"}}
}
func (c *Clockwork) State() Capture {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state
	s.Requests = append([]Request{}, s.Requests...)
	return s
}
func (c *Clockwork) setMessage(message string) {
	c.mu.Lock()
	c.state.Message = message
	c.mu.Unlock()
	c.publish()
}
func (c *Clockwork) publish() {
	if c.notify != nil {
		c.notify(c.State())
	}
}
func fetchMetadata(ctx context.Context, base, path string, target any) error {
	u, err := localBase(base)
	if err != nil {
		return err
	}
	u.Path += "/__clockwork/" + path
	transport := &http.Transport{Proxy: nil}
	// Herd's default loopback is reachable even before a new .test alias enters DNS.
	// Keep the original Host and TLS server name; never disable certificate checks.
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err == nil && (strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".localhost")) {
			address = net.JoinHostPort("127.0.0.1", port)
		}
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, address)
	}
	client := http.Client{Timeout: 8 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("Metadata redirected; check the project's local URL")
	}}
	defer client.CloseIdleConnections()
	for attempt := 0; attempt < 3; attempt++ {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		req.Header.Set("X-Companion-Internal", "1")
		req.Header.Set("X-Clockwork", "1")
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return &metadataError{kind: "unreachable", message: fmt.Sprintf("Cannot reach the local Clockwork endpoint: %v. Check that Herd and this site are running, and that the site URL and certificate are correct.", err)}
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
		resp.Body.Close()
		if len(data) > 8*1024*1024 {
			return errors.New("Capture exceeds the 8 MiB read limit")
		}
		if resp.StatusCode == 200 && readErr == nil && json.Unmarshal(data, target) == nil {
			return nil
		}
		if resp.StatusCode == 404 {
			return &metadataError{kind: "endpoint_unavailable", message: "Clockwork metadata endpoint /__clockwork/ is unavailable (HTTP 404). Check APP_DEBUG/CLOCKWORK_ENABLE, package auto-discovery, cached configuration and any custom Clockwork API path; then visit a project route and recheck."}
		}
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return &metadataError{kind: "access_denied", message: fmt.Sprintf("Clockwork denied metadata access (HTTP %d). Check Clockwork authentication and local access settings.", resp.StatusCode)}
		}
		if resp.StatusCode != 200 && resp.StatusCode != 404 {
			return fmt.Errorf("Clockwork returned HTTP %d; check collection and local access settings", resp.StatusCode)
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 120 * time.Millisecond):
			}
		}
	}
	return &metadataError{kind: "invalid_response", message: "Clockwork returned HTTP 200 without valid metadata JSON. Check whether middleware, a login page, or an SPA fallback is intercepting /__clockwork/."}
}
func (c *Clockwork) Refresh(ctx context.Context) (Capture, error) {
	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()
	c.mu.Lock()
	base := c.base
	project := c.project
	c.mu.Unlock()
	if base == "" {
		return c.State(), errors.New("Select a project first")
	}
	// Dependency metadata can change after Composer runs outside Workbench.
	if _, err := os.Stat(filepath.Join(project.Path, "composer.lock")); err == nil {
		InspectDependencies(&project)
	}
	if status, message, command := clockworkSetup(project); status != "" {
		c.mu.Lock()
		c.state.Status = status
		c.state.Message = message
		c.state.SetupCommand = command
		c.mu.Unlock()
		c.publish()
		return c.State(), nil
	}
	var latest rawRequest
	if err := fetchMetadata(ctx, base, "latest", &latest); err != nil {
		c.mu.Lock()
		c.state.Status = "error"
		c.state.SetupCommand = ""
		var problem *metadataError
		if errors.As(err, &problem) {
			c.state.Status = problem.kind
		}
		c.state.Message = err.Error()
		c.mu.Unlock()
		c.publish()
		if problem != nil && problem.kind == "endpoint_unavailable" {
			return c.State(), nil
		}
		return c.State(), err
	}
	if !validCaptureID.MatchString(latest.ID) {
		c.mu.Lock()
		c.state.Status = "no_captures"
		c.state.SetupCommand = ""
		c.state.Message = "Clockwork has no captures yet. Open this project in your browser, make a request, then refresh."
		c.mu.Unlock()
		c.publish()
		return c.State(), nil
	}
	var previous []rawRequest
	// The latest request is separate; history failure must not hide a valid latest capture.
	historyErr := fetchMetadata(ctx, base, latest.ID+"/previous/19", &previous)
	all := append(previous, latest)
	safe := make([]Request, 0, len(all))
	internal := false
	for _, r := range all {
		if strings.HasPrefix(r.URI, "/__clockwork") {
			internal = true
			continue
		}
		if validCaptureID.MatchString(r.ID) {
			safe = append(safe, cleanRequest(r))
		}
	}
	c.mu.Lock()
	if c.base != base {
		c.mu.Unlock()
		return c.State(), errors.New("Project changed during capture read")
	}
	byID := map[string]Request{}
	for _, r := range c.state.Requests {
		byID[r.ID] = r
	}
	for _, r := range safe {
		byID[r.ID] = r
	}
	c.state.Requests = make([]Request, 0, len(byID))
	for _, r := range byID {
		c.state.Requests = append(c.state.Requests, r)
	}
	sort.Slice(c.state.Requests, func(i, j int) bool { return c.state.Requests[i].Time > c.state.Requests[j].Time })
	if len(c.state.Requests) > 500 {
		c.state.Requests = c.state.Requests[:500]
	}
	c.state.Status = "ready"
	c.state.SetupCommand = ""
	c.state.Updated = time.Now().Format(time.RFC3339)
	c.state.Message = "Capture read from Clockwork. Headers, bodies, bindings, and exception payloads are not imported"
	if historyErr != nil {
		c.state.Message = "Latest capture loaded; older history is unavailable"
	}
	if internal {
		c.state.Message = "Metadata routes are being captured. Automatic reads paused to prevent a feedback loop"
		if c.cancel != nil {
			c.cancel()
		}
		c.state.Watching = false
	}
	c.mu.Unlock()
	c.publish()
	return c.State(), nil
}
func (c *Clockwork) Stop() {
	c.mu.Lock()
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	c.state.Watching = false
	c.mu.Unlock()
}
func (c *Clockwork) Select(p Project) {
	c.Stop()
	c.fetchMu.Lock()
	c.mu.Lock()
	c.base = p.URL
	c.project = p
	c.state = Capture{ProjectID: p.ID, Requests: []Request{}, Message: "Visit a project route, then read captures. Automatic capture requires file storage"}
	if status, message, command := clockworkSetup(p); status != "" {
		c.state.Status = status
		c.state.Message = message
		c.state.SetupCommand = command
	}
	c.mu.Unlock()
	c.fetchMu.Unlock()
	c.publish()
}

func filesSnapshot(dir string) string {
	entries, _ := os.ReadDir(dir)
	var result strings.Builder
	for _, e := range entries {
		if e.IsDir() || (!strings.HasSuffix(e.Name(), ".json") && !strings.HasSuffix(e.Name(), ".json.gz")) {
			continue
		}
		info, err := e.Info()
		if err == nil {
			fmt.Fprintf(&result, "%s:%d:%d;", e.Name(), info.Size(), info.ModTime().UnixNano())
		}
	}
	return result.String()
}
func (c *Clockwork) Watch(p Project) error {
	c.Stop()
	if _, err := os.Stat(filepath.Join(p.Path, "composer.lock")); err == nil {
		InspectDependencies(&p)
	}
	if status, message, command := clockworkSetup(p); status != "" {
		c.mu.Lock()
		c.state.Status = status
		c.state.Message = message
		c.state.SetupCommand = command
		c.mu.Unlock()
		c.publish()
		return nil
	}
	dir := filepath.Join(p.Path, "storage", "clockwork")
	if _, err := os.Stat(filepath.Join(p.Path, "storage")); err == nil {
		_ = os.MkdirAll(dir, 0755)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err = watcher.Add(dir); err != nil {
		watcher.Close()
		return errors.New("Clockwork file storage was not found at storage/clockwork. Use manual refresh for another storage path/backend")
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.cancel = cancel
	c.state.Watching = true
	c.state.Message = "Watching Clockwork files; no timer-based PHP polling"
	c.mu.Unlock()
	c.publish()
	go func() {
		defer watcher.Close()
		last := filesSnapshot(dir)
		var timer *time.Timer
		var tick <-chan time.Time
		schedule := func() {
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(220 * time.Millisecond)
			tick = timer.C
		}
		for {
			select {
			case <-ctx.Done():
				if timer != nil {
					timer.Stop()
				}
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				name := filepath.Base(event.Name)
				if name == "index" || strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".json.gz") {
					schedule()
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				last = ""
				schedule()
			case <-tick:
				tick = nil
				now := filesSnapshot(dir)
				if now == last {
					continue
				}
				last = now
				c.Refresh(ctx)
			}
		}
	}()
	return nil
}

type metadataError struct{ kind, message string }

func (err *metadataError) Error() string { return err.message }
func clockworkSetup(p Project) (string, string, string) {
	if p.DependencyState == "Dependencies not installed" {
		return "dependencies_missing", "Project dependencies are not installed. Run composer install in this project, then recheck Clockwork.", "composer install"
	}
	if p.Clockwork == "not installed" && p.DependencyState != "Lockfile unavailable" && p.DependencyState != "Unreadable lockfile" && p.DependencyState != "Installed dependencies not verified" {
		return "not_installed", "Request and SQL capture requires the Clockwork development package in this Laravel project. Install it locally, then visit a route and recheck. Other Workbench tools remain available.", "composer require --dev itsgoingd/clockwork"
	}
	for _, status := range dependencies.ProjectStatus(p.Path) {
		if status.ID == "clockwork" && status.State == "incomplete" {
			return "setup_incomplete", status.Message, "composer dump-autoload --no-scripts && php artisan package:discover"
		}
	}
	return "", "", ""
}
