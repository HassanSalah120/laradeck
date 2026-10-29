// Package runner contains only processes launched by this application.
package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Spec struct {
	Env        []string `json:"env,omitempty"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
	Directory  string   `json:"directory"`
}
type Event struct {
	Kind    string `json:"kind"`
	Text    string `json:"text,omitempty"`
	PID     int    `json:"pid,omitempty"`
	Partial bool   `json:"partial,omitempty"`
	Code    int    `json:"code"`
}
type jobAccounting struct {
	TotalUserTime, TotalKernelTime, PeriodUserTime, PeriodKernelTime int64
	PageFaults, TotalProcesses, ActiveProcesses, TerminatedProcesses uint32
}
type Process struct {
	cmd          *exec.Cmd
	input        io.WriteCloser
	job          windows.Handle
	done         chan struct{}
	mu           sync.Mutex
	writeMu      sync.Mutex
	events       []Event
	capture      []Event
	captureBytes int
	truncated    bool
	code         int
	stopSent     bool
}

func helperPath() (string, error) {
	if override := os.Getenv("WORKBENCH_RUNNER"); override != "" {
		return override, nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Support laradeck-runner.exe as primary, falling back to workbench-runner.exe
	path := filepath.Join(filepath.Dir(executable), "laradeck-runner.exe")
	if _, err = os.Stat(path); err == nil {
		return path, nil
	}
	path = filepath.Join(filepath.Dir(executable), "workbench-runner.exe")
	if _, err = os.Stat(path); err != nil {
		return "", errors.New("laradeck-runner.exe (or workbench-runner.exe) must be beside laradeck.exe")
	}
	return path, nil
}

func Start(spec Spec) (*Process, error) {
	helper, err := helperPath()
	if err != nil {
		return nil, err
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	cmd := exec.Command(helper)
	// A hidden console belongs to the helper. Its child process group shares it.
	// CREATE_NO_WINDOW would prevent console control delivery.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_CONSOLE}
	input, err := cmd.StdinPipe()
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		windows.CloseHandle(job)
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		input.Close()
		windows.CloseHandle(job)
		return nil, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, handle)
		windows.CloseHandle(handle)
	}
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		input.Close()
		windows.CloseHandle(job)
		return nil, fmt.Errorf("contain process: %w", err)
	}
	p := &Process{cmd: cmd, input: input, job: job, done: make(chan struct{}), code: -1}
	// The helper waits for this specification, so it cannot spawn before assignment.
	if err = json.NewEncoder(input).Encode(spec); err != nil {
		windows.TerminateJobObject(job, 1)
		cmd.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	go func() {
		scanner := bufio.NewScanner(output)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for scanner.Scan() {
			var e Event
			if json.Unmarshal(scanner.Bytes(), &e) == nil {
				p.mu.Lock()
				p.events = append(p.events, e)
				if e.Kind == "stdout" || e.Kind == "stderr" || e.Kind == "error" {
					if p.captureBytes+len(e.Text)+1 <= 16*1024*1024 {
						p.capture = append(p.capture, e)
						p.captureBytes += len(e.Text) + 1
					} else {
						p.truncated = true
					}
				}
				if len(p.events) > 400 {
					p.events = p.events[len(p.events)-400:]
				}
				p.mu.Unlock()
			}
		}
		err := cmd.Wait()
		p.mu.Lock()
		if err == nil {
			p.code = 0
		} else if exit, ok := err.(*exec.ExitError); ok {
			p.code = exit.ExitCode()
		}
		p.mu.Unlock()
		// If a shell exited while descendants remain, keep the containment handle alive.
		for {
			var a jobAccounting
			err := windows.QueryInformationJobObject(job, windows.JobObjectBasicAccountingInformation, uintptr(unsafe.Pointer(&a)), uint32(unsafe.Sizeof(a)), nil)
			if err != nil || a.ActiveProcesses == 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		input.Close()
		close(p.done)
	}()
	return p, nil
}

func (p *Process) Running() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
func (p *Process) Events() []Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Event{}, p.events...)
}
func (p *Process) PID() int              { return p.cmd.Process.Pid }
func (p *Process) Done() <-chan struct{} { return p.done }
func (p *Process) ExitCode() int         { p.mu.Lock(); defer p.mu.Unlock(); return p.code }
func (p *Process) Wait(timeout time.Duration) bool {
	select {
	case <-p.done:
		return true
	case <-time.After(timeout):
		return false
	}
}
func (p *Process) RequestStop() error {
	if !p.Running() {
		return nil
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if p.stopSent {
		return nil
	}
	p.stopSent = true
	_, err := io.WriteString(p.input, "stop\n")
	return err
}
func (p *Process) ForceStop() error {
	if !p.Running() {
		return nil
	}
	return windows.TerminateJobObject(p.job, 1)
}
func (p *Process) Close() error {
	if p.Running() {
		return errors.New("process tree still running")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.job == 0 {
		return nil
	}
	err := windows.CloseHandle(p.job)
	p.job = 0
	return err
}

type ExecutionResult struct {
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Combined  string `json:"combined"`
	ExitCode  int    `json:"exitCode"`
	TimedOut  bool   `json:"timedOut"`
	Cancelled bool   `json:"cancelled"`
	Truncated bool   `json:"truncated"`
}

func CaptureExec(spec Spec, timeout time.Duration) (*ExecutionResult, error) {
	return CaptureExecContext(context.Background(), spec, timeout)
}

func CaptureExecContext(ctx context.Context, spec Spec, timeout time.Duration) (*ExecutionResult, error) {
	return CaptureExecContextWithProgress(ctx, spec, timeout, nil)
}

// Progress receives bounded recent events while capture retains the complete
// bounded result. Cancellation and process-tree containment use the same path.
func CaptureExecContextWithProgress(ctx context.Context, spec Spec, timeout time.Duration, progress func([]Event)) (*ExecutionResult, error) {
	if err := ctx.Err(); err != nil {
		return &ExecutionResult{ExitCode: -1, Cancelled: true}, err
	}
	p, err := Start(spec)
	if err != nil {
		return nil, err
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	var stopErr error
	var timedOut, cancelled bool
	var ticks <-chan time.Time
	if progress != nil {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		ticks = ticker.C
	}
wait:
	for {
		select {
		case <-p.Done():
			break wait
		case <-ctx.Done():
			cancelled = true
			stopErr = ctx.Err()
			break wait
		case <-timer.C:
			timedOut = true
			stopErr = errors.New("command timed out")
			break wait
		case <-ticks:
			progress(p.Events())
		}
	}
	if stopErr != nil {
		_ = p.RequestStop()
		if !p.Wait(time.Second) {
			if err := p.ForceStop(); err != nil {
				stopErr = errors.Join(stopErr, err)
			}
			if !p.Wait(3 * time.Second) {
				stopErr = errors.Join(stopErr, errors.New("owned process tree has not exited"))
			}
		}
	}
	if !p.Running() {
		defer p.Close()
	}
	if progress != nil {
		progress(p.Events())
	}
	p.mu.Lock()
	events := append([]Event(nil), p.capture...)
	truncated := p.truncated
	p.mu.Unlock()
	var stdout, stderr, combined strings.Builder
	for _, e := range events {
		if e.Kind == "stdout" {
			stdout.WriteString(e.Text)
			if !e.Partial {
				stdout.WriteByte('\n')
			}
		} else {
			stderr.WriteString(e.Text)
			if !e.Partial {
				stderr.WriteByte('\n')
			}
		}
		combined.WriteString(e.Text)
		if !e.Partial {
			combined.WriteByte('\n')
		}
	}
	result := &ExecutionResult{Stdout: stdout.String(), Stderr: stderr.String(), Combined: combined.String(), ExitCode: p.ExitCode(), TimedOut: timedOut, Cancelled: cancelled, Truncated: truncated}
	if truncated {
		stopErr = errors.Join(stopErr, errors.New("command output exceeded 16 MiB capture limit"))
	}
	return result, stopErr
}

func Capture(spec Spec, timeout time.Duration) ([]byte, error) {
	return CaptureContext(context.Background(), spec, timeout)
}

func CaptureContext(ctx context.Context, spec Spec, timeout time.Duration) ([]byte, error) {
	res, err := CaptureExecContext(ctx, spec, timeout)
	if res == nil {
		return nil, err
	}
	if err != nil {
		return []byte(res.Combined), err
	}
	if res.ExitCode != 0 {
		return []byte(res.Combined), fmt.Errorf("command exited with code %d; check project dependencies and bootstrap", res.ExitCode)
	}
	return []byte(res.Stdout), nil
}

// RunHost is used only by the separate console helper, never by the WebView.
func RunHost() int {
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 4096), 1024*1024)
	if !scan.Scan() {
		return 2
	}
	var spec Spec
	if json.Unmarshal(scan.Bytes(), &spec) != nil {
		return 2
	}
	var outMu sync.Mutex
	emit := func(e Event) { outMu.Lock(); defer outMu.Unlock(); json.NewEncoder(os.Stdout).Encode(e) }
	cmd := exec.Command(spec.Executable, spec.Args...)
	cmd.Dir = spec.Directory
	if len(spec.Env) > 0 {
		cmd.Env = append(os.Environ(), spec.Env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		emit(Event{Kind: "error", Text: "Could not start executable: " + err.Error(), Code: 1})
		return 1
	}
	emit(Event{Kind: "started", PID: cmd.Process.Pid})
	var readers sync.WaitGroup
	read := func(r io.Reader, kind string) {
		defer readers.Done()
		// Chunk long lines instead of abandoning a pipe when a JSON response
		// exceeds Scanner's token limit. Partial chunks never add a newline.
		reader := bufio.NewReaderSize(r, 64*1024)
		for {
			chunk, err := reader.ReadSlice('\n')
			if len(chunk) > 0 {
				partial := err == bufio.ErrBufferFull
				text := string(chunk)
				if !partial {
					text = strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r")
				}
				emit(Event{Kind: kind, Text: text, Partial: partial})
			}
			if err != nil && err != bufio.ErrBufferFull {
				if err != io.EOF {
					emit(Event{Kind: "error", Text: "Could not read process output: " + err.Error()})
				}
				break
			}
		}
	}
	readers.Add(2)
	go read(stdout, "stdout")
	go read(stderr, "stderr")
	go func() {
		for scan.Scan() {
			if strings.TrimSpace(scan.Text()) == "stop" {
				err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(cmd.Process.Pid))
				if err != nil {
					emit(Event{Kind: "stop-error", Text: "Console control delivery failed"})
				} else {
					emit(Event{Kind: "stop-requested", Text: "Console control delivered; waiting for the process tree"})
				}
			}
		}
	}()
	readers.Wait()
	err := cmd.Wait()
	code := 0
	if err != nil {
		code = 1
		if e, ok := err.(*exec.ExitError); ok {
			code = e.ExitCode()
		}
	}
	emit(Event{Kind: "exit", Code: code})
	return code
}

// A user-requested system installer is an independent, visible workflow. It is
// deliberately outside dev-process Jobs so closing Workbench cannot kill an MSI
// halfway through changing Windows services or credentials.
func LaunchSystemInstaller(packageID string) error {
	if !regexp.MustCompile(`^[A-Za-z0-9]+(?:\.[A-Za-z0-9]+)+$`).MatchString(packageID) {
		return errors.New("invalid system package ID")
	}
	if _, err := exec.LookPath("winget.exe"); err != nil {
		return errors.New("WinGet is unavailable; install Windows App Installer first")
	}
	helper, err := helperPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(helper, "--system-install", packageID)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
func SystemInstallerArgs(packageID string) []string {
	return []string{"install", "--id", packageID, "--exact", "--source", "winget", "--interactive"}
}
func RunSystemInstallerHost(packageID string) int {
	if !regexp.MustCompile(`^[A-Za-z0-9]+(?:\.[A-Za-z0-9]+)+$`).MatchString(packageID) {
		return 2
	}
	// os/exec gives a GUI parent's child NUL standard handles by default.
	// Bind explicitly to this helper's newly allocated console so WinGet can
	// display license/source prompts and receive the user's answers.
	input, err := os.OpenFile("CONIN$", os.O_RDONLY, 0)
	if err != nil {
		return 2
	}
	defer input.Close()
	output, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		return 2
	}
	defer output.Close()
	fmt.Fprintf(output, "Herd Workbench: install %s\nReview license, administrator and service configuration prompts in the vendor installer.\n\n", packageID)
	winget, err := exec.LookPath("winget.exe")
	code := 1
	if err == nil {
		cmd := exec.Command(winget, SystemInstallerArgs(packageID)...)
		cmd.Stdin = input
		cmd.Stdout = output
		cmd.Stderr = output
		err = cmd.Run()
		if err == nil {
			code = 0
		} else if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		}
	}
	if err != nil {
		fmt.Fprintf(output, "\nInstaller failed: %v\n", err)
	} else {
		fmt.Fprintln(output, "\nWinGet finished. Return to Workbench and recheck detection.")
	}
	fmt.Fprintln(output, "Press Enter to close this installer console.")
	bufio.NewReader(input).ReadString('\n')
	return code
}
