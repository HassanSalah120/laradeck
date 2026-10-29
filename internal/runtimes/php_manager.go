package runtimes

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

type CatalogEntry struct {
	VersionMajorMinor string
	FullVersion       string
	URL               string
}

var StandaloneCatalog = []CatalogEntry{
	{
		VersionMajorMinor: "8.4",
		FullVersion:       "8.4.4",
		URL:               "https://windows.php.net/downloads/releases/php-8.4.4-Win32-vs17-x64.zip",
	},
	{
		VersionMajorMinor: "8.3",
		FullVersion:       "8.3.17",
		URL:               "https://windows.php.net/downloads/releases/php-8.3.17-Win32-vs16-x64.zip",
	},
	{
		VersionMajorMinor: "8.2",
		FullVersion:       "8.2.27",
		URL:               "https://windows.php.net/downloads/releases/php-8.2.27-Win32-vs16-x64.zip",
	},
	{
		VersionMajorMinor: "8.1",
		FullVersion:       "8.1.31",
		URL:               "https://windows.php.net/downloads/releases/archives/php-8.1.31-Win32-vs16-x64.zip",
	},
}

type Manager struct {
	baseDir string
	mu      sync.Mutex
}

func NewManager(customBaseDir string) *Manager {
	if customBaseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = os.Getenv("USERPROFILE")
		}
		customBaseDir = filepath.Join(home, ".laradeck")
	}
	_ = os.MkdirAll(filepath.Join(customBaseDir, "runtimes", "php"), 0755)
	_ = os.MkdirAll(filepath.Join(customBaseDir, "bin"), 0755)
	return &Manager{baseDir: customBaseDir}
}

func (m *Manager) BaseDir() string {
	return m.baseDir
}

func (m *Manager) PHPDir() string {
	return filepath.Join(m.baseDir, "runtimes", "php")
}

func (m *Manager) BinDir() string {
	return filepath.Join(m.baseDir, "bin")
}

// ListStandaloneRuntimes returns catalog entries with their local install status
func (m *Manager) ListStandaloneRuntimes() []RuntimeInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	phpRoot := m.PHPDir()
	results := make([]RuntimeInfo, 0, len(StandaloneCatalog))

	for _, entry := range StandaloneCatalog {
		installPath := filepath.Join(phpRoot, entry.VersionMajorMinor)
		phpExe := filepath.Join(installPath, "php.exe")
		iniPath := filepath.Join(installPath, "php.ini")

		installed := false
		if _, err := os.Stat(phpExe); err == nil {
			installed = true
		}

		info := RuntimeInfo{
			ID:          "standalone-" + entry.VersionMajorMinor,
			Name:        fmt.Sprintf("PHP %s (LaraDeck Managed)", entry.VersionMajorMinor),
			Version:     entry.FullVersion,
			Type:        "standalone",
			BinaryPath:  "",
			IniPath:     "",
			Installed:   installed,
			DownloadURL: entry.URL,
		}

		if installed {
			info.BinaryPath = phpExe
			info.IniPath = iniPath
			// Probe real version string if available
			if realVer, err := probePHPVersion(phpExe); err == nil && realVer != "" {
				info.Version = realVer
			}
			exts, _ := m.getExtensionsForPath(installPath, iniPath)
			info.Extensions = exts
		}

		results = append(results, info)
	}

	return results
}

func probePHPVersion(phpExe string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, phpExe, "-r", "echo PHP_VERSION;")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// InstallPHP downloads and extracts the requested major.minor version from official Windows PHP releases
func (m *Manager) InstallPHP(ctx context.Context, versionMajorMinor string, onProgress func(pct int, msg string)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var target *CatalogEntry
	for _, entry := range StandaloneCatalog {
		if entry.VersionMajorMinor == versionMajorMinor {
			target = &entry
			break
		}
	}
	if target == nil {
		return fmt.Errorf("unsupported PHP version: %s", versionMajorMinor)
	}

	installPath := filepath.Join(m.PHPDir(), target.VersionMajorMinor)
	if err := os.MkdirAll(installPath, 0755); err != nil {
		return fmt.Errorf("failed to create install dir: %w", err)
	}

	if onProgress != nil {
		onProgress(10, fmt.Sprintf("Downloading PHP %s from official releases...", target.FullVersion))
	}

	// Download zip archive to temp file
	tmpZip := filepath.Join(os.TempDir(), fmt.Sprintf("laradeck_php_%s_%d.zip", target.VersionMajorMinor, time.Now().UnixNano()))
	defer os.Remove(tmpZip)

	req, err := http.NewRequestWithContext(ctx, "GET", target.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LaraDeck-Windows-Runtime-Manager")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download PHP archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP %d from %s", resp.StatusCode, target.URL)
	}

	outZip, err := os.Create(tmpZip)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	totalSize := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 64*1024)
	for {
		select {
		case <-ctx.Done():
			outZip.Close()
			return ctx.Err()
		default:
		}

		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			_, _ = outZip.Write(buf[:n])
			downloaded += int64(n)
			if totalSize > 0 && onProgress != nil {
				pct := int(float64(downloaded) / float64(totalSize) * 60.0) // 0-60%
				onProgress(10+pct, fmt.Sprintf("Downloading PHP %s: %d%%", target.FullVersion, pct))
			}
		}
		if rErr != nil {
			if rErr == io.EOF {
				break
			}
			outZip.Close()
			return rErr
		}
	}
	outZip.Close()

	if onProgress != nil {
		onProgress(75, "Extracting PHP archive...")
	}

	// Extract zip archive to installPath
	if err := unzipArchive(tmpZip, installPath); err != nil {
		return fmt.Errorf("failed to extract PHP archive: %w", err)
	}

	if onProgress != nil {
		onProgress(90, "Configuring php.ini with Laravel development defaults...")
	}

	// Initialize php.ini
	if err := m.initializePhpIni(installPath); err != nil {
		return fmt.Errorf("failed to configure php.ini: %w", err)
	}

	if onProgress != nil {
		onProgress(100, fmt.Sprintf("PHP %s successfully installed and configured.", target.FullVersion))
	}

	return nil
}

func unzipArchive(srcZip, destDir string) error {
	r, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		cleanName := filepath.Clean(f.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			continue
		}
		targetPath := filepath.Join(destDir, cleanName)

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(targetPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// initializePhpIni creates an optimized php.ini with standard Laravel extensions and dev settings
func (m *Manager) initializePhpIni(installPath string) error {
	iniPath := filepath.Join(installPath, "php.ini")
	devTemplate := filepath.Join(installPath, "php.ini-development")

	var content string
	if data, err := os.ReadFile(devTemplate); err == nil {
		content = string(data)
	} else {
		content = "[PHP]\n"
	}

	// Set extension_dir
	if strings.Contains(content, ";extension_dir = \"ext\"") {
		content = strings.Replace(content, ";extension_dir = \"ext\"", "extension_dir = \"ext\"", 1)
	} else if !strings.Contains(content, "extension_dir = \"ext\"") {
		content += "\nextension_dir = \"ext\"\n"
	}

	// Essential Laravel extensions to enable
	essentialExtensions := []string{
		"curl",
		"fileinfo",
		"gd",
		"intl",
		"mbstring",
		"openssl",
		"pdo_mysql",
		"pdo_pgsql",
		"pdo_sqlite",
		"sodium",
		"zip",
	}

	for _, ext := range essentialExtensions {
		commented := fmt.Sprintf(";extension=%s", ext)
		enabled := fmt.Sprintf("extension=%s", ext)
		if strings.Contains(content, commented) {
			content = strings.Replace(content, commented, enabled, 1)
		} else if !strings.Contains(content, enabled) {
			content += "\n" + enabled
		}
	}

	// Ensure OPcache
	if strings.Contains(content, ";zend_extension=opcache") {
		content = strings.Replace(content, ";zend_extension=opcache", "zend_extension=opcache", 1)
	}

	// Optimization directives for local development
	directives := map[string]string{
		"memory_limit":        "512M",
		"max_execution_time":  "300",
		"upload_max_filesize": "128M",
		"post_max_size":       "128M",
		"display_errors":      "On",
		"display_startup_errors": "On",
	}

	for k, v := range directives {
		re := regexp.MustCompile(fmt.Sprintf(`(?m)^;?%s\s*=.*$`, regexp.QuoteMeta(k)))
		if re.MatchString(content) {
			content = re.ReplaceAllString(content, fmt.Sprintf("%s = %s", k, v))
		} else {
			content += fmt.Sprintf("\n%s = %s", k, v)
		}
	}

	return os.WriteFile(iniPath, []byte(content), 0644)
}

func (m *Manager) getExtensionsForPath(installPath, iniPath string) ([]ExtensionStatus, error) {
	extDir := filepath.Join(installPath, "ext")
	entries, err := os.ReadDir(extDir)
	if err != nil {
		return nil, err
	}

	iniContent := ""
	if data, err := os.ReadFile(iniPath); err == nil {
		iniContent = string(data)
	}

	var results []ExtensionStatus
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "php_") || !strings.HasSuffix(name, ".dll") {
			continue
		}
		extName := strings.TrimSuffix(strings.TrimPrefix(name, "php_"), ".dll")

		// Check if active in INI
		activeRe := regexp.MustCompile(fmt.Sprintf(`(?m)^\s*extension\s*=\s*(?:php_)?%s(?:\.dll)?\s*$`, regexp.QuoteMeta(extName)))
		enabled := activeRe.MatchString(iniContent)

		results = append(results, ExtensionStatus{
			Name:        extName,
			Enabled:     enabled,
			Available:   true,
			Description: extensionDescription(extName),
		})
	}

	// Also check OPcache
	opcacheRe := regexp.MustCompile(`(?m)^\s*zend_extension\s*=\s*(?:php_)?opcache(?:\.dll)?\s*$`)
	results = append(results, ExtensionStatus{
		Name:        "opcache",
		Enabled:     opcacheRe.MatchString(iniContent),
		Available:   true,
		Description: "Zend OPcache bytecode accelerator",
	})

	return results, nil
}

func extensionDescription(name string) string {
	switch strings.ToLower(name) {
	case "pdo_mysql":
		return "MySQL & MariaDB database driver for PDO"
	case "pdo_pgsql":
		return "PostgreSQL database driver for PDO"
	case "pdo_sqlite":
		return "SQLite 3 database driver for PDO"
	case "curl":
		return "cURL library for outbound HTTP requests"
	case "fileinfo":
		return "MIME type detection for file uploads"
	case "gd":
		return "Image processing and manipulation"
	case "intl":
		return "Internationalization & localization functions"
	case "mbstring":
		return "Multibyte string support for UTF-8"
	case "openssl":
		return "Cryptography and HTTPS / SSL encryption"
	case "sodium":
		return "Modern libsodium cryptography library"
	case "zip":
		return "Zip archive reading and writing"
	case "redis":
		return "High-performance PhpRedis client"
	case "xdebug":
		return "Step debugger, profiler, and code coverage"
	default:
		return "PHP extension module"
	}
}

// ToggleExtension toggles an extension in php.ini for a standalone or Herd runtime
func (m *Manager) ToggleExtension(iniPath, extensionName string, enable bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(iniPath)
	if err != nil {
		return fmt.Errorf("failed to read ini file: %w", err)
	}

	content := string(data)
	isZend := extensionName == "opcache" || extensionName == "xdebug"

	var activeDirective, commentedDirective string
	if isZend {
		activeDirective = fmt.Sprintf("zend_extension=%s", extensionName)
		commentedDirective = fmt.Sprintf(";zend_extension=%s", extensionName)
	} else {
		activeDirective = fmt.Sprintf("extension=%s", extensionName)
		commentedDirective = fmt.Sprintf(";extension=%s", extensionName)
	}

	if enable {
		if strings.Contains(content, commentedDirective) {
			content = strings.Replace(content, commentedDirective, activeDirective, 1)
		} else if !strings.Contains(content, activeDirective) {
			content += "\n" + activeDirective + "\n"
		}
	} else {
		if strings.Contains(content, activeDirective) {
			content = strings.Replace(content, activeDirective, commentedDirective, 1)
		}
	}

	return os.WriteFile(iniPath, []byte(content), 0644)
}

// GetRuntimeByVersion resolves an installed PHP executable for a given version or default
func (m *Manager) GetRuntimeByVersion(version string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 1. Direct standalone match
	if version != "" {
		targetPath := filepath.Join(m.PHPDir(), version, "php.exe")
		if _, err := os.Stat(targetPath); err == nil {
			return targetPath, nil
		}
	}

	// 2. Scan installed standalone runtimes
	entries, err := os.ReadDir(m.PHPDir())
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				p := filepath.Join(m.PHPDir(), e.Name(), "php.exe")
				if _, err := os.Stat(p); err == nil {
					return p, nil
				}
			}
		}
	}

	return "", errors.New("no installed PHP runtime found")
}

// GetLiveIniDirective probes the real active value from the running PHP binary ("Now: ...")
func (m *Manager) GetLiveIniDirective(ctx context.Context, phpBinary, directive string) (string, error) {
	if phpBinary == "" {
		phpBinary = "php"
	}
	code := fmt.Sprintf("echo ini_get('%s');", directive)
	cmd := exec.CommandContext(ctx, phpBinary, "-r", code)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SetSiteIniDirective sets a custom php.ini directive for a single site using .user.ini
func SetSiteIniDirective(projectPath, directive, value string) error {
	targetDir := filepath.Join(projectPath, "public")
	if fi, err := os.Stat(targetDir); err != nil || !fi.IsDir() {
		targetDir = projectPath
	}

	userIniPath := filepath.Join(targetDir, ".user.ini")
	directiveLine := fmt.Sprintf(`%s = "%s"`, directive, value)

	var lines []string
	if data, err := os.ReadFile(userIniPath); err == nil {
		rawLines := strings.Split(string(data), "\n")
		for _, line := range rawLines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, directive+" ") || strings.HasPrefix(trimmed, directive+"=") {
				continue
			}
			if trimmed != "" {
				lines = append(lines, line)
			}
		}
	}

	lines = append(lines, directiveLine)
	return os.WriteFile(userIniPath, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}
