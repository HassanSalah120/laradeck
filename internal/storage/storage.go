package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Settings represents the versioned user configuration.
type Settings struct {
	Version        int                    `json:"version"`
	RecentProject  string                 `json:"recentProject"`
	Trusted        map[string]bool        `json:"trusted"`
	CustomPaths    []string               `json:"customPaths"`
	Theme          string                 `json:"theme"`
	MaxRetention   int                    `json:"maxRetention"` // max records per project
	SavedCommands  map[string][]SavedCmd  `json:"savedCommands"`
	ServiceConfigs map[string]ServiceConf `json:"serviceConfigs"`
	AIConfig       AIConfig               `json:"aiConfig"`
	UpdatedAt      string                 `json:"updatedAt"`
}

type SavedCmd struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Executable  string   `json:"executable"`
	Args        []string `json:"args"`
	Directory   string   `json:"directory"`
	Destructive bool     `json:"destructive"`
}

type ServiceConf struct {
	Engine     string `json:"engine"`
	Port       int    `json:"port"`
	DataDir    string `json:"dataDir"`
	AutoStart  bool   `json:"autoStart"`
	CustomArgs string `json:"customArgs"`
}

type AIConfig struct {
	Enabled   bool   `json:"enabled"`
	Provider  string `json:"provider"`
	APIKeyRef string `json:"apiKeyRef"` // key name in credential store
	Model     string `json:"model"`
	Endpoint  string `json:"endpoint"`
}

type SettingsManager struct {
	mu   sync.Mutex
	path string
	data Settings
}

func defaultSettings() Settings {
	return Settings{
		Version:        1,
		Trusted:        make(map[string]bool),
		CustomPaths:    []string{},
		Theme:          "system",
		MaxRetention:   1000,
		SavedCommands:  make(map[string][]SavedCmd),
		ServiceConfigs: make(map[string]ServiceConf),
		AIConfig: AIConfig{
			Enabled:  false,
			Provider: "local",
			Model:    "llama3",
		},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func AppDataDir() string {
	if appData := os.Getenv("APPDATA"); appData != "" {
		return filepath.Join(appData, "HerdWorkbench")
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".config", "herd-workbench")
	}
	return filepath.Join(".", "data")
}

func NewSettingsManager(customPath string) (*SettingsManager, error) {
	path := customPath
	if path == "" {
		dir := AppDataDir()
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "settings.json")
	}
	sm := &SettingsManager{path: path, data: defaultSettings()}
	if err := sm.load(); err != nil && !os.IsNotExist(err) {
		// If load fails, preserve default and backup corrupt file
		_ = os.Rename(path, path+".corrupt."+time.Now().Format("20060102150405"))
		_ = sm.Save()
	} else if os.IsNotExist(err) {
		_ = sm.Save()
	}
	return sm, nil
}

func (sm *SettingsManager) load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	bytes, err := os.ReadFile(sm.path)
	if err != nil {
		return err
	}
	var s Settings
	if err := json.Unmarshal(bytes, &s); err != nil {
		return err
	}
	if s.Trusted == nil {
		s.Trusted = make(map[string]bool)
	}
	if s.SavedCommands == nil {
		s.SavedCommands = make(map[string][]SavedCmd)
	}
	if s.ServiceConfigs == nil {
		s.ServiceConfigs = make(map[string]ServiceConf)
	}
	if s.MaxRetention <= 0 {
		s.MaxRetention = 1000
	}
	sm.data = s
	return nil
}

func (sm *SettingsManager) Get() Settings {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	// Return an independent snapshot: callers must not mutate shared maps or slices.
	data, _ := json.Marshal(sm.data)
	var snapshot Settings
	_ = json.Unmarshal(data, &snapshot)
	return snapshot
}

func (sm *SettingsManager) Update(fn func(*Settings)) error {
	sm.mu.Lock()
	fn(&sm.data)
	if sm.data.Trusted == nil {
		sm.data.Trusted = make(map[string]bool)
	}
	if sm.data.SavedCommands == nil {
		sm.data.SavedCommands = make(map[string][]SavedCmd)
	}
	if sm.data.ServiceConfigs == nil {
		sm.data.ServiceConfigs = make(map[string]ServiceConf)
	}
	if sm.data.MaxRetention <= 0 {
		sm.data.MaxRetention = 1000
	}
	sm.data.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	sm.mu.Unlock()
	return sm.Save()
}

func (sm *SettingsManager) Save() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	bytes, err := json.MarshalIndent(sm.data, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(sm.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d", sm.path, time.Now().UnixNano())
	if err := os.WriteFile(tmp, bytes, 0600); err != nil {
		return err
	}
	return ReplaceFileAtomic(tmp, sm.path)
}

// DiagnosticsDB represents SQLite diagnostic read-model storage.
type DiagnosticsDB struct {
	db *sql.DB
	mu sync.Mutex
}

type DiagnosticRecord struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"projectId"`
	Source    string  `json:"source"`
	Type      string  `json:"type"`
	Timestamp string  `json:"timestamp"`
	Status    int     `json:"status"`
	URI       string  `json:"uri"`
	Duration  float64 `json:"duration"`
	Summary   string  `json:"summary"`
	Details   string  `json:"details"`
}

type ErrorLogRecord struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Fingerprint string `json:"fingerprint"`
	Level       string `json:"level"`
	Message     string `json:"message"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Stack       string `json:"stack"`
	Occurrences int    `json:"occurrences"`
	FirstSeen   string `json:"firstSeen"`
	LastSeen    string `json:"lastSeen"`
}

type TestRunRecord struct {
	ID        string  `json:"id"`
	ProjectID string  `json:"projectId"`
	Suite     string  `json:"suite"`
	Total     int     `json:"total"`
	Passed    int     `json:"passed"`
	Failed    int     `json:"failed"`
	Skipped   int     `json:"skipped"`
	Duration  float64 `json:"duration"`
	Results   string  `json:"results"`
	CreatedAt string  `json:"createdAt"`
}

type DumpRecord struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	File      string `json:"file"`
	Line      int    `json:"line"`
	Payload   string `json:"payload"`
	CreatedAt string `json:"createdAt"`
}

func OpenDiagnosticsDB(customPath string) (*DiagnosticsDB, error) {
	path := customPath
	if path == "" {
		dir := AppDataDir()
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
		path = filepath.Join(dir, "workbench-diagnostics.db")
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", filepath.ToSlash(path))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	d := &DiagnosticsDB{db: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	_ = d.PruneRetention(30)
	return d, nil
}

func (d *DiagnosticsDB) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS diagnostic_records (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			source TEXT,
			type TEXT,
			timestamp DATETIME,
			status INTEGER,
			uri TEXT,
			duration REAL,
			summary TEXT,
			details TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_diag_proj_time ON diagnostic_records(project_id, timestamp DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_diag_uri ON diagnostic_records(uri);`,
		`CREATE TABLE IF NOT EXISTS error_logs (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			fingerprint TEXT,
			level TEXT,
			message TEXT,
			file TEXT,
			line INTEGER,
			stack TEXT,
			occurrences INTEGER,
			first_seen DATETIME,
			last_seen DATETIME
		);`,
		`CREATE INDEX IF NOT EXISTS idx_err_proj_fp ON error_logs(project_id, fingerprint);`,
		`CREATE INDEX IF NOT EXISTS idx_err_last_seen ON error_logs(last_seen DESC);`,
		`CREATE TABLE IF NOT EXISTS seen_log_records (
			record_key TEXT PRIMARY KEY,
			created_at DATETIME
		);`,
		`CREATE TABLE IF NOT EXISTS test_runs (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			suite TEXT,
			total INTEGER,
			passed INTEGER,
			failed INTEGER,
			skipped INTEGER,
			duration REAL,
			results TEXT,
			created_at DATETIME
		);`,
		`CREATE INDEX IF NOT EXISTS idx_test_proj_time ON test_runs(project_id, created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS dumps (
			id TEXT PRIMARY KEY,
			project_id TEXT,
			file TEXT,
			line INTEGER,
			payload TEXT,
			created_at DATETIME
		);`,
		`CREATE INDEX IF NOT EXISTS idx_dumps_proj_time ON dumps(project_id, created_at DESC);`,
	}
	for _, q := range queries {
		if _, err := d.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (d *DiagnosticsDB) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *DiagnosticsDB) SaveRecord(r DiagnosticRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	r.Summary = RedactSensitive(r.Summary)
	r.Details = RedactSensitive(r.Details)
	query := `INSERT OR REPLACE INTO diagnostic_records 
		(id, project_id, source, type, timestamp, status, uri, duration, summary, details)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := d.db.Exec(query, r.ID, r.ProjectID, r.Source, r.Type, r.Timestamp, r.Status, r.URI, r.Duration, r.Summary, r.Details)
	return err
}

func (d *DiagnosticsDB) QueryRecords(projectID string, search string, limit int, offset int) ([]DiagnosticRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if search == "" {
		rows, err = d.db.Query(`SELECT id, project_id, source, type, timestamp, status, uri, duration, summary, details 
			FROM diagnostic_records WHERE project_id = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?`, projectID, limit, offset)
	} else {
		pattern := "%" + search + "%"
		rows, err = d.db.Query(`SELECT id, project_id, source, type, timestamp, status, uri, duration, summary, details 
			FROM diagnostic_records WHERE project_id = ? AND (uri LIKE ? OR summary LIKE ?) ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
			projectID, pattern, pattern, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]DiagnosticRecord, 0)
	for rows.Next() {
		var r DiagnosticRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Source, &r.Type, &r.Timestamp, &r.Status, &r.URI, &r.Duration, &r.Summary, &r.Details); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

// RecordErrorIdempotent indexes error log lines idempotently by tracking logRecordKey.
// If logRecordKey was already seen, it avoids inflating occurrences (Finding 16).
// Applies RedactSensitive before persistence (Finding 7).
func (d *DiagnosticsDB) RecordErrorIdempotent(projectID, logRecordKey, fingerprint, level, message, file string, line int, stack string) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	message = RedactSensitive(message)
	stack = RedactSensitive(stack)

	if logRecordKey != "" {
		var dummy int
		err := d.db.QueryRow(`SELECT 1 FROM seen_log_records WHERE record_key = ?`, logRecordKey).Scan(&dummy)
		if err == nil {
			// Already indexed this static log entry; do not increment occurrences!
			return false, nil
		}
		_, _ = d.db.Exec(`INSERT OR IGNORE INTO seen_log_records (record_key, created_at) VALUES (?, ?)`,
			logRecordKey, time.Now().UTC().Format(time.RFC3339))
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var existingID string
	var occurrences int
	err := d.db.QueryRow(`SELECT id, occurrences FROM error_logs WHERE project_id = ? AND fingerprint = ?`, projectID, fingerprint).Scan(&existingID, &occurrences)
	if errors.Is(err, sql.ErrNoRows) {
		id := randomID("err")
		_, insErr := d.db.Exec(`INSERT INTO error_logs 
			(id, project_id, fingerprint, level, message, file, line, stack, occurrences, first_seen, last_seen)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
			id, projectID, fingerprint, level, message, file, line, stack, now, now)
		return true, insErr
	} else if err != nil {
		return false, err
	}
	_, updErr := d.db.Exec(`UPDATE error_logs SET occurrences = occurrences + 1, last_seen = ?, message = ?, stack = ? WHERE id = ?`,
		now, message, stack, existingID)
	return true, updErr
}

func (d *DiagnosticsDB) RecordError(projectID, fingerprint, level, message, file string, line int, stack string) error {
	_, err := d.RecordErrorIdempotent(projectID, "", fingerprint, level, message, file, line, stack)
	return err
}

func (d *DiagnosticsDB) QueryErrors(projectID string, search string, limit int) ([]ErrorLogRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if search == "" {
		rows, err = d.db.Query(`SELECT id, project_id, fingerprint, level, message, file, line, stack, occurrences, first_seen, last_seen
			FROM error_logs WHERE project_id = ? ORDER BY last_seen DESC LIMIT ?`, projectID, limit)
	} else {
		pattern := "%" + search + "%"
		rows, err = d.db.Query(`SELECT id, project_id, fingerprint, level, message, file, line, stack, occurrences, first_seen, last_seen
			FROM error_logs WHERE project_id = ? AND (message LIKE ? OR file LIKE ?) ORDER BY last_seen DESC LIMIT ?`,
			projectID, pattern, pattern, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]ErrorLogRecord, 0)
	for rows.Next() {
		var r ErrorLogRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Fingerprint, &r.Level, &r.Message, &r.File, &r.Line, &r.Stack, &r.Occurrences, &r.FirstSeen, &r.LastSeen); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (d *DiagnosticsDB) SaveTestRun(r TestRunRecord) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if r.ID == "" {
		r.ID = randomID("run")
	}
	if r.CreatedAt == "" {
		r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := d.db.Exec(`INSERT INTO test_runs (id, project_id, suite, total, passed, failed, skipped, duration, results, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ProjectID, r.Suite, r.Total, r.Passed, r.Failed, r.Skipped, r.Duration, r.Results, r.CreatedAt)
	return err
}

func (d *DiagnosticsDB) QueryTestRuns(projectID string, limit int) ([]TestRunRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if limit <= 0 {
		limit = 30
	}
	rows, err := d.db.Query(`SELECT id, project_id, suite, total, passed, failed, skipped, duration, results, created_at 
		FROM test_runs WHERE project_id = ? ORDER BY created_at DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []TestRunRecord
	for rows.Next() {
		var r TestRunRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.Suite, &r.Total, &r.Passed, &r.Failed, &r.Skipped, &r.Duration, &r.Results, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (d *DiagnosticsDB) SaveDump(projectID, file string, line int, payload string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	payload = RedactSensitive(payload)
	id := randomID("dmp")
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := d.db.Exec(`INSERT INTO dumps (id, project_id, file, line, payload, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, projectID, file, line, payload, now)
	return err
}

func (d *DiagnosticsDB) QueryDumps(projectID string, limit int) ([]DumpRecord, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.db.Query(`SELECT id, project_id, file, line, payload, created_at FROM dumps WHERE project_id = ? ORDER BY created_at DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]DumpRecord, 0)
	for rows.Next() {
		var r DumpRecord
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.File, &r.Line, &r.Payload, &r.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

func (d *DiagnosticsDB) ClearProjectHistory(projectID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	tables := []string{"diagnostic_records", "error_logs", "test_runs", "dumps"}
	for _, t := range tables {
		if _, err := d.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE project_id = ?", t), projectID); err != nil {
			return err
		}
	}
	return nil
}

func (d *DiagnosticsDB) Prune(maxRecordsPerProject int) error {
	if maxRecordsPerProject <= 0 {
		return errors.New("retention count must be positive")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	// Retain up to maxRecordsPerProject in diagnostic_records
	q := `DELETE FROM diagnostic_records WHERE id IN (
		SELECT id FROM (
			SELECT id, ROW_NUMBER() OVER (PARTITION BY project_id ORDER BY timestamp DESC) as rn
			FROM diagnostic_records
		) WHERE rn > ?
	)`
	_, err := d.db.Exec(q, maxRecordsPerProject)
	return err
}

// PruneRetention enforces the 30-day retention policy across all diagnostic tables (Finding 16).
func (d *DiagnosticsDB) PruneRetention(days int) error {
	if days <= 0 {
		days = 30
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	cutoff := time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339)
	_, _ = d.db.Exec(`DELETE FROM diagnostic_records WHERE timestamp < ?`, cutoff)
	_, _ = d.db.Exec(`DELETE FROM error_logs WHERE last_seen < ?`, cutoff)
	_, _ = d.db.Exec(`DELETE FROM test_runs WHERE created_at < ?`, cutoff)
	_, _ = d.db.Exec(`DELETE FROM dumps WHERE created_at < ?`, cutoff)
	_, _ = d.db.Exec(`DELETE FROM seen_log_records WHERE created_at < ?`, cutoff)
	return nil
}

func randomID(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}
