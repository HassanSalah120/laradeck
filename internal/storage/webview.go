package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// WebviewProfile pins the browser profile across release executable names.
// Existing profiles are reused in place; cookies, credentials and browser files
// are never copied or rewritten by the migration.
func WebviewProfile(base string) (string, error) {
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	settings := filepath.Join(base, "HerdWorkbench")
	if err := os.MkdirAll(settings, 0700); err != nil {
		return "", err
	}
	manifest := filepath.Join(settings, "webview-profile.json")
	var saved struct {
		Folder string `json:"folder"`
	}
	raw, err := os.ReadFile(manifest)
	if err == nil {
		if json.Unmarshal(raw, &saved) != nil {
			return "", errors.New("Invalid saved WebView profile selection")
		}
		clean := filepath.Clean(saved.Folder)
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", errors.New("WebView profile must stay inside the user configuration directory")
		}
		return filepath.Join(base, clean), nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	type candidate struct {
		name     string
		reviewed bool
		modified time.Time
	}
	candidates := []candidate{}
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if !entry.IsDir() || !strings.HasSuffix(name, ".exe") || !(strings.HasPrefix(name, "laradeck") || strings.HasPrefix(name, "herd-workbench")) {
			continue
		}
		db := filepath.Join(base, entry.Name(), "EBWebView", "Default", "Local Storage", "leveldb")
		files, e := os.ReadDir(db)
		if e != nil {
			continue
		}
		item := candidate{name: entry.Name()}
		for _, file := range files {
			ext := filepath.Ext(file.Name())
			if file.IsDir() || (ext != ".log" && ext != ".ldb") {
				continue
			}
			info, e := file.Info()
			if e != nil {
				continue
			}
			if info.ModTime().After(item.modified) {
				item.modified = info.ModTime()
			}
			if info.Size() > 16<<20 {
				continue
			}
			data, e := os.ReadFile(filepath.Join(db, file.Name()))
			if e == nil {
				item.reviewed = item.reviewed || bytes.Contains(data, []byte("laradeck_audit_fp_")) || bytes.Contains(data, []byte("l\x00a\x00r\x00a\x00d\x00e\x00c\x00k\x00_\x00a\x00u\x00d\x00i\x00t\x00_\x00f\x00p\x00_\x00"))
			}
		}
		candidates = append(candidates, item)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].reviewed != candidates[j].reviewed {
			return candidates[i].reviewed
		}
		return candidates[i].modified.After(candidates[j].modified)
	})
	saved.Folder = filepath.Join("HerdWorkbench", "WebView")
	if len(candidates) > 0 {
		saved.Folder = candidates[0].name
	}
	data, _ := json.MarshalIndent(saved, "", "  ")
	if err = os.WriteFile(manifest, data, 0600); err != nil {
		return "", err
	}
	return filepath.Join(base, saved.Folder), nil
}
