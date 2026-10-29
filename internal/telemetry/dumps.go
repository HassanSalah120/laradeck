package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type DumpPayload struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Payload   string `json:"payload"`
	CreatedAt string `json:"createdAt"`
}

type TelemetryQuery struct {
	SQL        string  `json:"sql"`
	RawSQL     string  `json:"rawSql"`
	TimeMs     float64 `json:"timeMs"`
	Connection string  `json:"connection"`
}

type TelemetryLog struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Context string `json:"context,omitempty"`
	Time    string `json:"time"`
}

type TelemetryException struct {
	Message string   `json:"message"`
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Class   string   `json:"class"`
	Stack   []string `json:"stack,omitempty"`
}

type RequestTelemetryFrame struct {
	ID              string              `json:"id"`
	ProjectID       string              `json:"projectId,omitempty"`
	Method          string              `json:"method"`
	URL             string              `json:"url"`
	Status          int                 `json:"status"`
	DurationMs      float64             `json:"durationMs"`
	MemoryPeakBytes int64               `json:"memoryPeakBytes"`
	User            map[string]any      `json:"user,omitempty"`
	Queries         []TelemetryQuery    `json:"queries"`
	Logs            []TelemetryLog      `json:"logs"`
	Dumps           []DumpPayload       `json:"dumps"`
	Exception       *TelemetryException `json:"exception,omitempty"`
	CreatedAt       string              `json:"createdAt"`
}

type DumpServer struct {
	mu          sync.RWMutex
	listener    net.Listener
	port        int
	dumps       []DumpPayload
	maxDumps    int
	onDump      func(DumpPayload)
	frames      []RequestTelemetryFrame
	maxFrames   int
	onFrame     func(RequestTelemetryFrame)
	dumpsInHtml bool
	isClosed    bool
	httpServer  *http.Server
}

func NewDumpServer(preferredPort int, maxDumps int, onDump func(DumpPayload)) (*DumpServer, error) {
	if maxDumps <= 0 {
		maxDumps = 200
	}
	addr := fmt.Sprintf("127.0.0.1:%d", preferredPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Fallback to random loopback port if preferred is busy
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("failed to bind dump listener: %w", err)
		}
	}

	actualPort := ln.Addr().(*net.TCPAddr).Port

	maxFrames := 500
	ds := &DumpServer{
		listener:    ln,
		port:        actualPort,
		maxDumps:    maxDumps,
		dumps:       make([]DumpPayload, 0, maxDumps),
		onDump:      onDump,
		maxFrames:   maxFrames,
		frames:      make([]RequestTelemetryFrame, 0, maxFrames),
		dumpsInHtml: false,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/dumps", ds.handleDump)
	mux.HandleFunc("/telemetry", ds.handleTelemetry)
	mux.HandleFunc("/config", ds.handleConfig)
	mux.HandleFunc("/", ds.handleDump)

	ds.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	go func() {
		_ = ds.httpServer.Serve(ln)
	}()

	return ds, nil
}

func (ds *DumpServer) Port() int {
	return ds.port
}

func (ds *DumpServer) SetDumpsInHtml(enabled bool) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.dumpsInHtml = enabled
}

func (ds *DumpServer) GetDumpsInHtml() bool {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.dumpsInHtml
}

func (ds *DumpServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	ds.mu.RLock()
	dumpsInHtml := ds.dumpsInHtml
	ds.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"dumpsInHtml": dumpsInHtml,
	})
}

func (ds *DumpServer) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 10*1024*1024))
	if err != nil {
		http.Error(w, "payload too large or read failed", http.StatusBadRequest)
		return
	}

	var f RequestTelemetryFrame
	if err := json.Unmarshal(body, &f); err != nil {
		http.Error(w, "invalid telemetry JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if f.ID == "" {
		f.ID = fmt.Sprintf("frame_%d", time.Now().UnixNano())
	}
	if f.CreatedAt == "" {
		f.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	ds.mu.Lock()
	if len(ds.frames) >= ds.maxFrames {
		ds.frames = ds.frames[1:]
	}
	ds.frames = append(ds.frames, f)
	cb := ds.onFrame
	ds.mu.Unlock()

	if cb != nil {
		cb(f)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (ds *DumpServer) GetRecentFrames(projectID string) []RequestTelemetryFrame {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	result := make([]RequestTelemetryFrame, 0, len(ds.frames))
	for _, f := range ds.frames {
		if projectID == "" || f.ProjectID == projectID {
			result = append(result, f)
		}
	}
	return result
}

func (ds *DumpServer) ClearFrames(projectID string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if projectID == "" {
		ds.frames = make([]RequestTelemetryFrame, 0, ds.maxFrames)
		return
	}

	filtered := make([]RequestTelemetryFrame, 0, len(ds.frames))
	for _, f := range ds.frames {
		if f.ProjectID != projectID {
			filtered = append(filtered, f)
		}
	}
	ds.frames = filtered
}

func (ds *DumpServer) handleDump(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
	if err != nil {
		http.Error(w, "payload too large or read failed", http.StatusBadRequest)
		return
	}

	var d DumpPayload
	if err := json.Unmarshal(body, &d); err != nil {
		// Accept plain text dump payload
		d = DumpPayload{
			Payload: string(body),
			File:    r.Header.Get("X-Dump-File"),
		}
	}

	if d.ID == "" {
		d.ID = fmt.Sprintf("dump_%d", time.Now().UnixNano())
	}
	if d.CreatedAt == "" {
		d.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	ds.mu.Lock()
	if len(ds.dumps) >= ds.maxDumps {
		ds.dumps = ds.dumps[1:]
	}
	ds.dumps = append(ds.dumps, d)
	cb := ds.onDump
	ds.mu.Unlock()

	if cb != nil {
		cb(d)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (ds *DumpServer) GetRecentDumps(projectID string) []DumpPayload {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	result := make([]DumpPayload, 0, len(ds.dumps))
	for _, d := range ds.dumps {
		if projectID == "" || d.ProjectID == projectID {
			result = append(result, d)
		}
	}
	return result
}

func (ds *DumpServer) Clear(projectID string) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if projectID == "" {
		ds.dumps = make([]DumpPayload, 0, ds.maxDumps)
		return
	}

	filtered := make([]DumpPayload, 0, len(ds.dumps))
	for _, d := range ds.dumps {
		if d.ProjectID != projectID {
			filtered = append(filtered, d)
		}
	}
	ds.dumps = filtered
}

func (ds *DumpServer) Close() error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.isClosed {
		return nil
	}
	ds.isClosed = true
	return ds.httpServer.Close()
}

// GetDumpHelperPHP returns the PHP code snippet developers can include in their app
func (ds *DumpServer) GetDumpHelperPHP() string {
	return fmt.Sprintf(`<?php
// LaraDeck In-App Dump Helper
if (!function_exists('laradeck_dump')) {
    function laradeck_dump(...$vars) {
        $trace = debug_backtrace(DEBUG_BACKTRACE_IGNORE_ARGS, 1)[0] ?? [];
        $file = $trace['file'] ?? 'unknown';
        $line = $trace['line'] ?? 0;
        
        $payloads = [];
        foreach ($vars as $var) {
            $payloads[] = is_scalar($var) ? (string)$var : json_encode($var, JSON_PRETTY_PRINT | JSON_PARTIAL_OUTPUT_ON_ERROR);
        }
        
        $body = json_encode([
            'projectId' => basename(base_path()),
            'file' => str_replace(base_path() . DIRECTORY_SEPARATOR, '', $file),
            'line' => $line,
            'payload' => implode("\n", $payloads),
            'createdAt' => gmdate('Y-m-d\TH:i:s\Z')
        ]);
        
        $ctx = stream_context_create([
            'http' => [
                'method' => 'POST',
                'header' => "Content-Type: application/json\r\n",
                'content' => $body,
                'timeout' => 0.5,
                'ignore_errors' => true
            ]
        ]);
        @file_get_contents('http://127.0.0.1:%d', false, $ctx);
    }
}
`, ds.port)
}
