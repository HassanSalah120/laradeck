package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var logEntryHeader = regexp.MustCompile(`(?m)^\[(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\]\s+([a-zA-Z0-9_.-]+)\.([A-Z]+):\s+(.*)$`)
var exceptionFileRegex = regexp.MustCompile(`at\s+([A-Za-z]:\\[^:\s]+|/[^:\s]+):(\d+)`)

type LogEntry struct {
	ID          string `json:"id"`
	Timestamp   string `json:"timestamp"`
	Environment string `json:"environment"`
	Level       string `json:"level"`
	Message     string `json:"message"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Stack       string `json:"stack"`
	Fingerprint string `json:"fingerprint"`
}

// ReadLaravelLogs reads the recent portion of storage/logs/laravel.log and parses structured entries.
func ReadLaravelLogs(projectPath string, maxBytes int64) ([]LogEntry, error) {
	if maxBytes <= 0 {
		maxBytes = 512 * 1024 // 512KB tail
	}
	logPath := filepath.Join(projectPath, "storage", "logs", "laravel.log")
	file, err := os.Open(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make([]LogEntry, 0), nil
		}
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	offset := int64(0)
	if stat.Size() > maxBytes {
		offset = stat.Size() - maxBytes
		_, _ = file.Seek(offset, io.SeekStart)
	}

	contentBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	content := string(contentBytes)
	lines := strings.Split(content, "\n")
	var entryOffset int64
	lineOffset := offset

	entries := make([]LogEntry, 0)
	var current *LogEntry
	var currentStack []string

	finishEntry := func() {
		if current != nil {
			current.Stack = strings.Join(currentStack, "\n")
			// Fingerprint based on level + normalized first 100 chars of message
			fpData := current.Level + ":" + current.File + ":" + strconv.Itoa(current.Line)
			if current.File == "" && len(current.Message) > 0 {
				msgPrefix := current.Message
				if len(msgPrefix) > 80 {
					msgPrefix = msgPrefix[:80]
				}
				fpData = current.Level + ":" + msgPrefix
			}
			h := sha256.Sum256([]byte(fpData))
			current.Fingerprint = hex.EncodeToString(h[:8])
			current.ID = fmt.Sprintf("log_%s_%s_%d", current.Fingerprint, strings.ReplaceAll(current.Timestamp, ":", ""), entryOffset)
			entries = append(entries, *current)
		}
	}

	for _, rawLine := range lines {
		line := strings.TrimSuffix(rawLine, "\r")
		startOffset := lineOffset
		lineOffset += int64(len(rawLine) + 1)
		if match := logEntryHeader.FindStringSubmatch(line); len(match) > 4 {
			finishEntry()
			entryOffset = startOffset
			current = &LogEntry{
				Timestamp:   match[1],
				Environment: match[2],
				Level:       match[3],
				Message:     strings.TrimSpace(match[4]),
			}
			currentStack = nil

			// Check if file and line are in the first line
			if fMatch := exceptionFileRegex.FindStringSubmatch(current.Message); len(fMatch) > 2 {
				current.File = fMatch[1]
				current.Line, _ = strconv.Atoi(fMatch[2])
			}
		} else if current != nil {
			currentStack = append(currentStack, line)
			if current.File == "" {
				if fMatch := exceptionFileRegex.FindStringSubmatch(line); len(fMatch) > 2 {
					current.File = fMatch[1]
					current.Line, _ = strconv.Atoi(fMatch[2])
				}
			}
		}
	}
	finishEntry()

	// Reverse to have latest first
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}
