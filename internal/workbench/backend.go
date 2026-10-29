package workbench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"herdworkbench/internal/actions"
	"herdworkbench/internal/activity"
	"herdworkbench/internal/architecture"
	"herdworkbench/internal/companion"
	"herdworkbench/internal/database"
	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/environment"
	"herdworkbench/internal/flags"
	"herdworkbench/internal/health"
	"herdworkbench/internal/mail"
	"herdworkbench/internal/performance"
	"herdworkbench/internal/quality"
	"herdworkbench/internal/realtime"
	"herdworkbench/internal/recipes"
	"herdworkbench/internal/remote"
	"herdworkbench/internal/reports"
	"herdworkbench/internal/runner"
	"herdworkbench/internal/runtimes"
	"herdworkbench/internal/services"
	"herdworkbench/internal/sharing"
	"herdworkbench/internal/storage"
	"herdworkbench/internal/telemetry"
	wktesting "herdworkbench/internal/testing"
	"herdworkbench/internal/tinker"
	"herdworkbench/internal/webhooks"
	"herdworkbench/internal/wizard"
)

type Run struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectId"`
	Name      string   `json:"name"`
	Running   bool     `json:"running"`
	PID       int      `json:"pid"`
	ExitCode  int      `json:"exitCode"`
	Output    []string `json:"output"`
}

type State struct {
	AuditReports  map[string]quality.AuditReport          `json:"auditReports"`
	AuditBrowsers map[string]quality.BrowserSessionStatus `json:"auditBrowsers"`
	Projects      []Project                               `json:"projects"`
	Selected      string                                  `json:"selected"`
	Trusted       bool                                    `json:"trusted"`
	Capture       Capture                                 `json:"capture"`
	Runs          []Run                                   `json:"runs"`
	Services      []services.ServiceInstance              `json:"services"`
	Docker        services.DockerStatus                   `json:"docker"`
	Settings      storage.Settings                        `json:"settings"`
	Tunnel        sharing.TunnelStatus                    `json:"tunnel"`
	Error         string                                  `json:"error"`
	Loading       bool                                    `json:"loading"`
	Dependencies  []dependencies.Status                   `json:"dependencies"`
	Installation  DependencyInstallation                  `json:"installation"`
}

type ownedRun struct {
	id, project, name string
	p                 *runner.Process
}

type Backend struct {
	companionStore      *companion.Store
	companionMu         sync.Mutex
	companionMutationMu sync.Mutex
	snapshotMu          sync.Mutex
	companionPlans      map[string]RepairPlan
	profileSessions     map[string]ProfileSession
	auditReports        map[string]quality.AuditReport
	auditCancels        map[string]context.CancelFunc
	auditBrowsers       map[string]*quality.BrowserSession
	auditSetups         map[string]quality.AuditSetupPlan
	dependencies        *dependencies.Manager
	dependencyInstallMu sync.Mutex
	installation        DependencyInstallation
	installCancel       context.CancelFunc
	mu                  sync.Mutex
	startMu             sync.Mutex
	projects            []Project
	selected            string
	trusted             map[string]bool
	runs                []*ownedRun
	errorText           string
	loading             bool
	clock               *Clockwork
	notify              func()
	caps                map[string][]Capability
	storageMgr          *storage.SettingsManager
	diagDB              *storage.DiagnosticsDB
	serviceMgr          *services.ServiceManager
	mailClient          *mail.MailpitClient
	actionsCoord        *actions.ActionsCoordinator
	testCoord           *wktesting.TestCoordinator
	recipeCat           *recipes.RecipeCatalog
	tunnelMgr           *sharing.TunnelManager
	dockerStatus        services.DockerStatus
	tinkerSvc           *tinker.TinkerService
	dbSvc               *database.DatabaseService
	runtimeMgr          *runtimes.Manager
	remoteMgr           *remote.ProfileManager
	realtimeIns         *realtime.Inspector
	dumpServer          *telemetry.DumpServer
	webhookMgr          *webhooks.WebhookManager
	isShuttingDown      bool
	inFlightOps         int32
	ctx                 context.Context
	cancel              context.CancelFunc
	readmeBaselines     map[string]string
	iniPreviews         map[string]*environment.IniChangePreview
}

func (b *Backend) beginOperation() (context.Context, func(), error) {
	b.mu.Lock()
	if b.isShuttingDown {
		b.mu.Unlock()
		return nil, nil, errors.New("cannot start operation: workbench is shutting down")
	}
	atomic.AddInt32(&b.inFlightOps, 1)
	ctx := b.ctx
	b.mu.Unlock()

	done := func() {
		atomic.AddInt32(&b.inFlightOps, -1)
	}
	return ctx, done, nil
}

func NewBackend(notify func()) *Backend {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Backend{
		companionStore: companion.NewStore(""), companionPlans: map[string]RepairPlan{}, profileSessions: map[string]ProfileSession{},
		auditReports: map[string]quality.AuditReport{}, auditCancels: map[string]context.CancelFunc{}, auditBrowsers: map[string]*quality.BrowserSession{}, auditSetups: map[string]quality.AuditSetupPlan{},
		projects:        []Project{},
		trusted:         map[string]bool{},
		runs:            []*ownedRun{},
		notify:          notify,
		caps:            map[string][]Capability{},
		readmeBaselines: map[string]string{},
		iniPreviews:     map[string]*environment.IniChangePreview{},
		actionsCoord:    actions.NewActionsCoordinator(),
		testCoord:       wktesting.NewTestCoordinator(),
		recipeCat:       recipes.NewRecipeCatalog(),
		tunnelMgr:       sharing.NewTunnelManager(),
		dockerStatus:    services.DetectDocker(),
		tinkerSvc:       tinker.NewTinkerService(),
		dbSvc:           database.NewDatabaseService(),
		ctx:             ctx,
		cancel:          cancel,
	}

	b.dependencies = dependencies.NewManager("", b.emit)
	var err error
	b.storageMgr, err = storage.NewSettingsManager("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: storage manager init: %v\n", err)
	}

	b.diagDB, err = storage.OpenDiagnosticsDB("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: diagnostics db init: %v\n", err)
	}

	b.serviceMgr = services.NewServiceManager("")
	b.serviceMgr.RefreshStatuses()
	b.mailClient = mail.NewMailpitClient("http://127.0.0.1:8025")
	b.realtimeIns = realtime.NewInspector()
	b.runtimeMgr = runtimes.NewManager("")
	remoteDir := filepath.Join(os.TempDir(), "herd-workbench")
	b.remoteMgr, _ = remote.NewProfileManager(remoteDir)
	b.webhookMgr = webhooks.NewWebhookManager(200, func(d webhooks.WebhookDelivery) {
		b.emit()
	})
	dumpSrv, err := telemetry.NewDumpServer(9912, 250, func(d telemetry.DumpPayload) {
		if b.diagDB != nil && d.ProjectID != "" {
			_ = b.diagDB.SaveDump(d.ProjectID, d.File, d.Line, d.Payload)
		}
		b.emit()
	})
	if err == nil {
		b.dumpServer = dumpSrv
	}

	b.clock = NewClockwork(func(c Capture) {
		b.emit()
		// Save requests to diagnostic read model asynchronously
		if b.diagDB != nil {
			if c.ProjectID == "" {
				return
			}
			sel := c.ProjectID
			for _, r := range c.Requests {
				_ = b.diagDB.SaveRecord(storage.DiagnosticRecord{
					ID:        sel + ":" + r.ID,
					ProjectID: sel,
					Source:    "clockwork",
					Type:      "request",
					Timestamp: time.Unix(int64(r.Time), 0).UTC().Format(time.RFC3339),
					Status:    r.Status,
					URI:       r.URI,
					Duration:  r.Duration,
					Summary:   fmt.Sprintf("%s %s (%d) - %.1fms", r.Method, r.URI, r.Status, r.Duration),
					Details:   fmt.Sprintf(`{"queries":%d,"exceptions":%d}`, r.QueryCount, r.Exceptions),
				})
			}
			if b.storageMgr != nil {
				_ = b.diagDB.Prune(b.storageMgr.Get().MaxRetention)
			}
		}
	})

	// Restore trusted projects from persistent settings
	if b.storageMgr != nil {
		s := b.storageMgr.Get()
		for id, tr := range s.Trusted {
			if tr {
				b.trusted[id] = true
			}
		}
	}

	return b
}

func (b *Backend) emit() {
	if b.notify != nil {
		b.notify()
	}
}

func (b *Backend) Snapshot() State {
	b.mu.Lock()
	s := State{
		Projects:     append([]Project{}, b.projects...),
		Selected:     b.selected,
		Trusted:      b.trusted[b.selected],
		Error:        b.errorText,
		Loading:      b.loading,
		Installation: b.installation,
		Docker:       b.dockerStatus,
		Runs:         []Run{},
	}
	s.AuditReports = map[string]quality.AuditReport{}
	for id, report := range b.auditReports {
		s.AuditReports[id] = report.Clone()
	}
	s.AuditBrowsers = map[string]quality.BrowserSessionStatus{}
	for id, browser := range b.auditBrowsers {
		s.AuditBrowsers[id] = browser.Status()
	}
	runs := append([]*ownedRun{}, b.runs...)
	s.Installation.Output = append([]string{}, b.installation.Output...)
	b.mu.Unlock()

	s.Dependencies = b.DependencyStatus(s.Selected)
	s.Capture = b.clock.State()
	if s.Capture.ProjectID != "" && s.Capture.ProjectID != s.Selected {
		s.Capture = Capture{ProjectID: s.Selected, Requests: []Request{}, Message: "Switching project capture"}
	}
	for _, r := range runs {
		view := Run{
			ID:        r.id,
			ProjectID: r.project,
			Name:      r.name,
			PID:       r.p.PID(),
			Running:   r.p.Running(),
			ExitCode:  r.p.ExitCode(),
			Output:    []string{},
		}
		for _, e := range r.p.Events() {
			if e.Text != "" {
				view.Output = append(view.Output, Redact(e.Text))
			}
		}
		s.Runs = append(s.Runs, view)
	}

	if b.serviceMgr != nil {
		s.Services = b.serviceMgr.List()
	}
	if b.storageMgr != nil {
		s.Settings = b.storageMgr.Get()
	}
	if b.tunnelMgr != nil {
		s.Tunnel = b.tunnelMgr.GetStatus()
	}

	return s
}

func (b *Backend) Discover() error {
	b.mu.Lock()
	if b.loading {
		b.mu.Unlock()
		return nil
	}
	b.loading = true
	b.errorText = ""
	b.mu.Unlock()
	b.emit()

	h, err := FindHerd()
	var projects []Project
	if err == nil {
		projects, err = h.Discover()
	}

	b.mu.Lock()
	b.loading = false
	b.dockerStatus = services.DetectDocker()
	if err != nil {
		b.errorText = err.Error()
	} else {
		b.projects = projects
	}
	b.mu.Unlock()
	b.emit()
	return err
}

func (b *Backend) project(id string, trust bool) (Project, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, p := range b.projects {
		if p.ID == id {
			if trust && !b.trusted[id] {
				return Project{}, errors.New("Trust this project for this session before executing PHP or Node code")
			}
			if trust && p.PHPBinary == "" {
				return Project{}, errors.New("Could not resolve the project's PHP executable")
			}
			return p, nil
		}
	}
	return Project{}, errors.New("Project not found; rediscover Herd sites")
}

func (b *Backend) Select(id string) error {
	p, err := b.project(id, false)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.selected = id
	trusted := b.trusted[id]
	b.mu.Unlock()
	b.clock.Select(p)
	if trusted {
		_ = b.clock.Watch(p)
	}

	if b.storageMgr != nil {
		_ = b.storageMgr.Update(func(s *storage.Settings) {
			s.RecentProject = id
		})
	}
	return nil
}

func (b *Backend) Trust(id string) error {
	p, err := b.project(id, false)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.trusted[id] = true
	isSelected := b.selected == id
	b.mu.Unlock()

	if b.storageMgr != nil {
		_ = b.storageMgr.Update(func(s *storage.Settings) {
			s.Trusted[id] = true
		})
	}

	if isSelected {
		_ = b.clock.Watch(p)
	}

	b.emit()
	return nil
}

func (b *Backend) SetSecure(id string, secure bool) error {
	p, err := b.project(id, true)
	if err != nil {
		return err
	}
	h, err := FindHerd()
	if err != nil {
		return err
	}
	if secure {
		err = h.Secure(p.Path)
	} else {
		err = h.Unsecure(p.Path)
	}
	if err == nil {
		_ = b.Discover()
	}
	return err
}

func (b *Backend) Refresh() (Capture, error) {
	b.mu.Lock()
	id := b.selected
	b.mu.Unlock()
	if _, err := b.project(id, true); err != nil {
		return b.clock.State(), err
	}
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return b.clock.State(), err
	}
	defer done()
	return b.clock.Refresh(opCtx)
}

func (b *Backend) Watch(enable bool) error {
	if !enable {
		b.clock.Stop()
		b.emit()
		return nil
	}
	b.mu.Lock()
	id := b.selected
	b.mu.Unlock()
	p, err := b.project(id, true)
	if err != nil {
		return err
	}
	return b.clock.Watch(p)
}

func dependencyKey(p Project) string {
	h := sha256.New()
	for _, name := range []string{"composer.lock", "vendor/composer/installed.json"} {
		data, _ := os.ReadFile(filepath.Join(p.Path, name))
		h.Write(data)
	}
	h.Write([]byte(p.PHPBinary))
	return p.ID + hex.EncodeToString(h.Sum(nil))
}

func (b *Backend) Capabilities(id string) ([]Capability, error) {
	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	key := dependencyKey(p)
	b.mu.Lock()
	cached, ok := b.caps[key]
	b.mu.Unlock()
	if ok {
		return cached, nil
	}
	result := Capabilities(p)
	complete := true
	for _, c := range result {
		if c.Status != "verified" {
			complete = false
		}
	}
	if complete {
		b.mu.Lock()
		b.caps[key] = result
		b.mu.Unlock()
	}
	return result, nil
}

func (b *Backend) Routes(id string) ([]Route, error) {
	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	caps, err := b.Capabilities(id)
	if err != nil {
		return nil, err
	}
	for _, c := range caps {
		if c.Command == "route:list" && c.Status == "verified" && c.JSON {
			return Routes(p)
		}
	}
	return nil, errors.New("The installed route:list JSON option could not be verified")
}

func (b *Backend) Start(id, kind string) (Run, error) { return b.startProcess(id, kind, 0) }
func (b *Backend) startProcess(id, kind string, vitePort int) (Run, error) {
	b.startMu.Lock()
	defer b.startMu.Unlock()

	b.mu.Lock()
	if b.isShuttingDown {
		b.mu.Unlock()
		return Run{}, errors.New("Shutdown is in progress; new process starts are not allowed")
	}
	atomic.AddInt32(&b.inFlightOps, 1)
	defer atomic.AddInt32(&b.inFlightOps, -1)
	b.mu.Unlock()

	p, err := b.project(id, true)
	if err != nil {
		return Run{}, err
	}
	spec := runner.Spec{Executable: p.PHPBinary, Directory: p.Path}
	name := ""
	switch kind {
	case "queue-work":
		name = "Queue worker"
		spec.Args = []string{"artisan", "queue:work", "--no-interaction", "--no-ansi"}
	case "schedule-work":
		name = "Scheduler"
		spec.Args = []string{"artisan", "schedule:work", "--no-interaction", "--no-ansi"}
	case "queue-once":
		name = "Queue · one job"
		spec.Args = []string{"artisan", "queue:work", "--once", "--no-interaction", "--no-ansi"}
	case "vite":
		name = "Vite"
		node, err := exec.LookPath("node.exe")
		if err != nil {
			return Run{}, errors.New("Node was not found in PATH")
		}
		npm, err := exec.LookPath("npm.cmd")
		if err != nil {
			return Run{}, errors.New("npm was not found in PATH")
		}
		cli := filepath.Join(filepath.Dir(npm), "node_modules", "npm", "bin", "npm-cli.js")
		if _, err = os.Stat(cli); err != nil {
			return Run{}, errors.New("Cannot resolve npm-cli.js beside npm.cmd")
		}
		spec.Executable = node
		spec.Args = []string{cli, "run", "dev"}
		if vitePort > 0 {
			spec.Args = append(spec.Args, "--", "--port", fmt.Sprint(vitePort), "--strictPort")
		}
	default:
		return Run{}, errors.New("Unsupported process action")
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.runs {
		if r.project == id && r.name == name && r.p.Running() {
			return Run{}, errors.New("This process is already running")
		}
	}
	process, err := runner.Start(spec)
	if err != nil {
		return Run{}, err
	}
	go func() {
		<-process.Done()
		b.emit()
	}()
	run := &ownedRun{id: time.Now().Format("20060102150405.000000000"), project: id, name: name, p: process}
	b.runs = append(b.runs, run)
	return Run{ID: run.id, ProjectID: id, Name: name, Running: true, PID: process.PID()}, nil
}

func (b *Backend) Stop(id string, force bool) error {
	b.mu.Lock()
	runs := append([]*ownedRun{}, b.runs...)
	b.mu.Unlock()
	for _, r := range runs {
		if r.id == id {
			if force {
				return r.p.ForceStop()
			}
			return r.p.RequestStop()
		}
	}
	return errors.New("Owned process not found")
}

func (b *Backend) Active() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := 0
	for _, r := range b.runs {
		if r.p.Running() {
			n++
		}
	}
	if b.serviceMgr != nil {
		n += b.serviceMgr.ActiveOwnedCount()
	}
	if b.actionsCoord != nil && b.actionsCoord.GetWorkerStatus().Running {
		n++
	}
	if b.tunnelMgr != nil && b.tunnelMgr.GetStatus().Running {
		n++
	}
	for _, browser := range b.auditBrowsers {
		if browser.Status().Running {
			n++
		}
	}
	n += int(atomic.LoadInt32(&b.inFlightOps))
	return n
}

func (b *Backend) HasActiveRuns() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.runs {
		if r.p != nil && r.p.Running() {
			return true
		}
	}
	return false
}

func (b *Backend) StopAll(force bool) error {
	b.startMu.Lock()

	b.clock.Stop()
	b.mu.Lock()
	b.isShuttingDown = true
	if b.cancel != nil {
		b.cancel()
	}
	runs := append([]*ownedRun{}, b.runs...)
	b.mu.Unlock()
	b.startMu.Unlock()

	for _, r := range runs {
		if r.p.Running() {
			if force {
				_ = r.p.ForceStop()
			} else {
				_ = r.p.RequestStop()
			}
		}
	}
	// Stop owned services as well
	if b.serviceMgr != nil {
		b.serviceMgr.StopAll(force)
	}
	if b.actionsCoord != nil && b.actionsCoord.GetWorkerStatus().Running {
		_ = b.actionsCoord.StopQueueWorker(force)
	}
	if b.tunnelMgr != nil {
		_ = b.tunnelMgr.StopTunnel(force)
	}

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		if b.Active() == 0 {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("Some process trees are still running. Wait, cancel exit, or explicitly force-stop owned processes")
}

func (b *Backend) Shutdown() {
	b.startMu.Lock()

	b.clock.Stop()
	b.mu.Lock()
	b.isShuttingDown = true
	if b.cancel != nil {
		b.cancel()
	}
	runs := append([]*ownedRun{}, b.runs...)
	browsers := make([]*quality.BrowserSession, 0, len(b.auditBrowsers))
	for _, browser := range b.auditBrowsers {
		browsers = append(browsers, browser)
	}
	b.mu.Unlock()
	b.startMu.Unlock()
	for _, browser := range browsers {
		browser.Close()
	}

	if b.serviceMgr != nil {
		b.serviceMgr.StopAll(true)
	}
	if b.actionsCoord != nil {
		_ = b.actionsCoord.StopQueueWorker(true)
	}
	if b.tunnelMgr != nil {
		_ = b.tunnelMgr.StopTunnel(true)
	}
	if b.dumpServer != nil {
		_ = b.dumpServer.Close()
	}
	for _, r := range runs {
		if r.p.Running() {
			_ = r.p.ForceStop()
		}
		if r.p.Wait(3 * time.Second) {
			_ = r.p.Close()
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt32(&b.inFlightOps) != 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if b.diagDB != nil {
		_ = b.diagDB.Close()
	}
}

func (b *Backend) CancelShutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.isShuttingDown {
		return
	}
	b.isShuttingDown = false
	b.ctx, b.cancel = context.WithCancel(context.Background())
}

func (b *Backend) ProjectURL(id string) (string, error) {
	p, err := b.project(id, false)
	if err != nil {
		return "", err
	}
	_, err = localBase(p.URL)
	return p.URL, err
}

// -------------------------------------------------------------
// ENVIRONMENT & PHP DIRECTIVES (A12, B02, X01)
// -------------------------------------------------------------

var allowedIniDirectives = map[string]bool{
	"memory_limit":        true,
	"upload_max_filesize": true,
	"post_max_size":       true,
	"max_execution_time":  true,
	"max_input_time":      true,
	"opcache.enable":      true,
	"opcache.enable_cli":  true,
	"display_errors":      true,
	"error_reporting":     true,
	"date.timezone":       true,
	"xdebug.mode":         true,
}

func (b *Backend) resolveProjectIni(id string) (string, error) {
	p, err := b.project(id, true)
	if err != nil {
		return "", err
	}
	info, err := environment.InspectPHP(p.PHPBinary)
	if err != nil {
		return "", fmt.Errorf("could not inspect PHP runtime: %w", err)
	}
	if info.LoadedIni == "" {
		return "", errors.New("no loaded php.ini file detected for this PHP runtime")
	}
	cleanIni := filepath.Clean(info.LoadedIni)
	if _, err := os.Stat(cleanIni); err != nil {
		return "", fmt.Errorf("detected php.ini at %s does not exist on disk: %w", cleanIni, err)
	}
	return cleanIni, nil
}

func (b *Backend) GetEnv(id string) (*environment.EnvFile, error) {
	p, err := b.project(id, false)
	if err != nil {
		return nil, err
	}
	return environment.ParseEnv(filepath.Join(p.Path, ".env"))
}

func (b *Backend) SaveEnv(id string, env *environment.EnvFile, expectedFingerprint string) error {
	p, err := b.project(id, true)
	if err != nil {
		return err
	}
	if env == nil {
		return errors.New("env file payload cannot be nil")
	}
	// Derive canonical path from backend-owned project (Finding 2)
	canonicalPath := filepath.Clean(filepath.Join(p.Path, ".env"))
	rel, err := filepath.Rel(p.Path, canonicalPath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return errors.New("invalid path: path escape detected")
	}
	if env.Path != "" && filepath.Clean(env.Path) != canonicalPath {
		return errors.New("caller path substitution rejected: path must match project .env")
	}

	// Verify symlink target if .env is a symlink
	if fi, err := os.Lstat(canonicalPath); err == nil && (fi.Mode()&os.ModeSymlink != 0) {
		realTarget, err := filepath.EvalSymlinks(canonicalPath)
		if err != nil {
			return fmt.Errorf("could not evaluate .env symlink: %w", err)
		}
		realProject, _ := filepath.EvalSymlinks(p.Path)
		if realProject == "" {
			realProject = filepath.Clean(p.Path)
		}
		targetRel, relErr := filepath.Rel(realProject, realTarget)
		if relErr != nil || targetRel == ".." || strings.HasPrefix(targetRel, ".."+string(filepath.Separator)) || filepath.IsAbs(targetRel) {
			return errors.New("security guard: .env symlink resolves outside project directory")
		}
	}

	env.Path = canonicalPath
	return environment.SaveEnv(env, expectedFingerprint)
}

func (b *Backend) GetEnvDrift(id string) (environment.DriftReport, error) {
	p, err := b.project(id, false)
	if err != nil {
		return environment.DriftReport{}, err
	}
	return environment.CompareDrift(filepath.Join(p.Path, ".env"), filepath.Join(p.Path, ".env.example"))
}

func (b *Backend) GetPHPInfo(id string) (*environment.PHPInfo, error) {
	p, err := b.project(id, false)
	if err != nil {
		return nil, err
	}
	return environment.InspectPHP(p.PHPBinary)
}

func (b *Backend) PreviewIniChange(id, key, val string) (*environment.IniChangePreview, error) {
	if !allowedIniDirectives[strings.ToLower(strings.TrimSpace(key))] {
		return nil, fmt.Errorf("directive %q is not in the approved configurable directives list", key)
	}
	if err := environment.ValidateIniDirective(key, val); err != nil {
		return nil, err
	}
	iniPath, err := b.resolveProjectIni(id)
	if err != nil {
		return nil, err
	}
	preview, err := environment.PreviewIniChange(iniPath, key, val)
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	b.iniPreviews[id+":"+key] = preview
	b.mu.Unlock()
	return preview, nil
}

func (b *Backend) ApplyIniChange(id, key, val string) error {
	if !allowedIniDirectives[strings.ToLower(strings.TrimSpace(key))] {
		return fmt.Errorf("directive %q is not in the approved configurable directives list", key)
	}
	if err := environment.ValidateIniDirective(key, val); err != nil {
		return err
	}
	iniPath, err := b.resolveProjectIni(id)
	if err != nil {
		return err
	}
	if err := environment.ValidateIniDirective(key, val); err != nil {
		return err
	}
	b.mu.Lock()
	preview := b.iniPreviews[id+":"+key]
	b.mu.Unlock()
	if preview == nil || preview.IniPath != iniPath || preview.NewValue != val {
		return errors.New("preview this php.ini change before applying it")
	}
	if err := environment.ApplyIniChangeChecked(iniPath, key, val, preview.Fingerprint); err != nil {
		return err
	}
	b.mu.Lock()
	delete(b.iniPreviews, id+":"+key)
	b.mu.Unlock()
	return nil
}

// -------------------------------------------------------------
// LOGS, ERRORS & DUMPS (A16, A21)
// -------------------------------------------------------------

func (b *Backend) GetRawLogs(id string) ([]LogEntry, error) {
	p, err := b.project(id, false)
	if err != nil {
		return nil, err
	}
	entries, err := ReadLaravelLogs(p.Path, 512*1024)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		return make([]LogEntry, 0), nil
	}
	// Index error entries in diagnostics DB idempotently (Findings 7 & 16)
	if b.diagDB != nil {
		_ = b.diagDB.PruneRetention(30)
		if b.storageMgr != nil {
			maxRet := b.storageMgr.Get().MaxRetention
			if maxRet > 0 {
				_ = b.diagDB.Prune(maxRet)
			}
		}
		for _, e := range entries {
			if e.Level == "ERROR" || e.Level == "CRITICAL" {
				logKey := fmt.Sprintf("%s:%s", id, e.ID)
				_, _ = b.diagDB.RecordErrorIdempotent(id, logKey, e.Fingerprint, e.Level, e.Message, e.File, e.Line, e.Stack)
			}
		}
	}
	return entries, nil
}

func (b *Backend) GetErrors(id, search string, limit int) ([]storage.ErrorLogRecord, error) {
	if b.diagDB == nil {
		return make([]storage.ErrorLogRecord, 0), errors.New("diagnostic storage is unavailable")
	}
	records, err := b.diagDB.QueryErrors(id, search, limit)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return make([]storage.ErrorLogRecord, 0), nil
	}
	return records, nil
}

func (b *Backend) GetDumps(id string, limit int) ([]storage.DumpRecord, error) {
	if b.diagDB == nil {
		return make([]storage.DumpRecord, 0), errors.New("diagnostic storage is unavailable")
	}
	records, err := b.diagDB.QueryDumps(id, limit)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return make([]storage.DumpRecord, 0), nil
	}
	return records, nil
}

func (b *Backend) SaveDump(id, file string, line int, payload string) error {
	if _, err := b.project(id, false); err != nil {
		return err
	}
	if b.diagDB == nil {
		return errors.New("diagnostic storage is unavailable")
	}
	return b.diagDB.SaveDump(id, file, line, payload)
}

func (b *Backend) GetHistory(id, search string, limit, offset int) ([]storage.DiagnosticRecord, error) {
	if b.diagDB == nil {
		return make([]storage.DiagnosticRecord, 0), errors.New("diagnostic storage is unavailable")
	}
	records, err := b.diagDB.QueryRecords(id, search, limit, offset)
	if err != nil {
		return nil, err
	}
	if records == nil {
		return make([]storage.DiagnosticRecord, 0), nil
	}
	return records, nil
}

func (b *Backend) ClearHistory(id string) error {
	if b.diagDB == nil {
		return nil
	}
	return b.diagDB.ClearProjectHistory(id)
}

// -------------------------------------------------------------
// SERVICES & MAIL (A01, A17, B03, B04)
// -------------------------------------------------------------

func (b *Backend) RefreshServices() []services.ServiceInstance {
	if b.serviceMgr == nil {
		return make([]services.ServiceInstance, 0)
	}
	res := b.serviceMgr.RefreshStatuses()
	if res == nil {
		return make([]services.ServiceInstance, 0)
	}
	return res
}

func (b *Backend) StartService(id string, customBinary string) error {
	b.startMu.Lock()
	defer b.startMu.Unlock()

	b.mu.Lock()
	if b.isShuttingDown {
		b.mu.Unlock()
		return errors.New("cannot start service: workbench is shutting down")
	}
	atomic.AddInt32(&b.inFlightOps, 1)
	defer atomic.AddInt32(&b.inFlightOps, -1)
	b.mu.Unlock()
	if b.serviceMgr == nil {
		return errors.New("Service manager unavailable")
	}
	if customBinary == "" {
		customBinary, _ = b.dependencies.Resolve(id)
	}
	return b.serviceMgr.StartService(id, customBinary)
}

func (b *Backend) StopService(id string, force bool) error {
	if b.serviceMgr == nil {
		return errors.New("Service manager unavailable")
	}
	return b.serviceMgr.StopService(id, force)
}

func (b *Backend) GetMailbox(search string, limit, start int) (*mail.MailboxResult, error) {
	if b.mailClient == nil {
		return nil, errors.New("Mailpit client unavailable")
	}
	return b.mailClient.GetMessages(context.Background(), search, limit, start)
}

func (b *Backend) GetMailMessage(id string) (*mail.MailMessageDetail, error) {
	if b.mailClient == nil {
		return nil, errors.New("Mailpit client unavailable")
	}
	return b.mailClient.GetMessage(context.Background(), id)
}

func (b *Backend) DeleteAllMail() error {
	if b.mailClient == nil {
		return errors.New("Mailpit client unavailable")
	}
	return b.mailClient.DeleteAll(context.Background())
}

func (b *Backend) DeleteMailMessage(id string) error {
	if b.mailClient == nil {
		return errors.New("Mailpit client unavailable")
	}
	return b.mailClient.DeleteMessage(context.Background(), id)
}

func (b *Backend) ApplyServicesToEnv(projectID string, opts services.EnvSyncOptions) (*services.EnvSyncResult, error) {
	p, err := b.project(projectID, true)
	if err != nil {
		return nil, err
	}
	envPath := filepath.Join(p.Path, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		examplePath := filepath.Join(p.Path, ".env.example")
		if data, err := os.ReadFile(examplePath); err == nil {
			_ = os.WriteFile(envPath, data, 0644)
		} else {
			return nil, fmt.Errorf(".env file does not exist in %s", p.Path)
		}
	}
	return services.ApplyServicesToEnv(envPath, opts)
}

func (b *Backend) AutoProvisionDatabase(projectID string, engine, dbName string) error {
	p, err := b.project(projectID, false)
	if err != nil {
		return err
	}
	phpBin, _ := b.dependencies.Resolve("php")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return services.AutoProvisionDatabase(ctx, engine, dbName, phpBin, p.Path)
}

// -------------------------------------------------------------
// ACTIONS, EVENTS & QUEUES (A03, B05)
// -------------------------------------------------------------

func (b *Backend) GetEvents(id string) ([]actions.EventListenerItem, error) {
	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	return b.actionsCoord.ListEvents(p.PHPBinary, p.Path)
}

func (b *Backend) GetFailedJobs(id string) ([]actions.FailedJobItem, error) {
	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	return b.actionsCoord.ListFailedJobs(p.PHPBinary, p.Path)
}

func (b *Backend) RetryFailedJob(id, jobID string) (actions.ActionRunResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	return b.actionsCoord.RetryFailedJobContext(opCtx, p.PHPBinary, p.Path, jobID)
}

func (b *Backend) StartQueueWorker(id, queue string) (*actions.QueueWorkerStatus, error) {
	b.startMu.Lock()
	defer b.startMu.Unlock()

	b.mu.Lock()
	if b.isShuttingDown {
		b.mu.Unlock()
		return nil, errors.New("cannot start queue worker: workbench is shutting down")
	}
	atomic.AddInt32(&b.inFlightOps, 1)
	defer atomic.AddInt32(&b.inFlightOps, -1)
	b.mu.Unlock()

	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	return b.actionsCoord.StartQueueWorker(p.PHPBinary, p.Path, queue)
}

func (b *Backend) StopQueueWorker(force bool) error {
	return b.actionsCoord.StopQueueWorker(force)
}

func (b *Backend) GetQueueWorkerStatus() actions.QueueWorkerStatus {
	return b.actionsCoord.GetWorkerStatus()
}

func (b *Backend) RetryAllFailedJobs(id string) (actions.ActionRunResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	return b.actionsCoord.RetryAllFailedJobsContext(opCtx, p.PHPBinary, p.Path)
}

func (b *Backend) ForgetFailedJob(id, jobID string) (actions.ActionRunResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	return b.actionsCoord.ForgetFailedJobContext(opCtx, p.PHPBinary, p.Path, jobID)
}

func (b *Backend) FlushFailedJobs(id string) (actions.ActionRunResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	return b.actionsCoord.FlushFailedJobsContext(opCtx, p.PHPBinary, p.Path)
}

func (b *Backend) ListScheduledTasks(id string) ([]actions.ScheduledTaskItem, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	return b.actionsCoord.ListScheduledTasksContext(opCtx, p.PHPBinary, p.Path)
}

func (b *Backend) RunScheduledTask(id, command string) (actions.ActionRunResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	return b.actionsCoord.RunScheduledTaskContext(opCtx, p.PHPBinary, p.Path, command)
}

// -------------------------------------------------------------
// TINKER REPL
// -------------------------------------------------------------

func (b *Backend) EvaluateTinker(id, snippet string) (tinker.EvaluationResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return tinker.EvaluationResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return tinker.EvaluationResult{}, err
	}
	return b.tinkerSvc.EvaluateContext(opCtx, p.PHPBinary, p.Path, snippet, 30*time.Second)
}

// -------------------------------------------------------------
// ELOQUENT DATABASE EXPLORER
// -------------------------------------------------------------

func (b *Backend) ListModels(id string) ([]database.ModelInfo, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	return b.dbSvc.ListModelsContext(opCtx, p.PHPBinary, p.Path)
}

func (b *Backend) GetModelRecords(id, modelClass string, page, perPage int, search, sortColumn, sortDirection string) (database.RecordQueryResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return database.RecordQueryResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return database.RecordQueryResult{}, err
	}
	return b.dbSvc.GetModelRecordsContext(opCtx, p.PHPBinary, p.Path, modelClass, page, perPage, search, sortColumn, sortDirection)
}

func (b *Backend) UpdateModelRecord(id, modelClass string, recordId any, attributes map[string]any) (database.ModelRecordUpdateResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return database.ModelRecordUpdateResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return database.ModelRecordUpdateResult{}, err
	}
	return b.dbSvc.UpdateModelRecordContext(opCtx, p.PHPBinary, p.Path, modelClass, recordId, attributes)
}

func (b *Backend) DeleteModelRecord(id, modelClass string, recordId any) (bool, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return false, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return false, err
	}
	return b.dbSvc.DeleteModelRecordContext(opCtx, p.PHPBinary, p.Path, modelClass, recordId)
}

func (b *Backend) GetErdSchema(id string) (database.ERDSchemaResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return database.ERDSchemaResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return database.ERDSchemaResult{}, err
	}
	return b.dbSvc.GetErdSchemaContext(opCtx, p.PHPBinary, p.Path)
}

func (b *Backend) ExplainQuery(id, sql string) (database.ExplainResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return database.ExplainResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return database.ExplainResult{}, err
	}
	return b.dbSvc.ExplainQueryContext(opCtx, p.PHPBinary, p.Path, sql)
}

func (b *Backend) GetNPlusOneReport(id, requestID string) (telemetry.NPlusOneReport, error) {
	capture := b.clock.State()
	for _, req := range capture.Requests {
		if req.ID == requestID {
			rawQueries := make([]telemetry.RawQueryItem, len(req.Queries))
			for i, q := range req.Queries {
				rawQueries[i] = telemetry.RawQueryItem{
					Query:      q.SQL,
					Duration:   q.Duration,
					Connection: q.Connection,
				}
			}
			return telemetry.DetectNPlusOne(req.ID, req.URI, rawQueries), nil
		}
	}
	return telemetry.NPlusOneReport{RequestID: requestID, Issues: []telemetry.NPlusOneIssue{}}, nil
}

func (b *Backend) GetWaterfallFlamegraph(id, requestID string) (telemetry.WaterfallFlamegraph, error) {
	capture := b.clock.State()
	for _, req := range capture.Requests {
		if req.ID == requestID {
			rawQueries := make([]telemetry.RawQueryItem, len(req.Queries))
			for i, q := range req.Queries {
				startMs := 0.0
				if q.Time > 0 && req.Time > 0 && q.Time >= req.Time {
					startMs = (q.Time - req.Time) * 1000.0
				}
				rawQueries[i] = telemetry.RawQueryItem{
					Query:      q.SQL,
					Duration:   q.Duration,
					Connection: q.Connection,
					StartMs:    startMs,
				}
			}
			rawEvents := make([]telemetry.RawTimelineEvent, len(req.Timeline))
			for i, ev := range req.Timeline {
				evStartMs := 0.0
				if ev.Start > 0 && req.Time > 0 && ev.Start >= req.Time {
					evStartMs = (ev.Start - req.Time) * 1000.0
				}
				rawEvents[i] = telemetry.RawTimelineEvent{
					Description: ev.Description,
					Start:       evStartMs,
					End:         evStartMs + ev.Duration,
					Duration:    ev.Duration,
				}
			}
			return telemetry.BuildWaterfall(req.ID, req.Duration, rawEvents, rawQueries), nil
		}
	}
	return telemetry.BuildWaterfall(requestID, 0, nil, nil), nil
}

// -------------------------------------------------------------
// REMOTE SSH PROFILES & PRODUCTION SAFETY VAULT
// -------------------------------------------------------------

func (b *Backend) ListRemoteProfiles() []remote.RemoteProfile {
	if b.remoteMgr == nil {
		return []remote.RemoteProfile{}
	}
	return b.remoteMgr.ListProfiles()
}

func (b *Backend) SaveRemoteProfile(p remote.RemoteProfile) (remote.RemoteProfile, error) {
	if b.remoteMgr == nil {
		return remote.RemoteProfile{}, errors.New("remote manager not initialized")
	}
	return b.remoteMgr.SaveProfile(p)
}

func (b *Backend) DeleteRemoteProfile(id string) error {
	if b.remoteMgr == nil {
		return errors.New("remote manager not initialized")
	}
	return b.remoteMgr.DeleteProfile(id)
}

func (b *Backend) TestRemoteConnection(id string) (remote.RemoteTestResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return remote.RemoteTestResult{Reachable: false, Error: err.Error()}, err
	}
	defer done()

	if b.remoteMgr == nil {
		return remote.RemoteTestResult{Reachable: false, Error: "remote manager not initialized"}, errors.New("remote manager not initialized")
	}
	return b.remoteMgr.TestConnection(opCtx, id)
}

func (b *Backend) AssessRemoteCommandSafety(profileID, command, token string) (remote.SafetyAssessment, error) {
	if b.remoteMgr == nil {
		return remote.SafetyAssessment{Allowed: false, Reason: "remote manager not initialized"}, errors.New("remote manager not initialized")
	}
	for _, p := range b.remoteMgr.ListProfiles() {
		if p.ID == profileID {
			return remote.AssessCommandSafety(p, command, token), nil
		}
	}
	return remote.SafetyAssessment{Allowed: false, Reason: "Profile not found"}, errors.New("profile not found")
}

// -------------------------------------------------------------
// REAL-TIME WEBSOCKET & REVERB INSPECTOR
// -------------------------------------------------------------

func (b *Backend) GetReverbConfig(id string) realtime.ReverbConfig {
	p, err := b.project(id, false)
	if err != nil {
		return realtime.ReverbConfig{Host: "127.0.0.1", Port: 8080, Driver: "reverb"}
	}
	envPath := filepath.Join(p.Path, ".env")
	return realtime.DetectConfig(envPath)
}

func (b *Backend) GetWebSocketFrames() []realtime.WebSocketFrame {
	if b.realtimeIns == nil {
		return []realtime.WebSocketFrame{}
	}
	return b.realtimeIns.GetFrames()
}

func (b *Backend) GetWebSocketChannels() []realtime.ChannelInfo {
	if b.realtimeIns == nil {
		return []realtime.ChannelInfo{}
	}
	return b.realtimeIns.GetChannels()
}

func (b *Backend) SimulateBroadcastEvent(spec realtime.EventSimulationSpec) realtime.SimulationResult {
	if b.realtimeIns == nil {
		return realtime.SimulationResult{Success: false, Message: "realtime inspector not initialized"}
	}
	return b.realtimeIns.SimulateEvent(spec)
}

func (b *Backend) ClearWebSocketFrames() {
	if b.realtimeIns != nil {
		b.realtimeIns.ClearFrames()
	}
}



func (b *Backend) GetSavedCommands(id string) []storage.SavedCmd {
	if b.storageMgr == nil {
		return make([]storage.SavedCmd, 0)
	}
	res := b.storageMgr.Get().SavedCommands[id]
	if res == nil {
		return make([]storage.SavedCmd, 0)
	}
	return res
}

func (b *Backend) SaveCommand(id string, cmd storage.SavedCmd) error {
	p, err := b.project(id, false)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cmd.ID) == "" || strings.TrimSpace(cmd.Name) == "" || len(cmd.Args) == 0 {
		return errors.New("command ID, name and arguments are required")
	}
	if cmd.Directory == "" {
		cmd.Directory = p.Path
	}
	rel, err := filepath.Rel(p.Path, cmd.Directory)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return errors.New("saved command directory must be inside its project")
	}

	if b.storageMgr == nil {
		return errors.New("Settings unavailable")
	}
	return b.storageMgr.Update(func(s *storage.Settings) {
		cmds := s.SavedCommands[id]
		replaced := false
		for i, c := range cmds {
			if c.ID == cmd.ID {
				cmds[i] = cmd
				replaced = true
				break
			}
		}
		if !replaced {
			cmds = append(cmds, cmd)
		}
		s.SavedCommands[id] = cmds
	})
}

func (b *Backend) DeleteCommand(id, cmdID string) error {
	if b.storageMgr == nil {
		return errors.New("Settings unavailable")
	}
	return b.storageMgr.Update(func(s *storage.Settings) {
		cmds := s.SavedCommands[id]
		var kept []storage.SavedCmd
		for _, c := range cmds {
			if c.ID != cmdID {
				kept = append(kept, c)
			}
		}
		s.SavedCommands[id] = kept
	})
}

func (b *Backend) ExecuteSavedCommand(id, cmdID string) (actions.ActionRunResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	cmds := b.GetSavedCommands(id)
	var target *storage.SavedCmd
	for _, c := range cmds {
		if c.ID == cmdID {
			target = &c
			break
		}
	}
	if target == nil {
		return actions.ActionRunResult{}, errors.New("Saved command not found")
	}
	execBin := target.Executable
	if execBin == "php" || execBin == "" {
		execBin = p.PHPBinary
	}
	dir := target.Directory
	if dir == "" {
		dir = p.Path
	}
	return b.actionsCoord.ExecuteCommandContext(opCtx, execBin, target.Args, dir, 30*time.Second)
}

// -------------------------------------------------------------
// TESTING & QUALITY (A04, A07, A10, A19, B06, B08, X02)
// -------------------------------------------------------------

func (b *Backend) ExecuteAPIRequest(spec wktesting.APIRequestSpec) (*wktesting.APIResponseResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()
	return b.testCoord.ExecuteAPIRequest(opCtx, spec)
}

func (b *Backend) GenerateBoundaryCases(spec wktesting.APIRequestSpec) []wktesting.APIRequestSpec {
	return b.testCoord.GenerateBoundaryCases(spec)
}

func (b *Backend) RunLoadTest(spec wktesting.LoadTestSpec) (*wktesting.LoadTestResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()
	return b.testCoord.RunLoadTest(opCtx, spec)
}

func (b *Backend) RunProjectTests(id, filter string) (*wktesting.ProjectTestResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	res, err := b.testCoord.RunProjectTestsContext(opCtx, p.PHPBinary, p.Path, filter)
	if err == nil && b.diagDB != nil && res != nil {
		_ = b.diagDB.SaveTestRun(storage.TestRunRecord{
			ProjectID: id,
			Suite:     res.Suite,
			Total:     res.Total,
			Passed:    res.Passed,
			Failed:    res.Failed,
			Skipped:   res.Skipped,
			Duration:  res.Duration,
			Results:   strings.Join(res.Output, "\n"),
		})
	}
	return res, err
}

func (b *Backend) RunPint(id string, testOnly bool) (*quality.QualityReport, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	return quality.RunPintContext(opCtx, p.PHPBinary, p.Path, testOnly)
}

func (b *Backend) RunPHPStan(id string) (*quality.QualityReport, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	return quality.RunPHPStanContext(opCtx, p.PHPBinary, p.Path)
}

func (b *Backend) AuditProject(id string) ([]quality.ConfigAuditItem, error) {
	p, err := b.project(id, false)
	if err != nil {
		return nil, err
	}
	envFile, err := environment.ParseEnv(filepath.Join(p.Path, ".env"))
	if err != nil {
		return nil, err
	}
	envMap := make(map[string]string)
	for _, e := range envFile.Entries {
		if e.Type == "keyvalue" {
			envMap[e.Key] = e.Value
		}
	}
	phpInfo, _ := environment.InspectPHP(p.PHPBinary)
	var phpDirectives map[string]string
	if phpInfo != nil {
		phpDirectives = phpInfo.KeySettings
	}
	return quality.AuditConfiguration(envMap, phpDirectives), nil
}

// -------------------------------------------------------------
// RECIPES & WIZARD (A13, A14, A15, B07)
// -------------------------------------------------------------

func (b *Backend) ListRecipes() []recipes.Recipe {
	if b.recipeCat == nil {
		return make([]recipes.Recipe, 0)
	}
	res := b.recipeCat.ListRecipes()
	if res == nil {
		return make([]recipes.Recipe, 0)
	}
	return res
}

func (b *Backend) PreviewRecipe(recipeID string) (*recipes.RecipePreview, error) {
	return b.recipeCat.PreviewRecipe(recipeID)
}

func (b *Backend) ExecuteRecipe(id, recipeID string) (*recipes.RecipeExecutionResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()

	p, err := b.project(id, true)
	if err != nil {
		return nil, err
	}
	return b.recipeCat.ExecuteRecipeContext(opCtx, recipeID, p.PHPBinary, p.Path)
}

func (b *Backend) ExecuteProjectWizard(spec recipes.ProjectWizardSpec) (string, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return "", err
	}
	defer done()

	return recipes.ExecuteProjectWizardContext(opCtx, spec)
}

func (b *Backend) CreateProjectFromWizard(spec wizard.WizardSpec) (*wizard.WizardResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()

	if spec.PHPBinary == "" {
		spec.PHPBinary, _ = b.dependencies.Resolve("php")
	}
	return wizard.CreateProject(opCtx, spec)
}

// -------------------------------------------------------------
// REPORTS, ARCHITECTURE & SHARING (A08, A11, A18, A20, X03, X04)
// -------------------------------------------------------------

func (b *Backend) GetProjectSnapshot(id string) (reports.ProjectSnapshot, error) {
	p, err := b.project(id, false)
	if err != nil {
		return reports.ProjectSnapshot{}, err
	}

	envFile, _ := environment.ParseEnv(filepath.Join(p.Path, ".env"))
	envMap := make(map[string]string)
	var envKeys []string
	if envFile != nil {
		for _, e := range envFile.Entries {
			if e.Type == "keyvalue" {
				envMap[e.Key] = e.Value
				envKeys = append(envKeys, e.Key)
			}
		}
	}

	dbDriver := envMap["DB_CONNECTION"]
	cacheDriver := envMap["CACHE_STORE"]
	if cacheDriver == "" {
		cacheDriver = envMap["CACHE_DRIVER"]
	}
	queueDriver := envMap["QUEUE_CONNECTION"]

	hasMailpit := false
	if b.serviceMgr != nil {
		inst, err := b.serviceMgr.GetInstance("mailpit")
		if err == nil && (inst.Status == services.StatusRunning || inst.Status == services.StatusExternal) {
			hasMailpit = true
		}
	}

	routes, _ := b.Routes(id)
	var routeSample []string
	for idx, r := range routes {
		if idx >= 10 {
			break
		}
		routeSample = append(routeSample, fmt.Sprintf("%s %s", r.Method, r.URI))
	}

	auditItems, _ := b.AuditProject(id)
	good := 0
	warn := 0
	info := 0
	for _, it := range auditItems {
		switch it.Status {
		case "good":
			good++
		case "warning":
			warn++
		case "info":
			info++
		}
	}

	graph := reports.BuildArchitectureGraph(p.Name, dbDriver, cacheDriver, queueDriver, hasMailpit, nil)
	hostAdvice := reports.GenerateHostingAdvice(dbDriver, cacheDriver, queueDriver, len(routes), 0)

	return reports.ProjectSnapshot{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		ProjectName:    p.Name,
		ProjectPath:    p.Path,
		ProjectURL:     p.URL,
		PHPVersion:     p.PHP,
		Framework:      p.Framework,
		Clockwork:      p.Clockwork,
		RoutesCount:    len(routes),
		Routes:         routeSample,
		DatabaseDriver: dbDriver,
		CacheDriver:    cacheDriver,
		QueueDriver:    queueDriver,
		EnvKeys:        envKeys,
		AuditGood:      good,
		AuditWarn:      warn,
		AuditInfo:      info,
		HostAdvice:     hostAdvice,
		Graph:          graph,
	}, nil
}

func (b *Backend) GenerateREADME(id string) (string, error) {
	snap, err := b.GetProjectSnapshot(id)
	if err != nil {
		return "", err
	}
	p, err := b.project(id, false)
	if err != nil {
		return "", err
	}
	if err := b.rememberReadme(id, p.Path); err != nil {
		return "", err
	}
	return reports.GenerateREADME(snap), nil
}

func (b *Backend) MergeREADME(id, generated string, selectedSections []string) (string, error) {
	p, err := b.project(id, false)
	if err != nil {
		return "", err
	}
	readmePath := filepath.Join(p.Path, "README.md")
	existingBytes, readErr := os.ReadFile(readmePath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return "", readErr
	}
	if err := b.rememberReadme(id, p.Path); err != nil {
		return "", err
	}
	return reports.MergeREADME(string(existingBytes), generated, selectedSections), nil
}

func (b *Backend) rememberReadme(id, projectPath string) error {
	data, err := os.ReadFile(filepath.Join(projectPath, "README.md"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	sum := sha256.Sum256(data)
	b.mu.Lock()
	b.readmeBaselines[id] = hex.EncodeToString(sum[:])
	b.mu.Unlock()
	return nil
}
func (b *Backend) SaveREADME(id, content string) error {
	p, err := b.project(id, true)
	if err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	expected, ok := b.readmeBaselines[id]
	if !ok {
		return errors.New("preview or merge README before saving")
	}
	path := filepath.Join(p.Path, "README.md")
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("README symlink cannot be overwritten")
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != expected {
		return errors.New("README changed externally; regenerate or merge before saving")
	}
	if err == nil {
		backup := fmt.Sprintf("%s.backup.%d", path, time.Now().UnixNano())
		if err := os.WriteFile(backup, data, 0600); err != nil {
			return fmt.Errorf("README backup failed: %w", err)
		}
	}
	tmp := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	defer os.Remove(tmp)
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return err
	}
	if err := environment.ReplaceFileAtomic(tmp, path); err != nil {
		return err
	}
	hash = sha256.Sum256([]byte(content))
	b.readmeBaselines[id] = hex.EncodeToString(hash[:])
	return nil
}

func (b *Backend) ExportTeamManifest(id, outPath string) error {
	snap, err := b.GetProjectSnapshot(id)
	if err != nil {
		return err
	}
	return reports.ExportTeamManifest(snap, outPath)
}

func (b *Backend) ExportSupportBundle(id, outZipPath string) error {
	snap, err := b.GetProjectSnapshot(id)
	if err != nil {
		return err
	}
	var logLines []string
	rawLogs, _ := b.GetRawLogs(id)
	for _, l := range rawLogs {
		logLines = append(logLines, fmt.Sprintf("[%s] %s.%s: %s", l.Timestamp, l.Environment, l.Level, Redact(l.Message)))
	}
	return reports.ExportOfflineDiagnosticBundle(snap, logLines, outZipPath)
}

func (b *Backend) StartTunnel(provider, target string) (*sharing.TunnelStatus, error) {
	b.startMu.Lock()
	defer b.startMu.Unlock()

	b.mu.Lock()
	if b.isShuttingDown {
		b.mu.Unlock()
		return nil, errors.New("cannot start tunnel: workbench is shutting down")
	}
	atomic.AddInt32(&b.inFlightOps, 1)
	defer atomic.AddInt32(&b.inFlightOps, -1)
	b.mu.Unlock()

	if b.tunnelMgr == nil {
		return nil, errors.New("Tunnel manager unavailable")
	}
	phpBinary := ""
	if strings.EqualFold(provider, "expose") {
		b.mu.Lock()
		selected := b.selected
		b.mu.Unlock()
		if p, err := b.project(selected, false); err == nil {
			phpBinary = p.PHPBinary
		} else if herd, err := FindHerd(); err == nil {
			phpBinary = herd.PHP
		}
	}
	return b.tunnelMgr.StartTunnelWithPHP(provider, target, phpBinary)
}

func (b *Backend) StopTunnel() error {
	if b.tunnelMgr == nil {
		return nil
	}
	return b.tunnelMgr.StopTunnel(false)
}

func (b *Backend) GetTunnelStatus() sharing.TunnelStatus {
	if b.tunnelMgr == nil {
		return sharing.TunnelStatus{Running: false}
	}
	return b.tunnelMgr.GetStatus()
}

func (b *Backend) DispatchWebhook(scenario sharing.WebhookScenario) (*sharing.WebhookDispatchResult, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return nil, err
	}
	defer done()
	return sharing.DispatchWebhook(opCtx, scenario)
}

func (b *Backend) DetectIntegrations(id string) ([]sharing.IntegrationItem, error) {
	p, err := b.project(id, false)
	if err != nil {
		return make([]sharing.IntegrationItem, 0), nil
	}
	envFile, err := environment.ParseEnv(filepath.Join(p.Path, ".env"))
	if err != nil {
		return make([]sharing.IntegrationItem, 0), nil
	}
	envMap := make(map[string]string)
	for _, e := range envFile.Entries {
		if e.Type == "keyvalue" {
			envMap[e.Key] = e.Value
		}
	}
	res := sharing.DetectIntegrations(envMap)
	if res == nil {
		return make([]sharing.IntegrationItem, 0), nil
	}
	return res, nil
}

func (b *Backend) UpdateSettings(s storage.Settings) error {
	if b.storageMgr == nil {
		return errors.New("Settings unavailable")
	}
	return b.storageMgr.Update(func(cur *storage.Settings) {
		*cur = s
	})
}

func (b *Backend) LaunchProjectTerminal(id, shellType string) error {
	p, err := b.project(id, false)
	if err != nil {
		return err
	}
	if p.Path == "" {
		return errors.New("Project path is empty")
	}
	phpDir := ""
	if p.PHPBinary != "" {
		phpDir = filepath.Dir(p.PHPBinary)
	}
	return launchTerminal(p.Path, phpDir, shellType)
}

// -------------------------------------------------------------
// LOG PAIL, HEALTH HEARTBEAT, FEATURE FLAGS & ARCHITECTURE CANVAS
// -------------------------------------------------------------

func (b *Backend) GetLogPailReport(id string, max int) (storage.LogPailReport, error) {
	p, err := b.project(id, false)
	if err != nil {
		return storage.LogPailReport{}, err
	}
	logFile := filepath.Join(p.Path, "storage", "logs", "laravel.log")
	if max <= 0 {
		max = 50
	}
	return storage.ParseLaravelLog(logFile, max)
}

func (b *Backend) GetHealthHeartbeat(id string) (health.HeartbeatReport, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return health.HeartbeatReport{}, err
	}
	defer done()

	p, err := b.project(id, false)
	if err != nil {
		return health.HeartbeatReport{}, err
	}
	return health.ProbeHeartbeat(opCtx, p.PHPBinary, p.Path), nil
}

func (b *Backend) ListFeatureFlags(id string) (flags.FlagsReport, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return flags.FlagsReport{}, err
	}
	defer done()

	p, err := b.project(id, false)
	if err != nil {
		return flags.FlagsReport{}, err
	}
	return flags.ScanFlags(opCtx, p.Path), nil
}

func (b *Backend) GetArchitectureCanvas(id string) (architecture.ArchitectureCanvasReport, error) {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return architecture.ArchitectureCanvasReport{}, err
	}
	defer done()

	p, err := b.project(id, false)
	if err != nil {
		return architecture.ArchitectureCanvasReport{}, err
	}
	return architecture.BuildArchitectureCanvas(opCtx, p.ID, p.Path), nil
}

// -------------------------------------------------------------
// PHP RUNTIME & COMPOSER MANAGEMENT
// -------------------------------------------------------------

func (b *Backend) ListRuntimes() []runtimes.RuntimeInfo {
	standalone := b.runtimeMgr.ListStandaloneRuntimes()
	herd := runtimes.DiscoverHerdRuntimes()

	merged := make([]runtimes.RuntimeInfo, 0, len(standalone)+len(herd))
	merged = append(merged, standalone...)
	merged = append(merged, herd...)
	return merged
}

func (b *Backend) InstallPHP(version string) error {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return err
	}
	defer done()

	return b.runtimeMgr.InstallPHP(opCtx, version, func(pct int, msg string) {
		b.emit()
	})
}

func (b *Backend) ToggleExtension(iniPath, extensionName string, enable bool) error {
	return b.runtimeMgr.ToggleExtension(iniPath, extensionName, enable)
}

func (b *Backend) GetComposerStatus() runtimes.ComposerStatus {
	return b.runtimeMgr.GetComposerStatus()
}

func (b *Backend) InstallComposer() error {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return err
	}
	defer done()

	return b.runtimeMgr.InstallComposer(opCtx, func(pct int, msg string) {
		b.emit()
	})
}

func (b *Backend) AddBinToUserPath() error {
	return b.runtimeMgr.AddBinToUserPath()
}

func (b *Backend) IsolateHerdSite(projectID, version string) error {
	opCtx, done, err := b.beginOperation()
	if err != nil {
		return err
	}
	defer done()

	p, err := b.project(projectID, false)
	if err != nil {
		return err
	}
	return runtimes.IsolateHerdSite(opCtx, p.Path, version)
}

// -------------------------------------------------------------
// IN-APP DUMPS & XDEBUG (Phase 5)
// -------------------------------------------------------------

func (b *Backend) GetInAppDumps(projectID string) []telemetry.DumpPayload {
	if b.dumpServer == nil {
		return make([]telemetry.DumpPayload, 0)
	}
	return b.dumpServer.GetRecentDumps(projectID)
}

func (b *Backend) ClearInAppDumps(projectID string) {
	if b.dumpServer != nil {
		b.dumpServer.Clear(projectID)
	}
	if b.diagDB != nil {
		_ = b.diagDB.ClearProjectHistory(projectID)
	}
	b.emit()
}

func (b *Backend) GetDumpHelperPHP() string {
	if b.dumpServer == nil {
		return ""
	}
	return b.dumpServer.GetDumpHelperPHP()
}

func (b *Backend) GetXdebugStatus(projectID string) (*telemetry.XdebugStatus, error) {
	p, err := b.project(projectID, false)
	if err != nil {
		return telemetry.CheckXdebugStatus(context.Background(), "", "")
	}
	targetURL, _ := b.ProjectURL(projectID)
	return telemetry.CheckXdebugStatus(context.Background(), p.PHPBinary, targetURL)
}

func (b *Backend) GenerateVSCodeConfig(projectID string) error {
	p, err := b.project(projectID, true)
	if err != nil {
		return err
	}
	return telemetry.GenerateVSCodeConfig(p.Path)
}

// -------------------------------------------------------------
// ACTIVITY TRACING (Phase 6)
// -------------------------------------------------------------

func (b *Backend) GetActivityTrace(projectID, requestID string) (*activity.ActivityTrace, error) {
	if b.clock == nil {
		return nil, errors.New("telemetry clockwork unavailable")
	}

	state := b.clock.State()
	var targetReq *Request
	for _, r := range state.Requests {
		if r.ID == requestID {
			reqCopy := r
			targetReq = &reqCopy
			break
		}
	}

	if targetReq == nil {
		return nil, fmt.Errorf("request %s not found in active session", requestID)
	}

	mem := 0.0
	if targetReq.Memory != nil {
		mem = *targetReq.Memory / (1024 * 1024)
	}

	rawQueries := make([]activity.RawQuery, 0, len(targetReq.Queries))
	baseTime := targetReq.Time
	for _, q := range targetReq.Queries {
		startOffset := 0.0
		if q.Time > 0 && baseTime > 0 && q.Time >= baseTime {
			startOffset = (q.Time - baseTime) * 1000.0
		}
		rawQueries = append(rawQueries, activity.RawQuery{
			Query:    q.SQL,
			Duration: q.Duration,
			StartMs:  startOffset,
		})
	}

	input := activity.RawActivityInput{
		Method:     targetReq.Method,
		URI:        targetReq.URI,
		Status:     targetReq.Status,
		Duration:   targetReq.Duration,
		Memory:     mem,
		Controller: "",
		Middleware: []string{"web"},
		Queries:    rawQueries,
	}

	trace := activity.BuildActivityTrace(targetReq.ID, projectID, input)
	return &trace, nil
}

func (b *Backend) ExportTraceHTML(projectID, requestID string) (string, error) {
	trace, err := b.GetActivityTrace(projectID, requestID)
	if err != nil {
		return "", err
	}
	return activity.ExportTraceHTML(*trace), nil
}

// -------------------------------------------------------------
// EVENT DISPATCH & QUEUE SUPERVISOR (Phase 6)
// -------------------------------------------------------------

func (b *Backend) DispatchEvent(projectID, eventClass, payloadJSON string) (actions.ActionRunResult, error) {
	p, err := b.project(projectID, true)
	if err != nil {
		return actions.ActionRunResult{}, err
	}
	return b.actionsCoord.DispatchEvent(context.Background(), p.PHPBinary, p.Path, eventClass, payloadJSON)
}

// -------------------------------------------------------------
// WEBHOOK WORKBENCH (Phase 6)
// -------------------------------------------------------------

func (b *Backend) GetWebhookDeliveries(endpointID string) []webhooks.WebhookDelivery {
	if b.webhookMgr == nil {
		return make([]webhooks.WebhookDelivery, 0)
	}
	return b.webhookMgr.GetDeliveries(endpointID)
}

func (b *Backend) ClearWebhookDeliveries(endpointID string) {
	if b.webhookMgr != nil {
		b.webhookMgr.Clear(endpointID)
	}
	b.emit()
}

func (b *Backend) ReplayWebhook(deliveryID, targetURL string) (*webhooks.WebhookReplayResult, error) {
	if b.webhookMgr == nil {
		return nil, errors.New("webhook manager unavailable")
	}
	return b.webhookMgr.Replay(context.Background(), deliveryID, targetURL)
}

func (b *Backend) DispatchSignedWebhook(targetURL, provider, secret, payload string) (*webhooks.WebhookReplayResult, error) {
	return webhooks.DispatchSignedTestEvent(context.Background(), targetURL, provider, secret, payload)
}

// -------------------------------------------------------------
// PERFORMANCE & LOAD TESTING (Phase 6)
// -------------------------------------------------------------

func (b *Backend) RunProjectCheckup(projectID string) (*performance.CheckupReport, error) {
	p, err := b.project(projectID, false)
	if err != nil {
		return nil, err
	}
	rep := performance.RunProjectCheckup(context.Background(), p.Path, p.PHPBinary)
	return &rep, nil
}

func (b *Backend) RunNativeLoadTest(spec performance.LoadTestSpec) (*performance.LoadTestResult, error) {
	return performance.RunLoadTest(context.Background(), spec)
}

// -------------------------------------------------------------
// REVERSE-ENGINEERED LARAMONSTER PARITY FEATURES
// -------------------------------------------------------------

func (b *Backend) ToggleDebuggerAgent(projectID string, enable bool) error {
	p, err := b.project(projectID, false)
	if err != nil {
		return err
	}
	if enable {
		return telemetry.AttachDebugger(p.Path)
	}
	return telemetry.DetachDebugger(p.Path)
}

func (b *Backend) IsDebuggerAgentActive(projectID string) bool {
	p, err := b.project(projectID, false)
	if err != nil {
		return false
	}
	return telemetry.IsDebuggerAttached(p.Path)
}

func (b *Backend) GetTelemetryFrames(projectID string) []telemetry.RequestTelemetryFrame {
	if b.dumpServer == nil {
		return make([]telemetry.RequestTelemetryFrame, 0)
	}
	return b.dumpServer.GetRecentFrames(projectID)
}

func (b *Backend) ClearTelemetryFrames(projectID string) {
	if b.dumpServer != nil {
		b.dumpServer.ClearFrames(projectID)
	}
	b.emit()
}

func (b *Backend) SetDumpsInHtml(enabled bool) {
	if b.dumpServer != nil {
		b.dumpServer.SetDumpsInHtml(enabled)
	}
}

func (b *Backend) GetDumpsInHtml() bool {
	if b.dumpServer == nil {
		return false
	}
	return b.dumpServer.GetDumpsInHtml()
}

func (b *Backend) IntrospectEvent(projectID, eventClass string) (*actions.EventIntrospection, error) {
	p, err := b.project(projectID, false)
	if err != nil {
		return nil, err
	}
	return b.actionsCoord.IntrospectEvent(context.Background(), p.PHPBinary, p.Path, eventClass)
}

func (b *Backend) IntrospectJob(projectID, jobClass string) (*actions.JobIntrospection, error) {
	p, err := b.project(projectID, false)
	if err != nil {
		return nil, err
	}
	return b.actionsCoord.IntrospectJob(context.Background(), p.PHPBinary, p.Path, jobClass)
}

func (b *Backend) RunJobNow(projectID, jobClass, payloadJSON string) (*actions.JobRunResult, error) {
	p, err := b.project(projectID, false)
	if err != nil {
		return nil, err
	}
	return b.actionsCoord.RunJobNow(context.Background(), p.PHPBinary, p.Path, jobClass, payloadJSON)
}

func (b *Backend) InspectWebhookRoutes(projectID string) ([]webhooks.WebhookRouteInspection, error) {
	p, err := b.project(projectID, false)
	if err != nil {
		return nil, err
	}
	routes, err := b.Routes(projectID)
	if err != nil {
		return nil, err
	}

	var adapted []struct{ Method, URI, Name, Action string }
	for _, r := range routes {
		adapted = append(adapted, struct{ Method, URI, Name, Action string }{
			Method: r.Method,
			URI:    r.URI,
			Name:   r.Name,
			Action: r.Action,
		})
	}

	return webhooks.InspectWebhookRoutes(p.Path, adapted), nil
}

func (b *Backend) ExplainStatusCode(code int) string {
	return webhooks.ExplainStatusCode(code)
}

func (b *Backend) ValidateWebhookSignature(provider, secret, payload, headerSig string) string {
	return webhooks.ValidateDeliverySignature(provider, secret, payload, headerSig)
}

func (b *Backend) ExecuteCheckupFix(projectID, fixID string) error {
	p, err := b.project(projectID, false)
	if err != nil {
		return err
	}
	return performance.ExecuteCheckupFix(context.Background(), p.Path, p.PHPBinary, fixID)
}

func (b *Backend) GenerateAuthCredentials(projectID, identifier string) (*performance.AuthCredentials, error) {
	p, err := b.project(projectID, false)
	if err != nil {
		return nil, err
	}
	return performance.GenerateAuthCredentials(context.Background(), p.Path, p.PHPBinary, identifier)
}

func (b *Backend) GetLiveIniDirective(projectID, directive string) (string, error) {
	p, err := b.project(projectID, false)
	if err != nil {
		return "", err
	}
	mgr := runtimes.NewManager("")
	return mgr.GetLiveIniDirective(context.Background(), p.PHPBinary, directive)
}

func (b *Backend) SetSiteIniDirective(projectID, directive, value string) error {
	p, err := b.project(projectID, false)
	if err != nil {
		return err
	}
	return runtimes.SetSiteIniDirective(p.Path, directive, value)
}




