import React, { useState, useEffect, useRef } from "react";
import { EvaluateTinker } from "../wailsjs/go/main/App";
import { tinker } from "../wailsjs/go/models";

interface TinkerPanelProps {
  projectId: string;
  trusted: boolean;
}

const PRESETS = [
  { label: "App Version", code: "app()->version();" },
  { label: "Database Connection", code: "DB::connection()->getDatabaseName();" },
  { label: "Users Sample", code: "App\\Models\\User::take(5)->get();" },
  { label: "Cache Check", code: "cache()->put('workbench_probe', now()->toDateTimeString(), 60);\ncache()->get('workbench_probe');" },
  { label: "Config Overview", code: "config('app.name');" },
  { label: "Recent Failed Jobs", code: "DB::table('failed_jobs')->latest()->take(5)->get();" },
];

export const TinkerPanel: React.FC<TinkerPanelProps> = ({ projectId, trusted }) => {
  const [code, setCode] = useState<string>("App\\Models\\User::count();");
  const [running, setRunning] = useState<boolean>(false);
  const [result, setResult] = useState<tinker.EvaluationResult | null>(null);
  const [activeView, setActiveView] = useState<"output" | "queries">("output");
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState<boolean>(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    // If project changed, reset result
    setResult(null);
    setError(null);
  }, [projectId]);

  const handleRun = async () => {
    if (!trusted) {
      setError("This project is untrusted. Trust it first from the header banner to run interactive PHP Tinker code.");
      return;
    }
    if (!code.trim()) {
      return;
    }
    setRunning(true);
    setError(null);
    try {
      const res = await EvaluateTinker(projectId, code);
      setResult(res);
      if (res.error) {
        setError(res.error);
      } else if (res.queries && res.queries.length > 0 && activeView === "queries") {
        setActiveView("queries");
      }
    } catch (err: any) {
      setError(err?.message || String(err));
    } finally {
      setRunning(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.ctrlKey || e.metaKey) && e.key === "Enter") {
      e.preventDefault();
      handleRun();
    } else if (e.key === "Tab") {
      e.preventDefault();
      const target = e.currentTarget;
      const start = target.selectionStart;
      const end = target.selectionEnd;
      const nextCode = code.substring(0, start) + "    " + code.substring(end);
      setCode(nextCode);
      setTimeout(() => {
        target.selectionStart = target.selectionEnd = start + 4;
      }, 0);
    }
  };

  const copyOutput = () => {
    if (!result?.output) return;
    navigator.clipboard.writeText(result.output);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "calc(100vh - 120px)", gap: 16 }}>
      {/* Top Controls Bar */}
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 12 }}>
        <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
          <span style={{ fontSize: "0.85rem", fontWeight: 600, color: "var(--text-muted)" }}>Quick Snippets:</span>
          {PRESETS.map((p) => (
            <button
              key={p.label}
              className="action-btn secondary small"
              onClick={() => setCode(p.code)}
              title={p.code}
              style={{ fontSize: "0.78rem", padding: "4px 8px" }}
            >
              {p.label}
            </button>
          ))}
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          {result && (
            <span style={{ fontSize: "0.8rem", color: "var(--text-muted)" }}>
              ⏱️ {result.durationMs.toFixed(1)} ms
            </span>
          )}
          <button
            className="action-btn secondary"
            onClick={() => {
              setCode("");
              setResult(null);
              setError(null);
            }}
            disabled={running}
          >
            Clear
          </button>
          <button
            className="action-btn primary"
            onClick={handleRun}
            disabled={running || !code.trim() || !trusted}
            title="Execute PHP code (Ctrl+Enter)"
            style={{ minWidth: 120 }}
          >
            {running ? "Evaluating..." : "Run (Ctrl+↵)"}
          </button>
        </div>
      </div>

      {/* Editor & Output Split */}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16, flex: 1, minHeight: 0 }}>
        {/* Editor Pane */}
        <div style={{ display: "flex", flexDirection: "column", background: "var(--bg-card)", borderRadius: 12, border: "1px solid var(--border-card)", overflow: "hidden" }}>
          <div style={{ padding: "8px 12px", borderBottom: "1px solid var(--border-card)", display: "flex", justifyContent: "space-between", alignItems: "center", background: "var(--bg-card-secondary)" }}>
            <span style={{ fontSize: "0.82rem", fontWeight: 600, color: "var(--text-muted)" }}>PHP Scratchpad</span>
            <span style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>Laravel environment bootstrapped</span>
          </div>
          <textarea
            aria-label="PHP scratchpad"
            ref={textareaRef}
            value={code}
            onChange={(e) => setCode(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="// Write PHP expression or statements here...&#10;User::where('active', true)->count();"
            spellCheck={false}
            style={{
              flex: 1,
              padding: 16,
              background: "transparent",
              color: "var(--text-main)",
              border: "none",
              resize: "none",
              fontFamily: "'Fira Code', 'Cascadia Code', Consolas, Monaco, monospace",
              fontSize: "0.92rem",
              lineHeight: 1.5,
              outline: "none",
            }}
          />
        </div>

        {/* Output & Queries Pane */}
        <div style={{ display: "flex", flexDirection: "column", background: "var(--bg-card)", borderRadius: 12, border: "1px solid var(--border-card)", overflow: "hidden" }}>
          {/* Header tabs */}
          <div style={{ padding: "6px 12px", borderBottom: "1px solid var(--border-card)", display: "flex", justifyContent: "space-between", alignItems: "center", background: "var(--bg-card-secondary)" }}>
            <div style={{ display: "flex", gap: 8 }}>
              <button
                className={`tab-btn ${activeView === "output" ? "active" : ""}`}
                onClick={() => setActiveView("output")}
                style={{
                  background: activeView === "output" ? "rgba(255,255,255,0.1)" : "transparent",
                  border: "none",
                  padding: "4px 10px",
                  borderRadius: 4,
                  color: activeView === "output" ? "#fff" : "var(--text-dim)",
                  cursor: "pointer",
                  fontSize: "0.82rem",
                  fontWeight: 600,
                }}
              >
                Output {result?.output ? "✓" : ""}
              </button>
              <button
                className={`tab-btn ${activeView === "queries" ? "active" : ""}`}
                onClick={() => setActiveView("queries")}
                style={{
                  background: activeView === "queries" ? "rgba(255,255,255,0.1)" : "transparent",
                  border: "none",
                  padding: "4px 10px",
                  borderRadius: 4,
                  color: activeView === "queries" ? "#fff" : "var(--text-dim)",
                  cursor: "pointer",
                  fontSize: "0.82rem",
                  fontWeight: 600,
                }}
              >
                Queries {result?.queries ? `(${result.queries.length})` : "(0)"}
              </button>
            </div>

            {result?.output && (
              <button
                onClick={copyOutput}
                style={{
                  background: "transparent",
                  border: "1px solid rgba(255,255,255,0.15)",
                  color: "var(--text-dim)",
                  borderRadius: 4,
                  padding: "3px 8px",
                  fontSize: "0.75rem",
                  cursor: "pointer",
                }}
              >
                {copied ? "Copied!" : "Copy Output"}
              </button>
            )}
          </div>

          {/* Body */}
          <div style={{ flex: 1, overflowY: "auto", padding: 16 }}>
            {error && (
              <div style={{ background: "rgba(239, 68, 68, 0.15)", border: "1px solid rgba(239, 68, 68, 0.4)", borderRadius: 6, padding: "12px 14px", color: "#fca5a5", fontSize: "0.88rem", marginBottom: 12, whiteSpace: "pre-wrap", fontFamily: "monospace" }}>
                <strong>Execution Error:</strong>
                <div>{error}</div>
              </div>
            )}

            {activeView === "output" ? (
              result?.output ? (
                <pre
                  style={{
                    margin: 0,
                    fontFamily: "'Fira Code', 'Cascadia Code', Consolas, Monaco, monospace",
                    fontSize: "0.88rem",
                    lineHeight: 1.5,
                    whiteSpace: "pre-wrap",
                    wordBreak: "break-all",
                    color: "var(--text-main)",
                  }}
                >
                  {result.output}
                </pre>
              ) : (
                <div style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", height: "100%", color: "var(--text-muted)", textAlign: "center" }}>
                  <div style={{ fontSize: "1.6rem", marginBottom: 8 }}>⚡</div>
                  <div style={{ fontSize: "0.9rem", fontWeight: 500 }}>No evaluation result yet</div>
                  <div style={{ fontSize: "0.8rem", marginTop: 4 }}>Write your PHP snippet and press Run or Ctrl+Enter</div>
                </div>
              )
            ) : (
              <div>
                {!result?.queries || result.queries.length === 0 ? (
                  <div style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", height: "100%", color: "var(--text-muted)", padding: "32px 0" }}>
                    <div style={{ fontSize: "1.4rem", marginBottom: 6 }}>🔍</div>
                    <div style={{ fontSize: "0.88rem" }}>No database queries executed</div>
                  </div>
                ) : (
                  <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
                    {result.queries.map((q, idx) => {
                      const timeColor = q.timeMs < 10 ? "#34d399" : q.timeMs < 50 ? "#fbbf24" : "#f87171";
                      return (
                        <div
                          key={idx}
                          style={{
                            background: "var(--bg-input)",
                            border: "1px solid var(--border-card)",
                            borderRadius: 6,
                            padding: "10px 12px",
                          }}
                        >
                          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 6 }}>
                            <span style={{ fontSize: "0.75rem", color: "var(--text-muted)", fontWeight: 600 }}>
                              #{idx + 1} • {q.connection || "default"}
                            </span>
                            <span
                              style={{
                                fontSize: "0.75rem",
                                fontWeight: 700,
                                color: timeColor,
                                background: "rgba(0,0,0,0.3)",
                                padding: "2px 6px",
                                borderRadius: 4,
                              }}
                            >
                              {q.timeMs.toFixed(2)} ms
                            </span>
                          </div>
                          <div
                            style={{
                              fontFamily: "'Fira Code', Consolas, monospace",
                              fontSize: "0.84rem",
                              color: "#93c5fd",
                              wordBreak: "break-all",
                            }}
                          >
                            {q.sql}
                          </div>
                          {q.bindings && q.bindings.length > 0 && (
                            <div style={{ marginTop: 6, display: "flex", gap: 6, flexWrap: "wrap", alignItems: "center" }}>
                              <span style={{ fontSize: "0.72rem", color: "var(--text-muted)" }}>Bindings:</span>
                              {q.bindings.map((b, bIdx) => (
                                <span
                                  key={bIdx}
                                  style={{
                                    fontSize: "0.72rem",
                                    fontFamily: "monospace",
                                    background: "rgba(255,255,255,0.08)",
                                    padding: "1px 6px",
                                    borderRadius: 3,
                                    color: "#e2e8f0",
                                  }}
                                >
                                  {typeof b === "object" ? JSON.stringify(b) : String(b)}
                                </span>
                              ))}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
