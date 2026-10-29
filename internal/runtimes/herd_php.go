package runtimes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// DiscoverHerdRuntimes inspects Herd's configuration and bin directories for installed PHP runtimes
func DiscoverHerdRuntimes() []RuntimeInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("USERPROFILE")
	}

	results := make([]RuntimeInfo, 0)
	seen := make(map[string]bool)

	// Herd stores PHP configuration in ~/.config/herd/config/php/<version>/php.ini
	herdPhpConfigDir := filepath.Join(home, ".config", "herd", "config", "php")
	if entries, err := os.ReadDir(herdPhpConfigDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			verName := e.Name() // e.g. "83", "84", "8.3", "8.4"
			iniFile := filepath.Join(herdPhpConfigDir, verName, "php.ini")

			// Try to find corresponding php executable
			phpExe := findHerdPhpBinary(home, verName)
			if phpExe != "" && !seen[phpExe] {
				seen[phpExe] = true
				realVer, _ := probePHPVersion(phpExe)
				if realVer == "" {
					realVer = verName
				}
				results = append(results, RuntimeInfo{
					ID:         "herd-" + verName,
					Name:       fmt.Sprintf("PHP %s (Herd)", realVer),
					Version:    realVer,
					Type:       "herd",
					BinaryPath: phpExe,
					IniPath:    iniFile,
					Installed:  true,
				})
			}
		}
	}

	// Also check Herd bin directory (~/.config/herd/bin)
	herdBins := filepath.Join(home, ".config", "herd", "bin")
	if entries, err := os.ReadDir(herdBins); err == nil {
		for _, e := range entries {
			name := strings.ToLower(e.Name())
			if strings.HasPrefix(name, "php") && strings.HasSuffix(name, ".exe") {
				exePath := filepath.Join(herdBins, e.Name())
				if seen[exePath] {
					continue
				}
				seen[exePath] = true
				realVer, _ := probePHPVersion(exePath)
				if realVer == "" {
					realVer = strings.TrimSuffix(strings.TrimPrefix(name, "php"), ".exe")
				}
				results = append(results, RuntimeInfo{
					ID:         "herd-" + name,
					Name:       fmt.Sprintf("PHP %s (Herd CLI)", realVer),
					Version:    realVer,
					Type:       "herd",
					BinaryPath: exePath,
					IniPath:    "",
					Installed:  true,
				})
			}
		}
	}

	return results
}

func findHerdPhpBinary(home, version string) string {
	// 1. ~/.config/herd/bin/php<version>.exe
	cleanVer := strings.ReplaceAll(version, ".", "")
	candidate1 := filepath.Join(home, ".config", "herd", "bin", fmt.Sprintf("php%s.exe", cleanVer))
	if _, err := os.Stat(candidate1); err == nil {
		return candidate1
	}

	// 2. LocalAppData/Herd/resources/bin/php/<version>/php.exe
	localApp := os.Getenv("LOCALAPPDATA")
	if localApp != "" {
		candidate2 := filepath.Join(localApp, "Programs", "Herd", "resources", "bin", "php", cleanVer, "php.exe")
		if _, err := os.Stat(candidate2); err == nil {
			return candidate2
		}
	}

	// 3. Fallback to system path `where php`
	return ""
}

// IsolateHerdSite executes `herd isolate <version>` inside the project directory
func IsolateHerdSite(ctx context.Context, projectPath, version string) error {
	cleanVer := strings.ReplaceAll(version, ".", "")
	cleanVer = strings.TrimPrefix(cleanVer, "php")

	cmd := exec.CommandContext(ctx, "herd", "isolate", cleanVer)
	cmd.Dir = projectPath
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("herd isolate failed: %s (%w)", strings.TrimSpace(string(output)), err)
	}

	return nil
}

// ReadHerdConfig reads the active PHP version for a site from Herd config
func GetHerdSitePhpVersion(siteName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("USERPROFILE")
	}

	configPath := filepath.Join(home, ".config", "herd", "config", "valet", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	// Look for site isolation mapping in config.json
	re := regexp.MustCompile(fmt.Sprintf(`"%s"\s*:\s*"([a-zA-Z0-9.]+)"`, regexp.QuoteMeta(siteName)))
	match := re.FindStringSubmatch(string(data))
	if len(match) > 1 {
		return match[1]
	}

	return ""
}
