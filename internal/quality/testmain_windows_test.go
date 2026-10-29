package quality

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "workbench-quality-tests-")
	if err != nil {
		panic(err)
	}
	helper := filepath.Join(dir, "workbench-runner.exe")
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go.exe"), "build", "-o", helper, "../../cmd/runner")
	if output, err := command.CombinedOutput(); err != nil {
		panic(string(output))
	}
	os.Setenv("WORKBENCH_RUNNER", helper)
	os.Setenv("APPDATA", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
