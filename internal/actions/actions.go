package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"herdworkbench/internal/runner"
)

type EventListenerItem struct {
	Event     string   `json:"event"`
	Listeners []string `json:"listeners"`
}

type FailedJobItem struct {
	ID         string `json:"id"`
	UUID       string `json:"uuid"`
	Connection string `json:"connection"`
	Queue      string `json:"queue"`
	Class      string `json:"class"`
	FailedAt   string `json:"failedAt"`
	Exception  string `json:"exception"`
}

type QueueWorkerStatus struct {
	Running   bool     `json:"running"`
	PID       int      `json:"pid"`
	Queue     string   `json:"queue"`
	Output    []string `json:"output"`
	StartedAt string   `json:"startedAt"`
}

type ActionRunResult struct {
	Success  bool     `json:"success"`
	ExitCode int      `json:"exitCode"`
	Output   []string `json:"output"`
	Duration float64  `json:"durationMs"`
	Error    string   `json:"error,omitempty"`
}

type ActionsCoordinator struct {
	mu            sync.Mutex
	workerProcess *runner.Process
	workerQueue   string
	workerStart   time.Time
}

func NewActionsCoordinator() *ActionsCoordinator {
	return &ActionsCoordinator{}
}

// ListEvents discovers registered events and their listeners in the project.
func (ac *ActionsCoordinator) ListEvents(phpBinary, projectPath string) ([]EventListenerItem, error) {
	// Try artisan event:list --json
	data, err := runner.Capture(runner.Spec{
		Executable: phpBinary,
		Args:       []string{"artisan", "event:list", "--json", "--no-interaction", "--no-ansi"},
		Directory:  projectPath,
	}, 15*time.Second)

	if err == nil {
		var rawEvents []struct {
			Event     string   `json:"event"`
			Listeners []string `json:"listeners"`
		}
		if json.Unmarshal(data, &rawEvents) == nil && len(rawEvents) > 0 {
			items := make([]EventListenerItem, 0, len(rawEvents))
			for _, re := range rawEvents {
				items = append(items, EventListenerItem{
					Event:     re.Event,
					Listeners: re.Listeners,
				})
			}
			return items, nil
		}
	}

	// Fallback to scanning app/Events without inventing fake listeners (Finding 9)
	eventsDir := filepath.Join(projectPath, "app", "Events")
	var items []EventListenerItem
	if entries, err := os.ReadDir(eventsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".php") {
				name := "App\\Events\\" + strings.TrimSuffix(e.Name(), ".php")
				items = append(items, EventListenerItem{
					Event:     name,
					Listeners: []string{}, // No fabricated listeners
				})
			}
		}
	}

	if len(items) == 0 {
		return make([]EventListenerItem, 0), nil
	}
	return items, nil
}

// ListFailedJobs parses failed jobs via artisan queue:failed.
func (ac *ActionsCoordinator) ListFailedJobs(phpBinary, projectPath string) ([]FailedJobItem, error) {
	return ac.ListFailedJobsContext(context.Background(), phpBinary, projectPath)
}
func (ac *ActionsCoordinator) ListFailedJobsContext(ctx context.Context, phpBinary, projectPath string) ([]FailedJobItem, error) {
	args := []string{"artisan", "queue:failed", "--no-interaction", "--no-ansi"}
	// The JSON option was introduced in Laravel 13. Never probe it on 10–12.
	var lock struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}
	bytes, _ := os.ReadFile(filepath.Join(projectPath, "composer.lock"))
	if json.Unmarshal(bytes, &lock) == nil {
		for _, pkg := range lock.Packages {
			if pkg.Name == "laravel/framework" {
				major, _ := strconv.Atoi(strings.Split(strings.TrimPrefix(pkg.Version, "v"), ".")[0])
				if major >= 13 {
					args = append(args, "--json")
				}
			}
		}
	}
	res, err := runner.CaptureExecContext(ctx, runner.Spec{Executable: phpBinary, Args: args, Directory: projectPath}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("queue:failed exited with code %d: %s", res.ExitCode, res.Combined)
	}
	if strings.Contains(strings.Join(args, " "), "--json") {
		var list []FailedJobItem
		if err := json.Unmarshal([]byte(res.Stdout), &list); err != nil {
			return nil, fmt.Errorf("invalid failed jobs JSON: %w", err)
		}
		return list, nil
	}
	return parseFailedJobsText(res.Stdout), nil
}

func parseFailedJobsText(output string) []FailedJobItem {
	var items []FailedJobItem
	lines := strings.Split(output, "\n")
	uuidRegex := regexp.MustCompile(`([a-f0-9-]{36})`)

	// Pattern 1: Laravel 12 single-line twoColumnDetail with dot leaders
	// e.g.: "  2026-10-16 02:45:00 48b0c8ef-7a08-410c-b26a-d43c2242fa52 .................. database@default App\Jobs\WorkbenchProbe  "
	singleLineTwoCol := regexp.MustCompile(`(?i)^\s*(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2})\s+(\S+(?:\s*/\s*[a-f0-9-]{36})?)\s*[\.\s]{2,}\s*([\w_-]+)@([\w_-]+)(?:\s+([\w\\]+))?\s*$`)

	// Pattern 2: Multi-line two-column header and detail
	twoColHeader := regexp.MustCompile(`(?i)^\s*(\S+)\s*/\s*([a-f0-9-]{36})\s*[\. ]*(.*)$`)
	twoColDetail := regexp.MustCompile(`(?i)^\s*([\w\\]+)\s*[\. ]+([\w_-]+)\s*/\s*([\w_-]+)\s*$`)

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.Contains(line, "No failed jobs") {
			continue
		}

		// 1. Check single-line twoColumnDetail (Standard Laravel 12 console output)
		if sm := singleLineTwoCol.FindStringSubmatch(line); len(sm) > 5 {
			failedAt := sm[1]
			idOrUUID := sm[2]
			conn := sm[3]
			queue := sm[4]
			class := sm[5]

			id := idOrUUID
			uuid := idOrUUID
			if strings.Contains(idOrUUID, "/") {
				parts := strings.Split(idOrUUID, "/")
				id = strings.TrimSpace(parts[0])
				uuid = strings.TrimSpace(parts[1])
			} else if u := uuidRegex.FindString(idOrUUID); u != "" {
				uuid = u
			}

			items = append(items, FailedJobItem{
				ID:         id,
				UUID:       uuid,
				Connection: conn,
				Queue:      queue,
				Class:      class,
				FailedAt:   failedAt,
			})
			continue
		}

		// 2. Check two-column multi-line header/detail
		if hMatch := twoColHeader.FindStringSubmatch(line); len(hMatch) > 2 {
			id := hMatch[1]
			uuid := hMatch[2]
			failedAt := ""
			if len(hMatch) > 3 {
				failedAt = strings.Trim(strings.TrimSpace(hMatch[3]), ".")
			}

			class := ""
			conn := ""
			queue := ""
			if i+1 < len(lines) {
				nextLine := strings.TrimSpace(lines[i+1])
				if dMatch := twoColDetail.FindStringSubmatch(nextLine); len(dMatch) > 3 {
					class = dMatch[1]
					conn = dMatch[2]
					queue = dMatch[3]
					i++ // consume detail line
				}
			}

			items = append(items, FailedJobItem{
				ID:         id,
				UUID:       uuid,
				Connection: conn,
				Queue:      queue,
				Class:      class,
				FailedAt:   failedAt,
			})
			continue
		}

		// 3. Pipe table: "| ID | UUID | Connection | Queue | Class | Failed At |" or 5-column variant
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 6 {
				col1 := strings.TrimSpace(parts[1])
				if col1 == "ID" || col1 == "" || strings.HasPrefix(col1, "-") {
					continue
				}

				if len(parts) >= 7 {
					// 6 data columns: ID, UUID, Connection, Queue, Class, Failed At
					id := strings.TrimSpace(parts[1])
					uuid := strings.TrimSpace(parts[2])
					conn := strings.TrimSpace(parts[3])
					q := strings.TrimSpace(parts[4])
					class := strings.TrimSpace(parts[5])
					failedAt := strings.TrimSpace(parts[6])
					if uuid == "" {
						uuid = uuidRegex.FindString(line)
					}
					items = append(items, FailedJobItem{
						ID:         id,
						UUID:       uuid,
						Connection: conn,
						Queue:      q,
						Class:      class,
						FailedAt:   failedAt,
					})
				} else if len(parts) == 6 {
					// 5 data columns: ID, Connection, Queue, Class, Failed At
					id := strings.TrimSpace(parts[1])
					conn := strings.TrimSpace(parts[2])
					q := strings.TrimSpace(parts[3])
					class := strings.TrimSpace(parts[4])
					failedAt := strings.TrimSpace(parts[5])
					uuid := uuidRegex.FindString(id)
					if uuid == "" {
						uuid = id
					}
					items = append(items, FailedJobItem{
						ID:         id,
						UUID:       uuid,
						Connection: conn,
						Queue:      q,
						Class:      class,
						FailedAt:   failedAt,
					})
				}
			}
			continue
		}
	}
	return items
}

// RetryFailedJob retries a failed queue job.
func (ac *ActionsCoordinator) RetryFailedJob(phpBinary, projectPath, id string) (ActionRunResult, error) {
	return ac.RetryFailedJobContext(context.Background(), phpBinary, projectPath, id)
}
func (ac *ActionsCoordinator) RetryFailedJobContext(ctx context.Context, phpBinary, projectPath, id string) (ActionRunResult, error) {
	if id == "" {
		return ActionRunResult{}, errors.New("Job ID required")
	}
	start := time.Now()
	data, err := runner.CaptureContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{"artisan", "queue:retry", id, "--no-interaction", "--no-ansi"},
		Directory:  projectPath,
	}, 20*time.Second)

	dur := float64(time.Since(start).Milliseconds())
	lines := strings.Split(string(data), "\n")
	var cleanOutput []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			cleanOutput = append(cleanOutput, strings.TrimSpace(l))
		}
	}

	if err != nil {
		return ActionRunResult{
			Success:  false,
			ExitCode: 1,
			Output:   cleanOutput,
			Duration: dur,
			Error:    err.Error(),
		}, err
	}

	return ActionRunResult{
		Success:  true,
		ExitCode: 0,
		Output:   cleanOutput,
		Duration: dur,
	}, nil
}

// RetryAllFailedJobs retries all failed queue jobs.
func (ac *ActionsCoordinator) RetryAllFailedJobs(phpBinary, projectPath string) (ActionRunResult, error) {
	return ac.RetryAllFailedJobsContext(context.Background(), phpBinary, projectPath)
}

func (ac *ActionsCoordinator) RetryAllFailedJobsContext(ctx context.Context, phpBinary, projectPath string) (ActionRunResult, error) {
	start := time.Now()
	data, err := runner.CaptureContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{"artisan", "queue:retry", "all", "--no-interaction", "--no-ansi"},
		Directory:  projectPath,
	}, 30*time.Second)

	dur := float64(time.Since(start).Milliseconds())
	lines := strings.Split(string(data), "\n")
	var cleanOutput []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			cleanOutput = append(cleanOutput, strings.TrimSpace(l))
		}
	}

	if err != nil {
		return ActionRunResult{
			Success:  false,
			ExitCode: 1,
			Output:   cleanOutput,
			Duration: dur,
			Error:    err.Error(),
		}, err
	}

	return ActionRunResult{
		Success:  true,
		ExitCode: 0,
		Output:   cleanOutput,
		Duration: dur,
	}, nil
}

// ForgetFailedJob deletes a specific failed queue job from the failed_jobs table.
func (ac *ActionsCoordinator) ForgetFailedJob(phpBinary, projectPath, id string) (ActionRunResult, error) {
	return ac.ForgetFailedJobContext(context.Background(), phpBinary, projectPath, id)
}

func (ac *ActionsCoordinator) ForgetFailedJobContext(ctx context.Context, phpBinary, projectPath, id string) (ActionRunResult, error) {
	if id == "" {
		return ActionRunResult{}, errors.New("Job ID required")
	}
	start := time.Now()
	data, err := runner.CaptureContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{"artisan", "queue:forget", id, "--no-interaction", "--no-ansi"},
		Directory:  projectPath,
	}, 15*time.Second)

	dur := float64(time.Since(start).Milliseconds())
	lines := strings.Split(string(data), "\n")
	var cleanOutput []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			cleanOutput = append(cleanOutput, strings.TrimSpace(l))
		}
	}

	if err != nil {
		return ActionRunResult{
			Success:  false,
			ExitCode: 1,
			Output:   cleanOutput,
			Duration: dur,
			Error:    err.Error(),
		}, err
	}

	return ActionRunResult{
		Success:  true,
		ExitCode: 0,
		Output:   cleanOutput,
		Duration: dur,
	}, nil
}

// FlushFailedJobs removes all failed jobs from the failed_jobs table.
func (ac *ActionsCoordinator) FlushFailedJobs(phpBinary, projectPath string) (ActionRunResult, error) {
	return ac.FlushFailedJobsContext(context.Background(), phpBinary, projectPath)
}

func (ac *ActionsCoordinator) FlushFailedJobsContext(ctx context.Context, phpBinary, projectPath string) (ActionRunResult, error) {
	start := time.Now()
	data, err := runner.CaptureContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{"artisan", "queue:flush", "--no-interaction", "--no-ansi"},
		Directory:  projectPath,
	}, 20*time.Second)

	dur := float64(time.Since(start).Milliseconds())
	lines := strings.Split(string(data), "\n")
	var cleanOutput []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			cleanOutput = append(cleanOutput, strings.TrimSpace(l))
		}
	}

	if err != nil {
		return ActionRunResult{
			Success:  false,
			ExitCode: 1,
			Output:   cleanOutput,
			Duration: dur,
			Error:    err.Error(),
		}, err
	}

	return ActionRunResult{
		Success:  true,
		ExitCode: 0,
		Output:   cleanOutput,
		Duration: dur,
	}, nil
}

type ScheduledTaskItem struct {
	Expression  string `json:"expression"`
	Command     string `json:"command"`
	Interval    string `json:"interval"`
	Description string `json:"description"`
	NextDue     string `json:"nextDue"`
	HasMutex    bool   `json:"hasMutex"`
}

// ListScheduledTasks inspects scheduled tasks using artisan schedule:list.
func (ac *ActionsCoordinator) ListScheduledTasks(phpBinary, projectPath string) ([]ScheduledTaskItem, error) {
	return ac.ListScheduledTasksContext(context.Background(), phpBinary, projectPath)
}

func (ac *ActionsCoordinator) ListScheduledTasksContext(ctx context.Context, phpBinary, projectPath string) ([]ScheduledTaskItem, error) {
	// First probe if --json is supported
	jsonArgs := []string{"artisan", "schedule:list", "--json", "--no-interaction", "--no-ansi"}
	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       jsonArgs,
		Directory:  projectPath,
	}, 15*time.Second)

	if err == nil && res.ExitCode == 0 && strings.HasPrefix(strings.TrimSpace(res.Stdout), "[") {
		var rawList []struct {
			Expression   string `json:"expression"`
			Command      string `json:"command"`
			Interval     string `json:"interval"`
			Description  string `json:"description"`
			Next         string `json:"next"`
			NextDue      string `json:"nextDue"`
			NextDueDate  string `json:"next_due_date"`
			NextDueHuman string `json:"next_due_date_human"`
			HasMutex     bool   `json:"has_mutex"`
		}
		if json.Unmarshal([]byte(res.Stdout), &rawList) == nil {
			tasks := make([]ScheduledTaskItem, 0, len(rawList))
			for _, r := range rawList {
				next := r.NextDueHuman
				if next == "" {
					next = r.NextDueDate
				}
				if next == "" {
					next = r.NextDue
				}
				if next == "" {
					next = r.Next
				}
				tasks = append(tasks, ScheduledTaskItem{
					Expression:  r.Expression,
					Command:     r.Command,
					Interval:    r.Interval,
					Description: r.Description,
					NextDue:     next,
					HasMutex:    r.HasMutex,
				})
			}
			return tasks, nil
		}
	}

	// Fallback to text parsing of standard schedule:list
	textArgs := []string{"artisan", "schedule:list", "--no-interaction", "--no-ansi"}
	textRes, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       textArgs,
		Directory:  projectPath,
	}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if textRes.ExitCode != 0 {
		return nil, fmt.Errorf("schedule:list exited with code %d: %s", textRes.ExitCode, textRes.Combined)
	}

	return parseScheduledTasksText(textRes.Stdout), nil
}

func parseScheduledTasksText(output string) []ScheduledTaskItem {
	var items []ScheduledTaskItem
	lines := strings.Split(output, "\n")

	// Standard Laravel 9-12 two-column dot leaders:
	// "  0 0 * * *  php artisan backup:clean .................... Next Due: 5 hours from now"
	// "  */5 * * * *  App\Jobs\ProcessQueue ...................... Next Due: 3 minutes from now"
	// "  @daily  php artisan prune ............................... Next Due: 1 day from now"
	reCronDots := regexp.MustCompile(`(?i)^\s*(@\w+|(?:\S+\s+){4}\S+)\s+(.+?)[\.\s]{2,}Next Due:\s*(.+)$`)
	reGenericDots := regexp.MustCompile(`(?i)^\s*(@\w+|(?:\S+\s+){4}\S+)\s+(.+?)[\.\s]{2,}(.+)$`)

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.Contains(line, "No scheduled tasks") {
			continue
		}

		// Table format: | Command | Interval | Description | Next Due |
		if strings.HasPrefix(line, "|") {
			parts := strings.Split(line, "|")
			if len(parts) >= 4 {
				col1 := strings.TrimSpace(parts[1])
				if col1 == "Command" || col1 == "" || strings.HasPrefix(col1, "-") {
					continue
				}
				if len(parts) >= 5 {
					// 4 columns: Command | Interval | Description | Next Due
					items = append(items, ScheduledTaskItem{
						Command:     strings.TrimSpace(parts[1]),
						Expression:  strings.TrimSpace(parts[2]),
						Interval:    strings.TrimSpace(parts[2]),
						Description: strings.TrimSpace(parts[3]),
						NextDue:     strings.TrimSpace(parts[4]),
						HasMutex:    strings.Contains(strings.ToLower(line), "mutex"),
					})
				} else if len(parts) == 4 {
					// 3 columns: Command | Interval | Next Due
					items = append(items, ScheduledTaskItem{
						Command:    strings.TrimSpace(parts[1]),
						Expression: strings.TrimSpace(parts[2]),
						Interval:   strings.TrimSpace(parts[2]),
						NextDue:    strings.TrimSpace(parts[3]),
						HasMutex:   strings.Contains(strings.ToLower(line), "mutex"),
					})
				}
			}
			continue
		}

		// Check dot leader format with explicit "Next Due:"
		if m := reCronDots.FindStringSubmatch(rawLine); len(m) > 3 {
			expr := strings.TrimSpace(m[1])
			cmd := strings.TrimSpace(m[2])
			nextDue := strings.TrimSpace(m[3])
			hasMutex := strings.Contains(strings.ToLower(cmd), "mutex")
			cmdClean := strings.TrimSpace(strings.ReplaceAll(cmd, "[has mutex]", ""))

			items = append(items, ScheduledTaskItem{
				Expression: expr,
				Command:    cmdClean,
				Interval:   expr,
				NextDue:    nextDue,
				HasMutex:   hasMutex,
			})
			continue
		}

		// Fallback check dot leader format
		if m := reGenericDots.FindStringSubmatch(rawLine); len(m) > 3 {
			expr := strings.TrimSpace(m[1])
			cmd := strings.TrimSpace(m[2])
			nextDue := strings.TrimSpace(m[3])
			if strings.HasPrefix(nextDue, "Next Due:") {
				nextDue = strings.TrimSpace(strings.TrimPrefix(nextDue, "Next Due:"))
			}
			hasMutex := strings.Contains(strings.ToLower(cmd), "mutex")
			cmdClean := strings.TrimSpace(strings.ReplaceAll(cmd, "[has mutex]", ""))

			items = append(items, ScheduledTaskItem{
				Expression: expr,
				Command:    cmdClean,
				Interval:   expr,
				NextDue:    nextDue,
				HasMutex:   hasMutex,
			})
			continue
		}
	}

	return items
}

// RunScheduledTask runs all scheduled tasks (artisan schedule:run) or a specific command.
func (ac *ActionsCoordinator) RunScheduledTask(phpBinary, projectPath, command string) (ActionRunResult, error) {
	return ac.RunScheduledTaskContext(context.Background(), phpBinary, projectPath, command)
}

func (ac *ActionsCoordinator) RunScheduledTaskContext(ctx context.Context, phpBinary, projectPath, command string) (ActionRunResult, error) {
	cmdTrim := strings.TrimSpace(command)
	var args []string

	if cmdTrim == "" || cmdTrim == "all" || cmdTrim == "schedule:run" {
		args = []string{"artisan", "schedule:run", "--no-interaction", "--no-ansi"}
	} else {
		// Clean up command if it includes "php artisan "
		cmdClean := cmdTrim
		if strings.HasPrefix(cmdClean, "php artisan ") {
			cmdClean = strings.TrimPrefix(cmdClean, "php artisan ")
		} else if strings.HasPrefix(cmdClean, "artisan ") {
			cmdClean = strings.TrimPrefix(cmdClean, "artisan ")
		}
		// Tokenize args
		parts := strings.Fields(cmdClean)
		args = append([]string{"artisan"}, parts...)
		args = append(args, "--no-interaction", "--no-ansi")
	}

	start := time.Now()
	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       args,
		Directory:  projectPath,
	}, 60*time.Second)

	dur := float64(time.Since(start).Milliseconds())
	var lines []string
	exitCode := 0
	if res != nil {
		exitCode = res.ExitCode
		for _, l := range strings.Split(res.Combined, "\n") {
			if strings.TrimSpace(l) != "" {
				lines = append(lines, strings.TrimSpace(l))
			}
		}
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	return ActionRunResult{
		Success:  err == nil && exitCode == 0,
		ExitCode: exitCode,
		Output:   lines,
		Duration: dur,
		Error:    errMsg,
	}, err
}

// StartQueueWorker launches an owned worker process for the specified queue.
func (ac *ActionsCoordinator) StartQueueWorker(phpBinary, projectPath, queue string) (*QueueWorkerStatus, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.workerProcess != nil && ac.workerProcess.Running() {
		return nil, errors.New("An owned queue worker is already running")
	}
	if queue == "" {
		queue = "default"
	}

	spec := runner.Spec{
		Executable: phpBinary,
		Args:       []string{"artisan", "queue:work", "--queue=" + queue, "--sleep=2", "--tries=3", "--timeout=60", "--no-interaction", "--no-ansi"},
		Directory:  projectPath,
	}

	proc, err := runner.Start(spec)
	if err != nil {
		return nil, err
	}

	if ac.workerProcess != nil {
		_ = ac.workerProcess.Close()
	}
	ac.workerProcess = proc
	ac.workerQueue = queue
	ac.workerStart = time.Now()

	return &QueueWorkerStatus{
		Running:   true,
		PID:       proc.PID(),
		Queue:     queue,
		StartedAt: ac.workerStart.Format(time.RFC3339),
	}, nil
}

// StopQueueWorker stops the owned worker process gracefully (or forcibly if force=true).
func (ac *ActionsCoordinator) StopQueueWorker(force bool) error {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.workerProcess == nil || !ac.workerProcess.Running() {
		if ac.workerProcess != nil {
			ac.workerProcess.Close()
			ac.workerProcess = nil
		}
		return nil
	}
	if force {
		err := ac.workerProcess.ForceStop()
		if !ac.workerProcess.Wait(3 * time.Second) {
			return errors.New("owned queue worker has not exited")
		}
		ac.workerProcess.Close()
		ac.workerProcess = nil
		return err
	}
	err := ac.workerProcess.RequestStop()
	// Wait up to 3 seconds for safe active-job completion
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && ac.workerProcess.Running() {
		time.Sleep(100 * time.Millisecond)
	}
	if ac.workerProcess.Running() {
		// Do NOT silently escalate to ForceStop!
		return errors.New("queue worker is still finishing active job; wait or use force stop to terminate immediately")
	}
	ac.workerProcess.Close()
	ac.workerProcess = nil
	return err
}

// GetWorkerStatus returns the current live status of the owned worker.
func (ac *ActionsCoordinator) GetWorkerStatus() QueueWorkerStatus {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.workerProcess == nil {
		return QueueWorkerStatus{Running: false}
	}
	running := ac.workerProcess.Running()
	var out []string
	for _, e := range ac.workerProcess.Events() {
		if e.Text != "" {
			out = append(out, e.Text)
		}
	}
	return QueueWorkerStatus{
		Running:   running,
		PID:       ac.workerProcess.PID(),
		Queue:     ac.workerQueue,
		Output:    out,
		StartedAt: ac.workerStart.Format(time.RFC3339),
	}
}

// ExecuteCommand executes a validated Artisan or project command with argument array.
func (ac *ActionsCoordinator) ExecuteCommand(executable string, args []string, dir string, timeout time.Duration) (ActionRunResult, error) {
	return ac.ExecuteCommandContext(context.Background(), executable, args, dir, timeout)
}

func (ac *ActionsCoordinator) ExecuteCommandContext(ctx context.Context, executable string, args []string, dir string, timeout time.Duration) (ActionRunResult, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	start := time.Now()
	execution, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: executable,
		Args:       args,
		Directory:  dir,
	}, timeout)

	var data []byte
	if execution != nil {
		data = []byte(execution.Combined)
		if err == nil && execution.ExitCode != 0 {
			err = fmt.Errorf("command exited with code %d", execution.ExitCode)
		}
	}
	dur := float64(time.Since(start).Milliseconds())
	lines := strings.Split(string(data), "\n")
	var cleanOutput []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			cleanOutput = append(cleanOutput, l)
		}
	}

	exitCode := -1
	if execution != nil {
		exitCode = execution.ExitCode
	}
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	return ActionRunResult{
		Success:  err == nil,
		ExitCode: exitCode,
		Output:   cleanOutput,
		Duration: dur,
		Error:    errMsg,
	}, err
}

// DispatchEvent dynamically instantiates and dispatches a project event with typed payload parameters
func (ac *ActionsCoordinator) DispatchEvent(ctx context.Context, phpBinary, projectPath, eventClass, payloadJSON string) (ActionRunResult, error) {
	if phpBinary == "" {
		phpBinary = "php"
	}
	if payloadJSON == "" {
		payloadJSON = "{}"
	}

	phpCode := `
require_once __DIR__ . '/vendor/autoload.php';
$app = require_once __DIR__ . '/bootstrap/app.php';
$kernel = $app->make(Illuminate\Contracts\Console\Kernel::class);
$kernel->bootstrap();

$class = $argv[1];
$payload = json_decode($argv[2], true) ?? [];

if (!class_exists($class)) {
    echo "ERROR: Class $class not found";
    exit(1);
}

try {
    $ref = new ReflectionClass($class);
    $ctor = $ref->getConstructor();
    if ($ctor && count($ctor->getParameters()) > 0) {
        $args = [];
        foreach ($ctor->getParameters() as $p) {
            $name = $p->getName();
            if (array_key_exists($name, $payload)) {
                $args[] = $payload[$name];
            } elseif ($p->isDefaultValueAvailable()) {
                $args[] = $p->getDefaultValue();
            } else {
                $args[] = null;
            }
        }
        $ev = $ref->newInstanceArgs($args);
    } else {
        $ev = new $class();
    }
    event($ev);
    echo "SUCCESS: Event $class dispatched";
} catch (Throwable $e) {
    echo "ERROR: " . $e->getMessage();
    exit(1);
}
`

	start := time.Now()
	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{"-r", phpCode, eventClass, payloadJSON},
		Directory:  projectPath,
	}, 15*time.Second)

	dur := float64(time.Since(start).Milliseconds())
	var outLines []string
	if res != nil {
		outLines = strings.Split(strings.TrimSpace(res.Combined), "\n")
	}

	success := err == nil && res != nil && res.ExitCode == 0 && strings.Contains(res.Combined, "SUCCESS")
	errMsg := ""
	if !success {
		if err != nil {
			errMsg = err.Error()
		} else if res != nil {
			errMsg = res.Combined
		}
	}

	return ActionRunResult{
		Success:  success,
		ExitCode: func() int { if res != nil { return res.ExitCode }; return 1 }(),
		Output:   outLines,
		Duration: dur,
		Error:    errMsg,
	}, err
}

type ParameterInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	HasDefault   bool   `json:"hasDefault"`
	DefaultValue any    `json:"defaultValue,omitempty"`
}

type EventIntrospection struct {
	EventClass     string          `json:"eventClass"`
	Parameters     []ParameterInfo `json:"parameters"`
	Listeners      []string        `json:"listeners"`
	DispatchedFrom []string        `json:"dispatchedFrom"`
	HasListeners   bool            `json:"hasListeners"`
}

type JobIntrospection struct {
	JobClass       string          `json:"jobClass"`
	Parameters     []ParameterInfo `json:"parameters"`
	Summary        string          `json:"summary"`
	Schedule       string          `json:"schedule,omitempty"`
	Queue          string          `json:"queue"`
	Connection     string          `json:"connection"`
	Tries          int             `json:"tries"`
	Timeout        int             `json:"timeout"`
	DispatchedFrom []string        `json:"dispatchedFrom"`
}

type JobRunResult struct {
	Success    bool     `json:"success"`
	Output     []string `json:"output"`
	DurationMs float64  `json:"durationMs"`
	Queries    int      `json:"queries"`
	Error      string   `json:"error,omitempty"`
}

// IntrospectEvent inspects constructor arguments and listeners for a Laravel event
func (ac *ActionsCoordinator) IntrospectEvent(ctx context.Context, phpBinary, projectPath, eventClass string) (*EventIntrospection, error) {
	if phpBinary == "" {
		phpBinary = "php"
	}

	phpCode := `
require_once __DIR__ . '/vendor/autoload.php';
$app = require_once __DIR__ . '/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();

$class = $argv[1];
if (!class_exists($class)) {
    echo json_encode(['error' => "Class $class not found"]);
    exit(1);
}

$ref = new ReflectionClass($class);
$ctor = $ref->getConstructor();
$params = [];
if ($ctor) {
    foreach ($ctor->getParameters() as $p) {
        $type = $p->hasType() ? (string)$p->getType() : 'mixed';
        $params[] = [
            'name' => $p->getName(),
            'type' => $type,
            'hasDefault' => $p->isDefaultValueAvailable(),
            'defaultValue' => $p->isDefaultValueAvailable() ? $p->getDefaultValue() : null,
        ];
    }
}

// Get registered listeners
$listeners = [];
if ($app->bound('events')) {
    $raw = $app['events']->getListeners($class) ?? [];
    foreach ($raw as $l) {
        if (is_string($l)) {
            $listeners[] = $l;
        } elseif (is_array($l) && count($l) > 0) {
            $listeners[] = is_object($l[0]) ? get_class($l[0]) : (string)$l[0];
        } else {
            $listeners[] = 'Closure';
        }
    }
}

echo json_encode([
    'eventClass' => $class,
    'parameters' => $params,
    'listeners' => array_values(array_unique($listeners)),
    'hasListeners' => count($listeners) > 0
]);
`

	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{"-r", phpCode, eventClass},
		Directory:  projectPath,
	}, 15*time.Second)

	if err != nil {
		return nil, err
	}

	var intro EventIntrospection
	if err := json.Unmarshal([]byte(res.Stdout), &intro); err != nil {
		return nil, fmt.Errorf("failed to parse event introspection: %w (output: %s)", err, res.Combined)
	}

	// Scan codebase for "Dispatched from"
	intro.DispatchedFrom = scanDispatchedFrom(projectPath, eventClass)
	return &intro, nil
}

// IntrospectJob inspects constructor parameters, queue configuration, and handle summary of a job
func (ac *ActionsCoordinator) IntrospectJob(ctx context.Context, phpBinary, projectPath, jobClass string) (*JobIntrospection, error) {
	if phpBinary == "" {
		phpBinary = "php"
	}

	phpCode := `
require_once __DIR__ . '/vendor/autoload.php';
$app = require_once __DIR__ . '/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();

$class = $argv[1];
if (!class_exists($class)) {
    echo json_encode(['error' => "Class $class not found"]);
    exit(1);
}

$ref = new ReflectionClass($class);
$ctor = $ref->getConstructor();
$params = [];
if ($ctor) {
    foreach ($ctor->getParameters() as $p) {
        $type = $p->hasType() ? (string)$p->getType() : 'mixed';
        $params[] = [
            'name' => $p->getName(),
            'type' => $type,
            'hasDefault' => $p->isDefaultValueAvailable(),
            'defaultValue' => $p->isDefaultValueAvailable() ? $p->getDefaultValue() : null,
        ];
    }
}

// Inspect default properties
$defaults = $ref->getDefaultProperties();
$queue = $defaults['queue'] ?? 'default';
$connection = $defaults['connection'] ?? 'sync';
$tries = $defaults['tries'] ?? 1;
$timeout = $defaults['timeout'] ?? 60;

// Read handle() docblock or basic summary
$summary = 'Processes job workflow';
if ($ref->hasMethod('handle')) {
    $doc = $ref->getMethod('handle')->getDocComment();
    if ($doc) {
        $clean = preg_replace('/(\/\*\*|\*\/|\*)/', '', $doc);
        $summary = trim(explode("\n", trim($clean))[0]);
    }
}

echo json_encode([
    'jobClass' => $class,
    'parameters' => $params,
    'summary' => $summary ?: 'Processes job workflow',
    'queue' => (string)$queue,
    'connection' => (string)$connection,
    'tries' => (int)$tries,
    'timeout' => (int)$timeout,
]);
`

	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{"-r", phpCode, jobClass},
		Directory:  projectPath,
	}, 15*time.Second)

	if err != nil {
		return nil, err
	}

	var intro JobIntrospection
	if err := json.Unmarshal([]byte(res.Stdout), &intro); err != nil {
		return nil, fmt.Errorf("failed to parse job introspection: %w (output: %s)", err, res.Combined)
	}

	// Check schedule in routes/console.php
	intro.Schedule = inspectJobSchedule(projectPath, jobClass)
	intro.DispatchedFrom = scanDispatchedFrom(projectPath, jobClass)
	return &intro, nil
}

// RunJobNow executes the job's handle() synchronously in an ephemeral context
func (ac *ActionsCoordinator) RunJobNow(ctx context.Context, phpBinary, projectPath, jobClass, payloadJSON string) (*JobRunResult, error) {
	if phpBinary == "" {
		phpBinary = "php"
	}
	if payloadJSON == "" {
		payloadJSON = "{}"
	}

	phpCode := `
require_once __DIR__ . '/vendor/autoload.php';
$app = require_once __DIR__ . '/bootstrap/app.php';
$app->make(Illuminate\Contracts\Console\Kernel::class)->bootstrap();

$class = $argv[1];
$payload = json_decode($argv[2], true) ?? [];

if (!class_exists($class)) {
    echo "ERROR: Class $class not found\n";
    exit(1);
}

try {
    $ref = new ReflectionClass($class);
    $ctor = $ref->getConstructor();
    $args = [];
    if ($ctor) {
        foreach ($ctor->getParameters() as $p) {
            $name = $p->getName();
            if (array_key_exists($name, $payload)) {
                $args[] = $payload[$name];
            } elseif ($p->isDefaultValueAvailable()) {
                $args[] = $p->getDefaultValue();
            } else {
                $args[] = null;
            }
        }
    }
    
    $job = $ctor ? $ref->newInstanceArgs($args) : new $class();

    // Enable DB query logging
    if ($app->bound('db')) {
        $app['db']->enableQueryLog();
    }

    $out = $app->call([$job, 'handle']);
    $queries = count($app['db']->getQueryLog() ?? []);

    echo "SUCCESS\n";
    echo "QUERIES: $queries\n";
    if (is_string($out)) echo $out . "\n";
} catch (Throwable $e) {
    echo "ERROR: " . $e->getMessage() . "\n" . $e->getTraceAsString();
    exit(1);
}
`

	start := time.Now()
	res, err := runner.CaptureExecContext(ctx, runner.Spec{
		Executable: phpBinary,
		Args:       []string{"-r", phpCode, jobClass, payloadJSON},
		Directory:  projectPath,
	}, 30*time.Second)

	dur := float64(time.Since(start).Milliseconds())

	queries := 0
	outLines := []string{}
	success := err == nil && res != nil && res.ExitCode == 0 && strings.Contains(res.Combined, "SUCCESS")
	errMsg := ""

	if res != nil {
		for _, line := range strings.Split(res.Combined, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "QUERIES:") {
				_, _ = fmt.Sscanf(trimmed, "QUERIES: %d", &queries)
			} else if trimmed != "SUCCESS" && trimmed != "" {
				outLines = append(outLines, trimmed)
			}
		}
	}

	if !success {
		if err != nil {
			errMsg = err.Error()
		} else if res != nil {
			errMsg = res.Combined
		}
	}

	return &JobRunResult{
		Success:    success,
		Output:     outLines,
		DurationMs: dur,
		Queries:    queries,
		Error:      errMsg,
	}, err
}

func scanDispatchedFrom(projectPath, targetClass string) []string {
	shortName := targetClass
	if idx := strings.LastIndex(targetClass, "\\"); idx != -1 {
		shortName = targetClass[idx+1:]
	}

	var results []string
	searchDirs := []string{
		filepath.Join(projectPath, "app"),
		filepath.Join(projectPath, "routes"),
	}

	for _, dir := range searchDirs {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".php") {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			content := string(data)
			if strings.Contains(content, shortName+"::dispatch") ||
				strings.Contains(content, "dispatch(new "+shortName) ||
				strings.Contains(content, "event(new "+shortName) ||
				strings.Contains(content, shortName+"::class") {
				rel, _ := filepath.Rel(projectPath, path)
				results = append(results, rel)
			}
			return nil
		})
	}

	return results
}

func inspectJobSchedule(projectPath, jobClass string) string {
	shortName := jobClass
	if idx := strings.LastIndex(jobClass, "\\"); idx != -1 {
		shortName = jobClass[idx+1:]
	}

	consoleRoutes := filepath.Join(projectPath, "routes", "console.php")
	data, err := os.ReadFile(consoleRoutes)
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, shortName) && (strings.Contains(trimmed, "Schedule::") || strings.Contains(trimmed, "->job(")) {
			return trimmed
		}
	}

	return ""
}
