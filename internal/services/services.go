package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"herdworkbench/internal/runner"
)

type ServiceEngine string

const (
	EngineMailpit     ServiceEngine = "mailpit"
	EnginePostgres    ServiceEngine = "postgres"
	EngineMySQL       ServiceEngine = "mysql"
	EngineRedis       ServiceEngine = "redis"
	EngineMeilisearch ServiceEngine = "meilisearch"
	EngineDockerSail  ServiceEngine = "sail"
)

type ServiceStatus string

const (
	StatusStopped  ServiceStatus = "stopped"
	StatusStarting ServiceStatus = "starting"
	StatusRunning  ServiceStatus = "running"
	StatusExternal ServiceStatus = "external"
	StatusFailed   ServiceStatus = "failed"
)

type ServiceInstance struct {
	ID        string        `json:"id"`
	Engine    ServiceEngine `json:"engine"`
	Name      string        `json:"name"`
	Port      int           `json:"port"`
	AdminPort int           `json:"adminPort,omitempty"` // e.g. Mailpit web UI port
	DataDir   string        `json:"dataDir"`
	Status    ServiceStatus `json:"status"`
	IsOwned   bool          `json:"isOwned"`
	PID       int           `json:"pid"`
	MemoryMB  float64       `json:"memoryMb"`
	CPUUsage  float64       `json:"cpuUsage"`
	Error     string        `json:"error,omitempty"`
}

type DockerStatus struct {
	DockerInstalled bool   `json:"dockerInstalled"`
	DockerRunning   bool   `json:"dockerRunning"`
	WSLInstalled    bool   `json:"wslInstalled"`
	Version         string `json:"version"`
	Message         string `json:"message"`
}

type ServiceManager struct {
	mu        sync.Mutex
	instances map[string]*ServiceInstance
	processes map[string]*runner.Process
	baseDir   string
}

func NewServiceManager(baseDir string) *ServiceManager {
	if baseDir == "" {
		appData := os.Getenv("APPDATA")
		if appData != "" {
			baseDir = filepath.Join(appData, "HerdWorkbench", "services")
		} else {
			baseDir = filepath.Join(".", "data", "services")
		}
	}
	_ = os.MkdirAll(baseDir, 0755)

	sm := &ServiceManager{
		instances: make(map[string]*ServiceInstance),
		processes: make(map[string]*runner.Process),
		baseDir:   baseDir,
	}

	// Register default standard service profiles
	sm.registerDefaults()
	return sm
}

func (sm *ServiceManager) registerDefaults() {
	sm.instances["mailpit"] = &ServiceInstance{
		ID:        "mailpit",
		Engine:    EngineMailpit,
		Name:      "Mailpit (Local Mail Sandbox)",
		Port:      1025,
		AdminPort: 8025,
		DataDir:   filepath.Join(sm.baseDir, "mailpit"),
		Status:    StatusStopped,
	}
	sm.instances["postgres"] = &ServiceInstance{
		ID:      "postgres",
		Engine:  EnginePostgres,
		Name:    "PostgreSQL",
		Port:    5432,
		DataDir: filepath.Join(sm.baseDir, "postgres"),
		Status:  StatusStopped,
	}
	sm.instances["mysql"] = &ServiceInstance{
		ID:      "mysql",
		Engine:  EngineMySQL,
		Name:    "MySQL / MariaDB",
		Port:    3306,
		DataDir: filepath.Join(sm.baseDir, "mysql"),
		Status:  StatusStopped,
	}
	sm.instances["redis"] = &ServiceInstance{
		ID:      "redis",
		Engine:  EngineRedis,
		Name:    "Redis / Compatible Engine",
		Port:    6379,
		DataDir: filepath.Join(sm.baseDir, "redis"),
		Status:  StatusStopped,
	}
	sm.instances["meilisearch"] = &ServiceInstance{
		ID:      "meilisearch",
		Engine:  EngineMeilisearch,
		Name:    "Meilisearch",
		Port:    7700,
		DataDir: filepath.Join(sm.baseDir, "meilisearch"),
		Status:  StatusStopped,
	}
}

// IsPortInUse checks if a TCP port on localhost is currently listening.
func IsPortInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 300*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

// WaitForPort waits until port is open or timeout expires.
func WaitForPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if IsPortInUse(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// WaitForPortClose waits until port is closed or timeout expires.
func WaitForPortClose(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsPortInUse(port) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// RefreshStatuses scans all registered service ports and reconciles their status.
func (sm *ServiceManager) RefreshStatuses() []ServiceInstance {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var list []ServiceInstance
	for id, inst := range sm.instances {
		proc := sm.processes[id]
		inUse := IsPortInUse(inst.Port)

		if proc != nil && proc.Running() {
			if inst.Status == StatusStarting && inUse {
				inst.Status = StatusRunning
				inst.Error = ""
			}
			inst.IsOwned = true
			inst.PID = proc.PID()
		} else if inUse {
			// Port is occupied by an external service (or an unowned process)
			inst.Status = StatusExternal
			inst.IsOwned = false
			inst.PID = 0
			inst.Error = ""
		} else {
			if proc != nil {
				if proc.ExitCode() != 0 {
					inst.Status = StatusFailed
					inst.Error = fmt.Sprintf("Process exited with code %d", proc.ExitCode())
				} else {
					inst.Status = StatusStopped
				}
				_ = proc.Close()
				delete(sm.processes, id)
			} else if inst.Status != StatusFailed {
				inst.Status = StatusStopped
			}
			inst.IsOwned = false
			inst.PID = 0
		}
		list = append(list, *inst)
	}

	order := map[string]int{
		"mailpit":     1,
		"redis":       2,
		"mysql":       3,
		"postgres":    4,
		"meilisearch": 5,
	}
	sort.Slice(list, func(i, j int) bool {
		oi, okI := order[list[i].ID]
		if !okI {
			oi = 99
		}
		oj, okJ := order[list[j].ID]
		if !okJ {
			oj = 99
		}
		if oi != oj {
			return oi < oj
		}
		return list[i].ID < list[j].ID
	})

	return list
}

// List returns the current cached instances without probing network ports.
func (sm *ServiceManager) List() []ServiceInstance {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	var list []ServiceInstance
	for _, inst := range sm.instances {
		list = append(list, *inst)
	}

	order := map[string]int{
		"mailpit":     1,
		"redis":       2,
		"mysql":       3,
		"postgres":    4,
		"meilisearch": 5,
	}
	sort.Slice(list, func(i, j int) bool {
		oi, okI := order[list[i].ID]
		if !okI {
			oi = 99
		}
		oj, okJ := order[list[j].ID]
		if !okJ {
			oj = 99
		}
		if oi != oj {
			return oi < oj
		}
		return list[i].ID < list[j].ID
	})

	return list
}

// GetInstance returns a snapshot of the requested service instance.
func (sm *ServiceManager) GetInstance(id string) (ServiceInstance, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	inst, ok := sm.instances[id]
	if !ok {
		return ServiceInstance{}, errors.New("Service instance not found")
	}
	return *inst, nil
}

// StartService launches an owned service instance.
func (sm *ServiceManager) StartService(id string, customBinary string) error {
	sm.mu.Lock()
	inst, ok := sm.instances[id]
	if !ok {
		sm.mu.Unlock()
		return errors.New("Service not found")
	}

	if sm.processes[id] != nil && sm.processes[id].Running() {
		sm.mu.Unlock()
		return errors.New("Service is already running")
	}

	if IsPortInUse(inst.Port) {
		sm.mu.Unlock()
		return fmt.Errorf("Port %d is already in use by an external service or another process", inst.Port)
	}

	if inst.AdminPort > 0 && IsPortInUse(inst.AdminPort) {
		sm.mu.Unlock()
		return fmt.Errorf("Admin port %d is already in use", inst.AdminPort)
	}
	if err := os.MkdirAll(inst.DataDir, 0755); err != nil {
		sm.mu.Unlock()
		return err
	}

	spec, err := sm.buildSpec(inst, customBinary)
	if err != nil {
		sm.mu.Unlock()
		return err
	}

	proc, err := runner.Start(spec)
	if err != nil {
		inst.Status = StatusFailed
		inst.Error = err.Error()
		sm.mu.Unlock()
		return err
	}

	if previous := sm.processes[id]; previous != nil {
		_ = previous.Close()
	}
	sm.processes[id] = proc
	inst.Status = StatusStarting
	inst.IsOwned = true
	inst.PID = proc.PID()
	sm.mu.Unlock()

	// Wait for readiness in background
	go func() {
		ready := WaitForPort(inst.Port, 8*time.Second)
		if ready && inst.AdminPort > 0 {
			ready = WaitForPort(inst.AdminPort, 3*time.Second)
		}
		sm.mu.Lock()
		defer sm.mu.Unlock()
		if sm.processes[id] != proc {
			return
		}
		if ready && proc.Running() {
			inst.Status = StatusRunning
			inst.Error = ""
		} else if proc.Running() {
			inst.Status = StatusFailed
			inst.Error = "Process is running but its port did not become ready within 8 seconds"
		} else if !proc.Running() {
			inst.Status = StatusFailed
			inst.Error = fmt.Sprintf("Process exited with code %d", proc.ExitCode())
		}
	}()

	return nil
}

// ActiveOwnedCount returns the count of running owned service processes.
func (sm *ServiceManager) ActiveOwnedCount() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	count := 0
	for _, proc := range sm.processes {
		if proc != nil && proc.Running() {
			count++
		}
	}
	return count
}

// StopService stops an owned service instance gracefully (or forcibly if force=true).
func (sm *ServiceManager) StopService(id string, force bool) error {
	sm.mu.Lock()
	inst, ok := sm.instances[id]
	if !ok {
		sm.mu.Unlock()
		return errors.New("Service not found")
	}

	proc := sm.processes[id]
	if proc == nil || !proc.Running() {
		if inst.Status == StatusExternal {
			sm.mu.Unlock()
			return errors.New("Cannot stop external service that is not owned by Workbench")
		}
		inst.Status = StatusStopped
		inst.IsOwned = false
		inst.PID = 0
		if proc != nil {
			_ = proc.Close()
		}
		delete(sm.processes, id)
		sm.mu.Unlock()
		return nil
	}
	sm.mu.Unlock()

	if force {
		if err := proc.ForceStop(); err != nil {
			return err
		}
		if !proc.Wait(3 * time.Second) {
			return errors.New("owned service process tree has not exited")
		}
		_ = proc.Close()
		sm.mu.Lock()
		defer sm.mu.Unlock()
		if sm.processes[id] != proc {
			return nil
		}
		inst.Status = StatusStopped
		inst.PID = 0
		inst.IsOwned = false
		if proc != nil {
			_ = proc.Close()
		}
		delete(sm.processes, id)
		return nil
	}

	// Graceful stop requested: signal and wait up to 4s
	if err := proc.RequestStop(); err != nil {
		return err
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) && proc.Running() {
		time.Sleep(100 * time.Millisecond)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.processes[id] != proc {
		_ = proc.Close()
		return nil
	}
	if proc.Running() {
		inst.Status = StatusRunning // Still running, do NOT escalate to force stop!
		return fmt.Errorf("service %s did not exit within timeout (still running); use force stop to terminate", id)
	}

	inst.Status = StatusStopped
	inst.PID = 0
	inst.IsOwned = false
	if proc != nil {
		_ = proc.Close()
	}
	delete(sm.processes, id)
	return nil
}

// StopAll stops all currently owned running services.
func (sm *ServiceManager) StopAll(force bool) {
	sm.mu.Lock()
	var ids []string
	for id, proc := range sm.processes {
		if proc != nil && proc.Running() {
			ids = append(ids, id)
		}
	}
	sm.mu.Unlock()

	for _, id := range ids {
		_ = sm.StopService(id, force)
	}
}

func (sm *ServiceManager) buildSpec(inst *ServiceInstance, customBinary string) (runner.Spec, error) {
	switch inst.Engine {
	case EngineMailpit:
		bin := customBinary
		if bin == "" {
			var err error
			bin, err = exec.LookPath("mailpit.exe")
			if err != nil {
				bin = filepath.Join(sm.baseDir, "bin", "mailpit.exe")
			}
		}
		if _, err := os.Stat(bin); err != nil {
			return runner.Spec{}, errors.New("Mailpit is not installed. Open Settings → Dependencies to install Mailpit or choose its existing binary.")
		}
		return runner.Spec{
			Executable: bin,
			Args: []string{
				"--smtp", fmt.Sprintf("127.0.0.1:%d", inst.Port),
				"--listen", fmt.Sprintf("127.0.0.1:%d", inst.AdminPort),
				"--database", filepath.Join(inst.DataDir, "mailpit.db"),
			},
			Directory: inst.DataDir,
		}, nil

	case EnginePostgres:
		bin := customBinary
		if bin == "" {
			var err error
			bin, err = exec.LookPath("postgres.exe")
			if err != nil {
				bin = filepath.Join(sm.baseDir, "bin", "postgres.exe")
			}
		}
		if _, err := os.Stat(bin); err != nil {
			return runner.Spec{}, errors.New("PostgreSQL executable not found. Configure path or use Docker/Sail")
		}

		// Verify cluster existence or initialize via initdb (Finding 15)
		pgVersion := filepath.Join(inst.DataDir, "PG_VERSION")
		if _, err := os.Stat(pgVersion); os.IsNotExist(err) {
			initdbBin := filepath.Join(filepath.Dir(bin), "initdb.exe")
			if _, err := os.Stat(initdbBin); err != nil {
				initdbBin, _ = exec.LookPath("initdb.exe")
			}
			if initdbBin == "" {
				return runner.Spec{}, fmt.Errorf("PostgreSQL data cluster at %s does not exist, and initdb.exe was not found. Initialize the cluster or configure PostgreSQL tools", inst.DataDir)
			}
			initSpec := runner.Spec{
				Executable: initdbBin,
				Args:       []string{"-D", inst.DataDir, "-U", "postgres", "-E", "UTF8", "--no-locale"},
				Directory:  inst.DataDir,
			}
			res, initErr := runner.CaptureExec(initSpec, 30*time.Second)
			if initErr != nil || (res != nil && res.ExitCode != 0) {
				outMsg := ""
				if res != nil {
					outMsg = res.Combined
				}
				return runner.Spec{}, fmt.Errorf("failed to initialize PostgreSQL cluster via initdb: %w %s", initErr, outMsg)
			}
		}

		return runner.Spec{
			Executable: bin,
			Args:       []string{"-D", inst.DataDir, "-p", strconv.Itoa(inst.Port), "-h", "127.0.0.1"},
			Directory:  inst.DataDir,
		}, nil

	case EngineMySQL:
		bin := customBinary
		if bin == "" {
			var err error
			bin, err = exec.LookPath("mysqld.exe")
			if err != nil {
				bin = filepath.Join(sm.baseDir, "bin", "mysqld.exe")
			}
		}
		if _, err := os.Stat(bin); err != nil {
			return runner.Spec{}, errors.New("MySQL executable not found. Configure path or use Docker/Sail")
		}

		// Verify cluster existence or initialize via mysqld --initialize-insecure (Finding 15)
		mysqlSystemDB := filepath.Join(inst.DataDir, "mysql")
		if _, err := os.Stat(mysqlSystemDB); os.IsNotExist(err) {
			initSpec := runner.Spec{
				Executable: bin,
				Args:       []string{"--initialize-insecure", "--datadir=" + inst.DataDir},
				Directory:  inst.DataDir,
			}
			res, initErr := runner.CaptureExec(initSpec, 45*time.Second)
			if initErr != nil || (res != nil && res.ExitCode != 0) {
				outMsg := ""
				if res != nil {
					outMsg = res.Combined
				}
				return runner.Spec{}, fmt.Errorf("failed to initialize MySQL data directory via mysqld --initialize-insecure: %w %s", initErr, outMsg)
			}
		}

		return runner.Spec{
			Executable: bin,
			Args:       []string{"--datadir=" + inst.DataDir, "--port=" + strconv.Itoa(inst.Port), "--bind-address=127.0.0.1"},
			Directory:  inst.DataDir,
		}, nil

	case EngineRedis:
		bin := customBinary
		if bin == "" {
			var err error
			bin, err = exec.LookPath("redis-server.exe")
			if err != nil {
				bin = filepath.Join(sm.baseDir, "bin", "redis-server.exe")
			}
		}
		if _, err := os.Stat(bin); err != nil {
			return runner.Spec{}, errors.New("Redis executable not found. Native Redis on Windows is optional; use Docker/Sail or configure binary")
		}
		return runner.Spec{
			Executable: bin,
			Args:       []string{"--port", strconv.Itoa(inst.Port), "--bind", "127.0.0.1", "--dir", inst.DataDir},
			Directory:  inst.DataDir,
		}, nil

	case EngineMeilisearch:
		bin := customBinary
		if bin == "" {
			var err error
			bin, err = exec.LookPath("meilisearch.exe")
			if err != nil {
				bin = filepath.Join(sm.baseDir, "bin", "meilisearch.exe")
			}
		}
		if _, err := os.Stat(bin); err != nil {
			return runner.Spec{}, errors.New("Meilisearch executable not found. Place binary in PATH or services/bin/meilisearch.exe")
		}
		return runner.Spec{
			Executable: bin,
			Args:       []string{"--http-addr", fmt.Sprintf("127.0.0.1:%d", inst.Port), "--db-path", inst.DataDir},
			Directory:  inst.DataDir,
		}, nil

	default:
		return runner.Spec{}, fmt.Errorf("Unsupported engine: %s", inst.Engine)
	}
}

var (
	cachedDocker     DockerStatus
	lastDockerCheck  time.Time
	dockerCheckMutex sync.Mutex
)

// DetectDocker inspects the system for Docker and WSL availability without modifying anything.
func DetectDocker() DockerStatus {
	dockerCheckMutex.Lock()
	defer dockerCheckMutex.Unlock()
	if time.Since(lastDockerCheck) < 30*time.Second && cachedDocker.Message != "" {
		return cachedDocker
	}

	status := DockerStatus{
		Message: "Docker not available; native services remain the default.",
	}

	wslPath, err := exec.LookPath("wsl.exe")
	status.WSLInstalled = (err == nil && wslPath != "")

	dockerPath, err := exec.LookPath("docker.exe")
	if err != nil || dockerPath == "" {
		cachedDocker = status
		lastDockerCheck = time.Now()
		return status
	}
	status.DockerInstalled = true

	// Check version with hidden window flags (CREATE_NO_WINDOW = 0x08000000)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, dockerPath, "--version")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	out, err := cmd.Output()
	if err == nil {
		status.Version = strings.TrimSpace(string(out))
	}

	// Check if daemon is running with hidden window flags
	infoCtx, infoCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer infoCancel()
	infoCmd := exec.CommandContext(infoCtx, dockerPath, "info", "--format", "{{.ServerVersion}}")
	infoCmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	if err := infoCmd.Run(); err == nil {
		status.DockerRunning = true
		status.Message = "Docker daemon is running. Docker/Sail environments are available."
	} else {
		status.Message = "Docker executable found, but Docker daemon is not currently running."
	}

	cachedDocker = status
	lastDockerCheck = time.Now()
	return status
}
