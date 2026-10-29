package environment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

type PHPInfo struct {
	Version            string            `json:"version"`
	LoadedIni          string            `json:"loadedIni"`
	AdditionalIniFiles []string          `json:"additionalIniFiles"`
	Extensions         []string          `json:"extensions"`
	KeySettings        map[string]string `json:"keySettings"`
}

type IniChangePreview struct {
	IniPath     string `json:"iniPath"`
	Fingerprint string `json:"fingerprint"`
	Key         string `json:"key"`
	OldValue    string `json:"oldValue"`
	NewValue    string `json:"newValue"`
	Line        int    `json:"line"`
}

// InspectPHP gathers PHP version, loaded ini, extensions, and core directives using the resolved php executable.
func InspectPHP(phpExecutable string) (*PHPInfo, error) {
	if phpExecutable == "" {
		return nil, errors.New("PHP executable not specified")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	hideAttr := &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}

	cmd := exec.CommandContext(ctx, phpExecutable, "-i")
	cmd.SysProcAttr = hideAttr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("php -i failed: %w", err)
	}
	infoOutput := stdout.String()

	info := &PHPInfo{
		KeySettings: make(map[string]string),
	}

	// Version
	if vCmd := exec.CommandContext(ctx, phpExecutable, "-v"); vCmd != nil {
		vCmd.SysProcAttr = hideAttr
		var vOut bytes.Buffer
		vCmd.Stdout = &vOut
		if err := vCmd.Run(); err == nil {
			firstLine := strings.Split(vOut.String(), "\n")[0]
			info.Version = strings.TrimSpace(firstLine)
		}
	}

	// Extensions list
	if mCmd := exec.CommandContext(ctx, phpExecutable, "-m"); mCmd != nil {
		mCmd.SysProcAttr = hideAttr
		var mOut bytes.Buffer
		mCmd.Stdout = &mOut
		if err := mCmd.Run(); err == nil {
			lines := strings.Split(mOut.String(), "\n")
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" && !strings.HasPrefix(trimmed, "[") {
					info.Extensions = append(info.Extensions, trimmed)
				}
			}
		}
	}

	// Parse php -i output for Loaded Configuration File
	iniRegex := regexp.MustCompile(`(?i)Loaded Configuration File => (.*)`)
	if match := iniRegex.FindStringSubmatch(infoOutput); len(match) > 1 {
		info.LoadedIni = strings.TrimSpace(match[1])
		if info.LoadedIni == "(none)" {
			info.LoadedIni = ""
		}
	}

	// Directives of interest
	directives := []string{
		"memory_limit",
		"upload_max_filesize",
		"post_max_size",
		"max_execution_time",
		"max_input_time",
		"opcache.enable",
		"opcache.enable_cli",
		"display_errors",
		"error_reporting",
		"date.timezone",
		"xdebug.mode",
	}

	for _, d := range directives {
		re := regexp.MustCompile(fmt.Sprintf(`(?m)^%s => ([^\s=]+)`, regexp.QuoteMeta(d)))
		if match := re.FindStringSubmatch(infoOutput); len(match) > 1 {
			info.KeySettings[d] = strings.TrimSpace(match[1])
		}
	}

	return info, nil
}

// ValidateIniDirective validates the key and value against allowed directives and syntax rules.
func ValidateIniDirective(key, val string) error {
	keyLower := strings.ToLower(strings.TrimSpace(key))
	if strings.ContainsAny(val, "\r\n\x00") {
		return fmt.Errorf("directive value cannot contain newline or control characters")
	}
	valTrimmed := strings.TrimSpace(val)

	switch keyLower {
	case "memory_limit", "upload_max_filesize", "post_max_size":
		if !regexp.MustCompile(`^(?i)(\-1|\d+[kmg]?)$`).MatchString(valTrimmed) {
			return fmt.Errorf("invalid size format for %s: %q (expected e.g. 256M, 1G, -1)", key, val)
		}
	case "max_execution_time", "max_input_time":
		validInteger := regexp.MustCompile(`^\d+$`).MatchString(valTrimmed) || (key == "max_input_time" && valTrimmed == "-1")
		if !validInteger {
			return fmt.Errorf("invalid integer format for %s: %q", key, val)
		}
	case "display_errors", "opcache.enable", "opcache.enable_cli":
		if !regexp.MustCompile(`^(?i)(0|1|on|off|true|false)$`).MatchString(valTrimmed) {
			return fmt.Errorf("invalid boolean toggle for %s: %q (expected 0, 1, On, Off)", key, val)
		}
	case "error_reporting":
		if !regexp.MustCompile(`^(?i)([0-9]+|~?E_[A-Z_]+(\s*[\&\|\^]\s*~?E_[A-Z_]+)*)$`).MatchString(valTrimmed) {
			return fmt.Errorf("invalid error_reporting format: %q", val)
		}
	case "xdebug.mode":
		if !regexp.MustCompile(`^[a-z,]+$`).MatchString(valTrimmed) {
			return fmt.Errorf("invalid xdebug.mode: %q", val)
		}
	case "date.timezone":
		if strings.ContainsAny(valTrimmed, " ;#\"'") {
			return fmt.Errorf("invalid date.timezone: %q", val)
		}
	default:
		return fmt.Errorf("directive %q is not in the approved configurable directives list", key)
	}
	return nil
}

// PreviewIniChange searches for key in iniPath and returns change preview.
func PreviewIniChange(iniPath, key, newValue string) (*IniChangePreview, error) {
	if err := ValidateIniDirective(key, newValue); err != nil {
		return nil, err
	}
	bytes, err := os.ReadFile(iniPath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(bytes), "\n")
	re := regexp.MustCompile(fmt.Sprintf(`^\s*%s\s*=\s*(.*)$`, regexp.QuoteMeta(key)))

	hash := sha256.Sum256(bytes)
	fingerprint := hex.EncodeToString(hash[:])
	for idx := len(lines) - 1; idx >= 0; idx-- {
		line := lines[idx]
		if match := re.FindStringSubmatch(line); len(match) > 1 {
			oldVal := strings.TrimSpace(match[1])
			return &IniChangePreview{
				IniPath:     iniPath,
				Fingerprint: fingerprint,
				Key:         key,
				OldValue:    oldVal,
				NewValue:    newValue,
				Line:        idx + 1,
			}, nil
		}
	}

	// If not found in file
	return &IniChangePreview{
		IniPath:     iniPath,
		Fingerprint: fingerprint,
		Key:         key,
		OldValue:    "(not set)",
		NewValue:    newValue,
		Line:        len(lines) + 1,
	}, nil
}

// ApplyIniChange creates backup and safely modifies the directive in php.ini.
var iniMu sync.Mutex

func ApplyIniChange(iniPath, key, newValue string) error {
	return ApplyIniChangeChecked(iniPath, key, newValue, "")
}
func ApplyIniChangeChecked(iniPath, key, newValue, expectedFingerprint string) error {
	iniMu.Lock()
	defer iniMu.Unlock()
	if err := ValidateIniDirective(key, newValue); err != nil {
		return err
	}
	bytes, err := os.ReadFile(iniPath)
	if err != nil {
		return err
	}

	if expectedFingerprint != "" {
		hash := sha256.Sum256(bytes)
		if hex.EncodeToString(hash[:]) != expectedFingerprint {
			return errors.New("php.ini changed externally; preview again before saving")
		}
	}
	// Create backup
	backupPath := fmt.Sprintf("%s.backup.%s", iniPath, time.Now().Format("20060102150405.000000000"))
	if err := os.WriteFile(backupPath, bytes, 0600); err != nil {
		return fmt.Errorf("could not create backup: %w", err)
	}

	lineEnding := "\n"
	if strings.Contains(string(bytes), "\r\n") {
		lineEnding = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(string(bytes), "\r\n", "\n"), "\n")
	re := regexp.MustCompile(fmt.Sprintf(`^\s*%s\s*=\s*(.*)$`, regexp.QuoteMeta(key)))

	replaced := false
	for idx := len(lines) - 1; idx >= 0; idx-- {
		line := lines[idx]
		if re.MatchString(line) {
			lines[idx] = fmt.Sprintf("%s = %s", key, newValue)
			replaced = true
			break
		}
	}

	if !replaced {
		// Append to the end
		lines = append(lines, fmt.Sprintf("%s = %s", key, newValue))
	}

	newContent := strings.Join(lines, lineEnding)
	tmpPath := fmt.Sprintf("%s.tmp.%d", iniPath, time.Now().UnixNano())
	if err := os.WriteFile(tmpPath, []byte(newContent), 0644); err != nil {
		return err
	}

	defer os.Remove(tmpPath)
	return ReplaceFileAtomic(tmpPath, iniPath)
}

// CheckExtensionsRequired verifies composer.json ext- requirements against loaded extensions.
func CheckExtensionsRequired(composerJSONPath string, loadedExtensions []string) ([]string, []string) {
	loadedMap := make(map[string]bool)
	for _, ext := range loadedExtensions {
		loadedMap[strings.ToLower(ext)] = true
	}

	bytes, err := os.ReadFile(composerJSONPath)
	if err != nil {
		return nil, nil
	}

	// Simple scan for "ext-..."
	re := regexp.MustCompile(`"ext-([a-zA-Z0-9_-]+)"`)
	matches := re.FindAllStringSubmatch(string(bytes), -1)

	var present []string
	var missing []string
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) > 1 {
			extName := strings.ToLower(m[1])
			if seen[extName] {
				continue
			}
			seen[extName] = true
			if loadedMap[extName] {
				present = append(present, extName)
			} else {
				missing = append(missing, extName)
			}
		}
	}

	return present, missing
}
