package runtimes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const ComposerStableURL = "https://getcomposer.org/composer-stable.phar"

func (m *Manager) GetComposerStatus() ComposerStatus {
	binDir := m.BinDir()
	pharPath := filepath.Join(binDir, "composer.phar")
	batPath := filepath.Join(binDir, "composer.bat")

	installed := false
	if _, err := os.Stat(pharPath); err == nil {
		if _, err := os.Stat(batPath); err == nil {
			installed = true
		}
	}

	version := ""
	if installed {
		version = m.probeComposerVersion(pharPath)
	}

	inPath := false
	pathEnv := os.Getenv("PATH")
	for _, p := range filepath.SplitList(pathEnv) {
		if strings.EqualFold(filepath.Clean(p), filepath.Clean(binDir)) {
			inPath = true
			break
		}
	}

	return ComposerStatus{
		Installed: installed,
		Version:   version,
		Path:      batPath,
		InPath:    inPath,
	}
}

func (m *Manager) probeComposerVersion(pharPath string) string {
	phpExe, err := m.GetRuntimeByVersion("")
	if err != nil {
		return "Installed"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, phpExe, pharPath, "--version")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "Installed"
	}

	firstLine := strings.Split(out.String(), "\n")[0]
	return strings.TrimSpace(firstLine)
}

func (m *Manager) InstallComposer(ctx context.Context, onProgress func(pct int, msg string)) error {
	binDir := m.BinDir()
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	pharPath := filepath.Join(binDir, "composer.phar")

	if onProgress != nil {
		onProgress(20, "Downloading latest stable composer.phar...")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", ComposerStableURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "LaraDeck-Composer-Installer")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download composer.phar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download composer.phar failed with HTTP %d", resp.StatusCode)
	}

	outFile, err := os.Create(pharPath)
	if err != nil {
		return fmt.Errorf("failed to create composer.phar: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write composer.phar: %w", err)
	}

	if onProgress != nil {
		onProgress(70, "Creating executable CLI shims (php.bat & composer.bat)...")
	}

	// Create CLI shims
	if err := m.generateCliShims(); err != nil {
		return fmt.Errorf("failed to generate CLI shims: %w", err)
	}

	if onProgress != nil {
		onProgress(100, "Composer successfully installed in ~/.laradeck/bin")
	}

	return nil
}

func (m *Manager) generateCliShims() error {
	binDir := m.BinDir()

	// 1. php.bat shim (invokes first installed standalone PHP or system PHP)
	phpBat := filepath.Join(binDir, "php.bat")
	phpExe, _ := m.GetRuntimeByVersion("")
	if phpExe == "" {
		phpExe = "php.exe"
	}

	phpBatContent := fmt.Sprintf("@echo off\n\"%s\" %%*\n", phpExe)
	if err := os.WriteFile(phpBat, []byte(phpBatContent), 0755); err != nil {
		return err
	}

	// 2. composer.bat shim
	composerBat := filepath.Join(binDir, "composer.bat")
	composerBatContent := "@echo off\n\"%~dp0php.bat\" \"%~dp0composer.phar\" %*\n"
	return os.WriteFile(composerBat, []byte(composerBatContent), 0755)
}

// AddBinToUserPath adds ~/.laradeck/bin to the Windows user PATH environment variable
func (m *Manager) AddBinToUserPath() error {
	binDir := m.BinDir()

	// Run PowerShell command to append to User PATH without modifying Machine PATH
	psCmd := fmt.Sprintf(`
$p = [Environment]::GetEnvironmentVariable("Path", "User")
if ($p -notlike "*%s*") {
    [Environment]::SetEnvironmentVariable("Path", "$p;%s", "User")
    Write-Output "added"
} else {
    Write-Output "already_present"
}
`, binDir, binDir)

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update PATH: %s (%w)", strings.TrimSpace(string(out)), err)
	}

	return nil
}
