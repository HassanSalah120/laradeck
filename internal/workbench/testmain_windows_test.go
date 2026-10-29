package workbench

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Unit tests must never open or update the developer's real Workbench profile.
func TestMain(m *testing.M) {
	if os.Getenv("WORKBENCH_COMPOSER_PROBE") == "1" && strings.EqualFold(filepath.Base(os.Args[0]), "composer.exe") {
		fmt.Println("Optimizing autoload files: token=probe-secret")
		for {
			time.Sleep(time.Second)
		}
	}

	dir, err := os.MkdirTemp("", "workbench-domain-tests-")
	if err != nil {
		panic(err)
	}
	os.Setenv("APPDATA", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
