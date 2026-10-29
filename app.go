package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"herdworkbench/internal/actions"
	"herdworkbench/internal/activity"
	"herdworkbench/internal/apphost"
	"herdworkbench/internal/architecture"
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
	"herdworkbench/internal/runtimes"
	"herdworkbench/internal/services"
	"herdworkbench/internal/sharing"
	"herdworkbench/internal/storage"
	"herdworkbench/internal/telemetry"
	wktesting "herdworkbench/internal/testing"
	"herdworkbench/internal/tinker"
	"herdworkbench/internal/webhooks"
	"herdworkbench/internal/wizard"
	"herdworkbench/internal/workbench"
)

// App struct
type App struct {
	ctx          context.Context
	backend      *workbench.Backend
	assets       fs.FS
	host         *apphost.Host
	mu           sync.Mutex
	closePending bool
	stopping     bool
	allowClose   bool
}

// NewApp creates a new App application struct
func NewApp(assets fs.FS) *App {
	a := &App{assets: assets}
	a.backend = workbench.NewBackend(func() {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "workbench:update")
		}
		if a.host != nil {
			a.host.Emit("workbench:update")
		}
	})
	return a
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	h, err := apphost.Start(a, nil, a.assets)
	if err == nil {
		a.host = h
	}
	go a.backend.Discover()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		wasActive := false
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				active := a.backend.HasActiveRuns()
				if active || wasActive {
					runtime.EventsEmit(ctx, "workbench:update")
					if a.host != nil {
						a.host.Emit("workbench:update")
					}
				}
				wasActive = active
			}
		}
	}()
}

func (a *App) GetState() workbench.State                   { return a.backend.Snapshot() }
func (a *App) DiscoverProjects() error                     { return a.backend.Discover() }
func (a *App) SelectProject(id string) error               { return a.backend.Select(id) }
func (a *App) TrustProject(id string) error                { return a.backend.Trust(id) }
func (a *App) SetSecureSite(id string, secure bool) error  { return a.backend.SetSecure(id, secure) }
func (a *App) RefreshCaptures() (workbench.Capture, error) { return a.backend.Refresh() }
func (a *App) SetWatching(enable bool) error               { return a.backend.Watch(enable) }
func (a *App) ProbeCapabilities(id string) ([]workbench.Capability, error) {
	return a.backend.Capabilities(id)
}
func (a *App) ListRoutes(id string) ([]workbench.Route, error) { return a.backend.Routes(id) }
func (a *App) StartProcess(id, kind string) (workbench.Run, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopping || a.allowClose {
		return workbench.Run{}, errors.New("Shutdown is in progress")
	}
	return a.backend.Start(id, kind)
}
func (a *App) StopProcess(id string, force bool) error { return a.backend.Stop(id, force) }
func (a *App) OpenProject(id string) error {
	target, err := a.backend.ProjectURL(id)
	if err == nil {
		runtime.BrowserOpenURL(a.ctx, target)
	}
	return err
}

// Environment & INI (A12, B02, X01)
func (a *App) GetEnv(id string) (*environment.EnvFile, error) { return a.backend.GetEnv(id) }
func (a *App) SaveEnv(id string, env *environment.EnvFile, expectedFingerprint string) error {
	return a.backend.SaveEnv(id, env, expectedFingerprint)
}
func (a *App) GetEnvDrift(id string) (environment.DriftReport, error) {
	return a.backend.GetEnvDrift(id)
}
func (a *App) GetPHPInfo(id string) (*environment.PHPInfo, error) { return a.backend.GetPHPInfo(id) }
func (a *App) PreviewIniChange(id, key, val string) (*environment.IniChangePreview, error) {
	return a.backend.PreviewIniChange(id, key, val)
}
func (a *App) ApplyIniChange(id, key, val string) error {
	return a.backend.ApplyIniChange(id, key, val)
}

// Logs, Errors & Dumps (A16, A21)
func (a *App) GetRawLogs(id string) ([]workbench.LogEntry, error) {
	return a.backend.GetRawLogs(id)
}
func (a *App) GetErrors(id, search string, limit int) ([]storage.ErrorLogRecord, error) {
	return a.backend.GetErrors(id, search, limit)
}
func (a *App) GetDumps(id string, limit int) ([]storage.DumpRecord, error) {
	return a.backend.GetDumps(id, limit)
}
func (a *App) SaveDump(id, file string, line int, payload string) error {
	return a.backend.SaveDump(id, file, line, payload)
}
func (a *App) GetHistory(id, search string, limit, offset int) ([]storage.DiagnosticRecord, error) {
	return a.backend.GetHistory(id, search, limit, offset)
}
func (a *App) ClearHistory(id string) error { return a.backend.ClearHistory(id) }

// Services & Mail (A01, A17, B03, B04)
func (a *App) RefreshServices() []services.ServiceInstance { return a.backend.RefreshServices() }
func (a *App) StartService(id string, customBinary string) error {
	return a.backend.StartService(id, customBinary)
}
func (a *App) StopService(id string, force bool) error { return a.backend.StopService(id, force) }
func (a *App) GetMailbox(search string, limit, start int) (*mail.MailboxResult, error) {
	return a.backend.GetMailbox(search, limit, start)
}
func (a *App) GetMailMessage(id string) (*mail.MailMessageDetail, error) {
	return a.backend.GetMailMessage(id)
}
func (a *App) DeleteAllMail() error              { return a.backend.DeleteAllMail() }
func (a *App) DeleteMailMessage(id string) error { return a.backend.DeleteMailMessage(id) }
func (a *App) ApplyServicesToEnv(projectID string, opts services.EnvSyncOptions) (*services.EnvSyncResult, error) {
	return a.backend.ApplyServicesToEnv(projectID, opts)
}
func (a *App) AutoProvisionDatabase(projectID string, engine, dbName string) error {
	return a.backend.AutoProvisionDatabase(projectID, engine, dbName)
}

// Actions, Queues & Commands (A03, B05)
func (a *App) GetEvents(id string) ([]actions.EventListenerItem, error) {
	return a.backend.GetEvents(id)
}
func (a *App) GetFailedJobs(id string) ([]actions.FailedJobItem, error) {
	return a.backend.GetFailedJobs(id)
}
func (a *App) RetryFailedJob(id, jobID string) (actions.ActionRunResult, error) {
	return a.backend.RetryFailedJob(id, jobID)
}
func (a *App) RetryAllFailedJobs(id string) (actions.ActionRunResult, error) {
	return a.backend.RetryAllFailedJobs(id)
}
func (a *App) ForgetFailedJob(id, jobID string) (actions.ActionRunResult, error) {
	return a.backend.ForgetFailedJob(id, jobID)
}
func (a *App) FlushFailedJobs(id string) (actions.ActionRunResult, error) {
	return a.backend.FlushFailedJobs(id)
}
func (a *App) ListScheduledTasks(id string) ([]actions.ScheduledTaskItem, error) {
	return a.backend.ListScheduledTasks(id)
}
func (a *App) RunScheduledTask(id, command string) (actions.ActionRunResult, error) {
	return a.backend.RunScheduledTask(id, command)
}
func (a *App) EvaluateTinker(id, snippet string) (tinker.EvaluationResult, error) {
	return a.backend.EvaluateTinker(id, snippet)
}
func (a *App) ListModels(id string) ([]database.ModelInfo, error) {
	return a.backend.ListModels(id)
}
func (a *App) GetModelRecords(id, modelClass string, page, perPage int, search, sortColumn, sortDirection string) (database.RecordQueryResult, error) {
	return a.backend.GetModelRecords(id, modelClass, page, perPage, search, sortColumn, sortDirection)
}
func (a *App) UpdateModelRecord(id, modelClass string, recordId any, attributes map[string]any) (database.ModelRecordUpdateResult, error) {
	return a.backend.UpdateModelRecord(id, modelClass, recordId, attributes)
}
func (a *App) DeleteModelRecord(id, modelClass string, recordId any) (bool, error) {
	return a.backend.DeleteModelRecord(id, modelClass, recordId)
}
func (a *App) GetErdSchema(id string) (database.ERDSchemaResult, error) {
	return a.backend.GetErdSchema(id)
}
func (a *App) ExplainQuery(id, sql string) (database.ExplainResult, error) {
	return a.backend.ExplainQuery(id, sql)
}
func (a *App) GetNPlusOneReport(id, requestID string) (telemetry.NPlusOneReport, error) {
	return a.backend.GetNPlusOneReport(id, requestID)
}
func (a *App) GetWaterfallFlamegraph(id, requestID string) (telemetry.WaterfallFlamegraph, error) {
	return a.backend.GetWaterfallFlamegraph(id, requestID)
}

// Remote Profiles & Safety Vault
func (a *App) ListRemoteProfiles() []remote.RemoteProfile {
	return a.backend.ListRemoteProfiles()
}
func (a *App) SaveRemoteProfile(p remote.RemoteProfile) (remote.RemoteProfile, error) {
	return a.backend.SaveRemoteProfile(p)
}
func (a *App) DeleteRemoteProfile(id string) error {
	return a.backend.DeleteRemoteProfile(id)
}
func (a *App) TestRemoteConnection(id string) (remote.RemoteTestResult, error) {
	return a.backend.TestRemoteConnection(id)
}
func (a *App) AssessRemoteCommandSafety(profileID, command, token string) (remote.SafetyAssessment, error) {
	return a.backend.AssessRemoteCommandSafety(profileID, command, token)
}

// Real-Time & Reverb Inspector
func (a *App) GetReverbConfig(id string) realtime.ReverbConfig {
	return a.backend.GetReverbConfig(id)
}
func (a *App) GetWebSocketFrames() []realtime.WebSocketFrame {
	return a.backend.GetWebSocketFrames()
}
func (a *App) GetWebSocketChannels() []realtime.ChannelInfo {
	return a.backend.GetWebSocketChannels()
}
func (a *App) SimulateBroadcastEvent(spec realtime.EventSimulationSpec) realtime.SimulationResult {
	return a.backend.SimulateBroadcastEvent(spec)
}
func (a *App) ClearWebSocketFrames() {
	a.backend.ClearWebSocketFrames()
}

// Log Pail, Health Heartbeat, Feature Flags & Architecture Canvas
func (a *App) GetLogPailReport(id string, max int) (storage.LogPailReport, error) {
	return a.backend.GetLogPailReport(id, max)
}
func (a *App) GetHealthHeartbeat(id string) (health.HeartbeatReport, error) {
	return a.backend.GetHealthHeartbeat(id)
}
func (a *App) ListFeatureFlags(id string) (flags.FlagsReport, error) {
	return a.backend.ListFeatureFlags(id)
}
func (a *App) GetArchitectureCanvas(id string) (architecture.ArchitectureCanvasReport, error) {
	return a.backend.GetArchitectureCanvas(id)
}

func (a *App) StartQueueWorker(id, queue string) (*actions.QueueWorkerStatus, error) {
	return a.backend.StartQueueWorker(id, queue)
}
func (a *App) StopQueueWorker(force bool) error { return a.backend.StopQueueWorker(force) }
func (a *App) GetQueueWorkerStatus() actions.QueueWorkerStatus {
	return a.backend.GetQueueWorkerStatus()
}
func (a *App) GetSavedCommands(id string) []storage.SavedCmd {
	return a.backend.GetSavedCommands(id)
}
func (a *App) SaveCommand(id string, cmd storage.SavedCmd) error {
	return a.backend.SaveCommand(id, cmd)
}
func (a *App) DeleteCommand(id, cmdID string) error { return a.backend.DeleteCommand(id, cmdID) }
func (a *App) ExecuteSavedCommand(id, cmdID string) (actions.ActionRunResult, error) {
	return a.backend.ExecuteSavedCommand(id, cmdID)
}

// Testing & Quality (A04, A07, A10, A19, B06, B08, X02)
func (a *App) ExecuteAPIRequest(spec wktesting.APIRequestSpec) (*wktesting.APIResponseResult, error) {
	return a.backend.ExecuteAPIRequest(spec)
}
func (a *App) GenerateBoundaryCases(spec wktesting.APIRequestSpec) []wktesting.APIRequestSpec {
	return a.backend.GenerateBoundaryCases(spec)
}
func (a *App) RunLoadTest(spec wktesting.LoadTestSpec) (*wktesting.LoadTestResult, error) {
	return a.backend.RunLoadTest(spec)
}
func (a *App) RunProjectTests(id, filter string) (*wktesting.ProjectTestResult, error) {
	return a.backend.RunProjectTests(id, filter)
}
func (a *App) RunPint(id string, testOnly bool) (*quality.QualityReport, error) {
	return a.backend.RunPint(id, testOnly)
}
func (a *App) RunPHPStan(id string) (*quality.QualityReport, error) {
	return a.backend.RunPHPStan(id)
}
func (a *App) AuditProject(id string) ([]quality.ConfigAuditItem, error) {
	return a.backend.AuditProject(id)
}
func (a *App) AuditFullStackBestPractices(id string, options quality.AuditOptions) (quality.AuditReport, error) {
	return a.backend.AuditFullStackBestPractices(id, options)
}
func (a *App) CancelAudit(id string)               { a.backend.CancelAudit(id) }
func (a *App) GetAuditRules() []quality.RulePolicy { return quality.AuditRules() }
func (a *App) PreviewAuditSetup(id, kind, manager string) (quality.AuditSetupPlan, error) {
	return a.backend.PreviewAuditSetup(id, kind, manager)
}
func (a *App) ApplyAuditSetup(id string) error { return a.backend.ApplyAuditSetup(id) }
func (a *App) OpenAuditBrowser(id string) (quality.BrowserSessionStatus, error) {
	return a.backend.OpenAuditBrowser(id)
}
func (a *App) CloseAuditBrowser(id string) { a.backend.CloseAuditBrowser(id) }
func (a *App) AuditBrowserPage(id, address string) (quality.AuditReport, error) {
	return a.backend.AuditBrowserPage(id, address)
}

// Recipes & Wizard (A13, A14, A15, B07)
func (a *App) ListRecipes() []recipes.Recipe { return a.backend.ListRecipes() }
func (a *App) PreviewRecipe(recipeID string) (*recipes.RecipePreview, error) {
	return a.backend.PreviewRecipe(recipeID)
}
func (a *App) ExecuteRecipe(id, recipeID string) (*recipes.RecipeExecutionResult, error) {
	return a.backend.ExecuteRecipe(id, recipeID)
}
func (a *App) ExecuteProjectWizard(spec recipes.ProjectWizardSpec) (string, error) {
	return a.backend.ExecuteProjectWizard(spec)
}
func (a *App) CreateProjectFromWizard(spec wizard.WizardSpec) (*wizard.WizardResult, error) {
	return a.backend.CreateProjectFromWizard(spec)
}

// Reports, Architecture & Sharing (A08, A11, A18, A20, X03, X04)
func (a *App) GetProjectSnapshot(id string) (reports.ProjectSnapshot, error) {
	return a.backend.GetProjectSnapshot(id)
}
func (a *App) GenerateREADME(id string) (string, error) { return a.backend.GenerateREADME(id) }
func (a *App) MergeREADME(id, generated string, selectedSections []string) (string, error) {
	return a.backend.MergeREADME(id, generated, selectedSections)
}
func (a *App) SaveREADME(id, content string) error { return a.backend.SaveREADME(id, content) }
func (a *App) ExportTeamManifest(id, outPath string) error {
	return a.backend.ExportTeamManifest(id, outPath)
}
func (a *App) ExportSupportBundle(id, outZipPath string) error {
	return a.backend.ExportSupportBundle(id, outZipPath)
}
func (a *App) StartTunnel(provider, target string) (*sharing.TunnelStatus, error) {
	return a.backend.StartTunnel(provider, target)
}
func (a *App) StopTunnel() error                     { return a.backend.StopTunnel() }
func (a *App) GetTunnelStatus() sharing.TunnelStatus { return a.backend.GetTunnelStatus() }
func (a *App) DispatchWebhook(scenario sharing.WebhookScenario) (*sharing.WebhookDispatchResult, error) {
	return a.backend.DispatchWebhook(scenario)
}
func (a *App) DetectIntegrations(id string) ([]sharing.IntegrationItem, error) {
	return a.backend.DetectIntegrations(id)
}
func (a *App) UpdateSettings(s storage.Settings) error { return a.backend.UpdateSettings(s) }

// Window Close Coordination
func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.allowClose || a.backend.Active() == 0 {
		return false
	}
	if !a.closePending {
		a.closePending = true
		runtime.EventsEmit(ctx, "workbench:close", a.backend.Active())
	}
	return true
}

func (a *App) CancelClose() {
	a.mu.Lock()
	if !a.stopping {
		a.closePending = false
		a.backend.CancelShutdown()
	}
	a.mu.Unlock()
}

func (a *App) ConfirmClose(force bool) error {
	a.mu.Lock()
	if a.stopping {
		a.mu.Unlock()
		return errors.New("Shutdown is already in progress")
	}
	a.stopping = true
	a.mu.Unlock()
	err := a.backend.StopAll(force)
	a.mu.Lock()
	a.stopping = false
	if err == nil {
		a.allowClose = true
	}
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "workbench:update")
	if err != nil {
		return err
	}
	runtime.Quit(a.ctx)
	return nil
}

func (a *App) shutdown(ctx context.Context) {
	if a.host != nil {
		_ = a.host.Close(ctx)
	}
	a.backend.Shutdown()
}

func (a *App) InstallDependency(id, projectID string) (dependencies.Status, error) {
	return a.backend.InstallDependency(id, projectID)
}
func (a *App) GetDependencies(projectID string) []dependencies.Status {
	return a.backend.DependencyStatus(projectID)
}
func (a *App) ChooseDependencyBinary(id string) error {
	if _, err := dependencies.Lookup(id); err != nil {
		return err
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Choose installed tool binary", Filters: []runtime.FileFilter{{DisplayName: "Tool binaries", Pattern: "*.exe;*.phar;*.bat"}}})
	if err != nil || path == "" {
		return err
	}
	return a.backend.SetDependencyBinary(id, path)
}
func (a *App) OpenDependencyWebsite(id string) error {
	spec, err := dependencies.Lookup(id)
	if err != nil {
		return err
	}
	runtime.BrowserOpenURL(a.ctx, spec.Website)
	return nil
}

func (a *App) CancelDependencyInstall() error { return a.backend.CancelDependencyInstall() }

func (a *App) WorkspaceHealth() []workbench.ProjectHealth { return a.backend.WorkspaceHealth() }
func (a *App) Companion(id, operation string, input map[string]any) (any, error) {
	return a.backend.Companion(id, operation, input)
}

func (a *App) OpenCompanionDashboard(id, kind string) error {
	value, err := a.backend.Companion(id, "dashboard-url", map[string]any{"kind": kind})
	if err != nil {
		return err
	}
	target, ok := value.(string)
	if !ok {
		return errors.New("Dashboard URL unavailable")
	}
	runtime.BrowserOpenURL(a.ctx, target)
	return nil
}

func (a *App) LaunchProjectTerminal(id, shellType string) error {
	return a.backend.LaunchProjectTerminal(id, shellType)
}

func (a *App) OpenBrowserDashboard() error {
	if a.host == nil {
		return errors.New("Browser dashboard is not active")
	}
	runtime.BrowserOpenURL(a.ctx, a.host.LaunchURL())
	return nil
}

func (a *App) GetBrowserDashboardURL() string {
	if a.host == nil {
		return ""
	}
	return a.host.LaunchURL()
}

func (a *App) BrowserOpenTarget(method string, args []any) (string, error) {
	switch method {
	case "OpenProject":
		if len(args) > 0 {
			if id, ok := args[0].(string); ok {
				return a.backend.ProjectURL(id)
			}
		}
		return "", errors.New("missing project id")
	case "OpenCompanionDashboard":
		if len(args) > 1 {
			id, ok1 := args[0].(string)
			kind, ok2 := args[1].(string)
			if ok1 && ok2 {
				val, err := a.backend.Companion(id, "dashboard-url", map[string]any{"kind": kind})
				if err != nil {
					return "", err
				}
				if urlStr, ok := val.(string); ok {
					return urlStr, nil
				}
			}
		}
		return "", errors.New("missing companion arguments")
	case "OpenDependencyWebsite":
		if len(args) > 0 {
			if id, ok := args[0].(string); ok {
				spec, err := dependencies.Lookup(id)
				if err != nil {
					return "", err
				}
				return spec.Website, nil
			}
		}
		return "", errors.New("missing dependency id")
	default:
		return "", fmt.Errorf("unsupported open target: %s", method)
	}
}

// Runtime & Composer Management
func (a *App) ListRuntimes() []runtimes.RuntimeInfo {
	return a.backend.ListRuntimes()
}

func (a *App) InstallPHP(version string) error {
	return a.backend.InstallPHP(version)
}

func (a *App) ToggleExtension(iniPath, extensionName string, enable bool) error {
	return a.backend.ToggleExtension(iniPath, extensionName, enable)
}

func (a *App) GetComposerStatus() runtimes.ComposerStatus {
	return a.backend.GetComposerStatus()
}

func (a *App) InstallComposer() error {
	return a.backend.InstallComposer()
}

func (a *App) AddBinToUserPath() error {
	return a.backend.AddBinToUserPath()
}

func (a *App) IsolateHerdSite(projectID, version string) error {
	return a.backend.IsolateHerdSite(projectID, version)
}

// In-App Dumps & Xdebug (Phase 5)
func (a *App) GetInAppDumps(projectID string) []telemetry.DumpPayload {
	return a.backend.GetInAppDumps(projectID)
}
func (a *App) ClearInAppDumps(projectID string) {
	a.backend.ClearInAppDumps(projectID)
}
func (a *App) GetDumpHelperPHP() string {
	return a.backend.GetDumpHelperPHP()
}
func (a *App) GetXdebugStatus(projectID string) (*telemetry.XdebugStatus, error) {
	return a.backend.GetXdebugStatus(projectID)
}
func (a *App) GenerateVSCodeConfig(projectID string) error {
	return a.backend.GenerateVSCodeConfig(projectID)
}

// Activity Tracing (Phase 6)
func (a *App) GetActivityTrace(projectID, requestID string) (*activity.ActivityTrace, error) {
	return a.backend.GetActivityTrace(projectID, requestID)
}
func (a *App) ExportTraceHTML(projectID, requestID string) (string, error) {
	return a.backend.ExportTraceHTML(projectID, requestID)
}

// Events & Queues Supervisor (Phase 6)
func (a *App) DispatchEvent(projectID, eventClass, payloadJSON string) (actions.ActionRunResult, error) {
	return a.backend.DispatchEvent(projectID, eventClass, payloadJSON)
}

// Webhook Workbench (Phase 6)
func (a *App) GetWebhookDeliveries(endpointID string) []webhooks.WebhookDelivery {
	return a.backend.GetWebhookDeliveries(endpointID)
}
func (a *App) ClearWebhookDeliveries(endpointID string) {
	a.backend.ClearWebhookDeliveries(endpointID)
}
func (a *App) ReplayWebhook(deliveryID, targetURL string) (*webhooks.WebhookReplayResult, error) {
	return a.backend.ReplayWebhook(deliveryID, targetURL)
}
func (a *App) DispatchSignedWebhook(targetURL, provider, secret, payload string) (*webhooks.WebhookReplayResult, error) {
	return a.backend.DispatchSignedWebhook(targetURL, provider, secret, payload)
}

// Performance & Load Testing (Phase 6)
func (a *App) RunProjectCheckup(projectID string) (*performance.CheckupReport, error) {
	return a.backend.RunProjectCheckup(projectID)
}
func (a *App) RunNativeLoadTest(spec performance.LoadTestSpec) (*performance.LoadTestResult, error) {
	return a.backend.RunNativeLoadTest(spec)
}

// -------------------------------------------------------------
// REVERSE-ENGINEERED LARAMONSTER PARITY WRAPPERS
// -------------------------------------------------------------

func (a *App) ToggleDebuggerAgent(projectID string, enable bool) error {
	return a.backend.ToggleDebuggerAgent(projectID, enable)
}

func (a *App) IsDebuggerAgentActive(projectID string) bool {
	return a.backend.IsDebuggerAgentActive(projectID)
}

func (a *App) GetTelemetryFrames(projectID string) []telemetry.RequestTelemetryFrame {
	return a.backend.GetTelemetryFrames(projectID)
}

func (a *App) ClearTelemetryFrames(projectID string) {
	a.backend.ClearTelemetryFrames(projectID)
}

func (a *App) SetDumpsInHtml(enabled bool) {
	a.backend.SetDumpsInHtml(enabled)
}

func (a *App) GetDumpsInHtml() bool {
	return a.backend.GetDumpsInHtml()
}

func (a *App) IntrospectEvent(projectID, eventClass string) (*actions.EventIntrospection, error) {
	return a.backend.IntrospectEvent(projectID, eventClass)
}

func (a *App) IntrospectJob(projectID, jobClass string) (*actions.JobIntrospection, error) {
	return a.backend.IntrospectJob(projectID, jobClass)
}

func (a *App) RunJobNow(projectID, jobClass, payloadJSON string) (*actions.JobRunResult, error) {
	return a.backend.RunJobNow(projectID, jobClass, payloadJSON)
}

func (a *App) InspectWebhookRoutes(projectID string) ([]webhooks.WebhookRouteInspection, error) {
	return a.backend.InspectWebhookRoutes(projectID)
}

func (a *App) ExplainStatusCode(code int) string {
	return a.backend.ExplainStatusCode(code)
}

func (a *App) ValidateWebhookSignature(provider, secret, payload, headerSig string) string {
	return a.backend.ValidateWebhookSignature(provider, secret, payload, headerSig)
}

func (a *App) ExecuteCheckupFix(projectID, fixID string) error {
	return a.backend.ExecuteCheckupFix(projectID, fixID)
}

func (a *App) GenerateAuthCredentials(projectID, identifier string) (*performance.AuthCredentials, error) {
	return a.backend.GenerateAuthCredentials(projectID, identifier)
}

func (a *App) GetLiveIniDirective(projectID, directive string) (string, error) {
	return a.backend.GetLiveIniDirective(projectID, directive)
}

func (a *App) SetSiteIniDirective(projectID, directive, value string) error {
	return a.backend.SetSiteIniDirective(projectID, directive, value)
}


