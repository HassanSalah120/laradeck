import React, { useMemo, useState } from "react";
import { workbench, health, flags } from "../wailsjs/go/models";
import { LaunchProjectTerminal, OpenProject } from "../wailsjs/go/main/App";

interface Props {
  state: workbench.State;
  switchTab: (tab: any) => void;
  act: (label: string, action: () => Promise<unknown>) => Promise<void>;
  heartbeat?: health.HeartbeatReport | null;
  flags?: flags.FlagsReport | null;
}

export default function DashboardPanel({ state, switchTab, act, heartbeat, flags: flagReport }: Props) {
  const [hoveredPoint, setHoveredPoint] = useState<{ index: number; uri: string; duration: number; status: number } | null>(null);

  const project = state.projects.find((p) => p.id === state.selected);
  const requests = useMemo(() => state.capture?.requests || [], [state.capture?.requests]);

  // Compute metrics
  const stats = useMemo(() => {
    if (requests.length === 0) {
      return {
        totalReqs: 0,
        avgLatency: 0,
        p95Latency: 0,
        totalQueries: 0,
        slowQueries: 0,
        s2xx: 0,
        s3xx: 0,
        s4xx: 0,
        s5xx: 0,
      };
    }

    const durations = requests.map((r) => r.duration || 0).sort((a, b) => a - b);
    const avg = durations.reduce((a, b) => a + b, 0) / durations.length;
    const p95Idx = Math.min(durations.length - 1, Math.floor(durations.length * 0.95));
    const p95 = durations[p95Idx] || avg;

    let queries = 0;
    let slow = 0;
    let c2 = 0;
    let c3 = 0;
    let c4 = 0;
    let c5 = 0;

    requests.forEach((r) => {
      queries += r.queries?.length || 0;
      r.queries?.forEach((q) => {
        if ((q.duration || 0) > 50 || q.repeated) slow++;
      });

      const s = r.status || 200;
      if (s >= 200 && s < 300) c2++;
      else if (s >= 300 && s < 400) c3++;
      else if (s >= 400 && s < 500) c4++;
      else if (s >= 500) c5++;
    });

    return {
      totalReqs: requests.length,
      avgLatency: avg,
      p95Latency: p95,
      totalQueries: queries,
      slowQueries: slow,
      s2xx: c2,
      s3xx: c3,
      s4xx: c4,
      s5xx: c5,
    };
  }, [requests]);

  // Compute SVG chart points (max 20 data points)
  const chartData = useMemo(() => {
    const slice = requests.slice(-20);
    if (slice.length === 0) return { path: "", area: "", points: [], maxVal: 100 };

    const maxVal = Math.max(...slice.map((r) => r.duration || 0), 20) * 1.15;
    const width = 640;
    const height = 160;

    const points = slice.map((r, i) => {
      const x = (i / Math.max(1, slice.length - 1)) * (width - 40) + 20;
      const y = height - 20 - ((r.duration || 0) / maxVal) * (height - 40);
      return { x, y, duration: r.duration || 0, uri: r.uri, status: r.status || 200 };
    });

    if (points.length === 1) {
      points.push({ ...points[0], x: width - 20 });
    }

    // Generate smooth curve
    let d = `M ${points[0].x} ${points[0].y}`;
    for (let i = 1; i < points.length; i++) {
      const prev = points[i - 1];
      const curr = points[i];
      const cx = (prev.x + curr.x) / 2;
      d += ` C ${cx} ${prev.y}, ${cx} ${curr.y}, ${curr.x} ${curr.y}`;
    }

    const area = `${d} L ${points[points.length - 1].x} ${height - 10} L ${points[0].x} ${height - 10} Z`;

    return { path: d, area, points, maxVal };
  }, [requests]);

  return (
    <div className="dashboard-container">
      {/* Hero Mission Control Header */}
      <header className="dashboard-hero">
        <div className="dashboard-hero-glow" />
        <div className="dashboard-hero-content">
          <div className="dashboard-hero-left">
            <div className="dashboard-hero-badge">
              <span className="pulse-dot" />
              <span>LIVE TELEMETRY ACTIVE</span>
            </div>
            <h1>{project ? project.name : "Mission Control"}</h1>
            <p className="dashboard-hero-subtitle">
              Real-time Laravel application vitals, request velocity, query radar, and developer controls.
            </p>
          </div>

          <div className="dashboard-hero-right">
            {project && (
              <div className="dashboard-vitals-pills">
                <span className="vital-pill">
                  <span className="vital-icon">⚡</span>
                  <span>PHP {project.php}</span>
                </span>
                <span className={`vital-pill ${project.secured ? "secured" : ""}`}>
                  <span className="vital-icon">{project.secured ? "🔒" : "🔓"}</span>
                  <span>{project.secured ? "HTTPS 256-bit" : "HTTP (Unsecured)"}</span>
                </span>
                <span className="vital-pill">
                  <span className="vital-icon">📁</span>
                  <span>{project.framework || "Laravel"}</span>
                </span>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Primary 4-Deck Stat Cards */}
      <section className="dashboard-stats-grid">
        {/* Card 1: Request Throughput */}
        <div className="stat-deck-card indigo">
          <div className="stat-deck-header">
            <span className="stat-deck-label">Request Velocity</span>
            <span className="stat-deck-icon">⚡</span>
          </div>
          <div className="stat-deck-metric">
            <span className="stat-number">{stats.totalReqs}</span>
            <span className="stat-unit">requests</span>
          </div>
          <div className="stat-deck-footer">
            <span className="stat-trend positive">▲ {requests.length > 0 ? "Stream active" : "Idle"}</span>
            <span className="stat-detail">Avg: {stats.avgLatency.toFixed(1)} ms</span>
          </div>
        </div>

        {/* Card 2: P95 Latency & Speed */}
        <div className="stat-deck-card sky">
          <div className="stat-deck-header">
            <span className="stat-deck-label">P95 Response Time</span>
            <span className="stat-deck-icon">⏱️</span>
          </div>
          <div className="stat-deck-metric">
            <span className="stat-number">{stats.p95Latency.toFixed(1)}</span>
            <span className="stat-unit">ms</span>
          </div>
          <div className="stat-deck-footer">
            <div className="stat-latency-bar">
              <div
                className="stat-latency-fill"
                style={{
                  width: `${Math.min(100, Math.max(10, (stats.p95Latency / 150) * 100))}%`,
                  background: stats.p95Latency < 50 ? "#10b981" : stats.p95Latency < 120 ? "#f59e0b" : "#f43f5e",
                }}
              />
            </div>
            <span className="stat-detail">
              {stats.p95Latency < 50 ? "🚀 Optimal" : stats.p95Latency < 120 ? "⚡ Good" : "⚠️ Needs Review"}
            </span>
          </div>
        </div>

        {/* Card 3: Database Query Radar */}
        <div className="stat-deck-card emerald">
          <div className="stat-deck-header">
            <span className="stat-deck-label">Database Queries</span>
            <span className="stat-deck-icon">🗄️</span>
          </div>
          <div className="stat-deck-metric">
            <span className="stat-number">{stats.totalQueries}</span>
            <span className="stat-unit">queries</span>
          </div>
          <div className="stat-deck-footer">
            <span className={`stat-trend ${stats.slowQueries > 0 ? "negative" : "positive"}`}>
              {stats.slowQueries > 0 ? `⚠️ ${stats.slowQueries} slow queries` : "✓ 0 slow queries"}
            </span>
            <span className="stat-detail">Eloquent active</span>
          </div>
        </div>

        {/* Card 4: System Health & Errors */}
        <div className="stat-deck-card amber">
          <div className="stat-deck-header">
            <span className="stat-deck-label">Error & Guard Shield</span>
            <span className="stat-deck-icon">🛡️</span>
          </div>
          <div className="stat-deck-metric">
            <span className="stat-number">{stats.s5xx}</span>
            <span className="stat-unit">5xx errors</span>
          </div>
          <div className="stat-deck-footer">
            <span className={`stat-trend ${stats.s5xx > 0 ? "negative" : "positive"}`}>
              {stats.s5xx === 0 ? "✓ Error-free stream" : `✖ ${stats.s5xx} failures`}
            </span>
            <span className="stat-detail">{stats.s4xx} 4xx client</span>
          </div>
        </div>
      </section>

      {/* Health Heartbeat & Feature Flags Mission Radar */}
      {(heartbeat || flagReport) && (
        <section style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(320px, 1fr))", gap: "14px", marginBottom: "16px" }}>
          {/* Heartbeat Card */}
          {heartbeat && (
            <div className="workbench-card" style={{ margin: 0, padding: "16px 20px" }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px" }}>
                <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                  <span style={{ fontSize: "18px" }}>💓</span>
                  <div>
                    <h4 style={{ margin: 0, color: "#fff", fontSize: "13px" }}>Application Health Heartbeat</h4>
                    <span style={{ fontSize: "11px", color: "var(--text-muted)" }}>Kernel Boot &amp; Storage Integrity</span>
                  </div>
                </div>
                <span className={`workbench-badge ${heartbeat.overall === "healthy" ? "badge-good" : heartbeat.overall === "degraded" ? "badge-warn" : "badge-error"}`}>
                  {heartbeat.healthScore}/100 {heartbeat.overall.toUpperCase()}
                </span>
              </div>

              <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                {heartbeat.components.map((c, idx) => (
                  <div key={idx} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", fontSize: "12px", background: "rgba(0,0,0,0.2)", padding: "6px 10px", borderRadius: "6px" }}>
                    <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                      <span style={{ color: c.status === "healthy" ? "#34d399" : "#fbbf24" }}>●</span>
                      <strong style={{ textTransform: "capitalize", color: "#f8fafc" }}>{c.component}</strong>
                    </div>
                    <span style={{ color: "var(--text-muted)", fontSize: "11px" }}>
                      {c.latencyMs ? `${c.latencyMs.toFixed(1)} ms · ` : ""}{c.status}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Feature Flags Card */}
          {flagReport && (
            <div className="workbench-card" style={{ margin: 0, padding: "16px 20px" }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px" }}>
                <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                  <span style={{ fontSize: "18px" }}>🚩</span>
                  <div>
                    <h4 style={{ margin: 0, color: "#fff", fontSize: "13px" }}>Pennant &amp; Feature Flags</h4>
                    <span style={{ fontSize: "11px", color: "var(--text-muted)" }}>
                      {flagReport.hasPennant ? "laravel/pennant installed" : "Dynamic code flags"}
                    </span>
                  </div>
                </div>
                <span className="workbench-badge badge-info">
                  {flagReport.totalFlags} Discovered
                </span>
              </div>

              <div style={{ display: "flex", flexWrap: "wrap", gap: "6px", maxHeight: "110px", overflowY: "auto" }}>
                {flagReport.flags.map((f, idx) => (
                  <span
                    key={idx}
                    style={{
                      fontSize: "11px",
                      fontFamily: "monospace",
                      padding: "4px 8px",
                      borderRadius: "6px",
                      background: "rgba(99, 102, 241, 0.15)",
                      color: "#c7d2fe",
                      border: "1px solid rgba(99, 102, 241, 0.3)",
                    }}
                    title={`Defined in: ${f.definedIn || "Unknown"}`}
                  >
                    🚩 {f.name}
                  </span>
                ))}
                {flagReport.flags.length === 0 && (
                  <div style={{ color: "var(--text-muted)", fontSize: "12px", padding: "10px 0" }}>
                    No feature flags defined in project code.
                  </div>
                )}
              </div>
            </div>
          )}
        </section>
      )}

      {/* Middle Visuals: Area Chart & HTTP Distribution */}
      <section className="dashboard-charts-grid">
        {/* Latency & Velocity Area Chart */}
        <div className="dashboard-chart-card">
          <div className="chart-card-header">
            <div>
              <h3>Response Time & Traffic Flow</h3>
              <p>Real-time request duration trajectory over last {chartData.points.length} captured endpoints</p>
            </div>
            <div className="chart-legend-pills">
              <span className="chart-legend-pill latency">
                <span className="legend-dot-glow" /> Latency (ms)
              </span>
            </div>
          </div>

          <div className="chart-svg-wrap">
            {chartData.points.length > 0 ? (
              <svg viewBox="0 0 640 160" className="latency-svg" preserveAspectRatio="none">
                <defs>
                  <linearGradient id="latencyGradient" x1="0%" y1="0%" x2="0%" y2="100%">
                    <stop offset="0%" stopColor="#6366f1" stopOpacity="0.45" />
                    <stop offset="70%" stopColor="#4f46e5" stopOpacity="0.1" />
                    <stop offset="100%" stopColor="#4f46e5" stopOpacity="0" />
                  </linearGradient>
                  <linearGradient id="lineGradient" x1="0%" y1="0%" x2="100%" y2="0%">
                    <stop offset="0%" stopColor="#818cf8" />
                    <stop offset="50%" stopColor="#6366f1" />
                    <stop offset="100%" stopColor="#38bdf8" />
                  </linearGradient>
                </defs>

                {/* Horizontal reference grid lines */}
                <line x1="20" y1="40" x2="620" y2="40" stroke="rgba(255,255,255,0.06)" strokeDasharray="4 4" />
                <line x1="20" y1="80" x2="620" y2="80" stroke="rgba(255,255,255,0.06)" strokeDasharray="4 4" />
                <line x1="20" y1="120" x2="620" y2="120" stroke="rgba(255,255,255,0.06)" strokeDasharray="4 4" />

                {/* Shaded Area */}
                <path d={chartData.area} fill="url(#latencyGradient)" />

                {/* Vibrant Curve Line */}
                <path d={chartData.path} fill="none" stroke="url(#lineGradient)" strokeWidth="2.5" strokeLinecap="round" />

                {/* Interactive Points */}
                {chartData.points.map((pt, idx) => (
                  <circle
                    key={idx}
                    cx={pt.x}
                    cy={pt.y}
                    r={hoveredPoint?.index === idx ? 6 : 3.5}
                    fill={hoveredPoint?.index === idx ? "#ffffff" : "#6366f1"}
                    stroke="#090d16"
                    strokeWidth="2"
                    className="chart-point"
                    onMouseEnter={() => setHoveredPoint({ index: idx, uri: pt.uri, duration: pt.duration, status: pt.status })}
                    onMouseLeave={() => setHoveredPoint(null)}
                  />
                ))}
              </svg>
            ) : (
              <div className="chart-empty-state">
                <span>📊</span>
                <p>Waiting for request telemetry. Trigger requests in your browser to visualize flow.</p>
              </div>
            )}

            {/* Hover Tooltip */}
            {hoveredPoint && (
              <div
                className="chart-tooltip"
                style={{
                  left: `${(hoveredPoint.index / Math.max(1, chartData.points.length - 1)) * 90 + 5}%`,
                }}
              >
                <div className="tooltip-uri">{hoveredPoint.uri}</div>
                <div className="tooltip-metrics">
                  <span className="tooltip-status">{hoveredPoint.status}</span>
                  <span className="tooltip-dur">{hoveredPoint.duration.toFixed(1)} ms</span>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* HTTP Status Breakdown Donut/Ratio Card */}
        <div className="dashboard-chart-card ratio-card">
          <div className="chart-card-header">
            <div>
              <h3>HTTP Status Distribution</h3>
              <p>Response ratio breakdown</p>
            </div>
          </div>

          <div className="status-ratio-container">
            {/* Horizontal Stacked Ratio Bar */}
            <div className="status-stacked-bar">
              {stats.totalReqs > 0 ? (
                <>
                  <div
                    className="ratio-seg seg-2xx"
                    style={{ width: `${(stats.s2xx / stats.totalReqs) * 100}%` }}
                    title={`2xx: ${stats.s2xx}`}
                  />
                  <div
                    className="ratio-seg seg-3xx"
                    style={{ width: `${(stats.s3xx / stats.totalReqs) * 100}%` }}
                    title={`3xx: ${stats.s3xx}`}
                  />
                  <div
                    className="ratio-seg seg-4xx"
                    style={{ width: `${(stats.s4xx / stats.totalReqs) * 100}%` }}
                    title={`4xx: ${stats.s4xx}`}
                  />
                  <div
                    className="ratio-seg seg-5xx"
                    style={{ width: `${(stats.s5xx / stats.totalReqs) * 100}%` }}
                    title={`5xx: ${stats.s5xx}`}
                  />
                </>
              ) : (
                <div className="ratio-seg seg-empty" style={{ width: "100%" }} />
              )}
            </div>

            {/* Detailed Status Pills List */}
            <div className="status-details-list">
              <div className="status-detail-row">
                <span className="status-badge-dot seg-2xx" />
                <span className="status-label">2xx Success</span>
                <span className="status-count">{stats.s2xx}</span>
                <span className="status-pct">
                  {stats.totalReqs > 0 ? `${Math.round((stats.s2xx / stats.totalReqs) * 100)}%` : "0%"}
                </span>
              </div>
              <div className="status-detail-row">
                <span className="status-badge-dot seg-3xx" />
                <span className="status-label">3xx Redirect</span>
                <span className="status-count">{stats.s3xx}</span>
                <span className="status-pct">
                  {stats.totalReqs > 0 ? `${Math.round((stats.s3xx / stats.totalReqs) * 100)}%` : "0%"}
                </span>
              </div>
              <div className="status-detail-row">
                <span className="status-badge-dot seg-4xx" />
                <span className="status-label">4xx Client Error</span>
                <span className="status-count">{stats.s4xx}</span>
                <span className="status-pct">
                  {stats.totalReqs > 0 ? `${Math.round((stats.s4xx / stats.totalReqs) * 100)}%` : "0%"}
                </span>
              </div>
              <div className="status-detail-row">
                <span className="status-badge-dot seg-5xx" />
                <span className="status-label">5xx Server Error</span>
                <span className="status-count">{stats.s5xx}</span>
                <span className="status-pct">
                  {stats.totalReqs > 0 ? `${Math.round((stats.s5xx / stats.totalReqs) * 100)}%` : "0%"}
                </span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Senior Developer Command Launchpad */}
      <section className="dashboard-launchpad">
        <div className="launchpad-header">
          <h3>Developer Control Launchpad</h3>
          <p>Instant shortcuts to LaraDeck mission control modules</p>
        </div>

        <div className="launchpad-grid">
          <button className="launchpad-tile" onClick={() => switchTab("database")}>
            <div className="tile-icon indigo">🗄️</div>
            <div className="tile-content">
              <strong>Database Studio</strong>
              <span>Explore Eloquent models, attributes &amp; records</span>
            </div>
            <span className="tile-arrow">→</span>
          </button>

          <button className="launchpad-tile" onClick={() => switchTab("quality")}>
            <div className="tile-icon purple">🛡️</div>
            <div className="tile-content">
              <strong>Quality &amp; Pest Audit</strong>
              <span>Run AST taint detection &amp; architectural rules</span>
            </div>
            <span className="tile-arrow">→</span>
          </button>

          <button className="launchpad-tile" onClick={() => switchTab("tinker")}>
            <div className="tile-icon sky">⚡</div>
            <div className="tile-content">
              <strong>Tinker REPL</strong>
              <span>Interactive PHP session with project context</span>
            </div>
            <span className="tile-arrow">→</span>
          </button>

          <button
            className="launchpad-tile"
            onClick={() => project && LaunchProjectTerminal(project.id, "powershell")}
          >
            <div className="tile-icon emerald">💻</div>
            <div className="tile-content">
              <strong>Herd Terminal</strong>
              <span>PowerShell with project PHP injected into PATH</span>
            </div>
            <span className="tile-arrow">↗</span>
          </button>

          <button className="launchpad-tile" onClick={() => switchTab("companion")}>
            <div className="tile-icon cyan">📦</div>
            <div className="tile-content">
              <strong>Companion &amp; Workspace</strong>
              <span>Package health, readiness checks &amp; migrations</span>
            </div>
            <span className="tile-arrow">→</span>
          </button>

          <button
            className="launchpad-tile"
            onClick={() => project && act("Open browser", () => OpenProject(project.id))}
          >
            <div className="tile-icon amber">🌐</div>
            <div className="tile-content">
              <strong>Open Application</strong>
              <span>Launch {project ? project.url : "site"} in browser</span>
            </div>
            <span className="tile-arrow">↗</span>
          </button>
        </div>
      </section>

      {/* Live Recent Stream Feed */}
      {requests.length > 0 && (
        <section className="dashboard-stream-card">
          <div className="stream-card-header">
            <h3>Recent Endpoint Telemetry</h3>
            <button className="btn-tiny" onClick={() => switchTab("requests")}>
              View All Telemetry ({requests.length}) →
            </button>
          </div>

          <div className="stream-list">
            {requests.slice(-5).reverse().map((r) => (
              <div key={r.id} className="stream-row" onClick={() => switchTab("requests")}>
                <span className={`method m-${(r.method || "GET").toLowerCase()}`}>{r.method}</span>
                <span className="stream-uri">{r.uri}</span>
                <div className="stream-latency-indicator">
                  <div
                    className="stream-bar"
                    style={{
                      width: `${Math.min(100, Math.max(8, ((r.duration || 0) / 100) * 100))}%`,
                    }}
                  />
                  <span className="stream-ms">{(r.duration || 0).toFixed(1)} ms</span>
                </div>
                <span className={`status s-${String(r.status || 200)[0]}`}>{r.status}</span>
                <span className="stream-queries">
                  {r.queries?.length ? `🗄️ ${r.queries.length} q` : ""}
                </span>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
