package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

type XdebugStatus struct {
	Installed    bool   `json:"installed"`
	Version      string `json:"version,omitempty"`
	Mode         string `json:"mode,omitempty"`
	Port         int    `json:"port"`
	IDEKey       string `json:"ideKey"`
	TriggerURL   string `json:"triggerUrl,omitempty"`
	ConfigStatus string `json:"configStatus"`
}

// CheckXdebugStatus inspects PHP runtime for Xdebug extension and settings
func CheckXdebugStatus(ctx context.Context, phpBinary, projectURL string) (*XdebugStatus, error) {
	status := &XdebugStatus{
		Installed:    false,
		Port:         9003,
		IDEKey:       "LARADECK",
		ConfigStatus: "Xdebug not detected in active PHP",
	}

	if phpBinary == "" {
		phpBinary = "php"
	}

	cmd := exec.CommandContext(ctx, phpBinary, "-v")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err == nil {
		outStr := string(out)
		if strings.Contains(strings.ToLower(outStr), "with xdebug") {
			status.Installed = true
			re := regexp.MustCompile(`with Xdebug v?([0-9\.]+)`)
			matches := re.FindStringSubmatch(outStr)
			if len(matches) > 1 {
				status.Version = matches[1]
			}
			status.ConfigStatus = fmt.Sprintf("Active (Xdebug v%s)", status.Version)
		}
	}

	if projectURL != "" {
		status.TriggerURL = BuildXdebugTriggerURL(projectURL, status.IDEKey)
	}

	return status, nil
}

// BuildXdebugTriggerURL appends Xdebug trigger session parameter
func BuildXdebugTriggerURL(rawURL, ideKey string) string {
	if ideKey == "" {
		ideKey = "LARADECK"
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		if strings.Contains(rawURL, "?") {
			return rawURL + "&XDEBUG_SESSION=" + ideKey
		}
		return rawURL + "?XDEBUG_SESSION=" + ideKey
	}
	q := u.Query()
	q.Set("XDEBUG_SESSION", ideKey)
	u.RawQuery = q.Encode()
	return u.String()
}

// GenerateVSCodeConfig creates or merges .vscode/launch.json for Xdebug
func GenerateVSCodeConfig(projectPath string) error {
	vscodeDir := filepath.Join(projectPath, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		return err
	}
	launchPath := filepath.Join(vscodeDir, "launch.json")

	launchConfig := map[string]interface{}{
		"version": "0.2.0",
		"configurations": []map[string]interface{}{
			{
				"name":          "Listen for Xdebug (LaraDeck)",
				"type":          "php",
				"request":       "launch",
				"port":          9003,
				"pathMappings": map[string]string{
					"/": "${workspaceFolder}",
				},
			},
		},
	}

	data, err := json.MarshalIndent(launchConfig, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(launchPath, data, 0644)
}
