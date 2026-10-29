import CompanionPanel from "./CompanionPanel";
import DashboardPanel from "./DashboardPanel";
import { NavIcon } from "./NavIcon";
import AuditPanel, {AuditSetupPanel} from "./AuditPanel";
import DependencySettings from "./DependencySettings";
import PhpRuntimePanel from "./PhpRuntimePanel";
import CommandPalette from "./CommandPalette";
import { useCallback, useEffect, useRef, useState, Component, ErrorInfo, ReactNode } from "react";
import {
  CancelClose,
  ConfirmClose,
  DiscoverProjects,
  GetState,
  LaunchProjectTerminal,
  ListRoutes,
  OpenProject,
  ProbeCapabilities,
  RefreshCaptures,
  SelectProject,
  SetWatching,
  StartProcess,
  StopProcess,
  TrustProject,
  SetSecureSite,
  GetEnv,
  SaveEnv,
  GetEnvDrift,
  GetPHPInfo,
  PreviewIniChange,
  ApplyIniChange,
  GetLiveIniDirective,
  SetSiteIniDirective,
  GetRawLogs,
  GetErrors,
  GetDumps,
  GetHistory,
  ClearHistory,
  RefreshServices,
  StartService,
  StopService,
  GetMailbox,
  GetMailMessage,
  DeleteAllMail,
  GetEvents,
  GetFailedJobs,
  GetQueueWorkerStatus,
  RetryFailedJob,
  RetryAllFailedJobs,
  ForgetFailedJob,
  FlushFailedJobs,
  ListScheduledTasks,
  RunScheduledTask,
  StartQueueWorker,
  StopQueueWorker,
  GetSavedCommands,
  SaveCommand,
  ExecuteSavedCommand,
  ExecuteAPIRequest,
  GenerateBoundaryCases,
  RunLoadTest,
  RunProjectTests,
  RunPint,
  AuditProject,
  ListRecipes,
  PreviewRecipe,
  ExecuteRecipe,
  ExecuteProjectWizard,
  GetProjectSnapshot,
  GenerateREADME,
  MergeREADME,
  SaveREADME,
  ExportTeamManifest,
  ExportSupportBundle,
  StartTunnel,
  StopTunnel,
  DetectIntegrations,
  GetNPlusOneReport,
  GetWaterfallFlamegraph,
  OpenBrowserDashboard,
  GetArchitectureCanvas,
  GetLogPailReport,
  GetHealthHeartbeat,
  ListFeatureFlags,
  ApplyServicesToEnv,
  AutoProvisionDatabase,
  CreateProjectFromWizard,
  GetInAppDumps,
  ClearInAppDumps,
  GetDumpHelperPHP,
  GetXdebugStatus,
  GenerateVSCodeConfig,
  GetActivityTrace,
  ExportTraceHTML,
  DispatchEvent,
  GetWebhookDeliveries,
  ClearWebhookDeliveries,
  ReplayWebhook,
  DispatchSignedWebhook,
  RunProjectCheckup,
  RunNativeLoadTest,
} from "../wailsjs/go/main/App";
import { DumpsPanel } from "./DumpsPanel";
import { ActivityPanel } from "./ActivityPanel";
import { EventsQueuesPanel } from "./EventsQueuesPanel";
import { WebhooksPanel } from "./WebhooksPanel";
import { PerformancePanel } from "./PerformancePanel";
import { ProjectWizardModal } from "./ProjectWizardModal";
import { TinkerPanel } from "./TinkerPanel";
import { DatabaseBrowser } from "./DatabaseBrowser";
import { NPlusOneRadar } from "./NPlusOneRadar";
import { WaterfallTimeline } from "./WaterfallTimeline";
import { RemoteProfilesPanel } from "./RemoteProfilesPanel";
import { RealtimeReverbPanel } from "./RealtimeReverbPanel";
import { ArchitectureCanvas } from "./ArchitectureCanvas";
import { EventsOn, ClipboardSetText } from "../wailsjs/runtime/runtime";
import {
  workbench,
  services,
  environment,
  mail,
  actions,
  testing as wktesting,
  quality,
  recipes,
  reports,
  sharing,
  storage,
  telemetry,
  architecture,
  flags,
  health,
} from "../wailsjs/go/models";

type Tab =
  | "dashboard"
  | "companion"
  | "requests"
  | "logs"
  | "tinker"
  | "database"
  | "environment"
  | "services"
  | "actions"
  | "testing"
  | "quality"
  | "recipes"
  | "reports"
  | "realtime"
  | "remote"
  | "webhooks"
  | "settings";

const ms = (value: number) => `${Number(value || 0).toFixed(1)} ms`;

const emptyState = new workbench.State({
  projects: [],
  selected: "",
  trusted: false,
  capture: {
    requests: [],
    watching: false,
    message: "Select a project to begin.",
    updated: "",
  },
  runs: [],
  services: [],
  docker: {
    dockerInstalled: false,
    dockerRunning: false,
    wslInstalled: false,
    version: "",
    message: "Checking Docker...",
  },
  settings: {
    version: 1,
    trusted: {},
    customPaths: [],
    theme: "system",
    maxRetention: 1000,
    savedCommands: {},
    serviceConfigs: {},
    aiConfig: { enabled: false, provider: "local", apiKeyRef: "", model: "llama3", endpoint: "" },
    updatedAt: "",
  },
  tunnel: {
    running: false,
    provider: "",
    localTarget: "",
    publicUrl: "",
    startedAt: "",
    pid: 0,
    error: "",
  },
  error: "",
  loading: true,
});

interface ErrorBoundaryProps {
  children: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
  error: Error | null;
}

class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false, error: null };
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error("Workbench ErrorBoundary caught an error:", error, errorInfo);
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="tab-pane" style={{ padding: "32px", maxWidth: "800px", margin: "40px auto" }}>
          <div className="workbench-card" style={{ borderColor: "rgba(239, 68, 68, 0.5)", background: "#111827", padding: "24px" }}>
            <h3 style={{ color: "#ef4444", display: "flex", alignItems: "center", gap: "8px" }}>
              <span>âš ï¸</span>
              <span>View Rendering Issue</span>
            </h3>
            <p style={{ color: "#94a3b8", margin: "12px 0", fontSize: "14px" }}>
              An error occurred while displaying this tab. You can reload this view or switch to any other tab without losing your session.
            </p>
            <pre style={{ background: "#090d16", padding: "14px", borderRadius: "8px", color: "#f87171", fontSize: "12px", overflowX: "auto", border: "1px solid #1e293b" }}>
              {this.state.error?.message || String(this.state.error)}
            </pre>
            <div style={{ marginTop: "20px", display: "flex", gap: "12px" }}>
              <button
                className="btn-primary"
                onClick={() => this.setState({ hasError: false, error: null })}
              >
                â†» Retry View
              </button>
            </div>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

export default function Workbench() {
  const [state, setState] = useState<workbench.State>(emptyState);
  const [tab, setTab] = useState<Tab>("dashboard");
  const [selectedID, setSelectedID] = useState("");
  const [search, setSearch] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState("");
  const [closeCount, setCloseCount] = useState<number | null>(null);
  const [closeError, setCloseError] = useState("");
  const [paletteOpen, setPaletteOpen] = useState(false);

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((prev) => !prev);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Subsystem state
  const [caps, setCaps] = useState<workbench.Capability[]>([]);
  const [routes, setRoutes] = useState<workbench.Route[]>([]);
  const [envData, setEnvData] = useState<environment.EnvFile | null>(null);
  const [drift, setDrift] = useState<environment.DriftReport | null>(null);
  const [phpInfo, setPhpInfo] = useState<environment.PHPInfo | null>(null);
  const [rawLogs, setRawLogs] = useState<workbench.LogEntry[]>([]);
  const [errorsList, setErrorsList] = useState<storage.ErrorLogRecord[]>([]);
  const [dumpsList, setDumpsList] = useState<storage.DumpRecord[]>([]);
  const [mailbox, setMailbox] = useState<mail.MailboxResult | null>(null);
  const [selectedMail, setSelectedMail] = useState<mail.MailMessageDetail | null>(null);
  const [eventsList, setEventsList] = useState<actions.EventListenerItem[]>([]);
  const [failedJobs, setFailedJobs] = useState<actions.FailedJobItem[]>([]);
  const [scheduledTasks, setScheduledTasks] = useState<actions.ScheduledTaskItem[]>([]);
  const [expandedJobException, setExpandedJobException] = useState<string | null>(null);
  const [workerStatus, setWorkerStatus] = useState<actions.QueueWorkerStatus | null>(null);
  const [savedCmds, setSavedCmds] = useState<storage.SavedCmd[]>([]);
  const [cmdResult, setCmdResult] = useState<actions.ActionRunResult | null>(null);
  const [auditItems, setAuditItems] = useState<quality.ConfigAuditItem[]>([]);
  const [pintReport, setPintReport] = useState<quality.QualityReport | null>(null);
  const [recipesList, setRecipesList] = useState<recipes.Recipe[]>([]);
  const [recipeResult, setRecipeResult] = useState<recipes.RecipeExecutionResult | null>(null);
  const [snapshot, setSnapshot] = useState<reports.ProjectSnapshot | null>(null);
  const [readmeDraft, setReadmeDraft] = useState<string>("");
  const [selectedMergeSections, setSelectedMergeSections] = useState<string[]>([
    "requirements",
    "local development",
    "infrastructure & deployment profile",
  ]);
  const [integrations, setIntegrations] = useState<sharing.IntegrationItem[]>([]);
  const [testRunResult, setTestRunResult] = useState<wktesting.ProjectTestResult | null>(null);
  const [loadResult, setLoadResult] = useState<wktesting.LoadTestResult | null>(null);
  const [apiResponse, setApiResponse] = useState<wktesting.APIResponseResult | null>(null);

  const [history, setHistory] = useState<storage.DiagnosticRecord[]>([]);
  const [worker, setWorker] = useState<actions.QueueWorkerStatus | null>(null);
  const [iniKey, setIniKey] = useState("memory_limit");
  const [iniValue, setIniValue] = useState("512M");
  const [iniPreview, setIniPreview] = useState<environment.IniChangePreview | null>(null);
  const [liveIniValue, setLiveIniValue] = useState<string>("");

  // Forms state
  const [apiMethod, setApiMethod] = useState("GET");
  const [apiUrl, setApiUrl] = useState("");
  const [apiBody, setApiBody] = useState("");
  const [loadUsers, setLoadUsers] = useState(5);
  const [loadDuration, setLoadDuration] = useState(5);
  const [webhookSecret, setWebhookSecret] = useState("whsec_test");
  const [webhookPayload, setWebhookPayload] = useState('{"event":"order.created","amount":100}');
  const [newCmdName, setNewCmdName] = useState("");
  const [newCmdArgs, setNewCmdArgs] = useState('["artisan", "route:list"]');
  const [wizardName, setWizardName] = useState("my-laravel-app");
  const [wizardDir, setWizardDir] = useState("");
  const [wizardDb, setWizardDb] = useState("sqlite");
  const [tunnelProvider, setTunnelProvider] = useState<string>("cloudflared");
  const [copiedType, setCopiedType] = useState<string | null>(null);
  const [copiedSqlIdx, setCopiedSqlIdx] = useState<number | null>(null);

  // Sub-tab & UI filters
  const [telemetryFilter, setTelemetryFilter] = useState<"all" | "2xx" | "4xx" | "5xx" | "slow">("all");
  const [telemetryDetailTab, setTelemetryDetailTab] = useState<"overview" | "queries" | "radar" | "waterfall">("overview");
  const [nPlusOneReport, setNPlusOneReport] = useState<telemetry.NPlusOneReport | null>(null);
  const [waterfallData, setWaterfallData] = useState<telemetry.WaterfallFlamegraph | null>(null);
  const [loadingTelemetryAnalysis, setLoadingTelemetryAnalysis] = useState<boolean>(false);

  const loadTelemetryAnalysis = async (reqId: string) => {
    if (!project) return;
    setLoadingTelemetryAnalysis(true);
    try {
      const [report, waterfall] = await Promise.all([
        GetNPlusOneReport(project.id, reqId),
        GetWaterfallFlamegraph(project.id, reqId),
      ]);
      setNPlusOneReport(report);
      setWaterfallData(waterfall);
    } catch (e) {
      console.error("Telemetry analysis failed", e);
    } finally {
      setLoadingTelemetryAnalysis(false);
    }
  };
  const [busyServiceId, setBusyServiceId] = useState<string | null>(null);
  const [envSubTab, setEnvSubTab] = useState<"env" | "php" | "runtimes">("runtimes");
  const [showSecrets, setShowSecrets] = useState(false);
  const [envFilterQuery, setEnvFilterQuery] = useState("");
  const [actionsSubTab, setActionsSubTab] = useState<"runners" | "failed" | "schedule" | "events" | "commands">("runners");
  const [testingSubTab, setTestingSubTab] = useState<"api" | "pest" | "load" | "checkup">("api");
  const [recipeCategory, setRecipeCategory] = useState<string>("all");
  const [recipeSearch, setRecipeSearch] = useState<string>("");
  const [recipeSubTab, setRecipeSubTab] = useState<"recipes" | "wizard">("recipes");
  const [reportsSubTab, setReportsSubTab] = useState<"canvas" | "graph" | "hosting" | "readme" | "manifest">("canvas");
  const [archReport, setArchReport] = useState<architecture.ArchitectureCanvasReport | null>(null);
  const [logPailReport, setLogPailReport] = useState<storage.LogPailReport | null>(null);
  const [copiedAIPromptFp, setCopiedAIPromptFp] = useState<string | null>(null);
  const [logsSubTab, setLogsSubTab] = useState<"incidents" | "raw" | "dumps">("incidents");
  const [heartbeatReport, setHeartbeatReport] = useState<health.HeartbeatReport | null>(null);
  const [flagsReport, setFlagsReport] = useState<flags.FlagsReport | null>(null);
  const [testFilter, setTestFilter] = useState<string>("");
  const [copiedMcp, setCopiedMcp] = useState<string | null>(null);
  const [archProjectSearch, setArchProjectSearch] = useState<string>("");
  const [archEdgeSearch, setArchEdgeSearch] = useState<string>("");
  const [archProvenanceFilter, setArchProvenanceFilter] = useState<"all" | "declared" | "inferred" | "observed">("all");
  const [settingsSubTab, setSettingsSubTab] = useState<"tunnels" | "tooling" | "ai" | "general">("tunnels");
  const [toolingMode, setToolingMode] = useState<"deps" | "doctor">("deps");
  const [inAppDumps, setInAppDumps] = useState<any[]>([]);
  const [dumpHelperSnippet, setDumpHelperSnippet] = useState<string>("");
  const [xdebugStatus, setXdebugStatus] = useState<any>(null);
  const [activeTrace, setActiveTrace] = useState<any>(null);
  const [webhookDeliveries, setWebhookDeliveries] = useState<any[]>([]);
  const [checkupReport, setCheckupReport] = useState<any>(null);
  const [isWizardOpen, setIsWizardOpen] = useState<boolean>(false);
  const [telemetrySubTab, setTelemetrySubTab] = useState<"requests" | "dumps" | "activity" | "xdebug">("requests");

  const copyText = async (text: string): Promise<boolean> => {
    try {
      const ok = await ClipboardSetText(text);
      if (ok) return true;
    } catch {
      // Fallback to browser clipboard
    }
    try {
      if (navigator?.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
        return true;
      }
    } catch {
      // Fallback to document.execCommand
    }
    try {
      const el = document.createElement("textarea");
      el.value = text;
      el.style.position = "fixed";
      el.style.opacity = "0";
      document.body.appendChild(el);
      el.select();
      const success = document.execCommand("copy");
      document.body.removeChild(el);
      return success;
    } catch {
      return false;
    }
  };

  const handleCopyRequest = async (format: "url" | "curl" | "json") => {
    if (!request) return;
    const fullUrl = project ? `${project.url.replace(/\/$/, "")}${request.uri}` : request.uri;
    let textToCopy = "";
    if (format === "url") {
      textToCopy = fullUrl;
    } else if (format === "curl") {
      textToCopy = `curl -X ${request.method} "${fullUrl}"`;
    } else if (format === "json") {
      textToCopy = JSON.stringify(request, null, 2);
    }

    const ok = await copyText(textToCopy);
    if (ok) {
      setCopiedType(format);
      setTimeout(() => setCopiedType(null), 2000);
    } else {
      setNotice("Failed to copy to clipboard.");
    }
  };

  const handleCopySql = async (sql: string, idx: number) => {
    const ok = await copyText(sql);
    if (ok) {
      setCopiedSqlIdx(idx);
      setTimeout(() => setCopiedSqlIdx(null), 2000);
    } else {
      setNotice("Failed to copy SQL to clipboard.");
    }
  };



  const sequence = useRef(0);
  const dialog = useRef<HTMLElement>(null);

  const handleCancelClose = async () => {
    try {
      await CancelClose();
    } catch (err) {
      console.error("CancelClose error:", err);
    }
    setCloseCount(null);
    setCloseError("");
  };

  useEffect(() => {
    if (closeCount === null) return;
    const prev = document.activeElement as HTMLElement | null;
    dialog.current?.querySelector<HTMLButtonElement>("button")?.focus();
    return () => prev?.focus();
  }, [closeCount]);

  const load = useCallback(async () => {
    const id = ++sequence.current;
    try {
      const val = await GetState();
      if (id === sequence.current) setState(val);
    } catch {
      setNotice("Desktop connection unavailable.");
    }
  }, []);

  useEffect(() => {
    load();
    const update = EventsOn("workbench:update", load);
    const close = EventsOn("workbench:close", (count: number) => {
      setCloseCount(count);
      setCloseError("");
    });
    return () => {
      update();
      close();
    };
  }, [load]);

  const project = state.projects.find((p) => p.id === state.selected);

  useEffect(() => {
    if (project?.id && iniKey) {
      GetLiveIniDirective(project.id, iniKey).then((v) => setLiveIniValue(v || "")).catch(() => setLiveIniValue(""));
    }
  }, [project?.id, iniKey]);
  const clockworkNeedsSetup = ["not_installed", "dependencies_missing", "setup_incomplete"].includes(
    state.capture?.status || "",
  );
  const lastAutoWatchedProject = useRef<string>("");

  useEffect(() => {
    if (tab === "requests" && state.selected && state.trusted && !clockworkNeedsSetup) {
      if (lastAutoWatchedProject.current !== state.selected) {
        lastAutoWatchedProject.current = state.selected;
        RefreshCaptures().catch(() => {});
        if (!state.capture?.watching) {
          SetWatching(true).catch(() => {});
        }
      }
    }
  }, [tab, state.selected, state.trusted, clockworkNeedsSetup, state.capture?.watching]);

  const act = async (label: string, action: () => Promise<unknown>) => {
    if (busy) return;
    setBusy(label);
    setNotice("");
    try {
      await action();
    } catch (e) {
      setNotice(String(e));
    } finally {
      setBusy("");
      await load();
    }
  };

  const select = (id: string) =>
    act("Select project", async () => {
      lastAutoWatchedProject.current = "";
      await SelectProject(id);
      setSelectedID("");
      setCaps([]);
      setRoutes([]);
      setSearch("");
      setEnvData(null);
      setDrift(null);
      setPhpInfo(null);
      setSnapshot(null);
      setRawLogs([]); setErrorsList([]); setDumpsList([]);
      setEventsList([]); setFailedJobs([]); setScheduledTasks([]); setSavedCmds([]); setCmdResult(null);
      setAuditItems([]); setPintReport(null); 
      setReadmeDraft(""); setTestRunResult(null); setLoadResult(null); setApiResponse(null);
      setApiUrl(""); setIntegrations([]); setRecipeResult(null); setHistory([]); setIniPreview(null); setWorker(null);
    });

  const exit = async (force: boolean) => {
    setBusy("Stopping owned processes");
    setCloseError("");
    try {
      await ConfirmClose(force);
    } catch (e) {
      setCloseError(String(e));
    } finally {
      setBusy("");
      await load();
    }
  };

  // Tab Loaders
  const loadEnvironment = () => {
    if (!project) return;
    act("Load environment", async () => {
      const env = await GetEnv(project.id);
      setEnvData(env);
      const dr = await GetEnvDrift(project.id);
      setDrift(dr);
      const php = await GetPHPInfo(project.id);
      setPhpInfo(php);
    });
  };

  const loadLogs = () => {
    if (!project) return;
    act("Load logs and errors", async () => {
      try {
        const r = await GetRawLogs(project.id);
        setRawLogs(Array.isArray(r) ? r : []);
      } catch (error) { setNotice(String(error));
        setRawLogs([]);
      }
      try {
        const errs = await GetErrors(project.id, search, 50);
        setErrorsList(Array.isArray(errs) ? errs : []);
      } catch (error) { setNotice(String(error));
        setErrorsList([]);
      }
      try {
        const pail = await GetLogPailReport(project.id, 100);
        setLogPailReport(pail);
      } catch (error) {
        console.error("Log pail fetch error:", error);
      }
      try {
        setHistory(await GetHistory(project.id, search, 50, 0));
        const dmps = await GetDumps(project.id, 30);
        setDumpsList(Array.isArray(dmps) ? dmps : []);
      } catch (error) { setNotice(String(error));
        setDumpsList([]);
      }
    });
  };

  const loadServices = () => {
    act("Refresh services and mail", async () => {
      try {
        await RefreshServices();
      } catch (error) { setNotice(String(error));}
      try {
        const mb = await GetMailbox("", 30, 0);
        setMailbox(mb);
      } catch (error) { setNotice(String(error));
        setMailbox(null);
      }
    });
  };

  const loadActions = () => {
    if (!project) return;
    act("Load actions & queues", async () => {
      try {
        const ev = await GetEvents(project.id);
        setEventsList(Array.isArray(ev) ? ev : []);
      } catch (error) { setNotice(String(error));
        setEventsList([]);
      }
      try {
        const fj = await GetFailedJobs(project.id);
        setFailedJobs(Array.isArray(fj) ? fj : []);
      } catch (error) { setNotice(String(error));
        setFailedJobs([]);
      }
      try {
        const tasks = await ListScheduledTasks(project.id);
        setScheduledTasks(Array.isArray(tasks) ? tasks : []);
      } catch (error) {
        setScheduledTasks([]);
      }
      try {
        setWorker(await GetQueueWorkerStatus());
        const sc = await GetSavedCommands(project.id);
        setSavedCmds(Array.isArray(sc) ? sc : []);
      } catch (error) { setNotice(String(error));
        setSavedCmds([]);
      }
    });
  };

  const loadQuality = () => {
    if (!project) return;
    act("Run audit & quality", async () => {
      try {
        const audit = await AuditProject(project.id);
        setAuditItems(Array.isArray(audit) ? audit : []);
      } catch (error) { setNotice(String(error));
        setAuditItems([]);
      }
    });
  };

  const loadRecipes = () => {
    act("Load recipes", async () => {
      try {
        const r = await ListRecipes();
        setRecipesList(Array.isArray(r) ? r : []);
      } catch (error) { setNotice(String(error));
        setRecipesList([]);
      }
    });
  };

  const loadReports = () => {
    if (!project) return;
    act("Generate project snapshot", async () => {
      try {
        const snap = await GetProjectSnapshot(project.id);
        setSnapshot(snap);
      } catch (error) { setNotice(String(error));
        setSnapshot(null);
      }
      try {
        const rm = await GenerateREADME(project.id);
        setReadmeDraft(rm || "");
      } catch (error) { setNotice(String(error));
        setReadmeDraft("");
      }
      try {
        const integ = await DetectIntegrations(project.id);
        setIntegrations(Array.isArray(integ) ? integ : []);
      } catch (error) { setNotice(String(error));
        setIntegrations([]);
      }
      try {
        const arch = await GetArchitectureCanvas(project.id);
        setArchReport(arch);
      } catch (error) {
        console.error("Architecture canvas error:", error);
      }
    });
  };

  const loadArchitectureCanvas = async () => {
    if (!project) return;
    try {
      const arch = await GetArchitectureCanvas(project.id);
      setArchReport(arch);
    } catch (e) {
      console.error("Architecture canvas error:", e);
    }
  };

  const loadInAppDumps = async () => {
    try {
      const d = await GetInAppDumps(project?.id || "");
      setInAppDumps(d || []);
      const snip = await GetDumpHelperPHP();
      setDumpHelperSnippet(snip || "");
    } catch (e) {
      console.error("Failed to load in-app dumps", e);
    }
  };

  const handleClearInAppDumps = async () => {
    try {
      await ClearInAppDumps(project?.id || "");
      setInAppDumps([]);
    } catch (e) {
      console.error("Failed to clear in-app dumps", e);
    }
  };

  const loadXdebug = async () => {
    try {
      const xd = await GetXdebugStatus(project?.id || "");
      setXdebugStatus(xd);
    } catch (e) {
      console.error("Failed to load xdebug status", e);
    }
  };

  const loadWebhooks = async () => {
    try {
      const wh = await GetWebhookDeliveries("");
      setWebhookDeliveries(wh || []);
    } catch (e) {
      console.error("Failed to load webhooks", e);
    }
  };

  const handleClearWebhooks = async () => {
    try {
      await ClearWebhookDeliveries("");
      setWebhookDeliveries([]);
    } catch (e) {
      console.error("Failed to clear webhooks", e);
    }
  };

  const loadCheckup = async () => {
    if (!project?.id) return;
    try {
      const rep = await RunProjectCheckup(project.id);
      setCheckupReport(rep);
    } catch (e) {
      console.error("Failed to run checkup", e);
    }
  };

  useEffect(() => {
    if (project) {
      GetHealthHeartbeat(project.id).then(setHeartbeatReport).catch(() => {});
      ListFeatureFlags(project.id).then(setFlagsReport).catch(() => {});
      GetArchitectureCanvas(project.id).then(setArchReport).catch(() => {});
      loadInAppDumps();
      loadXdebug();
      loadWebhooks();
      loadCheckup();
    } else {
      setHeartbeatReport(null);
      setFlagsReport(null);
      setArchReport(null);
    }
  }, [project?.id]);

  // Change tab handler
  const switchTab = (nextTab: Tab) => {
    setTab(nextTab);
    if (!project && nextTab !== "services" && nextTab !== "recipes" && nextTab !== "settings" && nextTab !== "webhooks") return;
    if (nextTab === "dashboard" || nextTab === "requests") {
      loadInAppDumps();
      loadXdebug();
      if (state.trusted && !clockworkNeedsSetup) {
        RefreshCaptures().catch(() => {});
        if (!state.capture?.watching) {
          SetWatching(true).catch(() => {});
        }
      }
    }
    if (nextTab === "environment") loadEnvironment();
    if (nextTab === "logs") loadLogs();
    if (nextTab === "services") loadServices();
    if (nextTab === "actions") loadActions();
    if (nextTab === "webhooks") loadWebhooks();
    if (nextTab === "testing") {
      loadCheckup();
    }
    if (nextTab === "quality") loadQuality();
    if (nextTab === "recipes") loadRecipes();
    if (nextTab === "reports") loadReports();
  };

  const requests = state.capture.requests || [];
  const visible = requests.filter((r) => {
    const matchesSearch = `${r.method} ${r.uri} ${r.status}`.toLowerCase().includes(search.toLowerCase());
    if (!matchesSearch) return false;
    if (telemetryFilter === "2xx") return r.status >= 200 && r.status < 300;
    if (telemetryFilter === "4xx") return r.status >= 400 && r.status < 500;
    if (telemetryFilter === "5xx") return r.status >= 500;
    if (telemetryFilter === "slow") return r.duration > 100;
    return true;
  });
  const request = visible.find((r) => r.id === selectedID) || visible[0];
  const runs = state.runs.filter((r) => r.projectId === state.selected);

  return (
    <div className="app-shell">
      <aside className="sidebar" inert={closeCount !== null}>
        <div className="brand">
          <span className="brand-mark" style={{ background: "linear-gradient(135deg, #6366f1, #a855f7)", color: "#fff", fontWeight: 800 }}>
            L<span>_</span>
          </span>
          <div>
            LaraDeck<small>MISSION CONTROL FOR LARAVEL</small>
          </div>
        </div>

        <div className="sidebar-heading" style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
          <span>Projects</span>
          <div style={{ display: "flex", gap: "4px", alignItems: "center" }}>
            <button
              onClick={() => setIsWizardOpen(true)}
              title="Create new Laravel project with wizard"
              style={{
                fontSize: "10px",
                padding: "2px 6px",
                borderRadius: "4px",
                background: "rgba(99, 102, 241, 0.25)",
                color: "#a5b4fc",
                border: "1px solid rgba(99, 102, 241, 0.4)",
                cursor: "pointer",
                fontWeight: "bold",
              }}
            >
              + New
            </button>
            <button
              aria-label="Rediscover projects"
              title="Rediscover Herd sites"
              onClick={() => act("Discover projects", DiscoverProjects)}
              disabled={!!busy || state.loading}
            >
              ↻
            </button>
          </div>
        </div>

        <div className="project-list">
          {state.projects.map((p) => (
            <button
              key={p.id}
              className={`project-item ${p.id === state.selected ? "selected" : ""}`}
              onClick={() => select(p.id)}
            >
              <div className="project-avatar">{p.name.slice(0, 2).toUpperCase()}</div>
              <div>
                <strong>{p.name}</strong>
                <small>
                  PHP {p.php} Â· {p.secured ? "ðŸ”’ SSL" : "HTTP"}
                </small>
              </div>
              {p.id === state.selected && <span className="selection-dot" />}
            </button>
          ))}
        </div>

        <nav aria-label="LaraDeck Capabilities" className="sidebar-nav">
          <button className={`nav-item ${tab === "dashboard" ? "active" : ""}`} onClick={() => switchTab("dashboard")}>
            <NavIcon name="dashboard" /> Dashboard
          </button>
          <button className={`nav-item ${tab === "companion" ? "active" : ""}`} onClick={() => switchTab("companion")}>
            <NavIcon name="graph" /> Laravel Companion
          </button>
          <div className="sidebar-section-label">MONITOR</div>
          <button
            className={`nav-item ${tab === "requests" ? "active" : ""}`}
            onClick={() => switchTab("requests")}
          >
            <NavIcon name="activity" /> Telemetry
            {requests.length > 0 && <b>{requests.length}</b>}
          </button>
          <button
            className={`nav-item ${tab === "logs" ? "active" : ""}`}
            onClick={() => switchTab("logs")}
          >
            <NavIcon name="logs" /> Logs & Errors
          </button>
          <button
            className={`nav-item ${tab === "realtime" ? "active" : ""}`}
            onClick={() => switchTab("realtime")}
          >
            <NavIcon name="activity" /> WebSockets &amp; Reverb
          </button>
          <button
            className={`nav-item ${tab === "webhooks" ? "active" : ""}`}
            onClick={() => switchTab("webhooks")}
          >
            <NavIcon name="activity" /> Webhooks Workbench
            {webhookDeliveries && webhookDeliveries.length > 0 && <b>{webhookDeliveries.length}</b>}
          </button>

          <div className="sidebar-section-label">STUDIO</div>
          <button
            className={`nav-item ${tab === "tinker" ? "active" : ""}`}
            onClick={() => switchTab("tinker")}
          >
            <NavIcon name="terminal" /> Tinker REPL
          </button>
          <button
            className={`nav-item ${tab === "database" ? "active" : ""}`}
            onClick={() => switchTab("database")}
          >
            <NavIcon name="database" /> Database Browser
          </button>
          <button
            className={`nav-item ${tab === "actions" ? "active" : ""}`}
            onClick={() => switchTab("actions")}
          >
            <NavIcon name="play" /> Actions & Queues
            {failedJobs && failedJobs.length > 0 && <b className="badge-err">{failedJobs.length}</b>}
          </button>

          <div className="sidebar-section-label">QUALITY & HEALTH</div>
          <button
            className={`nav-item ${tab === "quality" ? "active" : ""}`}
            onClick={() => switchTab("quality")}
          >
            <NavIcon name="shield" /> Quality & Audit
            {state.auditReports?.[project?.id || ""]?.findings?.length ? (
              <b className="badge-warn">{state.auditReports[project?.id || ""].findings.length}</b>
            ) : null}
          </button>
          <button
            className={`nav-item ${tab === "testing" ? "active" : ""}`}
            onClick={() => switchTab("testing")}
          >
            <NavIcon name="test" /> API & Tests
          </button>

          <div className="sidebar-section-label">SETUP & CONFIG</div>
          <button
            className={`nav-item ${tab === "environment" ? "active" : ""}`}
            onClick={() => switchTab("environment")}
          >
            <NavIcon name="sliders" /> Environment & PHP
          </button>
          <button
            className={`nav-item ${tab === "services" ? "active" : ""}`}
            onClick={() => switchTab("services")}
          >
            <NavIcon name="mail" /> Services & Mail
          </button>
          <button
            className={`nav-item ${tab === "recipes" ? "active" : ""}`}
            onClick={() => switchTab("recipes")}
          >
            <NavIcon name="box" /> Recipes & Wizard
          </button>
          <button
            className={`nav-item ${tab === "reports" ? "active" : ""}`}
            onClick={() => switchTab("reports")}
          >
            <NavIcon name="graph" /> Architecture & Handoff
          </button>
          <button
            className={`nav-item ${tab === "remote" ? "active" : ""}`}
            onClick={() => switchTab("remote")}
          >
            <NavIcon name="shield" /> Remote SSH &amp; Safety
          </button>
          <button
            className={`nav-item ${tab === "settings" ? "active" : ""}`}
            onClick={() => switchTab("settings")}
          >
            <NavIcon name="settings" /> Tunnels & Settings
          </button>
        </nav>

        <div className="sidebar-footer">
          <div>
            <span className={`connection-dot ${state.trusted ? "good" : "neutral"}`} />
            {project ? (state.trusted ? "Trusted Session" : "Untrusted Session") : "No project"}
          </div>
          <small>LaraDeck v1.0.0 Â· Mission Control</small>
        </div>
      </aside>

      <main className="main-content" inert={closeCount !== null}>
        {/* Header bar */}
        <header className="topbar">
          <div className="topbar-left">
            <div className="project-title-group">
              <h2>{project ? project.name : "LaraDeck"}</h2>
              {project && (
                <div className="project-badges">
                  <a
                    href="#"
                    className="badge-pill badge-link"
                    title="Open in default browser"
                    onClick={(e) => {
                      e.preventDefault();
                      act("Open site", () => OpenProject(project.id));
                    }}
                  >
                    <span>ðŸŒ</span> {project.url} <small>â†—</small>
                  </a>
                  <button
                    className={`badge-pill badge-toggle ${project.secured ? "secured" : "unsecured"}`}
                    title="Toggle Herd SSL certificate"
                    onClick={() => act("Toggle SSL", () => SetSecureSite(project.id, !project.secured))}
                  >
                    <span>{project.secured ? "ðŸ”’" : "ðŸ”“"}</span>
                    {project.secured ? "HTTPS Secured" : "HTTP Only (Click to secure)"}
                  </button>
                  <span className="badge-pill badge-php">
                    PHP {project.php}
                  </span>
                </div>
              )}
            </div>
          </div>

          <div className="topbar-right">
            {project && (
              <button
                className="btn-topbar-action"
                title="Launch PowerShell with Herd PHP in PATH"
                onClick={() => LaunchProjectTerminal(project.id, "powershell")}
              >
                <span>ðŸ’»</span> Terminal
              </button>
            )}
            <button
              className="btn-topbar-action"
              title="Open LaraDeck Web Dashboard in Browser"
              onClick={() => act("Open dashboard in browser", () => OpenBrowserDashboard())}
            >
              <span>ðŸŒ</span> Web Dashboard
            </button>
            <button
              className="btn-palette-trigger"
              title="Command Palette (Ctrl+K)"
              onClick={() => setPaletteOpen(true)}
            >
              <span>ðŸ”</span> <span>Commands</span> <kbd>Ctrl+K</kbd>
            </button>
            {project && !state.trusted && (
              <button
                className="btn-trust-topbar"
                onClick={() => act("Trust project", () => TrustProject(project.id))}
              >
                ðŸ›¡ï¸ Trust Project
              </button>
            )}
            {busy && <span className="busy-badge">â³ {busy}...</span>}
            {notice && <span className="notice-badge">{notice}</span>}
          </div>
        </header>

        {/* Prominent Trust Banner when untrusted */}
        {project && !state.trusted && (
          <div className="trust-banner">
            <div className="trust-banner-info">
              <span className="trust-icon">ðŸ›¡ï¸</span>
              <div>
                <strong>Session Untrusted for {project.name}</strong>
                <p>Telemetry reading, process execution, and queue commands require explicit session trust to prevent unauthorized background execution.</p>
              </div>
            </div>
            <button
              className="btn-trust-banner"
              onClick={() => act("Trust project", () => TrustProject(project.id))}
            >
              Trust {project.name} for this Session
            </button>
          </div>
        )}

        <ErrorBoundary key={tab}>
        {tab === "dashboard" && <div className="tab-pane"><DashboardPanel state={state} switchTab={switchTab} act={act} heartbeat={heartbeatReport} flags={flagsReport} /></div>}
        {tab === "companion" && <div className="tab-pane"><CompanionPanel key={state.selected} state={state} reload={load}/></div>}
        {/* TAB 1: REQUESTS & TELEMETRY */}
        {tab === "requests" && (
          <div className="tab-pane">
            <div className="sub-segmented-nav" style={{ marginBottom: 14 }}>
              <button
                className={`sub-tab-pill ${telemetrySubTab === "requests" ? "active" : ""}`}
                onClick={() => setTelemetrySubTab("requests")}
              >
                <span>📡</span>
                <span>HTTP Requests & SQL</span>
                {requests.length > 0 && <span className="count-badge badge-total">{requests.length}</span>}
              </button>
              <button
                className={`sub-tab-pill ${telemetrySubTab === "dumps" ? "active" : ""}`}
                onClick={() => {
                  setTelemetrySubTab("dumps");
                  loadInAppDumps();
                }}
              >
                <span>🔍</span>
                <span>In-App Dumps</span>
                {inAppDumps.length > 0 && <span className="count-badge badge-good">{inAppDumps.length}</span>}
              </button>
              <button
                className={`sub-tab-pill ${telemetrySubTab === "activity" ? "active" : ""}`}
                onClick={() => {
                  setTelemetrySubTab("activity");
                  if (request && project?.id) {
                    GetActivityTrace(project.id, request.id)
                      .then(setActiveTrace)
                      .catch(() => {});
                  }
                }}
              >
                <span>⚡</span>
                <span>Activity Flamegraph</span>
              </button>
              <button
                className={`sub-tab-pill ${telemetrySubTab === "xdebug" ? "active" : ""}`}
                onClick={() => {
                  setTelemetrySubTab("xdebug");
                  loadXdebug();
                }}
              >
                <span>🐞</span>
                <span>Xdebug 3 Helper</span>
                {xdebugStatus?.installed && <span className="count-badge badge-good">Active</span>}
              </button>
            </div>

            {telemetrySubTab === "dumps" && (
              <DumpsPanel
                projectId={project?.id}
                dumps={inAppDumps}
                onClear={handleClearInAppDumps}
                onRefresh={loadInAppDumps}
                helperSnippet={dumpHelperSnippet}
              />
            )}

            {telemetrySubTab === "activity" && (
              <ActivityPanel
                trace={activeTrace}
                onExportHTML={() => {
                  if (request && project?.id) {
                    ExportTraceHTML(project.id, request.id).then((html) => {
                      const blob = new Blob([html], { type: "text/html" });
                      const url = URL.createObjectURL(blob);
                      const a = document.createElement("a");
                      a.href = url;
                      a.download = `trace-${request.id}.html`;
                      a.click();
                    });
                  }
                }}
                onExportJSON={() => {
                  if (activeTrace) {
                    const blob = new Blob([JSON.stringify(activeTrace, null, 2)], { type: "application/json" });
                    const url = URL.createObjectURL(blob);
                    const a = document.createElement("a");
                    a.href = url;
                    a.download = `trace-${activeTrace.id}.json`;
                    a.click();
                  }
                }}
              />
            )}

            {telemetrySubTab === "xdebug" && (
              <div className="workbench-card">
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
                  <h3 style={{ margin: 0 }}>Xdebug 3 Step Debugging Integration</h3>
                  <span className={`workbench-badge ${xdebugStatus?.installed ? "badge-good" : "badge-warn"}`}>
                    {xdebugStatus?.configStatus || "Checking Xdebug status..."}
                  </span>
                </div>
                <p style={{ fontSize: "12px", color: "var(--text-muted)", lineHeight: 1.6, maxWidth: 650 }}>
                  LaraDeck configures your local PHP runtime to route Xdebug step-debugging sessions into VS Code or PhpStorm without complex manual IDE keys.
                </p>

                <div style={{ display: "flex", gap: 10, marginTop: 16, flexWrap: "wrap" }}>
                  {project && (
                    <button
                      className="btn primary"
                      onClick={() => {
                        act("Launch Xdebug session", () => OpenProject(project.id));
                      }}
                    >
                      🚀 Launch Browser Session with XDEBUG_SESSION=LARADECK
                    </button>
                  )}
                  {project && (
                    <button
                      className="btn secondary"
                      onClick={() => {
                        GenerateVSCodeConfig(project.id)
                          .then(() => setNotice("Generated .vscode/launch.json configured for port 9003!"))
                          .catch((e) => setNotice(`Failed: ${e}`));
                      }}
                    >
                      🛠️ Generate .vscode/launch.json
                    </button>
                  )}
                </div>
              </div>
            )}

            {telemetrySubTab === "requests" && (
              <div className="workspace">
            <div className="toolbar" style={{ flexWrap: "wrap", gap: "10px" }}>
              <div className="search-wrap" style={{ flex: 1, minWidth: "260px" }}>
                <span className="search-icon">ðŸ”</span>
                <input
                  type="search"
                  placeholder="Filter requests by URL, route, SQL..."
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className="search-input"
                />
              </div>

              {/* Status & Velocity Filter Pills */}
              <div style={{ display: "flex", gap: "6px", alignItems: "center", flexWrap: "wrap" }}>
                {[
                  { id: "all", label: `All (${requests.length})` },
                  { id: "2xx", label: "2xx Success" },
                  { id: "4xx", label: "4xx Client" },
                  { id: "5xx", label: "5xx Server" },
                  { id: "slow", label: "Slow (>100ms)" },
                ].map((f) => (
                  <button
                    key={f.id}
                    className={`btn-tiny ${telemetryFilter === f.id ? "primary active" : ""}`}
                    style={{
                      borderRadius: "12px",
                      padding: "3px 10px",
                      fontSize: "11px",
                      background: telemetryFilter === f.id ? "rgba(99, 102, 241, 0.25)" : "rgba(255, 255, 255, 0.04)",
                      borderColor: telemetryFilter === f.id ? "#6366f1" : "rgba(255, 255, 255, 0.1)",
                      color: telemetryFilter === f.id ? "#fff" : "var(--text-muted)",
                    }}
                    onClick={() => setTelemetryFilter(f.id as any)}
                  >
                    {f.label}
                  </button>
                ))}
              </div>

              <div className="toolbar-actions">
                <button
                  className="btn-toolbar primary"
                  onClick={() => act("Refresh captures", RefreshCaptures)}
                  disabled={!state.trusted}
                  title={!state.trusted ? "Trust project first" : "Read latest Clockwork requests"}
                >
                  â†» {clockworkNeedsSetup ? "Recheck Clockwork" : "Refresh Captures"}
                </button>
                <button
                  className={`btn-toolbar ${state.capture.watching ? "active" : ""}`}
                  onClick={() => act("Toggle watch", () => SetWatching(!state.capture.watching))}
                  disabled={!state.trusted || clockworkNeedsSetup}
                  title={clockworkNeedsSetup ? "Install project dependencies and recheck Clockwork first" : !state.trusted ? "Trust project first" : "Watch storage/clockwork for new requests"}
                >
                  {state.capture.watching ? "â¹ Stop Watching" : "ðŸ‘ Watch Files"}
                </button>
              </div>
            </div>

            {state.capture.status && state.capture.status !== "ready" && requests.length > 0 && <p role="status">{state.capture.message}</p>}
            <div className="inspector">
              <div className="request-list">
                {visible.map((r) => (
                  <button
                    key={r.id}
                    className={`request-row ${r.id === request?.id ? "selected" : ""}`}
                    onClick={() => setSelectedID(r.id)}
                    style={{ display: "grid", gridTemplateColumns: "auto 1fr auto auto", alignItems: "center", gap: "8px" }}
                  >
                    <span className={`method m-${r.method.toLowerCase()}`}>{r.method}</span>
                    <span className="uri" style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {r.uri}
                    </span>
                    <span className={`status s-${String(r.status)[0]}`}>{r.status}</span>
                    <span className="dur" style={{ fontVariantNumeric: "tabular-nums" }}>{ms(r.duration)}</span>
                  </button>
                ))}
                {visible.length === 0 && (
                  <div className="empty-state-card">
                    <div className="empty-state-icon">âš¡</div>
                    <h3>{clockworkNeedsSetup ? "Set up Clockwork for this project" : state.capture.status === "endpoint_unavailable" ? "Clockwork endpoint unavailable" : "No Request Captures Yet"}</h3>
                    <p>{state.capture.message || "Visit a route in your project to start capturing real-time HTTP requests, SQL queries, memory metrics, and exceptions."}</p>
                    {clockworkNeedsSetup && <button className="btn-empty primary" onClick={() => switchTab("settings")}>Install from Settings</button>}
                    {state.capture.setupCommand && <pre style={{ userSelect: "text", whiteSpace: "pre-wrap" }}><code>{state.capture.setupCommand}</code></pre>}
                    {project && (
                      <div className="empty-state-actions">
                        <button
                          className="btn-empty primary"
                          onClick={() => act("Open site", () => OpenProject(project.id))}
                        >
                          Open {project.name} in Browser â†—
                        </button>
                        <button
                          className="btn-empty secondary"
                          onClick={() => act("Refresh captures", RefreshCaptures)}
                          disabled={!state.trusted}
                        >
                          â†» {clockworkNeedsSetup ? "Recheck Clockwork" : "Refresh Captures"}
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>

              {request && (
                <div className="request-detail">
                  <div className="detail-header" style={{ marginBottom: 14 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: "12px", marginBottom: "14px", flexWrap: "wrap" }}>
                      <h3 style={{ margin: 0, wordBreak: "break-all", display: "flex", alignItems: "center", gap: "8px", flex: 1, minWidth: "240px" }}>
                        <span className={`method m-${request.method.toLowerCase()}`}>{request.method}</span>
                        <span style={{ color: "#fff" }}>{request.uri}</span>
                      </h3>
                      <div style={{ display: "flex", gap: "6px", alignItems: "center", flexShrink: 0 }}>
                        <button
                          className="btn-tiny primary"
                          title="Copy full request URL to clipboard"
                          onClick={() => handleCopyRequest("url")}
                        >
                          {copiedType === "url" ? "âœ“ URL Copied" : "ðŸ“‹ Copy URL"}
                        </button>
                        <button
                          className="btn-tiny"
                          title="Copy request as cURL command"
                          onClick={() => handleCopyRequest("curl")}
                        >
                          {copiedType === "curl" ? "âœ“ cURL Copied" : "cURL"}
                        </button>
                        <button
                          className="btn-tiny"
                          title="Copy full request payload as JSON"
                          onClick={() => handleCopyRequest("json")}
                        >
                          {copiedType === "json" ? "âœ“ JSON Copied" : "JSON"}
                        </button>
                      </div>
                    </div>

                    <div className="metrics" style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(110px, 1fr))", gap: 8 }}>
                      <div style={{ background: "rgba(255,255,255,0.03)", padding: "8px 12px", borderRadius: 6, border: "1px solid rgba(255,255,255,0.06)" }}>
                        <span style={{ fontSize: "11px", color: "var(--text-dim)", display: "block" }}>HTTP Status</span>
                        <strong className={`status s-${String(request.status)[0]}`} style={{ fontSize: "16px" }}>{request.status}</strong>
                      </div>
                      <div style={{ background: "rgba(255,255,255,0.03)", padding: "8px 12px", borderRadius: 6, border: "1px solid rgba(255,255,255,0.06)" }}>
                        <span style={{ fontSize: "11px", color: "var(--text-dim)", display: "block" }}>Total Time</span>
                        <strong style={{ fontSize: "16px", color: request.duration > 200 ? "#f87171" : request.duration > 50 ? "#fbbf24" : "#34d399" }}>
                          {ms(request.duration)}
                        </strong>
                      </div>
                      <div style={{ background: "rgba(255,255,255,0.03)", padding: "8px 12px", borderRadius: 6, border: "1px solid rgba(255,255,255,0.06)" }}>
                        <span style={{ fontSize: "11px", color: "var(--text-dim)", display: "block" }}>DB Time</span>
                        <strong style={{ fontSize: "16px", color: "#38bdf8" }}>{ms(request.databaseDuration)}</strong>
                      </div>
                      <div style={{ background: "rgba(255,255,255,0.03)", padding: "8px 12px", borderRadius: 6, border: "1px solid rgba(255,255,255,0.06)" }}>
                        <span style={{ fontSize: "11px", color: "var(--text-dim)", display: "block" }}>SQL Queries</span>
                        <strong style={{ fontSize: "16px", color: "#c7d2fe" }}>{request.queryCount}</strong>
                      </div>
                      {request.exceptions > 0 && (
                        <div style={{ background: "rgba(239,68,68,0.1)", padding: "8px 12px", borderRadius: 6, border: "1px solid rgba(239,68,68,0.3)" }}>
                          <span style={{ fontSize: "11px", color: "#fca5a5", display: "block" }}>Exceptions</span>
                          <strong className="err" style={{ fontSize: "16px" }}>{request.exceptions}</strong>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Sub-tabs for Request Details */}
                  <div className="sub-segmented-nav" style={{ marginBottom: 12 }}>
                    <button
                      className={`sub-tab-pill ${telemetryDetailTab === "overview" ? "active" : ""}`}
                      onClick={() => setTelemetryDetailTab("overview")}
                    >
                      <span>âš¡</span> Overview &amp; Headers
                    </button>
                    <button
                      className={`sub-tab-pill ${telemetryDetailTab === "queries" ? "active" : ""}`}
                      onClick={() => setTelemetryDetailTab("queries")}
                    >
                      <span>ðŸ—„ï¸</span> Queries ({request.queries?.length || 0})
                    </button>
                    <button
                      className={`sub-tab-pill ${telemetryDetailTab === "radar" ? "active" : ""}`}
                      onClick={() => {
                        setTelemetryDetailTab("radar");
                        if (request.id) loadTelemetryAnalysis(request.id);
                      }}
                    >
                      <span>ðŸš¨</span> N+1 Radar
                    </button>
                    <button
                      className={`sub-tab-pill ${telemetryDetailTab === "waterfall" ? "active" : ""}`}
                      onClick={() => {
                        setTelemetryDetailTab("waterfall");
                        if (request.id) loadTelemetryAnalysis(request.id);
                      }}
                    >
                      <span>â±ï¸</span> Waterfall Flamegraph
                    </button>
                  </div>

                  {telemetryDetailTab === "overview" && (
                    <div className="workbench-card" style={{ padding: "14px" }}>
                      <h4 style={{ margin: "0 0 10px 0", color: "#fff" }}>Request Metadata</h4>
                      <div style={{ display: "grid", gridTemplateColumns: "140px 1fr", gap: "8px", fontSize: "12.5px" }}>
                        <span style={{ color: "var(--text-dim)" }}>Request ID:</span>
                        <code>{request.id}</code>
                        <span style={{ color: "var(--text-dim)" }}>Controller / Action:</span>
                        <code>{(request as any).controller || "Closure / N/A"}</code>
                        <span style={{ color: "var(--text-dim)" }}>Database Ratio:</span>
                        <span>{request.duration > 0 ? ((request.databaseDuration / request.duration) * 100).toFixed(1) : 0}% of response time</span>
                      </div>
                    </div>
                  )}

                  {telemetryDetailTab === "queries" && (
                    <div className="queries-list">
                      {request.queries?.map((q, idx) => (
                        <div key={idx} className={`query-item ${q.repeated ? "repeated" : ""}`} style={{ borderRadius: 8, padding: 12, marginBottom: 8 }}>
                          {q.repeated && <span className="tag warn" style={{ marginBottom: 6, display: "inline-block" }}>âš ï¸ N+1 Query Candidate</span>}
                          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: "10px" }}>
                            <code style={{ fontSize: "12px", lineHeight: "1.4" }}>{q.sql}</code>
                            <button
                              className="btn-tiny"
                              style={{ padding: "2px 8px", fontSize: "11px", flexShrink: 0 }}
                              title="Copy SQL query to clipboard"
                              onClick={() => handleCopySql(q.sql, idx)}
                            >
                              {copiedSqlIdx === idx ? "âœ“ Copied" : "Copy SQL"}
                            </button>
                          </div>
                          <div className="query-meta" style={{ marginTop: 8, display: "flex", gap: 12, fontSize: "11.5px" }}>
                            <span style={{ color: q.duration > 50 ? "#f87171" : "#34d399", fontWeight: 600 }}>{ms(q.duration)}</span>
                            <span style={{ color: "var(--text-dim)" }}>Connection: {q.connection || "default"}</span>
                          </div>
                        </div>
                      ))}
                      {(!request.queries || request.queries.length === 0) && (
                        <p className="empty-text">No database queries recorded for this request.</p>
                      )}
                    </div>
                  )}

                  {telemetryDetailTab === "radar" && (
                    <NPlusOneRadar
                      projectId={project?.id || ""}
                      report={nPlusOneReport}
                      loading={loadingTelemetryAnalysis}
                    />
                  )}

                  {telemetryDetailTab === "waterfall" && (
                    <WaterfallTimeline
                      flamegraph={waterfallData}
                      loading={loadingTelemetryAnalysis}
                    />
                  )}
                </div>
              )}
            </div>
          </div>
            )}
          </div>
        )}

        {/* TAB 2: LOGS & ERRORS */}
        {tab === "logs" && (
          <div className="tab-pane">
            {/* Top Toolbar and Sub-tabs */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "14px", flexWrap: "wrap", gap: "10px" }}>
              <div className="sub-segmented-nav" style={{ margin: 0 }}>
                <button
                  className={`sub-tab-pill ${logsSubTab === "incidents" ? "active" : ""}`}
                  onClick={() => setLogsSubTab("incidents")}
                >
                  ðŸª£ Smart Log Pail &amp; Fingerprints ({logPailReport?.incidents?.length || 0})
                </button>
                <button
                  className={`sub-tab-pill ${logsSubTab === "raw" ? "active" : ""}`}
                  onClick={() => setLogsSubTab("raw")}
                >
                  ðŸ“œ Monolog Stream ({rawLogs.length})
                </button>
                <button
                  className={`sub-tab-pill ${logsSubTab === "dumps" ? "active" : ""}`}
                  onClick={() => setLogsSubTab("dumps")}
                >
                  â±ï¸ Retained Captures ({history.length})
                </button>
              </div>

              <div style={{ display: "flex", gap: "8px" }}>
                <button className="btn-tiny primary" onClick={loadLogs}>
                  â†» Refresh Log Pail
                </button>
              </div>
            </div>

            {/* SUB-TAB 1: SMART LOG PAIL */}
            {logsSubTab === "incidents" && (
              <>
                <div className="arch-card-grid" style={{ marginBottom: "16px" }}>
                  <div className="arch-metric-card">
                    <h4>Total Log Entries</h4>
                    <div className="arch-metric-value" style={{ color: "#38bdf8" }}>
                      {logPailReport?.totalEntries || 0}
                    </div>
                  </div>
                  <div className="arch-metric-card">
                    <h4>Unique Incidents</h4>
                    <div className="arch-metric-value" style={{ color: (logPailReport?.incidents?.length || 0) > 0 ? "#f87171" : "#34d399" }}>
                      {logPailReport?.incidents?.length || 0}
                    </div>
                  </div>
                  <div className="arch-metric-card">
                    <h4>Errors / Critical</h4>
                    <div className="arch-metric-value" style={{ color: "#f87171" }}>
                      {logPailReport?.errorCount || 0}
                    </div>
                  </div>
                  <div className="arch-metric-card">
                    <h4>Warnings</h4>
                    <div className="arch-metric-value" style={{ color: "#fbbf24" }}>
                      {logPailReport?.warnCount || 0}
                    </div>
                  </div>
                </div>

                <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
                  {(logPailReport?.incidents || []).map((inc) => (
                    <div
                      key={inc.fingerprint}
                      style={{
                        background: "rgba(15, 23, 42, 0.75)",
                        border: "1px solid rgba(239, 68, 68, 0.3)",
                        borderRadius: "12px",
                        padding: "16px",
                        boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
                      }}
                    >
                      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: "12px", flexWrap: "wrap", marginBottom: "8px" }}>
                        <div style={{ display: "flex", alignItems: "center", gap: "8px", flexWrap: "wrap" }}>
                          <span className="workbench-badge badge-error">
                            {inc.exception || "Exception"}
                          </span>
                          <span
                            style={{
                              fontSize: "11px",
                              padding: "2px 8px",
                              borderRadius: "12px",
                              background: "rgba(239, 68, 68, 0.2)",
                              color: "#fca5a5",
                              fontWeight: 700,
                            }}
                          >
                            Ã— {inc.count} occurrence{inc.count === 1 ? "" : "s"}
                          </span>
                          <span style={{ fontSize: "11px", color: "var(--text-muted)" }}>
                            Fingerprint: <code>{inc.fingerprint}</code>
                          </span>
                        </div>

                        <button
                          type="button"
                          className="btn-tiny"
                          style={{
                            background: copiedAIPromptFp === inc.fingerprint ? "#059669" : "linear-gradient(135deg, #6366f1 0%, #a855f7 100%)",
                            color: "#fff",
                            border: "none",
                            padding: "4px 12px",
                            fontWeight: 600,
                          }}
                          onClick={() => {
                            navigator.clipboard.writeText(inc.aiPrompt);
                            setCopiedAIPromptFp(inc.fingerprint);
                            setTimeout(() => setCopiedAIPromptFp(null), 2000);
                          }}
                          title="Copies an AI-optimized debugging prompt with normalized stack frames"
                        >
                          {copiedAIPromptFp === inc.fingerprint ? "âœ“ Copied AI Prompt" : "ðŸ¤– 1-Click AI Prompt"}
                        </button>
                      </div>

                      <div style={{ fontSize: "13px", color: "#f8fafc", fontWeight: 600, marginBottom: "8px" }}>
                        {inc.message}
                      </div>

                      {inc.location && (
                        <div style={{ fontSize: "11.5px", color: "#94a3b8", fontFamily: "monospace", marginBottom: "10px" }}>
                          ðŸ“ {inc.location}
                        </div>
                      )}

                      {inc.sampleEntry?.appTrace && inc.sampleEntry.appTrace.length > 0 && (
                        <div
                          style={{
                            background: "#090d16",
                            borderRadius: "8px",
                            padding: "10px 12px",
                            border: "1px solid rgba(255, 255, 255, 0.06)",
                            maxHeight: "160px",
                            overflowY: "auto",
                          }}
                        >
                          <small style={{ color: "var(--text-dim)", display: "block", marginBottom: "4px", fontWeight: 700 }}>
                            APPLICATION CALL TRACE ({inc.sampleEntry.appTrace.length} frames):
                          </small>
                          {inc.sampleEntry.appTrace.map((frame, fIdx) => (
                            <div key={fIdx} style={{ fontSize: "11px", fontFamily: "monospace", color: "#c7d2fe", lineHeight: "1.5" }}>
                              {frame}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  ))}

                  {(!logPailReport?.incidents || logPailReport.incidents.length === 0) && (
                    <div className="workbench-card" style={{ textAlign: "center", padding: "40px" }}>
                      <div style={{ fontSize: "32px", marginBottom: "8px" }}>âœ¨</div>
                      <h4 style={{ margin: 0, color: "#34d399" }}>No Active Error Incidents in Log Pail</h4>
                      <p style={{ color: "var(--text-muted)", fontSize: "12px", marginTop: "4px" }}>
                        Laravel log pail is clean. Unhandled runtime exceptions and Monolog critical errors will appear grouped here.
                      </p>
                    </div>
                  )}
                </div>
              </>
            )}

            {/* SUB-TAB 2: RAW MONOLOG STREAM */}
            {logsSubTab === "raw" && (
              <div className="workbench-card">
                <h3>Recent Raw Laravel Log Entries ({rawLogs.length})</h3>
                <div className="mail-list" style={{ maxHeight: "450px" }}>
                  {rawLogs.map((l) => (
                    <div key={l.id} className="mail-item">
                      <div>
                        <span className={`workbench-badge ${l.level === "ERROR" ? "badge-error" : "badge-info"}`}>
                          {l.level}
                        </span>
                        <strong style={{ marginLeft: "8px" }}>{l.message}</strong>
                      </div>
                      {l.file && <small><code>{l.file}:{l.line}</code></small>}
                    </div>
                  ))}
                  {rawLogs.length === 0 && (
                    <p className="empty-text">No recent log entries found in storage/logs/laravel.log.</p>
                  )}
                </div>
              </div>
            )}

            {/* SUB-TAB 3: DUMPS & HISTORY */}
            {logsSubTab === "dumps" && (
              <div className="workbench-card">
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px" }}>
                  <h3 style={{ margin: 0 }}>Dumps &amp; Diagnostic History</h3>
                  <button
                    className="btn-tiny"
                    onClick={() => act("Clear history", async () => { await ClearHistory(project!.id); setHistory([]); setDumpsList([]); setErrorsList([]); })}
                  >
                    Clear History
                  </button>
                </div>
                <p style={{ fontSize: "12px", color: "var(--text-muted)" }}>{history.length} retained captures shown.</p>
                <table className="workbench-table">
                  <thead><tr><th>Time</th><th>Request</th><th>Status</th><th>Duration</th></tr></thead>
                  <tbody>{history.map((record) => <tr key={record.id}><td>{record.timestamp}</td><td>{record.uri}</td><td>{record.status}</td><td>{ms(record.duration)}</td></tr>)}</tbody>
                </table>
                {(dumpsList || []).map((dump) => <details key={dump.id} style={{ marginTop: "8px" }}><summary style={{ cursor: "pointer", color: "#c7d2fe" }}>{dump.file}:{dump.line} Â· {dump.createdAt}</summary><pre className="mcp-code-block">{dump.payload}</pre></details>)}
                {dumpsList.length === 0 && <p className="empty-text">No stored dumps recorded.</p>}
              </div>
            )}
          </div>
        )}

        {/* TAB: TINKER REPL */}
        {tab === "tinker" && (
          <div className="tab-pane">
            {project ? (
              <TinkerPanel key={project.id} projectId={project.id} trusted={state.trusted} />
            ) : (
              <div className="empty-state-card">
                <div className="empty-state-icon">âš¡</div>
                <h3>Select a Project</h3>
                <p>Select a project on the left to start interactive Tinker evaluation.</p>
              </div>
            )}
          </div>
        )}

        {/* TAB: DATABASE BROWSER */}
        {tab === "database" && (
          <div className="tab-pane">
            {project ? (
              <DatabaseBrowser key={project.id} projectId={project.id} trusted={state.trusted} />
            ) : (
              <div className="empty-state-card">
                <div className="empty-state-icon">ðŸ—„ï¸</div>
                <h3>Select a Project</h3>
                <p>Select a project on the left to explore Eloquent models and database records.</p>
              </div>
            )}
          </div>
        )}

        {/* TAB 3: ENVIRONMENT & PHP */}
        {tab === "environment" && (
          <div className="tab-pane">
            <div className="sub-segmented-nav">
              <button
                className={`sub-tab-pill ${envSubTab === "runtimes" ? "active" : ""}`}
                onClick={() => setEnvSubTab("runtimes")}
              >
                <span>ðŸ“¦</span>
                <span>PHP Versions &amp; Composer</span>
              </button>
              <button
                className={`sub-tab-pill ${envSubTab === "env" ? "active" : ""}`}
                onClick={() => setEnvSubTab("env")}
              >
                <span>ðŸ›¡ï¸</span>
                <span>.env Editor &amp; Drift</span>
                {drift?.missingKeys && drift.missingKeys.length > 0 && (
                  <span className="count-badge badge-warn">{drift.missingKeys.length}</span>
                )}
              </button>
              <button
                className={`sub-tab-pill ${envSubTab === "php" ? "active" : ""}`}
                onClick={() => setEnvSubTab("php")}
              >
                <span>ðŸ˜</span>
                <span>PHP CLI &amp; INI Directives</span>
              </button>
            </div>

            {envSubTab === "runtimes" && (
              <PhpRuntimePanel projectId={project?.id} projectName={project?.name} />
            )}

            {envSubTab === "env" && (
              !project ? (
                <div className="empty-project-prompt">
                  <div style={{ fontSize: "40px", marginBottom: "12px" }}>âš™ï¸</div>
                  <h3 style={{ color: "#fff", margin: 0 }}>Select a Project to Inspect Environment</h3>
                  <p style={{ color: "var(--text-muted)", fontSize: "13px", marginTop: "6px", maxWidth: 500, margin: "6px auto 0" }}>
                    Choose a project below to safely edit <code>.env</code> variables and check configuration drift against <code>.env.example</code>.
                  </p>
                  <div className="empty-project-grid">
                    {state.projects.map((p) => (
                      <button
                        key={p.id}
                        className="empty-project-tile"
                        onClick={() => select(p.id)}
                      >
                        <strong style={{ color: "#fff", display: "block", marginBottom: 4 }}>{p.name}</strong>
                        <small style={{ color: "var(--text-dim)" }}>PHP {p.php} Â· {p.secured ? "ðŸ”’ HTTPS" : "HTTP"}</small>
                        <div style={{ fontSize: "11px", color: "#6366f1", marginTop: 6 }}>Open Environment â†’</div>
                      </button>
                    ))}
                    {state.projects.length === 0 && (
                      <p style={{ gridColumn: "1 / -1", color: "var(--text-dim)", padding: 20 }}>
                        No Herd sites discovered. Refresh discovery from the left sidebar.
                      </p>
                    )}
                  </div>
                </div>
              ) : (
                <>
                    {drift && (
                      <div className="workbench-card" style={{ marginBottom: 14 }}>
                        <h4 style={{ margin: "0 0 10px 0", color: "#fff" }}>Environment Drift Comparison (.env vs .env.example)</h4>
                        <div style={{ display: "flex", gap: "12px", flexWrap: "wrap" }}>
                          <div>
                            <span className="workbench-badge badge-good">{drift.matching} Matching Keys</span>
                          </div>
                          {drift.missingKeys?.length > 0 && (
                            <div>
                              <span className="workbench-badge badge-warn">
                                {drift.missingKeys.length} Missing from .env
                              </span>
                              <small style={{ marginLeft: "8px", color: "var(--text-dim)" }}>({drift.missingKeys.join(", ")})</small>
                            </div>
                          )}
                          {drift.extraKeys?.length > 0 && (
                            <div>
                              <span className="workbench-badge badge-info">{drift.extraKeys.length} Extra Keys</span>
                            </div>
                          )}
                        </div>
                      </div>
                    )}

                    {envData ? (
                      <div className="workbench-card">
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14, flexWrap: "wrap", gap: 10 }}>
                          <div>
                            <h3 style={{ margin: 0 }}>
                              <span>Safe .env Editor ({envData.entries.filter((e) => e.type === "keyvalue").length} variables)</span>
                            </h3>
                            <small style={{ color: "var(--text-dim)" }}>Protected with AST multiline parsing and atomic backups</small>
                          </div>
                          <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                            <input
                              type="search"
                              placeholder="Search variables..."
                              value={envFilterQuery}
                              onChange={(e) => setEnvFilterQuery(e.target.value)}
                              style={{ padding: "5px 10px", fontSize: "12px", width: "180px" }}
                            />
                            <button
                              className="btn-tiny"
                              onClick={() => setShowSecrets(!showSecrets)}
                              title="Toggle masking for sensitive credentials"
                            >
                              {showSecrets ? "ðŸ™ˆ Mask Secrets" : "ðŸ‘ï¸ Reveal Secrets"}
                            </button>
                            <button
                              className="btn-action primary"
                              onClick={() =>
                                act("Save .env", async () => {
                                  await SaveEnv(project!.id, envData, envData.fingerprint);
                                  setEnvData(await GetEnv(project!.id));
                                })
                              }
                            >
                              ðŸ’¾ Save .env (Safe Write)
                            </button>
                          </div>
                        </div>

                        <div style={{ maxHeight: "480px", overflowY: "auto", border: "1px solid rgba(255,255,255,0.06)", borderRadius: 8, padding: 8 }}>
                          {envData.entries
                            .map((entry, idx) => ({ entry, idx }))
                            .filter(({ entry }) => {
                              if (entry.type !== "keyvalue") return false;
                              if (!envFilterQuery) return true;
                              return entry.key.toLowerCase().includes(envFilterQuery.toLowerCase());
                            })
                            .map(({ entry, idx }) => (
                              <div key={idx} className="env-row" style={{ display: "grid", gridTemplateColumns: "220px 1fr auto", gap: 10, alignItems: "center", padding: "6px 8px" }}>
                                <strong style={{ color: "#fff", fontSize: "12px", overflow: "hidden", textOverflow: "ellipsis" }}>{entry.key}</strong>
                                <input
                                  type={entry.isSensitive && !showSecrets ? "password" : "text"}
                                  value={entry.value}
                                  onChange={(e) => {
                                    const newEntries = [...envData.entries];
                                    newEntries[idx].value = e.target.value;
                                    setEnvData(new environment.EnvFile({ ...envData, entries: newEntries }));
                                  }}
                                  style={{ padding: "6px 10px", fontSize: "12px" }}
                                />
                                {entry.isSensitive ? (
                                  <span className="tag info" style={{ fontSize: "10px" }}>Secret</span>
                                ) : <span style={{ width: 40 }} />}
                              </div>
                            ))}
                        </div>
                      </div>
                    ) : (
                      <div className="workbench-card" style={{ textAlign: "center", padding: 32 }}>
                        <p style={{ color: "var(--text-dim)" }}>Loading environment file...</p>
                        <button className="btn-toolbar primary" onClick={loadEnvironment} style={{ marginTop: 10 }}>Load .env</button>
                      </div>
                    )}
                  </>
                )
              )}

            {envSubTab === "php" && (
              !project ? (
                <div className="empty-project-prompt">
                  <div style={{ fontSize: "40px", marginBottom: "12px" }}>ðŸ˜</div>
                  <h3 style={{ color: "#fff", margin: 0 }}>Select a Project to Inspect PHP CLI &amp; INI Directives</h3>
                  <p style={{ color: "var(--text-muted)", fontSize: "13px", marginTop: "6px", maxWidth: 500, margin: "6px auto 0" }}>
                    Choose a project below to inspect its active CLI PHP binary, loaded INI file, and modify runtime directives.
                  </p>
                  <div className="empty-project-grid">
                    {state.projects.map((p) => (
                      <button
                        key={p.id}
                        className="empty-project-tile"
                        onClick={() => select(p.id)}
                      >
                        <strong style={{ color: "#fff", display: "block", marginBottom: 4 }}>{p.name}</strong>
                        <small style={{ color: "var(--text-dim)" }}>PHP {p.php} Â· {p.secured ? "ðŸ”’ HTTPS" : "HTTP"}</small>
                        <div style={{ fontSize: "11px", color: "#6366f1", marginTop: 6 }}>Open PHP Settings â†’</div>
                      </button>
                    ))}
                    {state.projects.length === 0 && (
                      <p style={{ gridColumn: "1 / -1", color: "var(--text-dim)", padding: 20 }}>
                        No Herd sites discovered. Refresh discovery from the left sidebar.
                      </p>
                    )}
                  </div>
                </div>
              ) : (
                <>
                  {phpInfo ? (
                    <div className="workbench-card">
                      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 14 }}>
                        <div>
                          <h3 style={{ margin: 0 }}>PHP CLI Directives &amp; Runtime Configuration</h3>
                          <p style={{ margin: "4px 0 0", color: "var(--text-dim)", fontSize: "12px" }}>
                            Active CLI INI: <code>{phpInfo.loadedIni || "None"}</code>
                          </p>
                        </div>
                        <span className="badge-pill badge-php" style={{ fontSize: "13px", padding: "4px 10px" }}>
                          PHP {project.php}
                        </span>
                      </div>

                      {/* Presets and custom modifier */}
                      <div style={{ background: "rgba(255,255,255,0.03)", padding: 14, borderRadius: 8, border: "1px solid rgba(255,255,255,0.08)", marginBottom: 16 }}>
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
                          <h4 style={{ margin: 0, color: "#fff" }}>3-Tier INI Directive Management</h4>
                          {liveIniValue && (
                            <span className="badge-pill" style={{ background: "rgba(52, 211, 153, 0.15)", color: "#34d399", border: "1px solid rgba(52, 211, 153, 0.3)" }}>
                              Now (ini_get): <strong>{liveIniValue}</strong>
                            </span>
                          )}
                        </div>
                        <div style={{ display: "flex", gap: "8px", flexWrap: "wrap", alignItems: "center" }}>
                          <select aria-label="PHP directive" value={iniKey} onChange={(event) => { setIniKey(event.target.value); setIniPreview(null); }}>
                            {["memory_limit","max_execution_time","max_input_time","post_max_size","upload_max_filesize","opcache.enable","opcache.enable_cli","display_errors","error_reporting","date.timezone","xdebug.mode"].map((key) => <option key={key}>{key}</option>)}
                          </select>
                          <input aria-label="PHP directive value" value={iniValue} onChange={(event) => { setIniValue(event.target.value); setIniPreview(null); }} style={{ width: 140 }} />
                          <button className="btn-toolbar primary" disabled={!state.trusted || !!busy} onClick={() => act("Preview INI change", async () => setIniPreview(await PreviewIniChange(project!.id,iniKey,iniValue)))}>Preview Global change</button>
                          <button className="btn-toolbar" disabled={!state.trusted || !!busy} onClick={() => act("Save to site .user.ini", async () => {
                            await SetSiteIniDirective(project!.id, iniKey, iniValue);
                            setNotice(`.user.ini updated with ${iniKey} = ${iniValue}`);
                            const updatedLive = await GetLiveIniDirective(project!.id, iniKey);
                            setLiveIniValue(updatedLive || "");
                            setPhpInfo(await GetPHPInfo(project!.id));
                          })}>Save to Site .user.ini</button>
                        </div>

                        {iniPreview && (
                          <div className="workbench-card" style={{ marginTop: 12, background: "rgba(99, 102, 241, 0.1)", borderColor: "rgba(99, 102, 241, 0.3)" }}>
                            <p style={{ margin: "0 0 6px 0", color: "#fff" }}>
                              <strong>Preview:</strong> {iniPreview.key}: <code>{iniPreview.oldValue}</code> â†’ <code style={{ color: "#34d399" }}>{iniPreview.newValue}</code>
                            </p>
                            <p style={{ margin: "0 0 10px 0", fontSize: "11px", color: "var(--text-dim)" }}>Target: <code>{iniPreview.iniPath}</code></p>
                            <button className="btn-toolbar primary" disabled={!state.trusted || !!busy} onClick={() => act("Apply INI change", async () => {
                              await ApplyIniChange(project!.id,iniPreview.key,iniPreview.newValue); setIniPreview(null); setPhpInfo(await GetPHPInfo(project!.id));
                            })}>Back up and apply change</button>
                          </div>
                        )}
                      </div>

                      {/* Key Settings Grid */}
                      <h4 style={{ margin: "0 0 10px 0", color: "#fff" }}>Active Directives</h4>
                      <div className="panel-grid" style={{ marginBottom: 16 }}>
                        {Object.entries(phpInfo.keySettings || {}).map(([k, v]) => (
                          <div key={k} className="workbench-card" style={{ padding: "10px 14px" }}>
                            <small style={{ color: "var(--text-dim)", display: "block" }}>{k}</small>
                            <strong style={{ color: "#fff", fontSize: "14px" }}>{v}</strong>
                          </div>
                        ))}
                      </div>

                      {/* Loaded Extensions */}
                      <h4 style={{ margin: "0 0 10px 0", color: "#fff" }}>Loaded Extensions ({phpInfo.extensions?.length || 0})</h4>
                      <div style={{ display: "flex", flexWrap: "wrap", gap: 6, maxHeight: 180, overflowY: "auto", padding: 8, background: "rgba(0,0,0,0.2)", borderRadius: 6 }}>
                        {phpInfo.extensions?.map((ext) => (
                          <span key={ext} className="badge-pill" style={{ background: "rgba(99, 102, 241, 0.15)", color: "#c7d2fe", border: "1px solid rgba(99, 102, 241, 0.25)" }}>
                            {ext}
                          </span>
                        ))}
                      </div>
                    </div>
                  ) : (
                    <div className="workbench-card" style={{ textAlign: "center", padding: 32 }}>
                      <p style={{ color: "var(--text-dim)" }}>Loading PHP CLI configuration...</p>
                      <button className="btn-toolbar primary" onClick={loadEnvironment} style={{ marginTop: 10 }}>Load PHP Info</button>
                    </div>
                  )}
                </>
              )
            )}
          </div>
        )}

        {/* TAB 4: SERVICES & MAIL */}
        {tab === "services" && (() => {
          const serviceOrder: Record<string, number> = {
            mailpit: 1,
            redis: 2,
            mysql: 3,
            postgres: 4,
            meilisearch: 5,
          };
          const sortedServices = [...state.services].sort((a, b) => {
            return (serviceOrder[a.id] || 99) - (serviceOrder[b.id] || 99) || a.id.localeCompare(b.id);
          });
          const serviceIcons: Record<string, string> = {
            mailpit: "ðŸ“¬",
            redis: "âš¡",
            mysql: "ðŸ¬",
            postgres: "ðŸ˜",
            meilisearch: "ðŸ”",
          };

          return (
          <div className="tab-pane">
            <div className="workbench-card" style={{ marginBottom: 16 }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14 }}>
                <div>
                  <h3 style={{ margin: 0 }}>Native Local Services Lifecycle</h3>
                  <small style={{ color: "var(--text-dim)" }}>
                    Managed local instances running on native Windows ports with process ownership
                  </small>
                </div>
                <button className="btn-toolbar" disabled={!!busyServiceId} onClick={loadServices}>
                  â†» Refresh Ports
                </button>
              </div>

              <table className="workbench-table" style={{ tableLayout: "fixed", width: "100%" }}>
                <thead>
                  <tr>
                    <th style={{ width: "240px" }}>Service Engine</th>
                    <th style={{ width: "180px" }}>Port / Admin UI</th>
                    <th style={{ width: "140px" }}>Status</th>
                    <th>Process Details</th>
                    <th style={{ width: "160px", textAlign: "right" }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {sortedServices.map((srv) => (
                    <tr key={srv.id} className="service-row-stable">
                      <td>
                        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                          <span style={{ fontSize: "20px" }}>{serviceIcons[srv.id] || "âš™ï¸"}</span>
                          <div>
                            <strong style={{ color: "#fff", display: "block" }}>{srv.name}</strong>
                            <code style={{ fontSize: "10.5px", color: "var(--text-dim)" }}>{srv.id}</code>
                          </div>
                        </div>
                      </td>
                      <td>
                        <code>:{srv.port}</code>{" "}
                        {srv.adminPort ? (
                          <span style={{ color: "#38bdf8", fontSize: "11px", marginLeft: 4 }}>
                            (UI :{srv.adminPort})
                          </span>
                        ) : null}
                      </td>
                      <td>
                        <span
                          className={`workbench-badge ${
                            srv.status === "running" ? "badge-good" : srv.status === "external" ? "badge-info" : "badge-warn"
                          }`}
                        >
                          {srv.status === "running" ? "â— Running" : srv.status === "external" ? "â—‹ External" : "â—‹ Stopped"}
                        </span>
                      </td>
                      <td>
                        <span style={{ color: "var(--text-dim)", fontSize: "12px" }}>
                          {srv.isOwned ? `Managed PID ${srv.pid}` : srv.status === "external" ? "Active on port (External daemon)" : "Inactive"}
                        </span>
                      </td>
                      <td style={{ textAlign: "right" }}>
                        <div style={{ display: "inline-flex", gap: "6px", justifyContent: "flex-end", alignItems: "center" }}>
                          {srv.status === "running" && (
                            <button
                              className="btn-tiny danger"
                              disabled={busyServiceId === srv.id}
                              onClick={async () => {
                                setBusyServiceId(srv.id);
                                try {
                                  await StopService(srv.id, false);
                                } catch (e) {
                                  setNotice(String(e));
                                } finally {
                                  setBusyServiceId(null);
                                  await load();
                                }
                              }}
                            >
                              {busyServiceId === srv.id ? "Stopping..." : "Stop"}
                            </button>
                          )}
                          {srv.status === "stopped" && (
                            <button
                              className="btn-tiny primary"
                              disabled={busyServiceId === srv.id}
                              onClick={async () => {
                                setBusyServiceId(srv.id);
                                try {
                                  await StartService(srv.id, "");
                                } catch (e) {
                                  setNotice(String(e));
                                } finally {
                                  setBusyServiceId(null);
                                  await load();
                                }
                              }}
                            >
                              {busyServiceId === srv.id ? "Starting..." : "Start"}
                            </button>
                          )}
                          {srv.id === "mailpit" && srv.status === "running" && (
                            <a
                              href="http://localhost:8025"
                              target="_blank"
                              rel="noreferrer"
                              className="btn-tiny"
                              style={{ color: "#c7d2fe" }}
                              title="Open Mailpit Web UI in browser"
                            >
                              Web UI â†—
                            </a>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                  {sortedServices.length === 0 && (
                    <tr>
                      <td colSpan={5} className="empty-text">No native services detected.</td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {/* Mailpit Viewer */}
            <div className="workbench-card">
              <h3>
                <span>Mailpit Sandbox Inbox ({mailbox?.total || 0} messages)</span>
                <div>
                  <button className="btn-tiny" onClick={loadServices}>â†» Refresh Inbox</button>
                  <button
                    className="btn-tiny"
                    style={{ marginLeft: "8px" }}
                    onClick={() => act("Delete all mail", async () => {
                      await DeleteAllMail(); setSelectedMail(null); setMailbox(await GetMailbox("",30,0));
                    })}
                  >
                    Clear All
                  </button>
                </div>
              </h3>
              <div style={{ display: "grid", gridTemplateColumns: "320px 1fr", gap: "16px" }}>
                <div className="mail-list">
                  {mailbox?.messages?.map((m) => (
                    <div
                      key={m.id}
                      className={`mail-item ${selectedMail?.id === m.id ? "selected" : ""}`}
                      onClick={() =>
                        act("Read message", async () => {
                          const det = await GetMailMessage(m.id);
                          setSelectedMail(det);
                        })
                      }
                    >
                      <strong>{m.subject || "(No subject)"}</strong>
                      <small style={{ display: "block" }}>From: {m.from}</small>
                      <small style={{ color: "var(--text-dim)" }}>{m.snippet}</small>
                    </div>
                  ))}
                  {(!mailbox?.messages || mailbox.messages.length === 0) && (
                    <div className="empty-text">{mailbox === null ? "Mailpit is unavailable. Start Mailpit, then refresh the inbox." : "No emails received yet. Mailpit listens on 127.0.0.1:1025."}</div>
                  )}
                </div>

                <div className="mail-body-frame">
                  {selectedMail ? (
                    <div>
                      <h4>{selectedMail.subject}</h4>
                      <p><small>From: {selectedMail.from} | To: {selectedMail.to?.join(", ") || ""}</small></p>
                      {selectedMail.hasRemoteImages && (
                        <div className="tag warn" style={{ margin: "10px 0" }}>
                          ðŸ›¡ï¸ Remote tracking images were blocked for privacy.
                        </div>
                      )}
                      {selectedMail.safeHtml ? (
                        <iframe
                          sandbox=""
                          srcDoc={'<meta http-equiv="Content-Security-Policy" content="default-src \'none\'; style-src \'unsafe-inline\'; img-src data:;">' + selectedMail.safeHtml}
                          title="Email Message Body"
                          style={{
                            width: "100%",
                            height: "450px",
                            border: "1px solid #1e293b",
                            borderRadius: "6px",
                            background: "#ffffff",
                            marginTop: "16px",
                          }}
                        />
                      ) : (
                        <pre
                          style={{
                            marginTop: "16px",
                            padding: "16px",
                            background: "#0d131f",
                            border: "1px solid #1e293b",
                            borderRadius: "6px",
                            whiteSpace: "pre-wrap",
                            wordBreak: "break-word",
                            fontFamily: "inherit",
                            color: "#f8fafc",
                          }}
                        >
                          {selectedMail.text}
                        </pre>
                      )}
                    </div>
                  ) : (
                    <p className="empty-text">Select an email to view content.</p>
                  )}
                </div>
              </div>
            </div>
          </div>
          );
        })()}

        {/* TAB 5: ACTIONS & QUEUES */}
        {tab === "actions" && (
          <div className="tab-pane">
            {!project ? (
              <div className="empty-project-prompt">
                <div style={{ fontSize: "40px", marginBottom: "12px" }}>âš™ï¸</div>
                <h3 style={{ color: "#fff", margin: 0 }}>Select a Project to Manage Actions &amp; Queues</h3>
                <p style={{ color: "var(--text-muted)", fontSize: "13px", marginTop: "6px", maxWidth: 500, margin: "6px auto 0" }}>
                  Supervise background queue workers, retry failed jobs, inspect scheduled tasks, and trigger Artisan actions.
                </p>
                <div className="empty-project-grid">
                  {state.projects.map((p) => (
                    <button
                      key={p.id}
                      className="empty-project-tile"
                      onClick={() => select(p.id)}
                    >
                      <strong style={{ color: "#fff", display: "block", marginBottom: 4 }}>{p.name}</strong>
                      <small style={{ color: "var(--text-dim)" }}>PHP {p.php} Â· {p.secured ? "ðŸ”’ HTTPS" : "HTTP"}</small>
                      <div style={{ fontSize: "11px", color: "#6366f1", marginTop: 6 }}>Open Actions &amp; Queues â†’</div>
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              <>
                <div className="sub-segmented-nav">
                  <button
                    className={`sub-tab-pill ${actionsSubTab === "runners" ? "active" : ""}`}
                    onClick={() => setActionsSubTab("runners")}
                  >
                    <span>âš™ï¸</span>
                    <span>Processes &amp; Workers</span>
                    {runs.length > 0 && <span className="count-badge badge-good">{runs.length} active</span>}
                  </button>
                  <button
                    className={`sub-tab-pill ${actionsSubTab === "failed" ? "active" : ""}`}
                    onClick={() => setActionsSubTab("failed")}
                  >
                    <span>ðŸš¨</span>
                    <span>Failed Queue Jobs</span>
                    {failedJobs && failedJobs.length > 0 && (
                      <span className="count-badge badge-error">{failedJobs.length}</span>
                    )}
                  </button>
                  <button
                    className={`sub-tab-pill ${actionsSubTab === "schedule" ? "active" : ""}`}
                    onClick={() => setActionsSubTab("schedule")}
                  >
                    <span>â±ï¸</span>
                    <span>Scheduled Tasks</span>
                    {scheduledTasks && scheduledTasks.length > 0 && (
                      <span className="count-badge badge-total">{scheduledTasks.length}</span>
                    )}
                  </button>
                  <button
                    className={`sub-tab-pill ${actionsSubTab === "events" ? "active" : ""}`}
                    onClick={() => setActionsSubTab("events")}
                  >
                    <span>âš¡</span>
                    <span>Events &amp; Listeners</span>
                    {eventsList && eventsList.length > 0 && (
                      <span className="count-badge badge-total">{eventsList.length}</span>
                    )}
                  </button>
                  <button
                    className={`sub-tab-pill ${actionsSubTab === "commands" ? "active" : ""}`}
                    onClick={() => setActionsSubTab("commands")}
                  >
                    <span>ðŸ› ï¸</span>
                    <span>Artisan Presets</span>
                  </button>
                </div>

                {actionsSubTab === "runners" && (
                  <div className="workbench-card">
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14, flexWrap: "wrap", gap: 10 }}>
                      <div>
                        <h3 style={{ margin: 0 }}>Development Process Management</h3>
                        <small style={{ color: "var(--text-dim)" }}>
                          Control background Vite, Horizon, and Queue worker daemons
                        </small>
                      </div>
                      <div style={{ display: "flex", gap: "8px", flexWrap: "wrap" }}>
                        <button
                          className="btn-toolbar primary"
                          onClick={() => act("Start Vite", () => StartProcess(project!.id, "vite"))}
                          disabled={!state.trusted}
                        >
                          â–¶ Start Vite Dev Server
                        </button>
                        <button
                          className="btn-toolbar"
                          onClick={() => act("Queue one job", () => StartProcess(project!.id, "queue-once"))}
                          disabled={!state.trusted}
                        >
                          â–¶ queue:work --once
                        </button>
                        <button
                          className="btn-toolbar"
                          onClick={() =>
                            act("Start queue worker", async () => setWorker(await StartQueueWorker(project!.id, "default")))
                          }
                          disabled={!state.trusted}
                        >
                          â–¶ Start Queue Worker
                        </button>
                      </div>
                    </div>

                    {worker?.running && (
                      <div className="workbench-card" style={{ background: "rgba(99, 102, 241, 0.08)", borderColor: "rgba(99, 102, 241, 0.25)", marginBottom: 14 }}>
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
                          <strong>âš™ï¸ Queue Worker Â· PID {worker.pid} Â· Queue: <code>{worker.queue}</code></strong>
                          <div style={{ display: "flex", gap: 6 }}>
                            <button className="btn-tiny" onClick={() => act("Refresh worker", async () => setWorker(await GetQueueWorkerStatus()))}>Refresh Output</button>
                            <button className="btn-tiny" onClick={() => act("Stop worker", async () => {
                              try { await StopQueueWorker(false); } finally { setWorker(await GetQueueWorkerStatus()); }
                            })}>Stop Worker</button>
                            <button className="btn-tiny danger" onClick={() => act("Force stop worker", async () => {
                              try { await StopQueueWorker(true); } finally { setWorker(await GetQueueWorkerStatus()); }
                            })}>Force Kill</button>
                          </div>
                        </div>
                        <pre style={{ maxHeight: "140px", overflowY: "auto", fontSize: "11px", margin: 0, padding: 8, background: "#090d16", borderRadius: 6 }}>
                          {worker.output?.slice(-15).join("\n") || "Worker waiting for incoming jobs..."}
                        </pre>
                      </div>
                    )}

                    <h4 style={{ margin: "16px 0 10px 0", color: "#fff" }}>Managed Project Processes ({runs.length})</h4>
                    {runs.length > 0 ? (
                      <div style={{ display: "grid", gap: 10 }}>
                        {runs.map((r) => (
                          <div key={r.id} className="workbench-card" style={{ padding: 12 }}>
                            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                              <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                                <span className={`legend-dot ${r.running ? "good" : "neutral"}`} />
                                <strong style={{ color: "#fff" }}>{r.name}</strong>
                                <span className="badge-pill" style={{ background: "rgba(255,255,255,0.06)", fontSize: "11px" }}>PID {r.pid}</span>
                              </div>
                              {r.running ? (
                                <button className="btn-tiny danger" onClick={() => act("Stop process", () => StopProcess(r.id, false))}>
                                  Stop Process
                                </button>
                              ) : (
                                <span className="workbench-badge badge-info">Exited ({r.exitCode})</span>
                              )}
                            </div>
                            <pre style={{ maxHeight: "120px", overflowY: "auto", fontSize: "11px", margin: 0, padding: 8, background: "#090d16", borderRadius: 6 }}>
                              {r.output?.slice(-15).join("\n") || "No process output logged."}
                            </pre>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="empty-text">No processes currently running for {project.name}.</p>
                    )}
                  </div>
                )}

                {actionsSubTab === "failed" && (
                  <div className="workbench-card">
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14, flexWrap: "wrap", gap: 10 }}>
                      <div>
                        <h3 style={{ margin: 0 }}>Failed Queue Jobs ({(failedJobs || []).length})</h3>
                        <small style={{ color: "var(--text-dim)" }}>Inspected from <code>artisan queue:failed</code></small>
                      </div>
                      <div style={{ display: "flex", gap: 6 }}>
                        <button
                          className="btn-toolbar primary"
                          disabled={!state.trusted || (failedJobs || []).length === 0}
                          onClick={() => act("Retry all failed jobs", async () => {
                            await RetryAllFailedJobs(project!.id);
                            setFailedJobs(await GetFailedJobs(project!.id));
                          })}
                        >
                          Retry All Jobs
                        </button>
                        <button
                          className="btn-toolbar danger"
                          disabled={!state.trusted || (failedJobs || []).length === 0}
                          onClick={() => act("Flush failed jobs", async () => {
                            await FlushFailedJobs(project!.id);
                            setFailedJobs(await GetFailedJobs(project!.id));
                          })}
                        >
                          Flush All
                        </button>
                        <button
                          className="btn-toolbar"
                          onClick={() => act("Refresh failed jobs", async () => setFailedJobs(await GetFailedJobs(project!.id)))}
                        >
                          â†» Refresh
                        </button>
                      </div>
                    </div>

                    {(failedJobs || []).map((fj) => (
                      <div key={fj.id} className="workbench-card" style={{ marginBottom: 10, padding: 14, background: "rgba(239, 68, 68, 0.04)", borderColor: "rgba(239, 68, 68, 0.2)" }}>
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 10, flexWrap: "wrap" }}>
                          <div>
                            <strong style={{ color: "#fff", fontSize: "13px" }}>{fj.class}</strong>
                            <div style={{ fontSize: "11px", color: "var(--text-dim)", marginTop: 2 }}>
                              ID: <code>{fj.id}</code> {fj.uuid && `(${fj.uuid})`} Â· Connection: <strong>{fj.connection}</strong> Â· Queue: <strong>{fj.queue}</strong>
                            </div>
                            <small style={{ display: "block", color: "#f87171", marginTop: 2 }}>Failed: {fj.failedAt}</small>
                          </div>
                          <div style={{ display: "flex", gap: 6 }}>
                            <button
                              className="btn-tiny primary"
                              onClick={() => act("Retry job", async () => {
                                await RetryFailedJob(project!.id, fj.id);
                                setFailedJobs(await GetFailedJobs(project!.id));
                              })}
                            >
                              Retry
                            </button>
                            <button
                              className="btn-tiny"
                              onClick={() => act("Forget job", async () => {
                                await ForgetFailedJob(project!.id, fj.id);
                                setFailedJobs(await GetFailedJobs(project!.id));
                              })}
                            >
                              Forget
                            </button>
                          </div>
                        </div>

                        {fj.exception && (
                          <div style={{ marginTop: 8 }}>
                            <button
                              className="btn-tiny"
                              style={{ fontSize: "10.5px", padding: "2px 8px" }}
                              onClick={() => setExpandedJobException(expandedJobException === fj.id ? null : fj.id)}
                            >
                              {expandedJobException === fj.id ? "â–² Hide Exception Details" : "â–¼ Show Exception Stack"}
                            </button>
                            {expandedJobException === fj.id && (
                              <pre style={{ marginTop: 8, maxHeight: "180px", overflowY: "auto", fontSize: "11px", whiteSpace: "pre-wrap", color: "#fca5a5", background: "rgba(0,0,0,0.4)", padding: 10, borderRadius: 6 }}>
                                {fj.exception}
                              </pre>
                            )}
                          </div>
                        )}
                      </div>
                    ))}
                    {(!failedJobs || failedJobs.length === 0) && (
                      <div className="empty-text" style={{ padding: 24, textAlign: "center" }}>
                        âœ¨ Queue is clean! No failed jobs recorded.
                      </div>
                    )}
                  </div>
                )}

                {actionsSubTab === "schedule" && (
                  <div className="workbench-card">
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 14, flexWrap: "wrap", gap: 10 }}>
                      <div>
                        <h3 style={{ margin: 0 }}>Scheduled Tasks &amp; Cron ({(scheduledTasks || []).length})</h3>
                        <small style={{ color: "var(--text-dim)" }}>Inspected from <code>artisan schedule:list</code></small>
                      </div>
                      <div style={{ display: "flex", gap: 8 }}>
                        <button
                          className="btn-toolbar primary"
                          disabled={!state.trusted}
                          onClick={() => act("Run schedule:run", async () => {
                            const res = await RunScheduledTask(project!.id, "schedule:run");
                            setCmdResult(res);
                            setScheduledTasks(await ListScheduledTasks(project!.id));
                          })}
                        >
                          â–¶ Run Due Tasks (schedule:run)
                        </button>
                        <button
                          className="btn-toolbar"
                          onClick={() => act("Refresh scheduled tasks", async () => {
                            setScheduledTasks(await ListScheduledTasks(project!.id));
                          })}
                        >
                          â†» Refresh
                        </button>
                      </div>
                    </div>

                    {scheduledTasks.length > 0 ? (
                      <table className="workbench-table">
                        <thead>
                          <tr>
                            <th>Cron Expression</th>
                            <th>Command / Job</th>
                            <th>Next Due</th>
                            <th>Mutex Protection</th>
                            <th style={{ width: "100px", textAlign: "right" }}>Trigger</th>
                          </tr>
                        </thead>
                        <tbody>
                          {scheduledTasks.map((task, idx) => (
                            <tr key={idx}>
                              <td>
                                <code style={{ color: "#38bdf8", fontWeight: 700 }}>{task.expression || task.interval}</code>
                              </td>
                              <td>
                                <strong style={{ color: "#fff" }}>{task.command}</strong>
                                {task.description && <small style={{ display: "block", color: "var(--text-dim)" }}>{task.description}</small>}
                              </td>
                              <td>
                                <span className="workbench-badge badge-info">{task.nextDue || "N/A"}</span>
                              </td>
                              <td>
                                {task.hasMutex ? (
                                  <span className="workbench-badge badge-warn">Mutex Protected</span>
                                ) : (
                                  <span style={{ color: "var(--text-dim)", fontSize: "11px" }}>No</span>
                                )}
                              </td>
                              <td style={{ textAlign: "right" }}>
                                <button
                                  className="btn-tiny primary"
                                  disabled={!state.trusted}
                                  onClick={() => act(`Run ${task.command}`, async () => {
                                    const res = await RunScheduledTask(project!.id, task.command);
                                    setCmdResult(res);
                                    setScheduledTasks(await ListScheduledTasks(project!.id));
                                  })}
                                >
                                  Run Now
                                </button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    ) : (
                      <p className="empty-text">No scheduled tasks defined in console routes or Kernel.</p>
                    )}
                  </div>
                )}

                {actionsSubTab === "events" && (
                  <EventsQueuesPanel
                    projectId={project?.id}
                    events={eventsList || []}
                    failedJobs={failedJobs || []}
                    workerStatus={worker || { running: false, pid: 0, queue: "default", output: [], startedAt: "" }}
                    onStartWorker={async (q) => {
                      if (!project?.id) return;
                      const res = await StartQueueWorker(project.id, q);
                      setWorker(res);
                    }}
                    onStopWorker={async () => {
                      await StopQueueWorker(false);
                      setWorker(await GetQueueWorkerStatus());
                    }}
                    onRetryJob={async (id) => {
                      if (!project?.id) return;
                      await RetryFailedJob(project.id, id);
                      loadActions();
                    }}
                    onRetryAllJobs={async () => {
                      if (!project?.id) return;
                      await RetryAllFailedJobs(project.id);
                      loadActions();
                    }}
                    onRefresh={loadActions}
                  />
                )}

                {actionsSubTab === "commands" && (
                  <div className="workbench-card">
                    <h3 style={{ margin: "0 0 12px 0" }}>1-Click Artisan Command Presets</h3>
                    <div style={{ display: "flex", gap: 10, flexWrap: "wrap", marginBottom: 20 }}>
                      {[
                        { label: "Clear Caches (optimize:clear)", args: ["artisan", "optimize:clear"] },
                        { label: "Migration Status (migrate:status)", args: ["artisan", "migrate:status"] },
                        { label: "Route List (route:list)", args: ["artisan", "route:list"] },
                        { label: "Restart Queues (queue:restart)", args: ["artisan", "queue:restart"] },
                        { label: "Clear Views (view:clear)", args: ["artisan", "view:clear"] },
                      ].map((preset) => (
                        <button
                          key={preset.label}
                          className="btn-toolbar primary"
                          disabled={!state.trusted}
                          onClick={() => act(preset.label, async () => {
                            const cmd = new storage.SavedCmd({
                              id: "cmd_preset_" + Date.now(),
                              name: preset.label,
                              executable: "php",
                              args: preset.args,
                              directory: project!.path,
                              destructive: false,
                            });
                            await SaveCommand(project!.id, cmd);
                            const res = await ExecuteSavedCommand(project!.id, cmd.id);
                            setCmdResult(res);
                          })}
                        >
                          âš¡ {preset.label}
                        </button>
                      ))}
                    </div>

                    <h4 style={{ margin: "0 0 10px 0", color: "#fff" }}>Custom Command Launcher</h4>
                    <div style={{ display: "flex", gap: "8px", marginBottom: "12px" }}>
                      <input
                        type="text"
                        placeholder="Command Name (e.g. Seed Database)"
                        value={newCmdName}
                        onChange={(e) => setNewCmdName(e.target.value)}
                        style={{ flex: 1, padding: "6px 10px" }}
                      />
                      <input
                        type="text"
                        placeholder={'Arguments as JSON, e.g. ["artisan", "db:seed"]'}
                        value={newCmdArgs}
                        onChange={(e) => setNewCmdArgs(e.target.value)}
                        style={{ flex: 2, padding: "6px 10px" }}
                      />
                      <button
                        className="btn-toolbar primary"
                        onClick={() =>
                          act("Save command", async () => {
                            const args: unknown = JSON.parse(newCmdArgs);
                            if (!Array.isArray(args) || !args.every((value) => typeof value === "string") || args.length === 0) {
                              throw new Error("Arguments must be a non-empty JSON array of strings.");
                            }
                            const cmd = new storage.SavedCmd({
                              id: "cmd_" + Date.now(),
                              name: newCmdName,
                              executable: "php",
                              args,
                              directory: project!.path,
                              destructive: false,
                            });
                            await SaveCommand(project!.id, cmd);
                            setNewCmdName("");
                            setNewCmdArgs('["artisan", "route:list"]');
                            setSavedCmds(await GetSavedCommands(project!.id));
                          })
                        }
                      >
                        Save &amp; Add
                      </button>
                    </div>

                    {(savedCmds || []).map((sc) => (
                      <div key={sc.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "8px 0", borderBottom: "1px solid rgba(255,255,255,0.06)" }}>
                        <div>
                          <strong style={{ color: "#fff" }}>{sc.name}</strong>
                          <code style={{ marginLeft: "10px", color: "var(--text-dim)" }}>{sc.executable} {sc.args ? sc.args.join(" ") : ""}</code>
                        </div>
                        <button
                          className="btn-tiny primary"
                          onClick={() =>
                            act("Execute command", async () => {
                              const res = await ExecuteSavedCommand(project!.id, sc.id);
                              setCmdResult(res);
                            })
                          }
                        >
                          Execute
                        </button>
                      </div>
                    ))}

                    {cmdResult && (
                      <div style={{ marginTop: 14 }}>
                        <h5 style={{ margin: "0 0 6px 0", color: "#fff" }}>Execution Output</h5>
                        <pre style={{ background: "#090d16", padding: "12px", borderRadius: "6px", fontSize: "12px", border: "1px solid rgba(255,255,255,0.08)", maxHeight: 200, overflowY: "auto" }}>
                          {cmdResult.output.join("\n")}
                        </pre>
                      </div>
                    )}
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* TAB 6: TESTING & API CLIENT */}
        {tab === "testing" && (
          <div className="tab-pane">
            {!project ? (
              <div className="empty-project-prompt">
                <div className="empty-project-icon">ðŸ§ª</div>
                <h3>No Laravel Project Selected</h3>
                <p>Select a project from your workbench to run the HTTP API client, execute Pest/PHPUnit test suites, or conduct load tests.</p>
                <div className="empty-project-list">
                  {state.projects.map((p) => (
                    <button key={p.id} className="empty-project-card" onClick={() => select(p.id)}>
                      <div className="project-card-header">
                        <span className="project-card-name">{p.name}</span>
                        <span className="project-card-tag">{p.php || "PHP"}</span>
                      </div>
                      <div className="project-card-path">{p.path}</div>
                    </button>
                  ))}
                </div>
              </div>
            ) : (
              <>
                <div className="sub-segmented-nav">
                  <button
                    className={`sub-tab-pill ${testingSubTab === "api" ? "active" : ""}`}
                    onClick={() => setTestingSubTab("api")}
                  >
                    âš¡ HTTP API Client & Assertions
                  </button>
                  <button
                    className={`sub-tab-pill ${testingSubTab === "pest" ? "active" : ""}`}
                    onClick={() => setTestingSubTab("pest")}
                  >
                    ðŸ§ª Pest / PHPUnit Test Suite
                  </button>
                  <button
                    className={`sub-tab-pill ${testingSubTab === "load" ? "active" : ""}`}
                    onClick={() => setTestingSubTab("load")}
                  >
                    ðŸš€ Concurrency & Load Engine (k6)
                  </button>
                  <button
                    className={`sub-tab-pill ${testingSubTab === "checkup" ? "active" : ""}`}
                    onClick={() => {
                      setTestingSubTab("checkup");
                      loadCheckup();
                    }}
                  >
                    🩺 Performance Checkups & Fixes
                  </button>
                </div>

                {testingSubTab === "api" && (
                  <div className="workbench-card">
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px" }}>
                      <h3>HTTP API Client & Scenario Assertions</h3>
                      {routes.length > 0 && (
                        <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                          <span style={{ fontSize: "11px", color: "var(--text-muted)" }}>Quick Route:</span>
                          <select
                            className="text-input"
                            style={{ padding: "4px 8px", fontSize: "11px", maxWidth: "240px" }}
                            onChange={(e) => {
                              if (!e.target.value) return;
                              const r = routes.find((rt) => rt.uri === e.target.value);
                              if (r) {
                                const method = (r.method || "GET").split("|")[0];
                                setApiMethod(method);
                                setApiUrl(`${project.url.replace(/\/$/, "")}/${r.uri.replace(/^\//, "")}`);
                              }
                            }}
                          >
                            <option value="">-- Choose Route ({routes.length} found) --</option>
                            {routes.map((rt, idx) => (
                              <option key={idx} value={rt.uri}>
                                {rt.method} /{rt.uri}
                              </option>
                            ))}
                          </select>
                        </div>
                      )}
                    </div>

                    <div style={{ display: "grid", gridTemplateColumns: "110px 1fr auto", gap: "8px", alignItems: "center" }}>
                      <select
                        className="http-method-select"
                        value={apiMethod}
                        onChange={(e) => setApiMethod(e.target.value)}
                      >
                        <option value="GET">GET</option>
                        <option value="POST">POST</option>
                        <option value="PUT">PUT</option>
                        <option value="DELETE">DELETE</option>
                      </select>
                      <input
                        type="url"
                        placeholder="URL (e.g. http://my-app.test/api/users)"
                        value={apiUrl || (project ? `${project.url}/api` : "")}
                        onChange={(e) => setApiUrl(e.target.value)}
                      />
                      <button
                        className="btn-action primary"
                        disabled={!!busy}
                        onClick={() =>
                          act("Send API Request", async () => {
                            const spec = new wktesting.APIRequestSpec({
                              id: "req_" + Date.now(),
                              name: "Manual Test",
                              method: apiMethod,
                              url: apiUrl || (project ? project.url : ""),
                              body: apiBody,
                              assertions: [
                                new wktesting.APIAssertion({ type: "status_code", value: "200" }),
                                new wktesting.APIAssertion({ type: "max_latency", value: "3000" }),
                              ],
                            });
                            const res = await ExecuteAPIRequest(spec);
                            setApiResponse(res);
                          })
                        }
                      >
                        {busy === "Send API Request" ? "Sending..." : "Send Request"}
                      </button>
                    </div>

                    {(apiMethod === "POST" || apiMethod === "PUT") && (
                      <div style={{ marginTop: "12px" }}>
                        <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "4px" }}>
                          <small style={{ color: "var(--text-muted)" }}>JSON Payload Body:</small>
                          <div style={{ display: "flex", gap: "6px" }}>
                            <button
                              type="button"
                              className="btn-tiny"
                              onClick={() => setApiBody('{\n  "email": "test@example.com",\n  "name": "Jane Doe"\n}')}
                            >
                              Preset: User
                            </button>
                            <button
                              type="button"
                              className="btn-tiny"
                              onClick={() => setApiBody('{\n  "status": "active",\n  "amount": 100\n}')}
                            >
                              Preset: Order
                            </button>
                          </div>
                        </div>
                        <textarea
                          placeholder='{"key": "value"}'
                          value={apiBody}
                          onChange={(e) => setApiBody(e.target.value)}
                          style={{ width: "100%", height: "90px", fontFamily: "monospace", fontSize: "12px" }}
                        />
                      </div>
                    )}

                    {apiResponse && (
                      <div style={{ marginTop: "16px", background: "var(--bg-card-secondary)", padding: "14px", borderRadius: "8px", border: "1px solid var(--border-card)" }}>
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "8px" }}>
                          <div style={{ display: "flex", gap: "12px", alignItems: "center" }}>
                            <span className={`http-method-badge ${apiResponse.statusCode < 300 ? "http-post" : apiResponse.statusCode < 500 ? "http-put" : "http-delete"}`}>
                              HTTP {apiResponse.statusCode}
                            </span>
                            <span style={{ fontSize: "12px", color: "var(--text-dim)" }}>
                              Latency: <strong>{ms(apiResponse.durationMs)}</strong>
                            </span>
                            <span className={`workbench-badge ${apiResponse.passed ? "badge-good" : "badge-error"}`}>
                              {apiResponse.passed ? "âœ“ All Assertions Passed" : "âš  Assertion Mismatch"}
                            </span>
                          </div>
                          <button
                            type="button"
                            className="btn-tiny"
                            onClick={() => copyText(apiResponse.body || "")}
                          >
                            ðŸ“‹ Copy Response
                          </button>
                        </div>
                        <pre style={{ margin: 0, padding: "10px", maxHeight: "200px", overflowY: "auto", fontSize: "11px", background: "#090d16", borderRadius: "6px" }}>
                          {apiResponse.body || "<empty response body>"}
                        </pre>
                      </div>
                    )}
                  </div>
                )}

                {testingSubTab === "pest" && (
                  <div className="workbench-card">
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px" }}>
                      <h3>Pest / PHPUnit Test Suite Runner</h3>
                      <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
                        <input
                          type="text"
                          placeholder="Filter tests (e.g. Unit or Feature)"
                          value={testFilter}
                          onChange={(e) => setTestFilter(e.target.value)}
                          style={{ padding: "5px 8px", fontSize: "12px", width: "200px" }}
                        />
                        <button
                          className="btn-action primary"
                          disabled={!!busy}
                          onClick={() =>
                            act("Run project tests", async () => {
                              const res = await RunProjectTests(project.id, testFilter);
                              setTestRunResult(res);
                            })
                          }
                        >
                          {busy === "Run project tests" ? "Running Suite..." : "â–¶ Run Test Suite"}
                        </button>
                      </div>
                    </div>

                    {testRunResult ? (
                      <div>
                        <div style={{ display: "flex", gap: "12px", alignItems: "center", marginBottom: "12px" }}>
                          <span className={`workbench-badge ${testRunResult.success ? "badge-good" : "badge-error"}`}>
                            {testRunResult.success ? "âœ“ Test Suite Passed" : "âœ— Suite Failed"}
                          </span>
                          <span className="workbench-badge badge-good">{testRunResult.passed} Passed</span>
                          <span className="workbench-badge badge-error">{testRunResult.failed} Failed</span>
                          <span className="workbench-badge badge-info">{testRunResult.skipped} Skipped</span>
                          <span style={{ fontSize: "12px", color: "var(--text-muted)" }}>
                            Duration: <strong>{testRunResult.durationSeconds.toFixed(2)}s</strong>
                          </span>
                        </div>
                        <pre style={{ background: "#090d16", border: "1px solid var(--border-card)", borderRadius: "8px", padding: "12px", maxHeight: "300px", overflowY: "auto", fontSize: "11px", color: "#e2e8f0" }}>
                          {testRunResult.output.join("\n") || "No console output recorded."}
                        </pre>
                      </div>
                    ) : (
                      <p style={{ color: "var(--text-muted)", fontSize: "12px" }}>
                        Run your project's PHPUnit or Pest tests directly with real-time process streaming and failure isolation.
                      </p>
                    )}
                  </div>
                )}

                {testingSubTab === "load" && (
                  <div className="workbench-card">
                    <h3>Load & Concurrency Stress Engine</h3>
                    <p style={{ fontSize: "12px", color: "var(--text-muted)", marginBottom: "14px" }}>
                      Execute concurrent virtual user load against your local web server to benchmark request latency and throughput under pressure.
                    </p>

                    <div style={{ display: "flex", gap: "12px", alignItems: "center", flexWrap: "wrap", marginBottom: "14px" }}>
                      <label style={{ fontSize: "12px", display: "flex", alignItems: "center", gap: "6px" }}>
                        Virtual Users (VUs):
                        <input
                          type="number"
                          min={1}
                          max={100}
                          value={loadUsers}
                          onChange={(e) => setLoadUsers(Number(e.target.value))}
                          style={{ width: "65px", padding: "4px 8px" }}
                        />
                      </label>
                      <label style={{ fontSize: "12px", display: "flex", alignItems: "center", gap: "6px" }}>
                        Duration (sec):
                        <input
                          type="number"
                          min={1}
                          max={60}
                          value={loadDuration}
                          onChange={(e) => setLoadDuration(Number(e.target.value))}
                          style={{ width: "65px", padding: "4px 8px" }}
                        />
                      </label>
                      <div style={{ display: "flex", gap: "6px" }}>
                        <button type="button" className="btn-tiny" onClick={() => { setLoadUsers(5); setLoadDuration(5); }}>
                          Light (5 VUs)
                        </button>
                        <button type="button" className="btn-tiny" onClick={() => { setLoadUsers(20); setLoadDuration(10); }}>
                          Medium (20 VUs)
                        </button>
                        <button type="button" className="btn-tiny" onClick={() => { setLoadUsers(50); setLoadDuration(15); }}>
                          Stress (50 VUs)
                        </button>
                      </div>
                      <button
                        className="btn-action primary"
                        disabled={!!busy}
                        onClick={() =>
                          act("Run load test", async () => {
                            const res = await RunLoadTest(
                              new wktesting.LoadTestSpec({
                                targetUrl: apiUrl || project.url,
                                method: "GET",
                                virtualUsers: loadUsers,
                                durationSeconds: loadDuration,
                              }),
                            );
                            setLoadResult(res);
                          })
                        }
                      >
                        {busy === "Run load test" ? "Running Load Test..." : "ðŸš€ Run Load Test"}
                      </button>
                    </div>

                    {loadResult && (
                      <div className="panel-grid">
                        <div className="workbench-card">
                          <small style={{ color: "var(--text-dim)" }}>Throughput</small>
                          <h3 style={{ color: "#38bdf8", marginTop: "4px" }}>{loadResult.requestsPerSecond.toFixed(1)} req/s</h3>
                          <small style={{ color: "var(--text-muted)" }}>Total: {loadResult.totalRequests} | OK: {loadResult.successful}</small>
                        </div>
                        <div className="workbench-card">
                          <small style={{ color: "var(--text-dim)" }}>Latency Metrics</small>
                          <h3 style={{ color: "#a855f7", marginTop: "4px" }}>{ms(loadResult.avgLatencyMs)}</h3>
                          <small style={{ color: "var(--text-muted)" }}>Min: {ms(loadResult.minLatencyMs)} | Max: {ms(loadResult.maxLatencyMs)}</small>
                        </div>
                        <div className="workbench-card">
                          <small style={{ color: "var(--text-dim)" }}>Percentiles Breakdown</small>
                          <p style={{ margin: "6px 0 0", fontSize: "12px" }}>
                            p50: <strong>{ms(loadResult.p50LatencyMs)}</strong> | p90: <strong>{ms(loadResult.p90LatencyMs)}</strong> | p99: <strong>{ms(loadResult.p99LatencyMs)}</strong>
                          </p>
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {testingSubTab === "checkup" && (
                  <PerformancePanel
                    projectId={project?.id}
                    checkup={checkupReport}
                    defaultUrl={project?.url || ""}
                    onRunCheckup={loadCheckup}
                    onApplyFix={async (action) => {
                      if (!project?.id) return;
                      if (action.startsWith("artisan ")) {
                        const cmd = action.replace("artisan ", "");
                        await act(`Fix: ${action}`, () => StartProcess(project.id, cmd));
                      }
                    }}
                    onRunLoadTest={async (spec) => {
                      return await RunNativeLoadTest(spec);
                    }}
                  />
                )}
              </>
            )}
          </div>
        )}

        {/* TAB 7: QUALITY & AUDIT */}
        {tab === "quality" && (
          <div className="tab-pane">
            {!project ? (
              <div className="empty-project-prompt">
                <div className="empty-project-icon">ðŸ›¡ï¸</div>
                <h3>No Laravel Project Selected</h3>
                <p>Select a project from the workbench to run multi-engine quality audits, PHP Pint formatting, and security scans.</p>
                <div className="empty-project-list">
                  {state.projects.map((p) => (
                    <button key={p.id} className="empty-project-card" onClick={() => select(p.id)}>
                      <div className="project-card-header">
                        <span className="project-card-name">{p.name}</span>
                        <span className="project-card-tag">{p.php || "PHP"}</span>
                      </div>
                      <div className="project-card-path">{p.path}</div>
                    </button>
                  ))}
                </div>
              </div>
            ) : !state.trusted ? (
              <div className="empty-project-prompt" style={{ border: "1px dashed var(--accent-amber)" }}>
                <div className="empty-project-icon">ðŸ”’</div>
                <h3>Project Not Trusted</h3>
                <p>Security analyzers and Pint execution require trusting this project directory ({project.path}).</p>
                <button
                  className="btn-action primary"
                  style={{ marginTop: "12px" }}
                  onClick={() =>
                    act("Trust Project", async () => {
                      await TrustProject(project.id);
                      await load();
                    })
                  }
                >
                  Trust Project & Enable Auditing
                </button>
              </div>
            ) : (
              <AuditPanel
                key={project?.id}
                project={project}
                trusted={state.trusted}
                report={state.auditReports?.[project?.id || ""]}
                browser={state.auditBrowsers?.[project?.id || ""]}
                reload={load}
                auditItems={auditItems}
                pintReport={pintReport}
                onRunPint={() =>
                  act("Run Pint", async () => {
                    const rep = await RunPint(project!.id, true);
                    setPintReport(rep);
                  })
                }
                onRefreshConfigAudit={loadQuality}
              />
            )}
          </div>
        )}

        {/* TAB 8: RECIPES & WIZARD */}
        {tab === "recipes" && (
          <div className="tab-pane">
            <div className="sub-segmented-nav">
              <button
                className={`sub-tab-pill ${recipeSubTab === "recipes" ? "active" : ""}`}
                onClick={() => setRecipeSubTab("recipes")}
              >
                ðŸ“¦ Bundled Laravel Recipes ({(recipesList || []).length})
              </button>
              <button
                className={`sub-tab-pill ${recipeSubTab === "wizard" ? "active" : ""}`}
                onClick={() => setRecipeSubTab("wizard")}
              >
                âœ¨ New Project Starter Wizard
              </button>
            </div>

            {recipeSubTab === "recipes" && (
              <div className="workbench-card">
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: "10px", marginBottom: "14px" }}>
                  <div>
                    <h3>1-Click Laravel Recipes & Feature Bundles</h3>
                    <p style={{ fontSize: "12px", color: "var(--text-muted)", margin: "2px 0 0" }}>
                      Curated, pre-verified automation bundles to install authentication, queues, search, devtools, and modern frontends.
                    </p>
                  </div>
                  <input
                    type="text"
                    placeholder="ðŸ” Search recipes..."
                    value={recipeSearch}
                    onChange={(e) => setRecipeSearch(e.target.value)}
                    style={{ padding: "6px 10px", fontSize: "12px", width: "200px" }}
                  />
                </div>

                {/* Category Pills */}
                <div style={{ display: "flex", gap: "6px", flexWrap: "wrap", marginBottom: "14px" }}>
                  {[
                    { id: "all", label: "All Recipes" },
                    { id: "auth", label: "Auth & Admin" },
                    { id: "monitoring", label: "Monitoring & Dev" },
                    { id: "queues", label: "Queues & Async" },
                    { id: "testing", label: "Testing & Quality" },
                    { id: "frontend", label: "Frontend & UI" },
                    { id: "docker", label: "Docker & Billing" },
                  ].map((cat) => (
                    <button
                      key={cat.id}
                      type="button"
                      className={`btn-tiny ${recipeCategory === cat.id ? "primary" : ""}`}
                      onClick={() => setRecipeCategory(cat.id)}
                    >
                      {cat.label}
                    </button>
                  ))}
                </div>

                <div className="recipe-cards-grid">
                  {(recipesList || [])
                    .filter((r) => {
                      if (recipeCategory !== "all") {
                        if (recipeCategory === "auth" && !["auth", "admin"].includes(r.category)) return false;
                        if (recipeCategory === "monitoring" && !["monitoring", "dev"].includes(r.category)) return false;
                        if (recipeCategory === "queues" && !["queues", "search"].includes(r.category)) return false;
                        if (recipeCategory === "testing" && !["testing", "quality"].includes(r.category)) return false;
                        if (recipeCategory === "frontend" && !["frontend"].includes(r.category)) return false;
                        if (recipeCategory === "docker" && !["docker", "billing"].includes(r.category)) return false;
                      }
                      if (recipeSearch) {
                        const q = recipeSearch.toLowerCase();
                        return r.title.toLowerCase().includes(q) || r.description.toLowerCase().includes(q) || r.id.toLowerCase().includes(q);
                      }
                      return true;
                    })
                    .map((r) => (
                      <div key={r.id} className="recipe-deck-card">
                        <div>
                          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "6px" }}>
                            <span className={`recipe-deck-category ${r.category === "auth" || r.category === "admin" ? "package" : r.category === "frontend" ? "theme" : "setup"}`}>
                              {r.category}
                            </span>
                            <span style={{ fontSize: "10px", color: "var(--text-muted)", fontFamily: "monospace" }}>#{r.id}</span>
                          </div>
                          <h4 style={{ margin: "4px 0 8px 0", fontSize: "15px", color: "#fff" }}>{r.title}</h4>
                          <p style={{ fontSize: "12px", color: "var(--text-muted)", lineHeight: "1.5", margin: "0 0 14px 0" }}>
                            {r.description}
                          </p>
                        </div>
                        <div style={{ display: "flex", gap: "8px", justifyContent: "flex-end" }}>
                          <button
                            type="button"
                            className="btn-tiny"
                            onClick={() =>
                              act("Preview recipe", async () => {
                                const prev = await PreviewRecipe(r.id);
                                setNotice(`Recipe steps (${prev.commandsToRun?.length || 0}): ${prev.commandsToRun?.join(" && ")}`);
                              })
                            }
                          >
                            Preview Steps
                          </button>
                          <button
                            type="button"
                            className="btn-tiny primary"
                            disabled={!project || !state.trusted || !!busy}
                            title={!project ? "Select project first" : !state.trusted ? "Trust project first" : "Apply recipe to project"}
                            onClick={() =>
                              act("Execute recipe", async () => {
                                const res = await ExecuteRecipe(project!.id, r.id);
                                setRecipeResult(res);
                              })
                            }
                          >
                            Apply Recipe
                          </button>
                        </div>
                      </div>
                    ))}
                </div>

                {recipeResult && (
                  <div style={{ marginTop: "16px", padding: "12px", borderRadius: "8px", background: "var(--bg-card-secondary)", border: "1px solid var(--border-card)" }}>
                    <span className={`workbench-badge ${recipeResult.success ? "badge-good" : "badge-error"}`}>
                      {recipeResult.success ? "âœ“ Recipe Applied Successfully" : "âš  Recipe Failed"}
                    </span>
                    {recipeResult.recoveryGuidance && (
                      <p style={{ fontSize: "12px", color: "var(--text-dim)", marginTop: "6px" }}>
                        <strong>Recovery Guidance:</strong> {recipeResult.recoveryGuidance}
                      </p>
                    )}
                  </div>
                )}
              </div>
            )}

            {recipeSubTab === "wizard" && (
              <div className="workbench-card">
                <h3>New Project Starter Wizard</h3>
                <p style={{ fontSize: "12px", color: "var(--text-muted)", marginBottom: "14px" }}>
                  Scaffold a fresh, fully-configured Laravel installation with chosen database and starter stack.
                </p>
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 130px auto", gap: "10px", alignItems: "center" }}>
                  <input
                    type="text"
                    placeholder="Project Name (e.g. blog-app)"
                    value={wizardName}
                    onChange={(e) => setWizardName(e.target.value)}
                  />
                  <input
                    type="text"
                    placeholder="Parent Directory (e.g. C:\Users\Dev\Projects)"
                    value={wizardDir}
                    onChange={(e) => setWizardDir(e.target.value)}
                  />
                  <select value={wizardDb} onChange={(e) => setWizardDb(e.target.value)}>
                    <option value="sqlite">SQLite</option>
                    <option value="mysql">MySQL</option>
                    <option value="postgres">PostgreSQL</option>
                  </select>
                  <button
                    className="btn-action primary"
                    disabled={!wizardName || !wizardDir || !!busy}
                    onClick={() =>
                      act("Run wizard", async () => {
                        const spec = new recipes.ProjectWizardSpec({
                          name: wizardName,
                          directory: wizardDir,
                          starter: "none",
                          database: wizardDb,
                          phpBinary: "php",
                          gitInit: true,
                        });
                        await ExecuteProjectWizard(spec);
                        setNotice("Project generated successfully!");
                        await load();
                      })
                    }
                  >
                    Create Project
                  </button>
                </div>
              </div>
            )}
          </div>
        )}

        {/* TAB 9: REPORTS & ARCHITECTURE */}
        {tab === "reports" && (
          <div className="tab-pane">
            {/* Top Architecture Hero & Project Switcher */}
            <div className="arch-hero-card">
              <div className="arch-hero-left">
                <div className="arch-hero-icon">ðŸ—ºï¸</div>
                <div className="arch-hero-title-row">
                  <h2>Architecture & System Blueprint</h2>
                  <div className="arch-hero-desc">
                    Service topology graph, dependency inference, hosting capacity analysis, and automated team documentation.
                  </div>
                </div>
              </div>
              <div className="arch-hero-actions">
                <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                  <span style={{ fontSize: "12px", color: "var(--text-muted)", fontWeight: 500 }}>Active Project:</span>
                  <select
                    value={project?.id || ""}
                    onChange={(e) => {
                      if (e.target.value) {
                        select(e.target.value);
                      }
                    }}
                    style={{
                      padding: "6px 12px",
                      fontSize: "12px",
                      borderRadius: "8px",
                      background: "#090d16",
                      color: "#fff",
                      border: "1px solid rgba(255, 255, 255, 0.15)",
                      cursor: "pointer",
                    }}
                  >
                    <option value="">-- Switch Project ({state.projects.length}) --</option>
                    {state.projects.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name} (PHP {p.php})
                      </option>
                    ))}
                  </select>
                  {project && (
                    <button
                      type="button"
                      className="btn-tiny primary"
                      disabled={!!busy}
                      onClick={loadReports}
                      title="Re-analyze architectural relations and dependencies"
                    >
                      {busy === "Generate project snapshot" ? "Analyzing..." : "â†º Re-Analyze"}
                    </button>
                  )}
                </div>
              </div>
            </div>

            {!project ? (
              <div className="empty-project-prompt">
                <div className="empty-project-icon">ðŸ—ºï¸</div>
                <h3>Select a Laravel Project to Inspect</h3>
                <p>
                  Choose one of your {state.projects.length} discovered local projects to inspect component dependencies, queue topologies, hosting requirements, and generate team documentation.
                </p>
                <div className="empty-project-search-wrap">
                  <input
                    type="text"
                    className="empty-project-search"
                    placeholder={`ðŸ” Search ${state.projects.length} discovered projects...`}
                    value={archProjectSearch}
                    onChange={(e) => setArchProjectSearch(e.target.value)}
                  />
                </div>
                <div className="empty-project-list">
                  {state.projects
                    .filter((p) =>
                      !archProjectSearch ||
                      p.name.toLowerCase().includes(archProjectSearch.toLowerCase()) ||
                      p.path.toLowerCase().includes(archProjectSearch.toLowerCase())
                    )
                    .map((p) => (
                      <button
                        key={p.id}
                        type="button"
                        className="empty-project-card"
                        onClick={() => select(p.id)}
                      >
                        <div className="project-card-header">
                          <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                            <div
                              style={{
                                width: "30px",
                                height: "30px",
                                borderRadius: "8px",
                                background: "linear-gradient(135deg, rgba(99, 102, 241, 0.35) 0%, rgba(168, 85, 247, 0.2) 100%)",
                                border: "1px solid rgba(99, 102, 241, 0.45)",
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                                fontSize: "12px",
                                fontWeight: 700,
                                color: "#c7d2fe",
                              }}
                            >
                              {p.name.slice(0, 2).toUpperCase()}
                            </div>
                            <span className="project-card-name">{p.name}</span>
                          </div>
                          <span className="project-card-tag">PHP {p.php}</span>
                        </div>
                        <div className="project-card-path">
                          <span>ðŸ“</span> {p.path}
                        </div>
                        <div className="project-card-action">
                          <span>{p.url || "Local site"}</span>
                          <span>Analyze Architecture &rarr;</span>
                        </div>
                      </button>
                    ))}
                  {state.projects.filter((p) =>
                    !archProjectSearch ||
                    p.name.toLowerCase().includes(archProjectSearch.toLowerCase()) ||
                    p.path.toLowerCase().includes(archProjectSearch.toLowerCase())
                  ).length === 0 && (
                    <div style={{ gridColumn: "1 / -1", textAlign: "center", padding: "20px", color: "var(--text-muted)" }}>
                      No projects matched your search query "{archProjectSearch}".
                    </div>
                  )}
                </div>
              </div>
            ) : !snapshot ? (
              <div className="empty-project-prompt">
                <div className="empty-project-icon">ðŸ“Š</div>
                <h3>Architecture Blueprint Not Yet Generated</h3>
                <p>
                  Analyze <strong>{project.name}</strong> to map service topology, queue workers, cache stores, and sizing requirements.
                </p>
                <button
                  className="btn-action primary"
                  style={{ marginTop: "12px" }}
                  disabled={!!busy}
                  onClick={loadReports}
                >
                  {busy ? "Analyzing Project..." : "âš¡ Generate Architecture Blueprint"}
                </button>
              </div>
            ) : (
              <>
                <div className="sub-segmented-nav">
                  <button
                    className={`sub-tab-pill ${reportsSubTab === "canvas" ? "active" : ""}`}
                    onClick={() => setReportsSubTab("canvas")}
                  >
                    ðŸ›ï¸ Living Architecture Canvas & Boundary Linter ({archReport?.totalComponents || 0})
                  </button>
                  <button
                    className={`sub-tab-pill ${reportsSubTab === "graph" ? "active" : ""}`}
                    onClick={() => setReportsSubTab("graph")}
                  >
                    ðŸ—ºï¸ Component Dependency Graph ({snapshot.graph?.nodes?.length || 0})
                  </button>
                  <button
                    className={`sub-tab-pill ${reportsSubTab === "hosting" ? "active" : ""}`}
                    onClick={() => setReportsSubTab("hosting")}
                  >
                    â˜ï¸ Hosting & Capacity Plan
                  </button>
                  <button
                    className={`sub-tab-pill ${reportsSubTab === "readme" ? "active" : ""}`}
                    onClick={() => setReportsSubTab("readme")}
                  >
                    ðŸ“ Auto README Generator & Merge
                  </button>
                  <button
                    className={`sub-tab-pill ${reportsSubTab === "manifest" ? "active" : ""}`}
                    onClick={() => setReportsSubTab("manifest")}
                  >
                    ðŸ“¦ Team Manifest & Support Bundle
                  </button>
                </div>

                {reportsSubTab === "canvas" && (
                  <ArchitectureCanvas
                    report={archReport}
                    loading={busy === "Generate project snapshot"}
                    onRefresh={loadReports}
                    projectName={project.name}
                  />
                )}

                {reportsSubTab === "graph" && (
                  <div className="workbench-card">
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: "10px", marginBottom: "14px" }}>
                      <div>
                        <h3 style={{ margin: 0 }}>System Component Topology</h3>
                        <p style={{ fontSize: "12px", color: "var(--text-muted)", margin: "4px 0 0" }}>
                          Discovered component relationships categorized by declaration source, code inference, and runtime telemetry.
                        </p>
                      </div>
                      <div style={{ display: "flex", gap: "6px" }}>
                        <span className="provenance-tag prov-declared">Declared ({snapshot.graph?.edges?.filter((e) => e.provenance === "declared").length || 0})</span>
                        <span className="provenance-tag prov-inferred">Inferred ({snapshot.graph?.edges?.filter((e) => e.provenance === "inferred").length || 0})</span>
                        <span className="provenance-tag prov-observed">Observed ({snapshot.graph?.edges?.filter((e) => e.provenance === "observed").length || 0})</span>
                      </div>
                    </div>

                    <div className="arch-card-grid" style={{ marginBottom: "16px" }}>
                      <div className="arch-metric-card">
                        <h4>Discovered Components</h4>
                        <div className="arch-metric-value" style={{ color: "#38bdf8" }}>{snapshot.graph?.nodes?.length || 0}</div>
                      </div>
                      <div className="arch-metric-card">
                        <h4>Topology Edges</h4>
                        <div className="arch-metric-value" style={{ color: "#a855f7" }}>{snapshot.graph?.edges?.length || 0}</div>
                      </div>
                      <div className="arch-metric-card">
                        <h4>Code Inferences</h4>
                        <div className="arch-metric-value" style={{ color: "#c7d2fe" }}>
                          {snapshot.graph?.edges?.filter((e) => e.provenance === "inferred").length || 0}
                        </div>
                      </div>
                      <div className="arch-metric-card">
                        <h4>Runtime Observations</h4>
                        <div className="arch-metric-value" style={{ color: "#fde047" }}>
                          {snapshot.graph?.edges?.filter((e) => e.provenance === "observed").length || 0}
                        </div>
                      </div>
                    </div>

                    {/* Filter Bar for Graph */}
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: "10px", marginTop: "12px", marginBottom: "8px" }}>
                      <div style={{ display: "flex", gap: "6px" }}>
                        {(["all", "declared", "inferred", "observed"] as const).map((prov) => (
                          <button
                            key={prov}
                            type="button"
                            className={`btn-tiny ${archProvenanceFilter === prov ? "primary" : ""}`}
                            onClick={() => setArchProvenanceFilter(prov)}
                          >
                            {prov === "all" ? "All Relations" : prov.charAt(0).toUpperCase() + prov.slice(1)}
                          </button>
                        ))}
                      </div>
                      <input
                        type="text"
                        placeholder="ðŸ” Filter relationships or components..."
                        value={archEdgeSearch}
                        onChange={(e) => setArchEdgeSearch(e.target.value)}
                        style={{ padding: "5px 10px", fontSize: "12px", width: "240px" }}
                      />
                    </div>

                    <div className="graph-container">
                      {(snapshot.graph?.edges || [])
                        .filter((e) => {
                          if (archProvenanceFilter !== "all" && e.provenance !== archProvenanceFilter) return false;
                          if (archEdgeSearch) {
                            const q = archEdgeSearch.toLowerCase();
                            return e.source.toLowerCase().includes(q) || e.target.toLowerCase().includes(q) || e.label.toLowerCase().includes(q);
                          }
                          return true;
                        })
                        .map((e, idx) => (
                          <div key={idx} className="graph-edge">
                            <span className="graph-node" title={e.source}>
                              {e.source.includes("Controller") ? "ðŸŒ " : e.source.includes("Job") ? "âš¡ " : e.source.includes("Model") ? "ðŸ’¾ " : "ðŸ“¦ "}
                              {e.source}
                            </span>
                            <span className="graph-arrow">
                              &rarr; {e.label} &rarr;
                            </span>
                            <span className="graph-node" title={e.target}>
                              {e.target.includes("Database") ? "ðŸ—„ï¸ " : e.target.includes("Cache") ? "âš¡ " : e.target.includes("Mail") ? "ðŸ“¬ " : "ðŸ”— "}
                              {e.target}
                            </span>
                            <span className={`provenance-tag prov-${e.provenance}`}>{e.provenance}</span>
                          </div>
                        ))}
                      {(!snapshot.graph?.edges || snapshot.graph.edges.length === 0) && (
                        <div style={{ padding: "24px", textAlign: "center", color: "var(--text-muted)" }}>
                          No inter-service relationships identified yet.
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {reportsSubTab === "hosting" && (
                  <div className="workbench-card">
                    <h3>Hosting & Capacity Recommendations</h3>
                    <p style={{ fontSize: "12px", color: "var(--text-muted)", marginBottom: "14px" }}>
                      Based on static inference and dynamic resource telemetry gathered from your application profile:
                    </p>
                    <div className="arch-card-grid" style={{ marginBottom: "16px" }}>
                      <div className="arch-metric-card">
                        <h4>Recommended Plan</h4>
                        <div className="arch-metric-value" style={{ fontSize: "16px", color: "#38bdf8" }}>
                          {snapshot.hostingAdvice?.recommendedPlan || "Standard Compute"}
                        </div>
                      </div>
                      <div className="arch-metric-card">
                        <h4>Minimum RAM</h4>
                        <div className="arch-metric-value" style={{ fontSize: "16px", color: "#4ade80" }}>
                          {snapshot.hostingAdvice?.minimumRam || "1 GB"}
                        </div>
                      </div>
                      <div className="arch-metric-card">
                        <h4>OPcache Memory</h4>
                        <div className="arch-metric-value" style={{ fontSize: "16px", color: "#fde047" }}>
                          {snapshot.hostingAdvice?.opcacheMemory || "128 MB"}
                        </div>
                      </div>
                    </div>

                    <div style={{ background: "var(--bg-card-secondary)", padding: "14px", borderRadius: "8px", border: "1px solid var(--border-card)" }}>
                      <h4 style={{ margin: "0 0 10px 0", color: "#fff" }}>Key Infrastructure Directives:</h4>
                      <ul style={{ margin: 0, paddingLeft: "18px", fontSize: "12px", color: "var(--text-dim)", lineHeight: "1.7" }}>
                        {snapshot.hostingAdvice?.recommendations?.map((r, i) => (
                          <li key={i}>{r}</li>
                        ))}
                      </ul>
                    </div>
                  </div>
                )}

                {reportsSubTab === "readme" && (
                  <div className="workbench-card">
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px" }}>
                      <h3>Generated README Documentation</h3>
                      <button
                        className="btn-tiny primary"
                        disabled={!readmeDraft || !!busy}
                        onClick={() => act("Save README", () => SaveREADME(project.id, readmeDraft))}
                      >
                        ðŸ’¾ Save to README.md
                      </button>
                    </div>

                    <div style={{ marginBottom: "12px", padding: "10px", background: "var(--bg-card-secondary)", borderRadius: "6px", border: "1px solid var(--border-card)" }}>
                      <p style={{ margin: "0 0 8px 0", fontSize: "12px", color: "var(--text-muted)" }}>
                        <strong>Selective README Merge:</strong> Choose which generated sections to merge into your project's existing README without losing custom prose:
                      </p>
                      <div style={{ display: "flex", gap: "12px", flexWrap: "wrap", marginBottom: "8px", fontSize: "12px" }}>
                        {[
                          { id: "requirements", label: "Requirements" },
                          { id: "local development", label: "Local Development" },
                          { id: "infrastructure & deployment profile", label: "Deployment Profile" },
                          { id: "sample endpoints", label: "Sample Endpoints" },
                        ].map((sec) => (
                          <label key={sec.id} style={{ display: "flex", alignItems: "center", gap: "4px", cursor: "pointer" }}>
                            <input
                              type="checkbox"
                              checked={selectedMergeSections.includes(sec.id)}
                              onChange={(e) => {
                                if (e.target.checked) {
                                  setSelectedMergeSections([...selectedMergeSections, sec.id]);
                                } else {
                                  setSelectedMergeSections(selectedMergeSections.filter((s) => s !== sec.id));
                                }
                              }}
                            />
                            <span>{sec.label}</span>
                          </label>
                        ))}
                      </div>
                      <button
                        className="btn-tiny secondary"
                        disabled={!readmeDraft || !!busy}
                        onClick={() =>
                          act("Merge README", async () => {
                            const merged = await MergeREADME(project.id, readmeDraft, selectedMergeSections);
                            setReadmeDraft(merged);
                          })
                        }
                      >
                        Merge Selected Sections
                      </button>
                    </div>
                    <textarea
                      value={readmeDraft}
                      onChange={(e) => setReadmeDraft(e.target.value)}
                      style={{ width: "100%", height: "240px", fontFamily: "monospace", fontSize: "12px", background: "#090d16", color: "#e2e8f0" }}
                    />
                  </div>
                )}

                {reportsSubTab === "manifest" && (
                  <div className="workbench-card">
                    <h3>Export Team Handoff & Support Bundle</h3>
                    <p style={{ fontSize: "12px", color: "var(--text-muted)", marginBottom: "16px" }}>
                      Package environment dependencies, runtime snapshots, and sanitized configuration for seamless team onboarding or support diagnostics.
                    </p>
                    <div style={{ display: "flex", gap: "12px" }}>
                      <button
                        className="btn-action primary"
                        disabled={!!busy}
                        onClick={() =>
                          act("Export manifest", () =>
                            ExportTeamManifest(project.id, project.path + "/workbench.manifest.json"),
                          )
                        }
                      >
                        Export Team Manifest (.json)
                      </button>
                      <button
                        className="btn-action"
                        disabled={!!busy}
                        onClick={() =>
                          act("Export bundle", () =>
                            ExportSupportBundle(project.id, project.path + "/diagnostic_bundle.zip"),
                          )
                        }
                      >
                        Export Diagnostic Bundle (.zip)
                      </button>
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* TAB 10: SETTINGS & TUNNEL */}
        {tab === "settings" && (
          <div className="tab-pane">
            <div className="sub-segmented-nav">
              <button
                className={`sub-tab-pill ${settingsSubTab === "tunnels" ? "active" : ""}`}
                onClick={() => setSettingsSubTab("tunnels")}
              >
                ðŸŒ Client Preview Tunnels & Sharing
              </button>
              <button
                className={`sub-tab-pill ${settingsSubTab === "tooling" ? "active" : ""}`}
                onClick={() => setSettingsSubTab("tooling")}
              >
                ðŸ› ï¸ System Tooling & Dependencies
              </button>
              <button
                className={`sub-tab-pill ${settingsSubTab === "ai" ? "active" : ""}`}
                onClick={() => setSettingsSubTab("ai")}
              >
                ðŸ¤– AI Assistant & MCP Server
              </button>
              <button
                className={`sub-tab-pill ${settingsSubTab === "general" ? "active" : ""}`}
                onClick={() => setSettingsSubTab("general")}
              >
                âš™ï¸ General & Environment
              </button>
            </div>

            {settingsSubTab === "tunnels" && (
              <>
                <div className="workbench-card">
                  <h3>Client Preview Tunnel</h3>
                  <p style={{ fontSize: "12px", color: "var(--text-muted)" }}>
                    Share your local project through a secure tunnel provider. Starting a tunnel creates a publicly reachable HTTPS URL for clients, webhooks, or mobile testing.
                  </p>
                  <div style={{ display: "flex", gap: "10px", marginTop: "14px", alignItems: "center" }}>
                    <span className={`workbench-badge ${state.tunnel.running ? "badge-good" : "badge-info"}`}>
                      {state.tunnel.running ? "Tunnel Active" : "Tunnel Inactive"}
                    </span>
                    {state.tunnel.running ? (
                      <>
                        <a href={state.tunnel.publicUrl} target="_blank" rel="noreferrer" style={{ fontWeight: 600, color: "#38bdf8" }}>
                          {state.tunnel.publicUrl} â†—
                        </a>
                        <button className="btn-tiny" onClick={() => act("Stop tunnel", StopTunnel)}>
                          Stop Tunnel
                        </button>
                      </>
                    ) : (
                      <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
                        <select
                          className="text-input"
                          style={{ padding: "6px 10px", fontSize: "12px", width: "auto" }}
                          value={tunnelProvider}
                          onChange={(e) => setTunnelProvider(e.target.value)}
                        >
                          <option value="cloudflared">Cloudflare (trycloudflare.com)</option>
                          <option value="ngrok">ngrok</option>
                          <option value="expose">Expose</option>
                        </select>
                        <button
                          className="btn-action primary"
                          onClick={() =>
                            act("Start tunnel", () =>
                              StartTunnel(tunnelProvider, project ? project.url : "http://localhost"),
                            )
                          }
                        >
                          Start Preview Tunnel
                        </button>
                      </div>
                    )}
                  </div>
                </div>

                <div className="workbench-card">
                  <h3>Detected Third-Party Integrations</h3>
                  <p style={{ fontSize: "12px", color: "var(--text-muted)", marginBottom: "12px" }}>
                    Services identified from active environment configurations:
                  </p>
                  <table className="workbench-table">
                    <thead>
                      <tr>
                        <th>Service</th>
                        <th>Type</th>
                        <th>Status</th>
                        <th>Environment</th>
                      </tr>
                    </thead>
                    <tbody>
                      {(integrations || []).map((it, idx) => (
                        <tr key={idx}>
                          <td><strong>{it.name}</strong></td>
                          <td>{it.type}</td>
                          <td><span className="workbench-badge badge-good">{it.status}</span></td>
                          <td>{it.environment}</td>
                        </tr>
                      ))}
                      {(!integrations || integrations.length === 0) && (
                        <tr><td colSpan={4} className="empty-text">No external integrations detected in active .env.</td></tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </>
            )}

            {settingsSubTab === "tooling" && (
              <>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px", flexWrap: "wrap", gap: "10px" }}>
                  <div className="sub-segmented-nav" style={{ margin: 0 }}>
                    <button
                      className={`sub-tab-pill ${toolingMode === "deps" ? "active" : ""}`}
                      onClick={() => setToolingMode("deps")}
                    >
                      ðŸ“¦ Installed Tools &amp; Packages ({(state.dependencies || []).length})
                    </button>
                    <button
                      className={`sub-tab-pill ${toolingMode === "doctor" ? "active" : ""}`}
                      onClick={() => setToolingMode("doctor")}
                    >
                      ðŸ›¡ï¸ Best Practices Doctor Setup
                    </button>
                  </div>
                  <button className="btn-toolbar" disabled={!!busy} onClick={load}>
                    â†º Recheck System Binaries
                  </button>
                </div>
                {toolingMode === "doctor" ? (
                  <AuditSetupPanel key={project?.id} project={project} trusted={state.trusted} installing={!!state.installation?.running} reload={load} />
                ) : (
                  <DependencySettings installation={state.installation} items={state.dependencies || []} project={project} trusted={state.trusted} busy={busy || (state.installation?.running ? "Installing tools" : "")} act={act} />
                )}
              </>
            )}

            {settingsSubTab === "ai" && (
              <div className="workbench-card">
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "10px" }}>
                  <h3>Model Context Protocol (MCP) Server</h3>
                  <span className="workbench-badge badge-good">Ready for AI Agents</span>
                </div>
                <p style={{ fontSize: "12px", color: "var(--text-muted)", lineHeight: "1.6" }}>
                  LaraDeck includes an integrated stdio Model Context Protocol (MCP) server. AI pair programmers (such as Cursor, Claude Desktop, Antigravity, or VS Code Cline) can query your routes, run audits, inspect logs, and manage services autonomously.
                </p>

                <div style={{ marginTop: "14px" }}>
                  <small style={{ color: "var(--text-dim)", fontWeight: 600 }}>CLI Daemon Command:</small>
                  <div className="mcp-code-block" style={{ marginTop: "4px" }}>
                    herd-workbench.exe --mcp
                  </div>
                </div>

                <div style={{ marginTop: "16px" }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "6px" }}>
                    <small style={{ color: "var(--text-dim)", fontWeight: 600 }}>Claude Desktop / Cursor Configuration JSON:</small>
                    <button
                      type="button"
                      className="btn-tiny"
                      onClick={async () => {
                        const mcpJson = JSON.stringify({
                          mcpServers: {
                            laradeck: {
                              command: "C:\\Program Files\\LaraDeck\\laradeck.exe",
                              args: ["--mcp"],
                            },
                          },
                        }, null, 2);
                        await copyText(mcpJson);
                        setCopiedMcp("copied");
                        setTimeout(() => setCopiedMcp(null), 2000);
                      }}
                    >
                      {copiedMcp ? "âœ“ Copied Config" : "ðŸ“‹ Copy MCP Config"}
                    </button>
                  </div>
                  <pre className="mcp-code-block" style={{ margin: 0 }}>
{`{
  "mcpServers": {
    "laradeck": {
      "command": "laradeck.exe",
      "args": ["--mcp"]
    }
  }
}`}
                  </pre>
                </div>
              </div>
            )}

            {settingsSubTab === "general" && (
              <div className="workbench-card">
                <h3>General & Project Paths</h3>
                <div className="panel-grid" style={{ marginTop: "14px" }}>
                  <div className="workbench-card">
                    <small style={{ color: "var(--text-dim)" }}>Selected Project</small>
                    <h4 style={{ color: "#fff", margin: "6px 0 2px" }}>{project?.name || "None"}</h4>
                    <p style={{ fontSize: "11px", color: "var(--text-muted)", margin: 0, wordBreak: "break-all" }}>{project?.path || "No project selected"}</p>
                  </div>
                  <div className="workbench-card">
                    <small style={{ color: "var(--text-dim)" }}>Local Web URL</small>
                    <h4 style={{ color: "#38bdf8", margin: "6px 0 2px" }}>{project?.url || "N/A"}</h4>
                    <p style={{ fontSize: "11px", color: "var(--text-muted)", margin: 0 }}>Framework: {project?.framework || "Laravel"}</p>
                  </div>
                  <div className="workbench-card">
                    <small style={{ color: "var(--text-dim)" }}>PHP Runtime</small>
                    <h4 style={{ color: "#a855f7", margin: "6px 0 2px" }}>PHP {project?.php || "System"}</h4>
                    <p style={{ fontSize: "11px", color: "var(--text-muted)", margin: 0 }}>Status: {state.trusted ? "Trusted" : "Untrusted"}</p>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

        {/* TAB: WEBSOCKETS & REVERB */}
        {tab === "realtime" && (
          <div className="tab-pane" style={{ padding: 0, height: "100%" }}>
            <RealtimeReverbPanel projectId={project?.id || ""} />
          </div>
        )}

        {/* TAB: REMOTE SSH & SAFETY VAULT */}
        {tab === "remote" && (
          <div className="tab-pane" style={{ padding: 0, height: "100%" }}>
            <RemoteProfilesPanel />
          </div>
        )}

        {/* TAB: WEBHOOKS WORKBENCH */}
        {tab === "webhooks" && (
          <div className="tab-pane">
            <WebhooksPanel
              projectId={project?.id}
              deliveries={webhookDeliveries}
              localSiteUrl={project?.url || ""}
              onClear={handleClearWebhooks}
              onReplay={async (deliveryId, targetUrl) => {
                return await ReplayWebhook(deliveryId, targetUrl);
              }}
              onDispatchSigned={async (targetUrl, provider, secret, payload) => {
                return await DispatchSignedWebhook(targetUrl, provider, secret, payload);
              }}
              onRefresh={loadWebhooks}
            />
          </div>
        )}
        </ErrorBoundary>
      </main>

      <CommandPalette
        isOpen={paletteOpen}
        onClose={() => setPaletteOpen(false)}
        projects={state.projects}
        selectedProject={project}
        onSelectProject={(id) => select(id)}
        onSelectTab={(nextTab) => switchTab(nextTab)}
        routes={routes}
      />

      {/* Confirmation modal on close with active jobs */}
      {closeCount !== null && (
        <div className="modal-backdrop">
          <section ref={dialog} className="modal" role="dialog" aria-modal="true">
            <h2>Owned Processes Still Running</h2>
            <p>
              There are {closeCount} background process tree(s) still managed by Workbench.
              You can request graceful stop or force exit immediately.
            </p>
            {closeError && <div className="tag error">{closeError}</div>}
            <div className="actions" style={{ display: "flex", gap: "10px", justifyContent: "flex-end" }}>
              <button className="btn" onClick={handleCancelClose}>
                Keep Working
              </button>
              <button className="btn" onClick={() => exit(false)}>
                Request Stop & Exit
              </button>
              <button className="btn primary" onClick={() => exit(true)}>
                Force Stop & Exit
              </button>
            </div>
          </section>
        </div>
      )}

      <ProjectWizardModal
        isOpen={isWizardOpen}
        onClose={() => setIsWizardOpen(false)}
        defaultDirectory="C:\\Users\\hassa\\Herd"
        onSuccess={(_newPath) => {
          setIsWizardOpen(false);
          act("Discover projects", DiscoverProjects);
        }}
        onCreateProject={CreateProjectFromWizard}
      />
    </div>
  );
}
