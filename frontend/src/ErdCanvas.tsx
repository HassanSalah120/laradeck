import React, { useState, useEffect, useRef } from "react";
import { GetErdSchema, ExplainQuery } from "../wailsjs/go/main/App";
import { database } from "../wailsjs/go/models";

interface ErdCanvasProps {
  projectId: string;
  trusted: boolean;
  onSelectModel?: (modelClass: string) => void;
}

interface TablePosition {
  x: number;
  y: number;
  width: number;
  height: number;
}

export const ErdCanvas: React.FC<ErdCanvasProps> = ({
  projectId,
  trusted,
  onSelectModel,
}) => {
  const [schema, setSchema] = useState<database.ERDSchemaResult | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState<string>("");
  const [selectedTable, setSelectedTable] = useState<string | null>(null);
  const [hoveredRelation, setHoveredRelation] = useState<database.ERDRelation | null>(null);
  const [showAdvisories, setShowAdvisories] = useState<boolean>(false);

  // Pan & Zoom
  const [zoom, setZoom] = useState<number>(1);
  const [pan, setPan] = useState<{ x: number; y: number }>({ x: 40, y: 40 });
  const [isDragging, setIsDragging] = useState<boolean>(false);
  const dragStart = useRef<{ x: number; y: number }>({ x: 0, y: 0 });

  // Explain Query Modal
  const [showExplainModal, setShowExplainModal] = useState<boolean>(false);
  const [explainSql, setExplainSql] = useState<string>("");
  const [explainResult, setExplainResult] = useState<database.ExplainResult | null>(null);
  const [explaining, setExplaining] = useState<boolean>(false);
  const [explainError, setExplainError] = useState<string | null>(null);

  useEffect(() => {
    if (projectId && trusted) {
      loadSchema();
    }
  }, [projectId, trusted]);

  const loadSchema = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await GetErdSchema(projectId);
      if (res.error) {
        setError(res.error);
      } else {
        setSchema(res);
      }
    } catch (err: any) {
      setError(err?.message || String(err));
    } finally {
      setLoading(false);
    }
  };

  const handleRunExplain = async () => {
    if (!explainSql.trim()) return;
    setExplaining(true);
    setExplainError(null);
    try {
      const res = await ExplainQuery(projectId, explainSql);
      if (res.error) {
        setExplainError(res.error);
      } else {
        setExplainResult(res);
      }
    } catch (err: any) {
      setExplainError(err?.message || String(err));
    } finally {
      setExplaining(false);
    }
  };

  // Compute layout positions for tables
  const tablePositions: Record<string, TablePosition> = {};
  const filteredTables = (schema?.tables || []).filter(
    (t) =>
      t.table.toLowerCase().includes(search.toLowerCase()) ||
      t.modelName.toLowerCase().includes(search.toLowerCase())
  );

  const cardWidth = 260;
  const colGap = 80;
  const rowGap = 50;
  const cols = Math.max(1, Math.min(4, Math.ceil(Math.sqrt(filteredTables.length || 1))));

  filteredTables.forEach((t, i) => {
    const colIndex = i % cols;
    const rowIndex = Math.floor(i / cols);
    const cardHeight = Math.min(320, 60 + (t.columns?.length || 0) * 24);
    tablePositions[t.table] = {
      x: colIndex * (cardWidth + colGap),
      y: rowIndex * (320 + rowGap),
      width: cardWidth,
      height: cardHeight,
    };
  });

  const handleMouseDown = (e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest(".table-node") || (e.target as HTMLElement).closest("button")) {
      return;
    }
    setIsDragging(true);
    dragStart.current = { x: e.clientX - pan.x, y: e.clientY - pan.y };
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!isDragging) return;
    setPan({
      x: e.clientX - dragStart.current.x,
      y: e.clientY - dragStart.current.y,
    });
  };

  const handleMouseUp = () => setIsDragging(false);

  const resetView = () => {
    setZoom(1);
    setPan({ x: 40, y: 40 });
  };

  if (!trusted) {
    return (
      <div style={{ padding: "3rem", textAlign: "center", color: "var(--text-muted)" }}>
        <h3>Project Untrusted</h3>
        <p>Trust this project to inspect its schema and database relationships.</p>
      </div>
    );
  }

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        position: "relative",
        background: "#0d1117",
        color: "#c9d1d9",
        overflow: "hidden",
        userSelect: isDragging ? "none" : "auto",
      }}
    >
      {/* Top Toolbar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "0.75rem 1.25rem",
          background: "#161b22",
          borderBottom: "1px solid #30363d",
          gap: "1rem",
          zIndex: 10,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
          <span style={{ fontWeight: 600, fontSize: "0.95rem", color: "#58a6ff" }}>
            📐 ERD & Schema Intelligence
          </span>
          <span
            style={{
              fontSize: "0.75rem",
              background: "#21262d",
              padding: "2px 8px",
              borderRadius: "12px",
              color: "#8b949e",
            }}
          >
            {schema?.tables?.length || 0} Models • {schema?.relations?.length || 0} Relations
          </span>
          {schema?.advisories && schema.advisories.length > 0 && (
            <button
              onClick={() => setShowAdvisories(!showAdvisories)}
              style={{
                fontSize: "0.75rem",
                background: "#d2992222",
                color: "#e3b341",
                border: "1px solid #d2992255",
                padding: "2px 10px",
                borderRadius: "12px",
                cursor: "pointer",
                fontWeight: 600,
              }}
            >
              ⚠️ {schema.advisories.length} Missing Index Advisories
            </button>
          )}
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: "0.75rem" }}>
          <input
            type="text"
            placeholder="Search tables..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{
              padding: "4px 10px",
              fontSize: "0.8rem",
              background: "#0d1117",
              border: "1px solid #30363d",
              borderRadius: "6px",
              color: "#c9d1d9",
              width: "180px",
            }}
          />

          <button
            onClick={() => setShowExplainModal(true)}
            style={{
              padding: "4px 12px",
              fontSize: "0.8rem",
              background: "#238636",
              color: "#fff",
              border: "none",
              borderRadius: "6px",
              cursor: "pointer",
              fontWeight: 600,
            }}
          >
            ⚡ Explain SQL
          </button>

          <button
            onClick={loadSchema}
            disabled={loading}
            style={{
              padding: "4px 10px",
              fontSize: "0.8rem",
              background: "#21262d",
              color: "#c9d1d9",
              border: "1px solid #30363d",
              borderRadius: "6px",
              cursor: "pointer",
            }}
          >
            {loading ? "Refreshing..." : "↻ Refresh"}
          </button>

          {/* Zoom Controls */}
          <div style={{ display: "flex", alignItems: "center", gap: "2px" }}>
            <button
              onClick={() => setZoom((z) => Math.max(0.4, z - 0.1))}
              style={{
                background: "#21262d",
                color: "#c9d1d9",
                border: "1px solid #30363d",
                borderRadius: "4px 0 0 4px",
                padding: "4px 8px",
                cursor: "pointer",
              }}
            >
              -
            </button>
            <span
              style={{
                fontSize: "0.75rem",
                padding: "4px 6px",
                background: "#161b22",
                borderTop: "1px solid #30363d",
                borderBottom: "1px solid #30363d",
              }}
            >
              {Math.round(zoom * 100)}%
            </span>
            <button
              onClick={() => setZoom((z) => Math.min(2, z + 0.1))}
              style={{
                background: "#21262d",
                color: "#c9d1d9",
                border: "1px solid #30363d",
                borderRadius: "0 4px 4px 0",
                padding: "4px 8px",
                cursor: "pointer",
              }}
            >
              +
            </button>
            <button
              onClick={resetView}
              style={{
                background: "#21262d",
                color: "#8b949e",
                border: "1px solid #30363d",
                borderRadius: "4px",
                padding: "4px 8px",
                marginLeft: "4px",
                cursor: "pointer",
                fontSize: "0.75rem",
              }}
            >
              Reset
            </button>
          </div>
        </div>
      </div>

      {/* Advisories Bar */}
      {showAdvisories && schema?.advisories && schema.advisories.length > 0 && (
        <div
          style={{
            background: "#2d2200",
            borderBottom: "1px solid #9e6a03",
            padding: "0.75rem 1.25rem",
            fontSize: "0.85rem",
            maxHeight: "140px",
            overflowY: "auto",
            zIndex: 9,
          }}
        >
          <div style={{ fontWeight: 600, color: "#f2cc60", marginBottom: "0.25rem" }}>
            Database Index Optimization Advisories:
          </div>
          {schema.advisories.map((adv, idx) => (
            <div
              key={idx}
              style={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                padding: "3px 0",
                color: "#e3b341",
              }}
            >
              <div>
                • <strong>{adv.table}.{adv.column}</strong>: {adv.suggestion}
              </div>
              {adv.migrationSql && (
                <code
                  style={{
                    background: "#161b22",
                    padding: "2px 6px",
                    borderRadius: "4px",
                    fontSize: "0.75rem",
                    color: "#58a6ff",
                  }}
                >
                  {adv.migrationSql}
                </code>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Canvas Area */}
      <div
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        style={{
          flex: 1,
          position: "relative",
          cursor: isDragging ? "grabbing" : "grab",
          overflow: "hidden",
        }}
      >
        {error && (
          <div style={{ padding: "1.5rem", color: "#f85149", background: "#3d1314" }}>
            {error}
          </div>
        )}

        <div
          style={{
            transform: `translate(${pan.x}px, ${pan.y}px) scale(${zoom})`,
            transformOrigin: "0 0",
            position: "absolute",
            width: "5000px",
            height: "5000px",
          }}
        >
          {/* SVG Connection Lines */}
          <svg
            style={{
              position: "absolute",
              top: 0,
              left: 0,
              width: "100%",
              height: "100%",
              pointerEvents: "none",
            }}
          >
            <defs>
              <marker
                id="arrowhead"
                markerWidth="8"
                markerHeight="6"
                refX="7"
                refY="3"
                orient="auto"
              >
                <polygon points="0 0, 8 3, 0 6" fill="#58a6ff" />
              </marker>
              <marker
                id="arrowhead-hover"
                markerWidth="10"
                markerHeight="7"
                refX="9"
                refY="3.5"
                orient="auto"
              >
                <polygon points="0 0, 10 3.5, 0 7" fill="#f0883e" />
              </marker>
            </defs>

            {(schema?.relations || []).map((rel, idx) => {
              const src = tablePositions[rel.sourceTable];
              const tgt = tablePositions[rel.targetTable];
              if (!src || !tgt) return null;

              const isHovered =
                hoveredRelation === rel ||
                selectedTable === rel.sourceTable ||
                selectedTable === rel.targetTable;

              // Connect from right of source to left of target (or vice versa)
              const startX = src.x + src.width;
              const startY = src.y + 40;
              const endX = tgt.x;
              const endY = tgt.y + 40;

              const dx = Math.abs(endX - startX) * 0.5;
              const pathD = `M ${startX} ${startY} C ${startX + dx} ${startY}, ${endX - dx} ${endY}, ${endX} ${endY}`;

              return (
                <g key={idx}>
                  <path
                    d={pathD}
                    fill="none"
                    stroke={isHovered ? "#f0883e" : "#30363d"}
                    strokeWidth={isHovered ? 2.5 : 1.5}
                    strokeDasharray={rel.type === "BelongsToMany" ? "4 4" : undefined}
                    markerEnd={isHovered ? "url(#arrowhead-hover)" : "url(#arrowhead)"}
                  />
                  {isHovered && (
                    <text
                      x={(startX + endX) / 2}
                      y={(startY + endY) / 2 - 8}
                      fill="#f0883e"
                      fontSize="10"
                      fontWeight="bold"
                      textAnchor="middle"
                    >
                      {rel.type} ({rel.name})
                    </text>
                  )}
                </g>
              );
            })}
          </svg>

          {/* Table Cards */}
          {filteredTables.map((t) => {
            const pos = tablePositions[t.table];
            if (!pos) return null;

            const isSelected = selectedTable === t.table;

            return (
              <div
                key={t.table}
                className="table-node"
                onClick={() => setSelectedTable(isSelected ? null : t.table)}
                style={{
                  position: "absolute",
                  left: `${pos.x}px`,
                  top: `${pos.y}px`,
                  width: `${pos.width}px`,
                  background: isSelected ? "#1c2128" : "#161b22",
                  border: isSelected ? "2px solid #58a6ff" : "1px solid #30363d",
                  borderRadius: "8px",
                  boxShadow: isSelected
                    ? "0 0 16px rgba(88, 166, 255, 0.25)"
                    : "0 4px 12px rgba(0, 0, 0, 0.4)",
                  overflow: "hidden",
                  cursor: "pointer",
                  transition: "border-color 0.15s, box-shadow 0.15s",
                }}
              >
                {/* Header */}
                <div
                  style={{
                    padding: "8px 12px",
                    background: isSelected ? "#21262d" : "#0d1117",
                    borderBottom: "1px solid #30363d",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                  }}
                >
                  <div>
                    <div style={{ fontWeight: 600, fontSize: "0.85rem", color: "#e6edf3" }}>
                      {t.modelName}
                    </div>
                    <div style={{ fontSize: "0.7rem", color: "#8b949e" }}>
                      {t.table}
                    </div>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                    <span
                      style={{
                        fontSize: "0.65rem",
                        background: "#30363d",
                        color: "#8b949e",
                        padding: "2px 6px",
                        borderRadius: "10px",
                      }}
                    >
                      {t.rowCount} rows
                    </span>
                    {onSelectModel && (
                      <button
                        title="Browse records"
                        onClick={(e) => {
                          e.stopPropagation();
                          onSelectModel(t.modelClass);
                        }}
                        style={{
                          background: "none",
                          border: "none",
                          color: "#58a6ff",
                          cursor: "pointer",
                          fontSize: "0.75rem",
                          padding: "2px",
                        }}
                      >
                        ↗
                      </button>
                    )}
                  </div>
                </div>

                {/* Columns List */}
                <div style={{ maxHeight: "240px", overflowY: "auto", padding: "6px 0" }}>
                  {(t.columns || []).map((col) => {
                    const isPk = col.name === t.primaryKey || col.isPrimary;
                    const isFk = col.isForeign || col.name.endsWith("_id");

                    return (
                      <div
                        key={col.name}
                        style={{
                          display: "flex",
                          alignItems: "center",
                          justifyContent: "space-between",
                          padding: "3px 12px",
                          fontSize: "0.75rem",
                          borderBottom: "1px solid #21262d",
                        }}
                      >
                        <div style={{ display: "flex", alignItems: "center", gap: "6px" }}>
                          {isPk ? (
                            <span
                              style={{
                                color: "#f2cc60",
                                fontWeight: 700,
                                fontSize: "0.65rem",
                              }}
                            >
                              PK
                            </span>
                          ) : isFk ? (
                            <span
                              style={{
                                color: "#58a6ff",
                                fontWeight: 700,
                                fontSize: "0.65rem",
                              }}
                            >
                              FK
                            </span>
                          ) : (
                            <span style={{ color: "#484f58", fontSize: "0.65rem" }}>
                              •
                            </span>
                          )}
                          <span
                            style={{
                              color: isPk ? "#e6edf3" : isFk ? "#79c0ff" : "#c9d1d9",
                              fontWeight: isPk || isFk ? 600 : 400,
                            }}
                          >
                            {col.name}
                          </span>
                        </div>
                        <span style={{ color: "#8b949e", fontSize: "0.65rem" }}>
                          {col.type}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* SQL Explain Modal */}
      {showExplainModal && (
        <div
          style={{
            position: "absolute",
            inset: 0,
            background: "rgba(0,0,0,0.75)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 100,
            backdropFilter: "blur(2px)",
          }}
        >
          <div
            style={{
              width: "700px",
              maxHeight: "85vh",
              background: "#161b22",
              border: "1px solid #30363d",
              borderRadius: "10px",
              display: "flex",
              flexDirection: "column",
              overflow: "hidden",
              boxShadow: "0 16px 32px rgba(0,0,0,0.6)",
            }}
          >
            <div
              style={{
                padding: "1rem 1.25rem",
                borderBottom: "1px solid #30363d",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <span style={{ fontWeight: 600, color: "#58a6ff" }}>
                ⚡ SQL Explain & Query Optimizer
              </span>
              <button
                onClick={() => setShowExplainModal(false)}
                style={{
                  background: "none",
                  border: "none",
                  color: "#8b949e",
                  fontSize: "1.2rem",
                  cursor: "pointer",
                }}
              >
                ✕
              </button>
            </div>

            <div style={{ padding: "1.25rem", flex: 1, overflowY: "auto" }}>
              <label style={{ fontSize: "0.8rem", color: "#8b949e", display: "block", marginBottom: "6px" }}>
                Enter SQL Query to Analyze Execution Plan:
              </label>
              <textarea
                rows={4}
                value={explainSql}
                onChange={(e) => setExplainSql(e.target.value)}
                placeholder="SELECT * FROM users WHERE email = 'test@example.com'"
                style={{
                  width: "100%",
                  padding: "8px 12px",
                  background: "#0d1117",
                  border: "1px solid #30363d",
                  borderRadius: "6px",
                  color: "#c9d1d9",
                  fontFamily: "monospace",
                  fontSize: "0.85rem",
                  boxSizing: "border-box",
                }}
              />

              <div style={{ marginTop: "0.75rem", display: "flex", justifyContent: "flex-end" }}>
                <button
                  onClick={handleRunExplain}
                  disabled={explaining || !explainSql.trim()}
                  style={{
                    padding: "6px 16px",
                    background: "#238636",
                    color: "#fff",
                    border: "none",
                    borderRadius: "6px",
                    cursor: "pointer",
                    fontWeight: 600,
                  }}
                >
                  {explaining ? "Analyzing..." : "Run EXPLAIN"}
                </button>
              </div>

              {explainError && (
                <div style={{ marginTop: "1rem", padding: "8px 12px", background: "#3d1314", color: "#f85149", borderRadius: "6px" }}>
                  {explainError}
                </div>
              )}

              {explainResult && (
                <div style={{ marginTop: "1.25rem" }}>
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      padding: "8px 12px",
                      background: explainResult.score >= 80 ? "#1b4721" : explainResult.score >= 50 ? "#433519" : "#490202",
                      borderRadius: "6px",
                      marginBottom: "1rem",
                    }}
                  >
                    <div>
                      <span style={{ fontWeight: 700, fontSize: "1.1rem", color: "#fff" }}>
                        Health Score: {explainResult.score}/100
                      </span>
                      <div style={{ fontSize: "0.8rem", color: "#e6edf3" }}>
                        {explainResult.summary}
                      </div>
                    </div>
                  </div>

                  {explainResult.bottlenecks && explainResult.bottlenecks.length > 0 && (
                    <div style={{ marginBottom: "1rem" }}>
                      <div style={{ fontWeight: 600, fontSize: "0.85rem", color: "#f0883e", marginBottom: "6px" }}>
                        Bottlenecks Detected:
                      </div>
                      {explainResult.bottlenecks.map((b, i) => (
                        <div
                          key={i}
                          style={{
                            padding: "8px 12px",
                            background: "#21262d",
                            borderLeft: "3px solid #f85149",
                            borderRadius: "4px",
                            marginBottom: "6px",
                            fontSize: "0.8rem",
                          }}
                        >
                          <div style={{ fontWeight: 600, color: "#f85149" }}>{b.details}</div>
                          <div style={{ color: "#8b949e", marginTop: "3px" }}>💡 {b.suggestion}</div>
                        </div>
                      ))}
                    </div>
                  )}

                  {/* Raw Plan Output */}
                  {explainResult.raw && explainResult.raw.length > 0 && (
                    <div>
                      <div style={{ fontWeight: 600, fontSize: "0.85rem", color: "#8b949e", marginBottom: "4px" }}>
                        Raw Plan Rows:
                      </div>
                      <pre
                        style={{
                          background: "#0d1117",
                          padding: "8px",
                          borderRadius: "4px",
                          fontSize: "0.75rem",
                          overflowX: "auto",
                          color: "#7ee787",
                        }}
                      >
                        {JSON.stringify(explainResult.raw, null, 2)}
                      </pre>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
