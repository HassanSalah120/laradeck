package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestMain(m *testing.M) {
	if marker := os.Getenv("WORKBENCH_FAKE_WINGET_PROOF"); marker != "" && strings.EqualFold(filepath.Base(os.Args[0]), "winget.exe") {
		in, _ := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
		out, _ := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
		var mode uint32
		inputErr := windows.GetConsoleMode(in, &mode)
		outputErr := windows.GetConsoleMode(out, &mode)
		data, _ := json.Marshal(map[string]any{"args": os.Args[1:], "consoleInput": inputErr == nil, "consoleOutput": outputErr == nil})
		os.WriteFile(marker, data, 0600)
		os.Exit(0)
	}

	dir, err := os.MkdirTemp("", "workbench runner tests ")
	if err != nil {
		panic(err)
	}
	helper := filepath.Join(dir, "workbench-runner.exe")
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go.exe"), "build", "-o", helper, "../../cmd/runner")
	if output, err := cmd.CombinedOutput(); err != nil {
		panic(string(output))
	}
	os.Setenv("WORKBENCH_RUNNER", helper)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func node(t *testing.T, source string) *Process {
	t.Helper()
	exe, err := exec.LookPath("node.exe")
	if err != nil {
		t.Skip("Node is required for Windows console integration tests")
	}
	p, err := Start(Spec{Executable: exe, Args: []string{"-e", source}, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if p.Running() {
			p.ForceStop()
			p.Wait(5 * time.Second)
		}
		p.Close()
	})
	return p
}

func awaitText(t *testing.T, p *Process, text string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range p.Events() {
			if strings.Contains(e.Text, text) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("missing %q; events: %+v", text, p.Events())
}

func TestConsoleBreakAcknowledgedByChild(t *testing.T) {
	p := node(t, `process.on('SIGBREAK',()=>{console.log('acknowledged-stop');process.exit(0)});console.log('ready');setInterval(()=>{},1000)`)
	awaitText(t, p, "ready")
	if err := p.RequestStop(); err != nil {
		t.Fatal(err)
	}
	if !p.Wait(5 * time.Second) {
		t.Fatal("cooperative child did not exit")
	}
	awaitText(t, p, "acknowledged-stop")
	if p.ExitCode() != 0 {
		t.Fatalf("exit=%d", p.ExitCode())
	}
}

func TestForceStopsDescendantAfterRootExits(t *testing.T) {
	p := node(t, `const c=require('child_process').spawn(process.execPath,['-e','setInterval(()=>{},1000)'],{stdio:'ignore',detached:true}); console.log('descendant='+c.pid);c.unref()`)
	awaitText(t, p, "descendant=")
	var pid int
	for _, e := range p.Events() {
		if strings.HasPrefix(e.Text, "descendant=") {
			pid, _ = strconv.Atoi(strings.TrimPrefix(e.Text, "descendant="))
		}
	}
	if pid == 0 {
		t.Fatal("no descendant PID")
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	time.Sleep(200 * time.Millisecond)
	if !p.Running() {
		t.Fatal("tree was declared finished with descendant still alive")
	}
	if err = p.ForceStop(); err != nil {
		t.Fatal(err)
	}
	if !p.Wait(5 * time.Second) {
		t.Fatal("tree remained active")
	}
	status, err := windows.WaitForSingleObject(h, 1000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("descendant survived: %v %v", status, err)
	}
}

func TestCaptureArgumentsAndExit(t *testing.T) {
	exe, err := exec.LookPath("node.exe")
	if err != nil {
		t.Skip("Node required")
	}
	value := `a path with spaces & $ symbols`
	data, err := Capture(Spec{Executable: exe, Args: []string{"-e", "console.log(process.argv[1])", value}, Directory: t.TempDir()}, 5*time.Second)
	if err != nil || strings.TrimSpace(string(data)) != value {
		t.Fatalf("capture %q: %v", data, err)
	}
	p := node(t, `process.exit(7)`)
	if !p.Wait(5*time.Second) || p.ExitCode() != 7 {
		t.Fatal(fmt.Sprintf("exit=%d", p.ExitCode()))
	}
}

func TestCaptureRetainsMoreThanEventRing(t *testing.T) {
	exe, err := exec.LookPath("node.exe")
	if err != nil {
		t.Skip("Node required")
	}
	res, err := CaptureExec(Spec{Executable: exe, Args: []string{"-e", `for(let i=0;i<1000;i++) console.log('line-'+i);`}, Directory: t.TempDir()}, 5*time.Second)
	if err != nil || !strings.HasPrefix(res.Stdout, "line-0\n") || !strings.Contains(res.Stdout, "line-999\n") {
		t.Fatalf("output lost: err=%v", err)
	}
}
func TestCaptureCancellationRetainsOutput(t *testing.T) {
	exe, err := exec.LookPath("node.exe")
	if err != nil {
		t.Skip("Node required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := CaptureExecContext(ctx, Spec{Executable: exe, Args: []string{"-e", `console.log('started');setInterval(()=>{},1000);`}, Directory: t.TempDir()}, 30*time.Second)
	if err == nil || res == nil || !res.Cancelled || !strings.Contains(res.Stdout, "started") {
		t.Fatalf("cancel result=%+v err=%v", res, err)
	}
}

func TestCaptureLargeSingleLineJSON(t *testing.T) {
	exe, err := exec.LookPath("node.exe")
	if err != nil {
		t.Skip("Node required")
	}
	res, err := CaptureExec(Spec{Executable: exe, Args: []string{"-e", `console.log(JSON.stringify({payload:'x'.repeat(600000)}));`}, Directory: t.TempDir()}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &value); err != nil || len(value.Payload) != 600000 {
		t.Fatalf("large JSON output lost or chunk boundary inserted: %v length=%d", err, len(value.Payload))
	}
}

func TestSystemInstallerUsesExplicitInteractiveArguments(t *testing.T) {
	args := SystemInstallerArgs("Oracle.MySQL")
	if strings.Join(args, " ") != "install --id Oracle.MySQL --exact --source winget --interactive" {
		t.Fatal(args)
	}
	if err := LaunchSystemInstaller("Oracle.MySQL & calc"); err == nil {
		t.Fatal("shell-style package ID accepted")
	}
}

// No system installer runs: a fixture executable stands in for WinGet and
// checks the console handles handed to it by the production installer host.
func TestInstallerHostHasInteractiveConsoleHandles(t *testing.T) {
	root := t.TempDir()
	current, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "winget.exe"), bytes, 0700); err != nil {
		t.Fatal(err)
	}
	proof := filepath.Join(root, "proof.json")
	t.Setenv("WORKBENCH_FAKE_WINGET_PROOF", proof)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	p, err := Start(Spec{Executable: os.Getenv("WORKBENCH_RUNNER"), Args: []string{"--system-install", "Oracle.MySQL"}, Directory: root})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { p.ForceStop(); p.Wait(3 * time.Second); p.Close() }()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(proof)
		if err == nil {
			var result struct {
				Args                        []string
				ConsoleInput, ConsoleOutput bool
			}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if !result.ConsoleInput || !result.ConsoleOutput {
				t.Fatalf("interactive console unavailable: %+v", result)
			}
			if strings.Join(result.Args, " ") != strings.Join(SystemInstallerArgs("Oracle.MySQL"), " ") {
				t.Fatal(result.Args)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("fixture WinGet did not receive the installer handoff")
}
