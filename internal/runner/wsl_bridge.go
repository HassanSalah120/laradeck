package runner

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type WSLStatus struct {
	Available    bool   `json:"available"`
	Distro       string `json:"distro"`
	SupportsPHP  bool   `json:"supportsPhp"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// DetectWSL checks if WSL2 is installed and running on Windows
func DetectWSL() WSLStatus {
	wslPath, err := exec.LookPath("wsl.exe")
	if err != nil {
		return WSLStatus{Available: false, ErrorMessage: "wsl.exe not found in PATH"}
	}

	cmd := exec.Command(wslPath, "-l", "-q")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return WSLStatus{Available: false, ErrorMessage: "WSL is not initialized or has no distros installed"}
	}

	distros := strings.Fields(strings.ReplaceAll(out.String(), "\x00", ""))
	if len(distros) == 0 {
		return WSLStatus{Available: false, ErrorMessage: "No WSL distribution found"}
	}

	chosenDistro := distros[0]

	// Check if PHP with pcntl is available inside the distro
	checkPHP := exec.Command(wslPath, "-d", chosenDistro, "--", "php", "-m")
	var phpOut bytes.Buffer
	checkPHP.Stdout = &phpOut
	supportsPHP := false
	if err := checkPHP.Run(); err == nil {
		if strings.Contains(phpOut.String(), "pcntl") {
			supportsPHP = true
		}
	}

	return WSLStatus{
		Available:   true,
		Distro:      chosenDistro,
		SupportsPHP: supportsPHP,
	}
}

// WindowsToWSLPath converts a Windows path like "G:\laradeck\app" to "/mnt/g/laradeck/app"
func WindowsToWSLPath(winPath string) (string, error) {
	clean := filepath.Clean(winPath)
	if len(clean) < 2 || clean[1] != ':' {
		return "", errors.New("invalid windows absolute path")
	}
	driveLetter := strings.ToLower(string(clean[0]))
	rest := filepath.ToSlash(clean[2:])
	return fmt.Sprintf("/mnt/%s%s", driveLetter, rest), nil
}

// BuildHorizonWSLCommand generates the execution spec for running Laravel Horizon inside WSL2
func BuildHorizonWSLCommand(distro, projectPath string) ([]string, error) {
	wslPath, err := WindowsToWSLPath(projectPath)
	if err != nil {
		return nil, err
	}
	args := []string{
		"-d", distro,
		"--cd", wslPath,
		"--", "php", "artisan", "horizon",
	}
	return args, nil
}
