//go:build !windows

package workbench

import "errors"

func launchTerminal(projectDir, phpDir, shellType string) error {
	return errors.New("terminal launch is only supported on Windows")
}
