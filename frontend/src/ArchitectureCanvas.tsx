import React, { useState } from "react";
import { architecture } from "../wailsjs/go/models";

interface ArchitectureCanvasProps {
  report: architecture.ArchitectureCanvasReport | null;
  loading: boolean;
  onRefresh: () => void;
  projectName: string;
}

export const ArchitectureCanvas: React.FC<ArchitectureCanvasProps> = ({
  report,
  loading,
  onRefresh,
  projectName,
}) => {
  const [activeTab, setActiveTab] = useState<"canvas" | "violations" | "mermaid">("canvas");
  const [filterQuery, setFilterQuery] = useState("");
  const [copiedMermaid, setCopiedMermaid] = useState(false);
  const [selectedLayer, setSelectedLayer] = useState<string>("all");

  if (loading) {
    return (
      <div className="workbench-card" style={{ textAlign: "center", padding: "40px" }}>
        <div style={{ fontSize: "28px", marginBottom: "12px" }}>🔍</div>
        <h3>Analyzing Architectural Layers & Boundaries...</h3>
        <p style={{ color: "var(--text-muted)", fontSize: "13px" }}>
          Scanning controllers, actions, background jobs, listeners, and persistence models...
        </p>
      </div>
    );
  }

  if (!report) {
    return (
      <div className="empty-project-prompt">
        <div className="empty-project-icon">🏛️</div>
        <h3>No Architecture Analysis Available</h3>
        <p>Trigger an architectural scan to discover layered tiers and boundary linting.</p>
        <button className="btn-action primary" onClick={onRefresh} style={{ marginTop: "12px" }}>
          ⚡ Analyze Project Architecture
        </button>
      </div>
    );
  }

  const layers = report.layers || {};
  const violations = report.violations || [];
  const total = report.totalComponents || 0;

  const layerDefs = [
    {
      id: "1",
      title: "Layer 1: HTTP & Routing",
      desc: "Controllers, Middleware, Form Requests",
      icon: "🌐",
      color: "#818cf8",
      bg: "rgba(99, 102, 241, 0.08)",
      border: "rgba(99, 102, 241, 0.3)",
    },
    {
      id: "2",
      title: "Layer 2: Domain Workflows",
      desc: "Actions, Domain Services, Events",
      icon: "⚙️",
      color: "#c084fc",
      bg: "rgba(168, 85, 247, 0.08)",
      border: "rgba(168, 85, 247, 0.3)",
    },
    {
      id: "3",
      title: "Layer 3: Async & Background",
      desc: "Queued Jobs, Listeners, Notifications",
      icon: "⚡",
      color: "#38bdf8",
      bg: "rgba(56, 189, 248, 0.08)",
      border: "rgba(56, 189, 248, 0.3)",
    },
    {
      id: "4",
      title: "Layer 4: Persistence & Data",
      desc: "Eloquent Models, Repositories",
      icon: "💾",
      color: "#34d399",
      bg: "rgba(52, 211, 153, 0.08)",
      border: "rgba(52, 211, 153, 0.3)",
    },
  ];

  const copyMermaid = () => {
    if (report.mermaidCode) {
      navigator.clipboard.writeText(report.mermaidCode);
      setCopiedMermaid(true);
      setTimeout(() => setCopiedMermaid(false), 2000);
    }
  };

  const errorCount = violations.filter((v) => v.severity === "error").length;
  const warnCount = violations.filter((v) => v.severity === "warning").length;
  const infoCount = violations.filter((v) => v.severity === "info").length;

  return (
    <div className="arch-canvas-wrap">
      {/* Metrics Banner */}
      <div className="arch-card-grid" style={{ marginBottom: "16px" }}>
        <div className="arch-metric-card">
          <h4>Total Components</h4>
          <div className="arch-metric-value" style={{ color: "#38bdf8" }}>{total}</div>
        </div>
        <div className="arch-metric-card">
          <h4>Boundary Violations</h4>
          <div className="arch-metric-value" style={{ color: errorCount > 0 ? "#f87171" : warnCount > 0 ? "#fbbf24" : "#34d399" }}>
            {violations.length}
          </div>
        </div>
        <div className="arch-metric-card">
          <h4>Lint Errors / Warnings</h4>
          <div className="arch-metric-value" style={{ color: "#c7d2fe", fontSize: "16px", marginTop: "4px" }}>
            <span style={{ color: "#f87171" }}>{errorCount} err</span> / <span style={{ color: "#fbbf24" }}>{warnCount} warn</span>
          </div>
        </div>
        <div className="arch-metric-card">
          <h4>Architecture Health</h4>
          <div className="arch-metric-value" style={{ color: errorCount === 0 && warnCount === 0 ? "#34d399" : "#fbbf24" }}>
            {errorCount === 0 && warnCount === 0 ? "100% Clean" : errorCount === 0 ? "Good" : "Needs Review"}
          </div>
        </div>
      </div>

      {/* Sub Tabs */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "16px", flexWrap: "wrap", gap: "10px" }}>
        <div className="sub-segmented-nav" style={{ margin: 0 }}>
          <button
            className={`sub-tab-pill ${activeTab === "canvas" ? "active" : ""}`}
            onClick={() => setActiveTab("canvas")}
          >
            🏛️ 4-Tier Interactive Canvas
          </button>
          <button
            className={`sub-tab-pill ${activeTab === "violations" ? "active" : ""}`}
            onClick={() => setActiveTab("violations")}
          >
            🛡️ Boundary Linter ({violations.length})
          </button>
          <button
            className={`sub-tab-pill ${activeTab === "mermaid" ? "active" : ""}`}
            onClick={() => setActiveTab("mermaid")}
          >
            📊 Mermaid Diagram Export
          </button>
        </div>

        <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
          {activeTab === "canvas" && (
            <input
              type="text"
              placeholder="🔍 Search components..."
              value={filterQuery}
              onChange={(e) => setFilterQuery(e.target.value)}
              style={{
                padding: "6px 12px",
                fontSize: "12px",
                borderRadius: "8px",
                background: "#090d16",
                color: "#fff",
                border: "1px solid rgba(255, 255, 255, 0.15)",
                width: "200px",
              }}
            />
          )}
          <button
            type="button"
            className="btn-tiny"
            onClick={copyMermaid}
            title="Copy Mermaid syntax for GitHub PR or Notion"
          >
            {copiedMermaid ? "✓ Copied Mermaid" : "📋 Copy Mermaid"}
          </button>
          <button type="button" className="btn-tiny primary" onClick={onRefresh}>
            ↺ Re-Scan
          </button>
        </div>
      </div>

      {/* TAB 1: 4-TIER CANVAS */}
      {activeTab === "canvas" && (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(250px, 1fr))", gap: "14px" }}>
          {layerDefs.map((def) => {
            const rawNodes = layers[def.id] || [];
            const nodes = rawNodes.filter((n) =>
              !filterQuery ||
              n.name.toLowerCase().includes(filterQuery.toLowerCase()) ||
              n.kind.toLowerCase().includes(filterQuery.toLowerCase())
            );

            return (
              <div
                key={def.id}
                style={{
                  background: def.bg,
                  border: `1px solid ${def.border}`,
                  borderRadius: "12px",
                  padding: "16px",
                  display: "flex",
                  flexDirection: "column",
                  minHeight: "420px",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: "8px", marginBottom: "6px" }}>
                  <span style={{ fontSize: "18px" }}>{def.icon}</span>
                  <div>
                    <h4 style={{ margin: 0, fontSize: "13px", color: def.color, fontWeight: 700 }}>
                      {def.title}
                    </h4>
                    <span style={{ fontSize: "11px", color: "var(--text-muted)" }}>{def.desc}</span>
                  </div>
                </div>

                <div style={{ fontSize: "11px", color: "var(--text-dim)", margin: "8px 0 12px", borderBottom: "1px solid rgba(255, 255, 255, 0.08)", paddingBottom: "6px" }}>
                  {nodes.length} component{nodes.length === 1 ? "" : "s"} discovered
                </div>

                <div style={{ flex: 1, overflowY: "auto", display: "flex", flexDirection: "column", gap: "8px", maxHeight: "480px" }}>
                  {nodes.map((n) => (
                    <div
                      key={n.id}
                      style={{
                        background: "rgba(15, 23, 42, 0.75)",
                        border: "1px solid rgba(255, 255, 255, 0.08)",
                        borderRadius: "8px",
                        padding: "10px 12px",
                        boxShadow: "0 2px 6px rgba(0,0,0,0.3)",
                      }}
                    >
                      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "4px" }}>
                        <strong style={{ fontSize: "12px", color: "#f8fafc" }}>{n.name}</strong>
                        <span
                          style={{
                            fontSize: "10px",
                            padding: "2px 6px",
                            borderRadius: "4px",
                            background: "rgba(255, 255, 255, 0.1)",
                            color: def.color,
                            fontWeight: 600,
                          }}
                        >
                          {n.kind}
                        </span>
                      </div>
                      <div style={{ fontSize: "11px", color: "var(--text-muted)", fontFamily: "monospace", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }} title={n.filePath}>
                        {n.filePath}
                      </div>
                    </div>
                  ))}

                  {nodes.length === 0 && (
                    <div style={{ textAlign: "center", color: "var(--text-muted)", fontSize: "12px", padding: "40px 0" }}>
                      No components found in this tier.
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* TAB 2: BOUNDARY LINTER */}
      {activeTab === "violations" && (
        <div className="workbench-card">
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "16px" }}>
            <div>
              <h3 style={{ margin: 0 }}>Architectural Boundary Violations</h3>
              <p style={{ margin: "4px 0 0", fontSize: "12px", color: "var(--text-muted)" }}>
                Automated detection of inverted dependencies, leaky layer crossings, and transport coupling.
              </p>
            </div>
            <span className="workbench-badge badge-info">
              {violations.length} total finding{violations.length === 1 ? "" : "s"}
            </span>
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
            {violations.map((v, idx) => (
              <div
                key={idx}
                style={{
                  background: "rgba(15, 23, 42, 0.6)",
                  border: `1px solid ${v.severity === "error" ? "rgba(239, 68, 68, 0.4)" : v.severity === "warning" ? "rgba(245, 158, 11, 0.4)" : "rgba(56, 189, 248, 0.3)"}`,
                  borderRadius: "10px",
                  padding: "14px 16px",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "6px" }}>
                  <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                    <span
                      className={`workbench-badge ${v.severity === "error" ? "badge-error" : v.severity === "warning" ? "badge-warn" : "badge-info"}`}
                    >
                      {v.severity.toUpperCase()}
                    </span>
                    <strong style={{ fontSize: "13px", color: "#f8fafc" }}>{v.message}</strong>
                  </div>
                  <code style={{ fontSize: "11px", color: "var(--text-muted)" }}>{v.ruleId}</code>
                </div>

                <div style={{ fontSize: "12px", color: "#cbd5e1", margin: "6px 0 8px" }}>
                  {v.explanation}
                </div>

                <div style={{ fontSize: "11px", color: "var(--text-dim)", fontFamily: "monospace" }}>
                  Source: <strong>{v.sourceFile}</strong> ({v.sourceKind})
                </div>
              </div>
            ))}

            {violations.length === 0 && (
              <div style={{ textAlign: "center", padding: "40px", color: "#34d399" }}>
                <div style={{ fontSize: "32px", marginBottom: "8px" }}>🛡️</div>
                <h4 style={{ margin: 0, color: "#34d399" }}>Zero Architectural Boundary Violations!</h4>
                <p style={{ color: "var(--text-muted)", fontSize: "12px", marginTop: "4px" }}>
                  All controllers, domain actions, and persistence models adhere strictly to modern Laravel layered patterns.
                </p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 3: MERMAID EXPORT */}
      {activeTab === "mermaid" && (
        <div className="workbench-card">
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "12px" }}>
            <div>
              <h3 style={{ margin: 0 }}>Mermaid Graph TD Diagram</h3>
              <p style={{ margin: "4px 0 0", fontSize: "12px", color: "var(--text-muted)" }}>
                Render-ready diagram code for GitHub pull request descriptions, documentation, or Notion pages.
              </p>
            </div>
            <button className="btn-tiny primary" onClick={copyMermaid}>
              {copiedMermaid ? "✓ Copied to Clipboard" : "📋 Copy Diagram Code"}
            </button>
          </div>

          <pre
            style={{
              background: "#090d16",
              padding: "16px",
              borderRadius: "10px",
              border: "1px solid rgba(255, 255, 255, 0.1)",
              color: "#38bdf8",
              fontSize: "12px",
              fontFamily: "monospace",
              maxHeight: "450px",
              overflowY: "auto",
              whiteSpace: "pre-wrap",
            }}
          >
            {report.mermaidCode || "%% No components discovered."}
          </pre>
        </div>
      )}
    </div>
  );
};
