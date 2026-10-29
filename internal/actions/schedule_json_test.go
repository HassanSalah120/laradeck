package actions

import (
	"context"
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestScheduleDatesFromFrameworkJSON(t *testing.T) {
	php, err := exec.LookPath("php")
	if err != nil {
		t.Skip("PHP required")
	}
	helper := filepath.Join(t.TempDir(), "runner.exe")
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go.exe"), "build", "-o", helper, "../../cmd/runner")
	if data, err := command.CombinedOutput(); err != nil {
		t.Fatalf("helper build: %v %s", err, data)
	}
	t.Setenv("WORKBENCH_RUNNER", helper)
	cases := []struct {
		name, data, want string
		count            int
	}{
		{"framework human", `[{"expression":"0 * * * *","command":"php artisan inspire","next_due_date":"2026-10-18 03:00:00 +02:00","next_due_date_human":"1 hour from now"}]`, "1 hour from now", 1},
		{"framework date", `[{"expression":"0 * * * *","command":"php artisan inspire","next_due_date":"2026-10-18 03:00:00 +02:00"}]`, "2026-10-18 03:00:00 +02:00", 1},
		{"legacy", `[{"command":"php artisan inspire","nextDue":"tomorrow"}]`, "tomorrow", 1},
		{"no events", "[]", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			script := "<?php file_put_contents(__DIR__.'/calls.txt', 'x', FILE_APPEND); echo base64_decode('" + base64.StdEncoding.EncodeToString([]byte(tc.data)) + "');"
			os.WriteFile(filepath.Join(root, "artisan"), []byte(script), 0600)
			tasks, err := NewActionsCoordinator().ListScheduledTasksContext(context.Background(), php, root)
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) != tc.count {
				t.Fatalf("tasks %+v", tasks)
			}
			if len(tasks) > 0 && tasks[0].NextDue != tc.want {
				t.Fatalf("date=%q want=%q", tasks[0].NextDue, tc.want)
			}
			calls, _ := os.ReadFile(filepath.Join(root, "calls.txt"))
			if string(calls) != "x" {
				t.Fatalf("valid JSON unnecessarily fell back: %q", calls)
			}
		})
	}
}
