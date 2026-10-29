import React, { useState } from "react";
import { telemetry, database } from "../wailsjs/go/models";
import { ExplainQuery } from "../wailsjs/go/main/App";

interface NPlusOneRadarProps {
  projectId: string;
  report: telemetry.NPlusOneReport | null;
  loading?: boolean;
}

export const NPlusOneRadar: React.FC<NPlusOneRadarProps> = ({
  projectId,
  report,
  loading = false,
}) => {
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);
  const [selectedExplain, setSelectedExplain] = useState<database.ExplainResult | null>(null);
  const [explainingIndex, setExplainingIndex] = useState<number | null>(null);
  const [explainError, setExplainError] = useState<string | null>(null);

  const handleCopy = (fix: string, index: number) => {
    navigator.clipboard.writeText(fix);
    setCopiedIndex(index);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  const handleExplain = async (sql: string, index: number) => {
    setExplainingIndex(index);
    setExplainError(null);
    try {
      const res = await ExplainQuery(projectId, sql);
      if (res.error) {
        setExplainError(res.error);
      } else {
        setSelectedExplain(res);
      }
    } catch (err: any) {
      setExplainError(err?.message || String(err));
    } finally {
      setExplainingIndex(null);
    }
  };

  if (loading) {
    return (
      <div style={{ padding: "1.5rem", textAlign: "center", color: "#8b949e", fontSize: "0.85rem" }}>
        Analyzing query signatures for N+1 loops...
      </div>
    );
  }

  if (!report || !report.issues || report.issues.length === 0) {
    return (
      <div
        style={{
          padding: "1.5rem",
          background: "#161b22",
          border: "1px solid #30363d",
          borderRadius: "8px",
          textAlign: "center",
          color: "#7ee787",
          fontSize: "0.85rem",
        }}
      >
        <span style={{ fontSize: "1.2rem", marginRight: "8px" }}>✨</span>
        <strong>Zero N+1 Query Loops Detected!</strong>
        <div style={{ color: "#8b949e", fontSize: "0.75rem", marginTop: "4px" }}>
          All database relations in this request were efficiently loaded without repetition.
        </div>
      </div>
    );
  }

  return (
    <div
      style={{
        background: "#161b22",
        border: "1px solid #30363d",
        borderRadius: "8px",
        overflow: "hidden",
      }}
    >
      {/* Header Banner */}
      <div
        style={{
          padding: "10px 14px",
          background: "#2d1618",
          borderBottom: "1px solid #6e2729",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
          <span style={{ fontSize: "1.1rem" }}>🚨</span>
          <div>
            <div style={{ fontWeight: 700, fontSize: "0.85rem", color: "#f85149" }}>
              N+1 Query Radar: {report.totalLoops} Loop{report.totalLoops > 1 ? "s" : ""} Detected
            </div>
            <div style={{ fontSize: "0.75rem", color: "#ffa198" }}>
              Estimated {report.wastedTime}ms wasted in repetitive loop round-trips
            </div>
          </div>
        </div>
      </div>

      {/* Issues List */}
      <div style={{ padding: "12px", display: "flex", flexDirection: "column", gap: "10px" }}>
        {report.issues.map((issue, idx) => (
          <div
            key={idx}
            style={{
              background: "#0d1117",
              border: `1px solid ${issue.severity === "critical" ? "#f8514955" : "#d2992255"}`,
              borderRadius: "6px",
              padding: "12px",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: "8px" }}>
              <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                <span
                  style={{
                    background: issue.severity === "critical" ? "#f85149" : "#d29922",
                    color: "#fff",
                    fontWeight: 700,
                    fontSize: "0.7rem",
                    padding: "2px 6px",
                    borderRadius: "10px",
                  }}
                >
                  {issue.count}x EXECUTIONS
                </span>
                <span style={{ fontSize: "0.8rem", color: "#e6edf3", fontWeight: 600 }}>
                  Table: <code style={{ color: "#79c0ff" }}>{issue.table || "unknown"}</code>
                </span>
                <span style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                  Cumulative: <strong>{issue.totalDuration}ms</strong>
                </span>
              </div>

              <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                <button
                  type="button"
                  onClick={() => handleExplain(issue.normalizedQuery, idx)}
                  disabled={explainingIndex === idx}
                  style={{
                    background: "#21262d",
                    border: "1px solid #30363d",
                    color: "#58a6ff",
                    fontSize: "0.75rem",
                    padding: "3px 8px",
                    borderRadius: "4px",
                    cursor: "pointer",
                  }}
                >
                  {explainingIndex === idx ? "Explaining..." : "⚡ Explain"}
                </button>
              </div>
            </div>

            {/* Normalized Query */}
            <pre
              style={{
                background: "#161b22",
                padding: "8px 10px",
                borderRadius: "4px",
                fontSize: "0.75rem",
                color: "#c9d1d9",
                overflowX: "auto",
                margin: "0 0 8px 0",
                fontFamily: "monospace",
              }}
            >
              {issue.normalizedQuery}
            </pre>

            {/* Recommendation Card */}
            <div
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                background: "#121d2f",
                border: "1px solid #1f6feb44",
                padding: "6px 10px",
                borderRadius: "4px",
                fontSize: "0.75rem",
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                <span style={{ color: "#58a6ff", fontWeight: 600 }}>💡 Eager Load Fix:</span>
                <code style={{ color: "#7ee787", fontWeight: 700 }}>{issue.eagerLoadFix}</code>
              </div>
              <button
                type="button"
                onClick={() => handleCopy(issue.eagerLoadFix, idx)}
                style={{
                  background: copiedIndex === idx ? "#238636" : "#21262d",
                  color: "#fff",
                  border: "1px solid #30363d",
                  fontSize: "0.7rem",
                  padding: "2px 8px",
                  borderRadius: "4px",
                  cursor: "pointer",
                }}
              >
                {copiedIndex === idx ? "✓ Copied" : "Copy Fix"}
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* Explain Result Modal Overlay if opened */}
      {selectedExplain && (
        <div
          style={{
            padding: "12px",
            background: "#0d1117",
            borderTop: "1px solid #30363d",
          }}
        >
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "8px" }}>
            <span style={{ fontWeight: 600, fontSize: "0.85rem", color: "#58a6ff" }}>
              Explain Plan for Repeated Query (Health: {selectedExplain.score}/100)
            </span>
            <button
              onClick={() => setSelectedExplain(null)}
              style={{ background: "none", border: "none", color: "#8b949e", cursor: "pointer" }}
            >
              ✕ Close
            </button>
          </div>
          {selectedExplain.bottlenecks && selectedExplain.bottlenecks.map((b, i) => (
            <div key={i} style={{ color: "#f85149", fontSize: "0.75rem", marginBottom: "4px" }}>
              • {b.details} - {b.suggestion}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
