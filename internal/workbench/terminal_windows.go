package workbench

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func launchTerminal(projectDir, phpDir, shellType string) error {
	if projectDir == "" {
		return errors.New("empty project directory")
	}

	// Prepare path with PHP dir prepended if available
	pathEnv := os.Getenv("PATH")
	if phpDir != "" {
		pathEnv = phpDir + string(filepath.ListSeparator) + pathEnv
	}

	shellType = strings.ToLower(strings.TrimSpace(shellType))
	var cmd *exec.Cmd

	switch shellType {
	case "wt", "windowsterminal":
		wtPath, err := exec.LookPath("wt.exe")
		if err == nil {
			cmd = exec.Command(wtPath, "-d", projectDir, "powershell.exe", "-NoExit")
		} else {
			cmd = exec.Command("powershell.exe", "-NoExit", "-Command", fmt.Sprintf("Set-Location -LiteralPath '%s'", projectDir))
		}
	case "cmd":
		cmd = exec.Command("cmd.exe", "/K", fmt.Sprintf("cd /d \"%s\"", projectDir))
	case "bash", "gitbash":
		gitBashPath := `C:\Program Files\Git\bin\bash.exe`
		if _, err := os.Stat(gitBashPath); err == nil {
			cmd = exec.Command(gitBashPath, "--login", "-i")
			cmd.Dir = projectDir
		} else {
			cmd = exec.Command("powershell.exe", "-NoExit", "-Command", fmt.Sprintf("Set-Location -LiteralPath '%s'", projectDir))
		}
	default: // "powershell" or empty
		cmd = exec.Command("powershell.exe", "-NoExit", "-Command", fmt.Sprintf("Set-Location -LiteralPath '%s'", projectDir))
	}

	cmd.Dir = projectDir
	// Inject PATH environment with Herd PHP prioritised
	envMap := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[strings.ToUpper(parts[0])] = parts[1]
		}
	}
	envMap["PATH"] = pathEnv

	var envSlice []string
	for k, v := range envMap {
		envSlice = append(envSlice, k+"="+v)
	}
	cmd.Env = envSlice

	return cmd.Start()
}
