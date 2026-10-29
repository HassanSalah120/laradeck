package storage

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	logHeaderRegex = regexp.MustCompile(`^\[(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?)\]\s+([a-zA-Z0-9_-]+)\.([A-Z]+):\s+(.*)$`)
	hexNumberRegex = regexp.MustCompile(`0x[0-9a-fA-F]+|\b\d+\b`)
	uuidRegex       = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
)

type LogPailEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	DateStr   string    `json:"dateStr"`
	Env       string    `json:"env"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Context   string    `json:"context,omitempty"`
	Trace     []string  `json:"trace,omitempty"`
	AppTrace  []string  `json:"appTrace,omitempty"`
	IsError   bool      `json:"isError"`
}

type ExceptionIncident struct {
	Fingerprint string       `json:"fingerprint"`
	Exception   string       `json:"exception"`
	Message     string       `json:"message"`
	Location    string       `json:"location"`
	Count       int          `json:"count"`
	FirstSeen   time.Time    `json:"firstSeen"`
	LastSeen    time.Time    `json:"lastSeen"`
	SampleEntry LogPailEntry `json:"sampleEntry"`
	AIPrompt    string       `json:"aiPrompt"`
}

type LogPailReport struct {
	TotalEntries int                 `json:"totalEntries"`
	ErrorCount   int                 `json:"errorCount"`
	WarnCount    int                 `json:"warnCount"`
	Entries      []LogPailEntry      `json:"entries"`
	Incidents    []ExceptionIncident `json:"incidents"`
}

// FingerprintException normalizes error messages into a stable identifier
func FingerprintException(exceptionClass, message, location string) string {
	norm := uuidRegex.ReplaceAllString(message, "?")
	norm = hexNumberRegex.ReplaceAllString(norm, "?")
	norm = strings.TrimSpace(norm)
	raw := fmt.Sprintf("%s|%s|%s", exceptionClass, location, norm)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:8])
}

// ParseLaravelLog reads the last N lines or MB of a laravel.log file
func ParseLaravelLog(logPath string, maxEntries int) (LogPailReport, error) {
	if maxEntries <= 0 {
		maxEntries = 200
	}

	f, err := os.Open(logPath)
	if err != nil {
		return LogPailReport{}, err
	}
	defer f.Close()

	// If file is large, read from near end
	info, err := f.Stat()
	if err == nil && info.Size() > 4*1024*1024 {
		_, _ = f.Seek(-4*1024*1024, io.SeekEnd)
	}

	scanner := bufio.NewScanner(f)
	var rawLines []string
	for scanner.Scan() {
		rawLines = append(rawLines, scanner.Text())
	}

	var entries []LogPailEntry
	var currentEntry *LogPailEntry

	for _, line := range rawLines {
		if matches := logHeaderRegex.FindStringSubmatch(line); len(matches) == 5 {
			if currentEntry != nil {
				entries = append(entries, *currentEntry)
			}

			tStr := matches[1]
			envStr := matches[2]
			lvlStr := matches[3]
			msgStr := matches[4]

			ts, _ := time.Parse("2006-01-02 15:04:05", tStr)
			if ts.IsZero() {
				ts, _ = time.Parse("2006-01-02T15:04:05", tStr)
			}

			isErr := lvlStr == "ERROR" || lvlStr == "CRITICAL" || lvlStr == "ALERT" || lvlStr == "EMERGENCY"

			currentEntry = &LogPailEntry{
				ID:        fmt.Sprintf("log_%d_%d", ts.Unix(), len(entries)),
				Timestamp: ts,
				DateStr:   tStr,
				Env:       envStr,
				Level:     lvlStr,
				Message:   msgStr,
				Trace:     []string{},
				AppTrace:  []string{},
				IsError:   isErr,
			}
		} else if currentEntry != nil {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				currentEntry.Trace = append(currentEntry.Trace, trimmed)
				if (strings.Contains(trimmed, "/app/") || strings.Contains(trimmed, "\\app\\")) && !strings.Contains(trimmed, "/vendor/") {
					currentEntry.AppTrace = append(currentEntry.AppTrace, trimmed)
				}
			}
		}
	}

	if currentEntry != nil {
		entries = append(entries, *currentEntry)
	}

	// Limit to maxEntries most recent
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}

	// Deduplicate exceptions into incidents
	incidentMap := make(map[string]*ExceptionIncident)
	errorCount := 0
	warnCount := 0

	for _, e := range entries {
		if e.IsError {
			errorCount++
			excClass := "Exception"
			location := ""
			msg := e.Message

			if strings.Contains(msg, ":") {
				parts := strings.SplitN(msg, ":", 2)
				if strings.Contains(parts[0], "\\") || strings.Contains(parts[0], "Exception") || strings.Contains(parts[0], "Error") {
					excClass = strings.TrimSpace(parts[0])
					msg = strings.TrimSpace(parts[1])
				}
			}

			if len(e.AppTrace) > 0 {
				location = e.AppTrace[0]
			} else if len(e.Trace) > 0 {
				location = e.Trace[0]
			}

			fp := FingerprintException(excClass, msg, location)
			inc, exists := incidentMap[fp]
			if !exists {
				// Generate AI Prompt
				aiPrompt := fmt.Sprintf(
					"### Laravel Error Debugging Assistance\n**Exception:** `%s`\n**Message:** %s\n**Location:** `%s`\n\n```text\n%s\n```\n\nPlease identify the root cause and prescribe a clean Laravel solution following modern conventions.",
					excClass, msg, location, strings.Join(e.AppTrace, "\n"),
				)

				inc = &ExceptionIncident{
					Fingerprint: fp,
					Exception:   excClass,
					Message:     msg,
					Location:    location,
					Count:       0,
					FirstSeen:   e.Timestamp,
					LastSeen:    e.Timestamp,
					SampleEntry: e,
					AIPrompt:    aiPrompt,
				}
				incidentMap[fp] = inc
			}
			inc.Count++
			if e.Timestamp.After(inc.LastSeen) {
				inc.LastSeen = e.Timestamp
			}
			if e.Timestamp.Before(inc.FirstSeen) {
				inc.FirstSeen = e.Timestamp
			}
		} else if e.Level == "WARNING" {
			warnCount++
		}
	}

	incidents := make([]ExceptionIncident, 0, len(incidentMap))
	for _, inc := range incidentMap {
		incidents = append(incidents, *inc)
	}

	// Sort incidents by count descending
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].Count > incidents[j].Count
	})

	return LogPailReport{
		TotalEntries: len(entries),
		ErrorCount:   errorCount,
		WarnCount:    warnCount,
		Entries:      entries,
		Incidents:    incidents,
	}, nil
}
