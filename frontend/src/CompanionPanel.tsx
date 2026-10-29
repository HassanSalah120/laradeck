import { useEffect, useRef, useState } from "react";
import { auditAgeStatus, coverageStatus, releaseReference, versionStatus, type VisualStatus } from "./workspaceStatus";
import PackagesView from "./PackagesView";
import { Companion, WorkspaceHealth, SelectProject, TrustProject, InstallDependency, ExecuteAPIRequest, OpenCompanionDashboard } from "../wailsjs/go/main/App";
import { workbench, testing as wktesting } from "../wailsjs/go/models";

type Props = { state: workbench.State; reload: () => Promise<void> };
type Section = "Workspace" | "Packages" | "Startup" | "Diagnostics" | "Performance" | "Migrations" | "Audit history" | "Timeline" | "Cache & sessions" | "Route tests" | "Snapshots" | "Integrations";
const sections: Section[] = ["Workspace", "Packages", "Startup", "Diagnostics", "Performance", "Migrations", "Audit history", "Timeline", "Cache & sessions", "Route tests", "Snapshots", "Integrations"];

const sectionIcons: Record<Section, string> = {
  Workspace: "🏢",
  Packages: "📦",
  Startup: "⚡",
  Diagnostics: "🩺",
  Performance: "🚀",
  Migrations: "🗄️",
  "Audit history": "🛡️",
  Timeline: "⏱️",
  "Cache & sessions": "⚡",
  "Route tests": "🧪",
  Snapshots: "📸",
  Integrations: "🔌",
};

type Json = Record<string, any>;
const pretty = (value: unknown) => JSON.stringify(value, null, 2);

function Result({ value }: { value: unknown }) {
  const [showRaw, setShowRaw] = useState(false);
  if (value === null || value === undefined) return null;

  if (typeof value === "string") {
    return <pre className="companion-result" tabIndex={0}>{value}</pre>;
  }

  const obj = typeof value === "object" ? (value as Record<string, any>) : null;

  if (obj && Array.isArray(obj.extensions)) {
    return (
      <div className="companion-structured-result" style={{ marginTop: 14 }}>
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
          <h4 style={{ margin: 0, color: "#fff" }}>🩺 Platform &amp; Extension Diagnostics</h4>
          <button className="btn-tiny" onClick={() => setShowRaw(!showRaw)}>{showRaw ? "Formatted View" : "Raw JSON"}</button>
        </div>
        {showRaw ? (
          <pre className="companion-result">{pretty(value)}</pre>
        ) : (
          <div>
            {obj.php && (
              <div style={{ marginBottom: 12, display: "flex", gap: 10, alignItems: "center" }}>
                <span className="badge-pill" style={{ background: "rgba(99, 102, 241, 0.2)", color: "#c7d2fe" }}>PHP {obj.php}</span>
                {obj.os && <span className="badge-pill" style={{ background: "rgba(255, 255, 255, 0.08)", color: "#fff" }}>OS: {obj.os}</span>}
              </div>
            )}
            <small style={{ color: "var(--text-dim)", display: "block", marginBottom: 6, fontWeight: 700 }}>LOADED EXTENSIONS ({obj.extensions.length})</small>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 6, maxHeight: 220, overflowY: "auto", padding: 8, background: "rgba(0,0,0,0.2)", borderRadius: 6 }}>
              {obj.extensions.map((ext: string) => (
                <span key={ext} className="badge-pill" style={{ background: "rgba(99, 102, 241, 0.15)", color: "#c7d2fe", border: "1px solid rgba(99, 102, 241, 0.25)" }}>
                  {ext}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
    );
  }

  return (
    <div className="companion-structured-result" style={{ marginTop: 14 }}>
      <div style={{ display: "flex", justifyContent: "flex-end", marginBottom: 6 }}>
        <button className="btn-tiny" onClick={() => setShowRaw(!showRaw)}>{showRaw ? "Structured View" : "Toggle Raw JSON"}</button>
      </div>
      <pre className="companion-result" tabIndex={0}>{pretty(value)}</pre>
    </div>
  );
}

function StatusBadge({ tone, label, detail }: VisualStatus) {
  const mark = tone === "good" ? "m5 12 4 4L19 6" : tone === "danger" ? "m7 7 10 10M17 7 7 17" : tone === "warn" ? "M12 5v9m0 4h.01" : "M7 12h10";
  return <span className={`workspace-status status-${tone}`} title={detail} aria-label={detail}>
    <svg viewBox="0 0 24 24" aria-hidden="true" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d={mark}/></svg>{label}
  </span>;
}

function WorkspaceRow({ health: h, select }: { health: workbench.ProjectHealth; select: () => void }) {
  const attention = h.checks.filter(c => c.status === "attention").length;
  const failed = h.checks.filter(c => c.status === "failed" || c.status === "error").length;
  const unchecked = h.checks.filter(c => c.status !== "ready" && c.status !== "attention" && c.status !== "failed" && c.status !== "error").length;
  const audit = auditAgeStatus(h.lastAudit);
  return <tr>
    <td><button className="record-sort" onClick={select} style={{ fontWeight: 700, color: "#fff" }}>{h.project.name}</button><small className="companion-path">{h.project.path}</small>
      <div className="workspace-cell-statuses"><StatusBadge tone={h.project.secured ? "good" : "warn"} label={h.project.secured ? "HTTPS" : "HTTP"} detail={h.project.secured ? "Herd HTTPS is configured for this site." : "This Herd site is using HTTP."}/>
      {!h.trusted && <StatusBadge tone="neutral" label="Untrusted" detail="Project code execution requires your trust. Static checks remain available."/>}</div>
    </td>
    <td><div className="workspace-versions"><StatusBadge {...versionStatus("Laravel", h.project.framework)}/><StatusBadge {...versionStatus("PHP", h.project.php)}/></div></td>
    {[h.composer, h.npm].map((s, i) => <td key={i}><StatusBadge tone={s?.status === "ready" ? "good" : s?.status === "attention" ? "warn" : s?.status === "danger" ? "danger" : "neutral"} label={s?.label || "Not checked"} detail={s?.detail || "Open Companion → Packages to inspect dependencies."}/></td>)}
    <td><details className="workspace-readiness"><summary>
      <StatusBadge tone={failed ? "danger" : attention ? "warn" : unchecked ? "neutral" : "good"} label={failed ? `${failed} failed` : attention ? `${attention} attention` : unchecked ? "Checks pending" : "Ready"} detail={failed ? `${failed} readiness checks failed.` : attention ? `${attention} static checks need attention.` : unchecked ? "Available checks passed, but some checks have not run." : "All reported readiness checks passed."}/>
      {!!unchecked && <span className="workspace-unchecked">{unchecked} unchecked</span>}
    </summary>{h.checks.map(c => <div key={c.name}><StatusBadge tone={c.status === "ready" ? "good" : c.status === "attention" ? "warn" : c.status === "failed" || c.status === "error" ? "danger" : "neutral"} label={c.status.replaceAll("_", " ")} detail={c.detail || c.name}/> {c.name}<small className="companion-path">{c.detail || "Not detected"}</small></div>)}</details></td>
    <td><StatusBadge tone={h.running > 0 ? "good" : "neutral"} label={h.running > 0 ? `${h.running} running` : "Stopped"} detail={h.running > 0 ? `${h.running} project processes owned by LaraDeck are running.` : "No LaraDeck-owned project processes are running; this is not an error."}/></td>
    <td><div className="workspace-cell-statuses"><StatusBadge {...audit}/>{h.lastAudit && <StatusBadge tone={h.findings > 0 ? "warn" : "neutral"} label={`${h.findings} findings`} detail={h.findings > 0 ? `${h.findings} findings were reported; severity and review status are available in Quality & Audit.` : "No findings were reported. Check analyzer coverage before interpreting this result."}/>}</div></td>
    <td><StatusBadge {...coverageStatus(h.coverage)}/></td>
  </tr>;
}

export default function CompanionPanel({ state, reload }: Props) {
  const [section, setSection] = useState<Section>("Workspace");
  const [health, setHealth] = useState<workbench.ProjectHealth[]>([]);
  const [data, setData] = useState<any>(null);
  const [busy, setBusy] = useState("");
  const [error, setError] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [pending, setPending] = useState<{ label: string; target: string; detail: string; execute: () => Promise<unknown> } | null>(null);
  const [plan, setPlan] = useState<Json | null>(null);
  const [profiles, setProfiles] = useState<Json[]>([]);
  const [profile, setProfile] = useState<Json>({ name: "Development", vite: true, queue: false, scheduler: false, services: [], vitePort: 5173 });
  const [request, setRequest] = useState("");
  const [batch, setBatch] = useState("");
  const [store, setStore] = useState("cache");
  const [search, setSearch] = useState("");
  const [cursor, setCursor] = useState<string | number>(0);
  const [routes, setRoutes] = useState<Json[]>([]);
  const [route, setRoute] = useState<Json | null>(null);
  const [uri, setUri] = useState("/");
  const [method, setMethod] = useState("GET");
  const [body, setBody] = useState("");
  const [headers, setHeaders] = useState('{"Accept":"application/json"}');
  const [framework, setFramework] = useState("pest");
  const [status, setStatus] = useState("200");
  const [draft, setDraft] = useState("");
  const [testName, setTestName] = useState("RouteCompanionTest.php");
  const [authenticated, setAuthenticated] = useState(false);
  const [tableFilter, setTableFilter] = useState("");
  const [snapshots, setSnapshots] = useState<Json[]>([]);
  const [tool, setTool] = useState("");
  const [backupTool, setBackupTool] = useState("");
  const generation = useRef(0);
  const project = state.projects.find(p => p.id === state.selected);
  const id = project?.id || "";
  const captures = state.capture.projectId === id ? state.capture.requests || [] : [];
  const api = (op: string, input: Json = {}) => Companion(id, op, input);

  // Invalidate promises and confirmations when either context changes.
  useEffect(() => {
    generation.current++;
    setData(null); setError(""); setBusy(""); setPending(null); setPlan(null); setConfirmation("");
    setProfile({ name: "Development", vite: true, queue: false, scheduler: false, services: [], vitePort:5173 });
    setUri("/");setMethod("GET");setHeaders('{"Accept":"application/json"}');setBody("");setStatus("200");setTestName("RouteCompanionTest.php");setAuthenticated(false);setTableFilter("");
    setProfiles([]); setSnapshots([]); setRoutes([]); setRoute(null); setDraft(""); setRequest(""); setBatch("");setCursor(0);setSearch("");setTool("");setBackupTool("");
  }, [id, section]);

  async function run(label: string, action: () => Promise<any>, accept: (result: any) => void = setData) {
    const version = generation.current;
    setBusy(label); setError("");
    try { const result = await action(); if (version === generation.current) accept(result); }
    catch (e) { if (version === generation.current) setError(String(e)); }
    finally { if (version === generation.current) { setBusy(""); await reload(); } }
  }
  useEffect(() => { if (section === "Workspace") void run("Checking projects", WorkspaceHealth, setHealth); }, [section, state.projects.length]);
  useEffect(() => { if (section === "Packages" && id && state.trusted) void run("Loading packages", () => api("package-inspect")); }, [section, id, state.trusted]);
  const confirm = (label: string, target: string, detail: string, execute: () => Promise<unknown>) => { setConfirmation(""); setPending({ label, target, detail, execute }); };
  const button = (label: string, op: string, input: Json = {}) => <button className="btn-toolbar" disabled={!!busy} onClick={() => run(label, () => api(op, input))}>{label}</button>;
  const target = data?.target ? `${data.target.connection} / ${data.target.database}` : "";

  // Summary counts for workspace view
  const readyProjects = health.filter(h => !h.checks.some(c => c.status === "failed" || c.status === "error")).length;
  const attentionProjects = health.filter(h => h.checks.some(c => c.status === "attention")).length;
  const runningProcesses = health.reduce((acc, h) => acc + (h.running || 0), 0);

  return <div className="companion">
    <header className="companion-heading">
      <div>
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 4 }}>
          <h2 style={{ margin: 0, fontSize: "20px", fontWeight: 700, color: "#fff" }}>Laravel Companion</h2>
          <span style={{ fontSize: "11px", background: "rgba(99, 102, 241, 0.2)", color: "#c7d2fe", border: "1px solid rgba(99, 102, 241, 0.35)", padding: "2px 8px", borderRadius: 5, fontWeight: 600 }}>
            WORKBENCH v1.0
          </span>
        </div>
        <p style={{ margin: 0, color: "var(--text-muted)", fontSize: "13px" }}>
          Workspace ecosystem health, dependency audits, runtime diagnostics, and safe developer utilities.
        </p>
      </div>
      <div style={{ display: "flex", gap: 8 }}>
        <span className="badge-pill" style={{ background: "rgba(255, 255, 255, 0.06)", border: "1px solid rgba(255, 255, 255, 0.12)", color: "#fff" }}>
          📁 {state.projects.length} Discovered Sites
        </span>
      </div>
    </header>

    {/* Modern Segmented Navigation Pill Strip */}
    <nav className="companion-tabs" aria-label="Companion tools">
      {sections.map(s => (
        <button
          key={s}
          className={`btn-toolbar ${section === s ? "active" : ""}`}
          aria-current={section === s ? "page" : undefined}
          onClick={() => setSection(s)}
          style={{ display: "inline-flex", alignItems: "center", gap: "6px" }}
        >
          <span>{sectionIcons[s]}</span>
          <span>{s}</span>
        </button>
      ))}
    </nav>

    {error && <div className="companion-alert" role="alert">{error}</div>}
    {busy && <p role="status" style={{ color: "#fbbf24", fontWeight: 600 }}>⏳ {busy}…</p>}

    {section === "Workspace" ? (
      <>
        {/* Workspace Health Overview Cards */}
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 14 }}>
          <div className="stat-deck-card indigo" style={{ padding: "14px 18px" }}>
            <span className="stat-deck-label">Discovered Projects</span>
            <div style={{ fontSize: "24px", fontWeight: 800, color: "#fff", marginTop: 4 }}>{health.length}</div>
            <div style={{ fontSize: "11px", color: "var(--text-dim)", marginTop: 2 }}>Herd sites in active workspace</div>
          </div>
          <div className="stat-deck-card emerald" style={{ padding: "14px 18px" }}>
            <span className="stat-deck-label">Ready Status</span>
            <div style={{ fontSize: "24px", fontWeight: 800, color: "#34d399", marginTop: 4 }}>{readyProjects}</div>
            <div style={{ fontSize: "11px", color: "var(--text-dim)", marginTop: 2 }}>Passing all baseline health checks</div>
          </div>
          <div className="stat-deck-card amber" style={{ padding: "14px 18px" }}>
            <span className="stat-deck-label">Needs Attention</span>
            <div style={{ fontSize: "24px", fontWeight: 800, color: "#fbbf24", marginTop: 4 }}>{attentionProjects}</div>
            <div style={{ fontSize: "11px", color: "var(--text-dim)", marginTop: 2 }}>Outdated packages or missing audit</div>
          </div>
          <div className="stat-deck-card sky" style={{ padding: "14px 18px" }}>
            <span className="stat-deck-label">Active Processes</span>
            <div style={{ fontSize: "24px", fontWeight: 800, color: "#38bdf8", marginTop: 4 }}>{runningProcesses}</div>
            <div style={{ fontSize: "11px", color: "var(--text-dim)", marginTop: 2 }}>Managed background runners</div>
          </div>
        </div>

        <section className="workbench-card">
          <div className="companion-row" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
            <h3 style={{ margin: 0, fontSize: "15px", color: "#fff" }}>Project Readiness &amp; Runtime Overview</h3>
            <button className="btn-toolbar primary" disabled={!!busy} onClick={() => run("Checking projects", WorkspaceHealth, setHealth)}>
              ↻ Refresh Health
            </button>
          </div>
          <p style={{ color: "var(--text-muted)", fontSize: "12.5px", marginBottom: 14 }}>
            Static checks inspect configuration and lockfiles without booting project code. Click any project to inspect diagnostics.
          </p>
          <div className="workspace-legend" aria-label="Status color key">
            <span><i className="legend-dot good"/> Current / ready</span>
            <span><i className="legend-dot warn"/> Previous / attention</span>
            <span><i className="legend-dot danger"/> Older / blocked</span>
            <span><i className="legend-dot neutral"/> Unchecked / inactive</span>
          </div>
          <div className="companion-table-scroll">
            <table className="workbench-table workspace-health-table">
              <thead>
                <tr>
                  <th>Project</th>
                  <th>Laravel / PHP</th>
                  <th>Composer</th>
                  <th>npm</th>
                  <th>Readiness</th>
                  <th>Processes</th>
                  <th>Last audit</th>
                  <th>Coverage</th>
                </tr>
              </thead>
              <tbody>
                {health.map(h => (
                  <WorkspaceRow
                    key={h.project.id}
                    health={h}
                    select={() => void run("Selecting project", async () => {
                      await SelectProject(h.project.id);
                      await reload();
                      setSection("Diagnostics");
                    })}
                  />
                ))}
              </tbody>
            </table>
          </div>
          <p className="workspace-reference" style={{ marginTop: 12, fontSize: "11px", color: "var(--text-dim)" }}>
            Version colors compare release branches: Laravel {releaseReference.laravel} · PHP {releaseReference.php.join(".")} · checked {releaseReference.checked}. Hover a badge for details.
          </p>
          {health.length === 0 && !busy && <p style={{ padding: 24, textAlign: "center", color: "var(--text-dim)" }}>No discovered projects. Refresh Herd discovery first.</p>}
        </section>
      </>
    ) : !project ? (
      <div className="empty-project-prompt">
        <div className="empty-project-icon">📦</div>
        <h3>Select a Project to Inspect with Companion</h3>
        <p>
          Choose an active Laravel site below to run deep diagnostics, package audits, cache purges, or test generation:
        </p>
        <div className="empty-project-grid">
          {state.projects.map((p) => (
            <button
              key={p.id}
              className="empty-project-tile"
              onClick={() => void run("Selecting project", async () => { await SelectProject(p.id); await reload(); })}
            >
              <div className="project-card-header">
                <span className="project-card-name">{p.name}</span>
                <span className="project-card-tag">PHP {p.php}</span>
              </div>
              <div className="project-card-path">{p.path}</div>
              <div style={{ fontSize: "11px", color: "#6366f1", marginTop: "8px", fontWeight: 600 }}>
                Inspect {p.name} in Companion &rarr;
              </div>
            </button>
          ))}
          {state.projects.length === 0 && (
            <p style={{ textAlign: "center", gridColumn: "1 / -1", color: "var(--text-dim)", padding: 20 }}>
              No Herd sites discovered. Refresh discovery from the left sidebar.
            </p>
          )}
        </div>
      </div>
    ) : !state.trusted ? (
      <div className="workbench-card" style={{ textAlign: "center", padding: "40px 24px" }}>
        <div style={{ fontSize: "36px", marginBottom: 10 }}>🛡️</div>
        <h3 style={{ color: "#fff", margin: 0 }}>Session Untrusted for {project.name}</h3>
        <p style={{ color: "var(--text-muted)", maxWidth: 500, margin: "8px auto 18px", fontSize: "13px" }}>
          Trust {project.name} to execute companion commands, artisan scripts, and local processes safely.
        </p>
        <button
          className="btn-toolbar primary"
          onClick={() => void run("Trusting project", async () => { await TrustProject(project.id); await reload(); })}
        >
          🛡️ Trust {project.name} for this Session
        </button>
      </div>
    ) : (
      <>
        <div className="companion-context" style={{ background: "rgba(255, 255, 255, 0.04)", border: "1px solid rgba(255, 255, 255, 0.08)", padding: "10px 14px", borderRadius: 8 }}>
          <strong style={{ color: "#fff" }}>Active Context: {project.name}</strong>
          <span>📁 {project.path}</span>
          <span>⚡ Local / testing environments only</span>
        </div>

        {section === "Packages" && (
          <>
            <section className="workbench-card">
              <h3>Composer &amp; npm Dependency Health</h3>
              <p>Inspect lockfiles, installed versions, PHP requirements, and known advisories.</p>
              <div className="companion-row" style={{ marginTop: 12, display: "flex", gap: 8, flexWrap: "wrap" }}>
                {button("Refresh inventory", "package-inspect")}
                {button("Run local checks", "package-scan", { online: false })}
                <button className="btn-toolbar primary" disabled={!!busy} onClick={() => confirm("Check package registries", project.name, "Sends dependency names and versions to the project's configured Composer/npm registries for update and vulnerability checks.", () => api("package-scan", { online: true }))}>
                  Check updates &amp; advisories
                </button>
              </div>
            </section>
            <PackagesView key={id} report={data?.composer && data?.npm ? data : null}/>
          </>
        )}

        {section === "Startup" && (
          <section className="workbench-card">
            <h3>Saved Development Profiles</h3>
            <div className="companion-row" style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap", marginBottom: 14 }}>
              <button className="btn-toolbar" disabled={!!busy} onClick={() => run("Loading profiles", () => api("profile-load"), setProfiles)}>Load profiles</button>
              {profiles.map(p => <button className="btn-tiny" key={p.name} onClick={() => setProfile(p)}>{p.name}</button>)}
            </div>
            <div className="companion-form">
              <label>Profile name<input value={profile.name} onChange={e => setProfile({ ...profile, name: e.target.value })}/></label>
              <label>Vite port<input type="number" min="1" max="65535" value={profile.vitePort} onChange={e => setProfile({ ...profile, vitePort: +e.target.value })}/></label>
            </div>
            <div className="companion-row" style={{ display: "flex", gap: 16, margin: "14px 0" }}>
              {["vite", "queue", "scheduler"].map(key => (
                <label key={key} style={{ display: "inline-flex", alignItems: "center", gap: 6, cursor: "pointer" }}>
                  <input type="checkbox" checked={profile[key]} onChange={e => setProfile({ ...profile, [key]: e.target.checked })}/>
                  {key === "vite" ? "⚡ Vite Dev Server" : key === "queue" ? "⚙️ Queue Worker" : "⏱️ Task Scheduler"}
                </label>
              ))}
            </div>
            <fieldset style={{ border: "1px solid rgba(255, 255, 255, 0.08)", borderRadius: 8, padding: 12 }}>
              <legend style={{ color: "var(--text-dim)", fontSize: "11.5px", fontWeight: 700 }}>SHARED LOCAL SERVICES</legend>
              <div style={{ display: "flex", gap: 14, flexWrap: "wrap" }}>
                {state.services.map(service => (
                  <label key={service.id} style={{ display: "inline-flex", alignItems: "center", gap: 6, cursor: "pointer" }}>
                    <input type="checkbox" checked={profile.services.includes(service.id)} onChange={e => setProfile({ ...profile, services: e.target.checked ? [...profile.services, service.id] : profile.services.filter((s: string) => s !== service.id) })}/>
                    {service.name} ({service.status})
                  </label>
                ))}
              </div>
            </fieldset>
            <div className="companion-row" style={{ display: "flex", gap: 8, flexWrap: "wrap", marginTop: 14 }}>
              <button className="btn-toolbar" disabled={!!busy} onClick={() => run("Saving profile", () => api("profile-save", profile), setProfiles)}>Save profile</button>
              <button className="btn-toolbar primary" disabled={!!busy} onClick={() => confirm("Start profile", project.name, "Starts the saved profile, including selected workers and scheduled tasks.", () => api("profile-start", { name: profile.name }))}>Start profile</button>
              {button("Stop profile", "profile-stop")}
              <button className="btn-tiny danger" disabled={!!busy} onClick={()=>confirm("Force stop profile",project.name,"Immediately terminates only this profile’s owned processes.",()=>api("profile-stop",{force:true}))}>Force stop</button>
              {button("Profile status", "profile-status")}
            </div>
            <Result value={data}/>
            <h4 style={{ marginTop: 18, color: "#fff" }}>Owned Project Processes</h4>
            {state.runs.filter(r => r.projectId === id).map(r => (
              <div className="companion-process" key={r.id}>
                <strong>{r.name}</strong>
                <span>{r.running ? `Running · PID ${r.pid}` : `Exited · ${r.exitCode}`}</span>
                <details><summary>Output</summary><Result value={r.output?.join("\n")}/></details>
              </div>
            ))}
          </section>
        )}

        {section === "Diagnostics" && (
          <section className="workbench-card">
            <h3>Diagnose &amp; Repair</h3>
            <p>Inspect loaded PHP extensions, cached configuration, and runtime drivers.</p>
            <div className="companion-row" style={{ display: "flex", gap: 8, flexWrap: "wrap", margin: "14px 0" }}>
              {button("Inspect runtime", "runtime")}
              {button("Check PHP requirements", "platform-check")}
              <button className="btn-toolbar" disabled={!!busy} onClick={()=>confirm("Install or repair Clockwork",project.name,"Adds or repairs Clockwork development setup through Composer.",()=>InstallDependency("clockwork",id))}>
                Install / repair Clockwork
              </button>
              {["composer-install", "npm-install", "asset-build", "config-clear"].map(kind => (
                <button className="btn-toolbar" key={kind} disabled={!!busy} onClick={() => run("Preparing repair", () => api("repair-preview", { kind }), setPlan)}>
                  Preview {kind.replaceAll("-", " ")}
                </button>
              ))}
            </div>
            <Result value={data}/>
          </section>
        )}

        {section === "Performance" && (
          <section className="workbench-card">
            <h3>Performance Baseline Comparison</h3>
            <p>Pin a baseline, make code improvements, capture the same route, and compare delta.</p>
            <CaptureSelect captures={captures} value={request} change={setRequest}/>
            <div className="companion-row" style={{ display: "flex", gap: 8, marginTop: 12 }}>
              {button("Pin baseline", "performance-pin", { request })}
              {button("Compare with baseline", "performance-compare", { request })}
              {button("Show baseline", "performance-baseline")}
            </div>
            <Result value={data}/>
          </section>
        )}

        {section === "Migrations" && (
          <section className="workbench-card">
            <h3>Database Schema &amp; Migrations</h3>
            <p>Inspect the database and preview migrations with SQL pretend mode.</p>
            <div className="companion-row" style={{ marginBottom: 12 }}>
              <label style={{ flex: 1, maxWidth: 380 }}>
                Table filter (optional)
                <input value={tableFilter} onChange={e => setTableFilter(e.target.value)} placeholder="Filter table names..."/>
              </label>
            </div>
            <div className="companion-row" style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 14 }}>
              {button("Inspect migrations", "migrations", { tableFilter })}
              {button("Preview pending SQL", "migration-preview", { tableFilter })}
              {button("Save schema baseline", "schema-pin", { tableFilter })}
              {button("Compare schema", "schema-diff", { tableFilter })}
            </div>
            {data?.totalTables !== undefined && (
              <p style={{ color: "#38bdf8", fontWeight: 600 }}>Total tables in database: {data.totalTables.toLocaleString()}</p>
            )}
            {data?.note && <div className="companion-alert" role="alert">{data.note}</div>}
            {target && (
              <div className="companion-row" style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 12, padding: "10px 14px", background: "rgba(255, 255, 255, 0.04)", borderRadius: 8 }}>
                <strong>Target: {target}</strong>
                {["migrate", "rollback"].map(op => (
                  <button className="btn-toolbar" disabled={!!busy} key={op} onClick={() => confirm(op === "migrate" ? "Apply pending migrations" : "Rollback one migration", target, "Changes this database schema. Take a snapshot first.", () => api(op, { confirmation: target }))}>
                    {op === "migrate" ? "Apply pending migrations" : "Rollback one migration"}
                  </button>
                ))}
              </div>
            )}
            {data?.migrations && (
              <div className="companion-table-scroll" style={{ marginTop: 14 }}>
                <table className="workbench-table">
                  <thead>
                    <tr><th>Migration Name</th><th>Status</th></tr>
                  </thead>
                  <tbody>
                    {data.migrations.map((m: Json) => (
                      <tr key={m.name}>
                        <td><code>{m.name}</code></td>
                        <td>
                          <span className={`workbench-badge ${m.status === "ran" ? "badge-good" : "badge-warn"}`}>
                            {m.status}
                          </span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <Result value={data && !data.migrations ? data : data?.schema ? { target: data.target, schema: data.schema } : null}/>
          </section>
        )}

        {section === "Audit history" && (
          <section className="workbench-card">
            <h3>Audit Scans History &amp; Trends</h3>
            <p>Retains local scan history (latest 20) to compare architectural drift over time.</p>
            <div style={{ margin: "12px 0" }}>{button("Load scan history", "audit-history")}</div>
            {data?.reports && (
              <>
                <table className="workbench-table">
                  <thead>
                    <tr><th>Finished</th><th>Findings</th><th>Coverage</th></tr>
                  </thead>
                  <tbody>
                    {data.reports.map((r: Json) => (
                      <tr key={r.startedAt}>
                        <td>{r.finishedAt}</td>
                        <td><span className="badge-warn">{r.findings.length} findings</span></td>
                        <td>{r.analyzers.filter((a: Json) => a.status === "completed").length}/{r.analyzers.length} completed</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </>
            )}
          </section>
        )}

        {section === "Timeline" && (
          <section className="workbench-card">
            <h3>Recorded Telemetry Timeline</h3>
            <CaptureSelect captures={captures} value={request} change={setRequest}/>
            <div style={{ marginTop: 10 }}>{button("Inspect Clockwork request", "timeline", { request })}</div>
            <hr style={{ margin: "18px 0", borderColor: "rgba(255, 255, 255, 0.08)" }}/>
            <div className="companion-row" style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
              {button("Recent Telescope requests", "telescope")}
              <label style={{ display: "inline-flex", alignItems: "center", gap: 8 }}>
                Telescope batch ID:
                <input value={batch} onChange={e => setBatch(e.target.value)} style={{ width: 140 }}/>
              </label>
              {button("Load recorded batch", "telescope", { batch })}
            </div>
            {data?.entries ? (
              <table className="workbench-table" style={{ marginTop: 14 }}>
                <thead><tr><th>Time</th><th>Type</th><th>Summary</th><th>Batch</th></tr></thead>
                <tbody>
                  {data.entries.map((entry: Json) => (
                    <tr key={entry.id}>
                      <td>{entry.time}</td>
                      <td><span className="tag info">{entry.type}</span></td>
                      <td>{entry.summary}</td>
                      <td><button className="record-sort" onClick={() => { setBatch(entry.batch); void run("Loading batch", () => api("telescope", { batch: entry.batch })); }}>{entry.batch}</button></td>
                    </tr>
                  ))}
                </tbody>
              </table>
            ) : <Result value={data}/>}
          </section>
        )}

        {section === "Cache & sessions" && (
          <section className="workbench-card">
            <h3>Cache &amp; Session Inspector</h3>
            <p>Values stay masked until explicitly inspected. Delete individual keys safely.</p>
            <div className="companion-row" style={{ display: "flex", gap: 12, alignItems: "center", flexWrap: "wrap", margin: "14px 0" }}>
              <label>Store:
                <select value={store} onChange={e => {setStore(e.target.value);setCursor(0);setData(null);}} style={{ marginLeft: 6 }}>
                  <option value="cache">Cache</option>
                  <option value="sessions">Sessions</option>
                </select>
              </label>
              <label>Find identifier:
                <input value={search} onChange={e => {setSearch(e.target.value);setCursor(0);}} placeholder="Key prefix..." style={{ marginLeft: 6 }}/>
              </label>
              <button className="btn-toolbar primary" disabled={!!busy} onClick={() => run("Reading store", () => api(store, { search, cursor }))}>Read entries</button>
            </div>
            {data?.entries && (
              <>
                <p style={{ color: "#38bdf8", fontSize: "12px" }}>Driver: {data.driver} · {data.note}</p>
                <div className="companion-table-scroll">
                  <table className="workbench-table">
                    <thead><tr><th>Key / ID</th><th>TTL (s)</th><th>Value</th><th>Actions</th></tr></thead>
                    <tbody>
                      {data.entries.map((entry: Json) => (
                        <tr key={entry.key}>
                          <td><code>{entry.key}</code></td>
                          <td>{entry.ttl}</td>
                          <td><pre className="companion-value">{entry.value}</pre></td>
                          <td>
                            <div className="companion-row" style={{ display: "flex", gap: 6 }}>
                              <button className="btn-tiny" disabled={!!busy} onClick={() => run("Inspecting selected value", () => api(store, { search, cursor, key: entry.key, reveal: true }))}>Reveal</button>
                              <button className="btn-tiny danger" disabled={!!busy} onClick={() => confirm("Delete entry", entry.key, "Deletes this cache entry or session.", async () => { await api(store === "cache" ? "cache-delete" : "session-delete", { key: entry.key, confirmation: entry.key }); return api(store, { search, cursor }); })}>Delete</button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </section>
        )}

        {section === "Route tests" && (
          <section className="workbench-card">
            <h3>Route Test Generator</h3>
            <p>Send local requests and scaffold matching Pest / PHPUnit test cases in one click.</p>
            <div style={{ display: "flex", gap: 10, alignItems: "center", margin: "12px 0" }}>
              <button className="btn-toolbar" disabled={!!busy} onClick={() => run("Loading routes", () => api("routes"), value => setRoutes(value.routes))}>Load project routes</button>
              <select aria-label="Route" value={route ? `${route.methods.join("|")}:${route.uri}` : ""} onChange={e => {const selected = routes.find(r => `${r.methods.join("|")}:${r.uri}` === e.target.value);if(selected){setRoute(selected);setUri("/"+selected.uri.replace(/^\//,""));setMethod(selected.methods[0]);}}} style={{ flex: 1, maxWidth: 420 }}>
                <option value="">Choose route endpoint</option>
                {routes.map(r => <option key={`${r.methods.join("|")}:${r.uri}`} value={`${r.methods.join("|")}:${r.uri}`}>{r.methods.join("|")} /{r.uri}</option>)}
              </select>
            </div>
            <div className="companion-form">
              <label>Method<select value={method} onChange={e => setMethod(e.target.value)}>{["GET","POST","PUT","PATCH","DELETE","OPTIONS","HEAD"].map(m => <option key={m}>{m}</option>)}</select></label>
              <label>URI Path<input value={uri} onChange={e => setUri(e.target.value)}/></label>
              <label>Expected Status<input value={status} onChange={e => setStatus(e.target.value)}/></label>
            </div>
            <div className="companion-row" style={{ display: "flex", gap: 10, marginTop: 14 }}>
              <button className="btn-toolbar primary" disabled={!!busy} onClick={() => run("Generating test draft", () => api("test-draft", { method, uri, framework, status, headers, body, authenticated }), setDraft)}>
                Generate test draft
              </button>
            </div>
            {draft && (
              <div style={{ marginTop: 16 }}>
                <label>Pest / PHPUnit Test Draft:<textarea className="companion-editor" value={draft} onChange={e => setDraft(e.target.value)} spellCheck={false}/></label>
                <div style={{ display: "flex", gap: 10, marginTop: 10, alignItems: "center" }}>
                  <input value={testName} onChange={e => setTestName(e.target.value)} style={{ width: 260 }}/>
                  <button className="btn-toolbar primary" disabled={!!busy} onClick={() => confirm("Save test file", project.name, `Creates tests/Feature/${testName}.`, () => api("test-save", { method, uri, framework, status, headers, body, authenticated, name:testName, content:draft }))}>
                    Save Test to Disk
                  </button>
                </div>
              </div>
            )}
          </section>
        )}

        {section === "Snapshots" && (
          <section className="workbench-card">
            <h3>Database Snapshots</h3>
            <p>Create instantaneous local database backups before risky migrations or experiments.</p>
            <div className="companion-row" style={{ display: "flex", gap: 8, margin: "14px 0" }}>
              <button className="btn-toolbar" disabled={!!busy} onClick={() => run("Loading snapshots", () => api("snapshots"), setSnapshots)}>Refresh snapshots</button>
              <button className="btn-toolbar primary" disabled={!!busy} onClick={() => confirm("Create database snapshot", project.name, "Reads database into a private snapshot file.", async () => {const result=await api("snapshot-create",{tool});setSnapshots(await api("snapshots"));return result;})}>
                + Create snapshot now
              </button>
            </div>
            <div className="companion-table-scroll">
              <table className="workbench-table">
                <thead>
                  <tr><th>Created</th><th>Database</th><th>Engine</th><th>Size</th><th>Restore</th></tr>
                </thead>
                <tbody>
                  {snapshots.map(s => {
                    const label = `${s.target.connection} / ${s.target.database}`;
                    return (
                      <tr key={s.id}>
                        <td>{s.created}</td>
                        <td><strong>{label}</strong></td>
                        <td>{s.driver}</td>
                        <td>{s.bytes.toLocaleString()} bytes</td>
                        <td>
                          <button className="btn-tiny danger" disabled={!!busy} onClick={() => confirm("Restore database snapshot", label, "Replaces current database contents.", () => api("snapshot-restore", { snapshot: s.id, confirmation: label, tool, backupTool }))}>
                            Restore
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </section>
        )}

        {section === "Integrations" && (
          <section className="workbench-card">
            <h3>Laravel Telescope &amp; Pulse Integrations</h3>
            <p>Setup, monitor, and launch first-party debugging suites.</p>
            <div style={{ margin: "12px 0" }}>{button("Inspect integrations", "integrations")}</div>
            {Array.isArray(data) && data.map((item: Json) => (
              <div className="dependency-row" key={item.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "14px 0", borderBottom: "1px solid rgba(255, 255, 255, 0.08)" }}>
                <div>
                  <h4 style={{ margin: 0, color: "#fff" }}>{item.id === "telescope" ? "Laravel Telescope" : "Laravel Pulse"}</h4>
                  <p style={{ margin: "4px 0", color: "var(--text-muted)", fontSize: "12px" }}>{item.installed ? `Installed ${item.version}` : "Not installed"}. {item.note}</p>
                </div>
                <div style={{ display: "flex", gap: 8 }}>
                  <button className="btn-tiny primary" disabled={!!busy || item.installed} onClick={() => confirm("Install integration", project.name, "Adds Composer dependency.", () => InstallDependency(item.id, id))}>Install</button>
                  <button className="btn-tiny" disabled={!!busy || !item.installed} onClick={() => run("Opening dashboard", () => OpenCompanionDashboard(id, item.id))}>Open Dashboard ↗</button>
                </div>
              </div>
            ))}
          </section>
        )}

        {plan && (
          <section className="workbench-card">
            <h3>Review Operation Plan</h3>
            <code>{plan.command}</code>
            <p style={{ margin: "10px 0" }}>{plan.impact}</p>
            <div className="companion-row" style={{ display: "flex", gap: 8 }}>
              <button className="btn-toolbar primary" disabled={!!busy} onClick={() => confirm("Apply reviewed operation", project.name, plan.impact, () => api("repair-apply", { plan: plan.id, confirmation: project.name }))}>Apply operation</button>
              <button className="btn-tiny" onClick={() => setPlan(null)}>Dismiss</button>
            </div>
          </section>
        )}
      </>
    )}

    {pending && (
      <div className="modal-backdrop">
        <section role="dialog" aria-modal="true" aria-labelledby="companion-confirm-title" className="dialog-card">
          <h2 id="companion-confirm-title">{pending.label}</h2>
          <p>{pending.detail}</p>
          <label>Type <strong>{pending.target}</strong> to confirm<input autoFocus value={confirmation} onChange={e => setConfirmation(e.target.value)} style={{ marginTop: 6 }}/></label>
          <div className="dialog-actions">
            <button className="btn" onClick={() => setPending(null)}>Cancel</button>
            <button className="btn primary" disabled={!!busy || confirmation !== pending.target} onClick={() => {const action=pending;setPending(null);void run(action.label,action.execute);}}>Confirm</button>
          </div>
        </section>
      </div>
    )}
  </div>;
}

function CaptureSelect({ captures, value, change }: { captures: workbench.Request[]; value: string; change: (id: string) => void }) {
  return (
    <label style={{ display: "block", margin: "10px 0" }}>
      Captured request:
      <select value={value} onChange={e => change(e.target.value)} style={{ display: "block", width: "100%", marginTop: 4 }}>
        <option value="">Choose captured request</option>
        {captures.map(r => (
          <option key={r.id} value={r.id}>
            {r.method} {r.uri} · {r.duration.toFixed(1)} ms · {r.id}
          </option>
        ))}
      </select>
      {captures.length === 0 && <small style={{ color: "var(--text-dim)", display: "block", marginTop: 4 }}>Refresh Clockwork captures in Telemetry first.</small>}
    </label>
  );
}
