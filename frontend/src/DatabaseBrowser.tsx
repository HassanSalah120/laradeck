import React, { useState, useEffect, useRef } from "react";
import {
  ListModels,
  GetModelRecords,
  UpdateModelRecord,
  DeleteModelRecord,
} from "../wailsjs/go/main/App";
import { formatRecordValue, parseRecordChanges } from "./recordEditing";
import { database } from "../wailsjs/go/models";
import { ErdCanvas } from "./ErdCanvas";

interface DatabaseBrowserProps {
  projectId: string;
  trusted: boolean;
}

export const DatabaseBrowser: React.FC<DatabaseBrowserProps> = ({ projectId, trusted }) => {
  const [viewMode, setViewMode] = useState<"records" | "erd">("records");
  const [models, setModels] = useState<database.ModelInfo[]>([]);
  const [selectedModel, setSelectedModel] = useState<database.ModelInfo | null>(null);
  const [modelFilter, setModelFilter] = useState<string>("");
  const [loadingModels, setLoadingModels] = useState<boolean>(false);

  // Table records state
  const [records, setRecords] = useState<Record<string, any>[]>([]);
  const [columns, setColumns] = useState<string[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [page, setPage] = useState<number>(1);
  const [perPage, setPerPage] = useState<number>(25);
  const [search, setSearch] = useState<string>("");
  const [sortColumn, setSortColumn] = useState<string>("");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("desc");
  const [loadingRecords, setLoadingRecords] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [showFillables, setShowFillables] = useState<boolean>(false);

  // Edit record modal state
  const [editingRecord, setEditingRecord] = useState<Record<string, any> | null>(null);
  const [editFormData, setEditFormData] = useState<Record<string, any>>({});
  const [savingRecord, setSavingRecord] = useState<boolean>(false);
  const [editError, setEditError] = useState<string | null>(null);

  const modelRequest = useRef(0);
  const recordRequest = useRef(0);

  // Load models when project changes
  useEffect(() => {
    loadModels();
    return () => {
      modelRequest.current++;
      recordRequest.current++;
    };
  }, [projectId, trusted]);

  const loadModels = async () => {
    const request = ++modelRequest.current;
    recordRequest.current++;
    setModels([]);
    setSelectedModel(null);
    setRecords([]);
    setColumns([]);
    setTotal(0);
    setEditingRecord(null);
    setEditFormData({});
    setPage(1);

    if (!trusted) {
      setError("This project is untrusted. Trust it first from the header to browse database models.");
      return;
    }
    setLoadingModels(true);
    setError(null);
    try {
      const list = await ListModels(projectId);
      if (request !== modelRequest.current) return;
      setModels(list || []);
      if (list && list.length > 0) {
        setSelectedModel(list[0]);
      } else {
        setSelectedModel(null);
      }
    } catch (err: any) {
      if (request === modelRequest.current) setError(err?.message || String(err));
    } finally {
      if (request === modelRequest.current) setLoadingModels(false);
    }
  };

  // Load records when selected model, page, perPage, or sort changes
  useEffect(() => {
    if (selectedModel) {
      loadRecords();
    } else {
      setRecords([]);
      setColumns([]);
      setTotal(0);
    }
  }, [selectedModel, page, perPage, sortColumn, sortDirection]);

  const loadRecords = async (querySearch?: string) => {
    if (!selectedModel || !trusted) return;
    const request = ++recordRequest.current;
    setLoadingRecords(true);
    setError(null);
    try {
      const searchQuery = querySearch !== undefined ? querySearch : search;
      const res = await GetModelRecords(
        projectId,
        selectedModel.class,
        page,
        perPage,
        searchQuery,
        sortColumn,
        sortDirection
      );
      if (request !== recordRequest.current) return;
      if (res.error) {
        setRecords([]);
        setColumns([]);
        setTotal(0);
        setError(res.error);
      } else {
        setRecords(res.records || []);
        setColumns(res.columns || []);
        setTotal(res.total || 0);
      }
    } catch (err: any) {
      if (request === recordRequest.current) {
        setRecords([]);
        setColumns([]);
        setTotal(0);
        setError(err?.message || String(err));
      }
    } finally {
      if (request === recordRequest.current) setLoadingRecords(false);
    }
  };

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setPage(1);
    loadRecords(search);
  };

  const handleSort = (col: string) => {
    if (sortColumn === col) {
      setSortDirection(sortDirection === "asc" ? "desc" : "asc");
    } else {
      setSortColumn(col);
      setSortDirection("asc");
    }
    setPage(1);
  };

  const handleEditClick = (record: Record<string, any>) => {
    setEditingRecord(record);
    setEditFormData({ ...record });
    setEditError(null);
  };

  const handleSaveEdit = async () => {
    if (!selectedModel || !editingRecord || !trusted || loadingModels) return;
    const pk = selectedModel.primaryKey || "id";
    const recordId = editingRecord[pk];
    if (recordId === undefined || recordId === null) {
      setEditError("Cannot update record without primary key");
      return;
    }

    setSavingRecord(true);
    setEditError(null);
    try {
      const payload = parseRecordChanges(editingRecord, editFormData, selectedModel.casts || {}, pk);
      if (Object.keys(payload).length === 0) {
        setEditingRecord(null);
        return;
      }

      const res = await UpdateModelRecord(projectId, selectedModel.class, recordId, payload);
      if (res.error) {
        setEditError(res.error);
      } else {
        setEditingRecord(null);
        loadRecords();
      }
    } catch (err: any) {
      setEditError(err?.message || String(err));
    } finally {
      setSavingRecord(false);
    }
  };

  const handleDeleteClick = async (record: Record<string, any>) => {
    if (!selectedModel) return;
    const pk = selectedModel.primaryKey || "id";
    const recordId = record[pk];
    if (recordId === undefined || recordId === null) return;

    if (!confirm(`Are you sure you want to delete ${selectedModel.name} #${recordId}? This triggers Eloquent model events.`)) {
      return;
    }

    try {
      await DeleteModelRecord(projectId, selectedModel.class, recordId);
      loadRecords();
    } catch (err: any) {
      setError(err?.message || String(err));
    }
  };

  const filteredModels = models.filter(
    (m) =>
      m.name.toLowerCase().includes(modelFilter.toLowerCase()) ||
      m.table.toLowerCase().includes(modelFilter.toLowerCase())
  );

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", width: "100%", overflow: "hidden" }}>
      {/* Top View Switcher */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "6px 16px",
          background: "rgba(10, 14, 24, 0.95)",
          borderBottom: "1px solid rgba(255, 255, 255, 0.08)",
          zIndex: 5,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
          <button
            type="button"
            onClick={() => setViewMode("records")}
            style={{
              padding: "4px 12px",
              fontSize: "11.5px",
              fontWeight: 600,
              borderRadius: "6px",
              border: viewMode === "records" ? "1px solid #6366f1" : "1px solid rgba(255, 255, 255, 0.1)",
              background: viewMode === "records" ? "rgba(99, 102, 241, 0.2)" : "transparent",
              color: viewMode === "records" ? "#fff" : "var(--text-dim)",
              cursor: "pointer",
            }}
          >
            📊 Data Records
          </button>
          <button
            type="button"
            onClick={() => setViewMode("erd")}
            style={{
              padding: "4px 12px",
              fontSize: "11.5px",
              fontWeight: 600,
              borderRadius: "6px",
              border: viewMode === "erd" ? "1px solid #6366f1" : "1px solid rgba(255, 255, 255, 0.1)",
              background: viewMode === "erd" ? "rgba(99, 102, 241, 0.2)" : "transparent",
              color: viewMode === "erd" ? "#fff" : "var(--text-dim)",
              cursor: "pointer",
            }}
          >
            📐 Visual ERD Canvas
          </button>
        </div>
      </div>

      {viewMode === "erd" ? (
        <div style={{ flex: 1, minHeight: 0 }}>
          <ErdCanvas
            projectId={projectId}
            trusted={trusted}
            onSelectModel={(modelClass) => {
              const m = models.find((x) => x.class === modelClass);
              if (m) {
                setSelectedModel(m);
                setViewMode("records");
              }
            }}
          />
        </div>
      ) : (
        <div className="database-browser" style={{ flex: 1, minHeight: 0 }}>
          {/* Sidebar: Discovered Eloquent Models */}
          <aside className="studio-panel model-list-panel">
        <div style={{ padding: "14px 16px", borderBottom: "1px solid rgba(255, 255, 255, 0.08)" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
            <span style={{ fontSize: "11px", fontWeight: 700, textTransform: "uppercase", letterSpacing: "0.08em", color: "var(--text-dim)" }}>
              MODELS &amp; ENTITIES
            </span>
            <span style={{ fontSize: "11px", background: "rgba(99, 102, 241, 0.2)", color: "#c7d2fe", padding: "1px 7px", borderRadius: 10, fontWeight: 700 }}>
              {models.length}
            </span>
          </div>

          <div style={{ display: "flex", alignItems: "center", background: "rgba(7, 10, 18, 0.8)", border: "1px solid rgba(255, 255, 255, 0.1)", borderRadius: 7, padding: "0 8px" }}>
            <span style={{ fontSize: "12px", color: "var(--text-dim)", marginRight: 6 }}>🔍</span>
            <input
              type="text"
              placeholder="Filter models..."
              aria-label="Filter models"
              value={modelFilter}
              onChange={(e) => setModelFilter(e.target.value)}
              style={{
                width: "100%",
                background: "transparent",
                border: "none",
                color: "#fff",
                fontSize: "12px",
                padding: "6px 0",
                outline: "none",
              }}
            />
            {modelFilter && (
              <button
                type="button"
                onClick={() => setModelFilter("")}
                style={{ background: "transparent", border: "none", color: "var(--text-dim)", cursor: "pointer", fontSize: "11px" }}
              >
                ✕
              </button>
            )}
          </div>
        </div>

        <div style={{ flex: 1, overflowY: "auto", padding: "6px" }}>
          {loadingModels ? (
            <div style={{ padding: 24, fontSize: "12px", color: "var(--text-dim)", textAlign: "center" }}>
              ⏳ Discovering models...
            </div>
          ) : filteredModels.length === 0 ? (
            <div style={{ padding: 24, fontSize: "12px", color: "var(--text-dim)", textAlign: "center" }}>
              No models match filter
            </div>
          ) : (
            filteredModels.map((m) => {
              const active = selectedModel?.class === m.class;
              return (
                <button
                  type="button"
                  className={`model-option ${active ? "selected" : ""}`}
                  aria-pressed={active}
                  key={m.class}
                  onClick={() => {
                    setSelectedModel(m);
                    setPage(1);
                    setSearch("");
                    setSortColumn("");
                  }}
                  style={{
                    padding: "8px 12px",
                    borderRadius: 8,
                    marginBottom: 3,
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    gap: 8,
                  }}
                >
                  <div style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", flex: 1 }}>
                    <div style={{ fontSize: "13px", fontWeight: active ? 600 : 500, color: active ? "#ffffff" : "var(--text-main)" }}>
                      🗃️ {m.name}
                    </div>
                    <div style={{ fontSize: "11px", color: active ? "#c7d2fe" : "var(--text-dim)", fontFamily: "monospace", marginTop: 1 }}>
                      {m.table}
                    </div>
                  </div>
                  <span
                    style={{
                      fontSize: "10.5px",
                      background: active ? "rgba(99, 102, 241, 0.35)" : "rgba(255, 255, 255, 0.06)",
                      color: active ? "#ffffff" : "var(--text-dim)",
                      padding: "2px 7px",
                      borderRadius: 10,
                      fontWeight: 600,
                    }}
                  >
                    {m.count !== undefined ? m.count.toLocaleString() : "—"}
                  </span>
                </button>
              );
            })
          )}
        </div>
      </aside>

      {/* Main Studio Data Grid Pane */}
      <main className="studio-panel" style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
        {selectedModel ? (
          <>
            {/* Header: Model Metadata & Quick Schema Tags */}
            <div style={{ padding: "14px 20px", borderBottom: "1px solid rgba(255, 255, 255, 0.08)", background: "rgba(12, 16, 28, 0.75)" }}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", flexWrap: "wrap", gap: 12 }}>
                <div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <h2 style={{ fontSize: "17px", fontWeight: 700, color: "#fff", margin: 0 }}>
                      {selectedModel.name}
                    </h2>
                    <span style={{ fontSize: "11px", background: "rgba(99, 102, 241, 0.16)", color: "#c7d2fe", border: "1px solid rgba(99, 102, 241, 0.35)", padding: "2px 8px", borderRadius: 5, fontFamily: "monospace" }}>
                      table: {selectedModel.table}
                    </span>
                    <span style={{ fontSize: "11px", background: "rgba(56, 189, 248, 0.12)", color: "#7dd3fc", border: "1px solid rgba(56, 189, 248, 0.3)", padding: "2px 8px", borderRadius: 5 }}>
                      PK: {selectedModel.primaryKey || "id"}
                    </span>
                    <button
                      type="button"
                      onClick={() => setShowFillables(!showFillables)}
                      className="btn-tiny"
                      style={{ fontSize: "10.5px", padding: "2px 8px" }}
                    >
                      {showFillables ? "Hide Fillables ▲" : `Fillables (${selectedModel.fillable?.length || 0}) ▼`}
                    </button>
                  </div>
                  <div style={{ fontSize: "11px", color: "var(--text-dim)", fontFamily: "monospace", marginTop: 3 }}>
                    {selectedModel.class}
                  </div>
                </div>

                {/* Right controls: Reload & Total */}
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <button
                    type="button"
                    className="action-btn small"
                    onClick={() => loadRecords()}
                    disabled={loadingRecords}
                    title="Reload records"
                  >
                    ↻ Refresh
                  </button>
                  <span style={{ fontSize: "12px", background: "rgba(255, 255, 255, 0.05)", border: "1px solid rgba(255, 255, 255, 0.1)", padding: "4px 10px", borderRadius: 6, color: "#cbd5e1" }}>
                    <strong>{total.toLocaleString()}</strong> records
                  </span>
                </div>
              </div>

              {/* Expandable Fillables Tag List */}
              {showFillables && selectedModel.fillable && selectedModel.fillable.length > 0 && (
                <div style={{ marginTop: 10, paddingTop: 10, borderTop: "1px solid rgba(255, 255, 255, 0.05)", display: "flex", flexWrap: "wrap", gap: 5 }}>
                  <span style={{ fontSize: "11px", color: "var(--text-dim)", alignSelf: "center", marginRight: 4 }}>Mass Assignable:</span>
                  {selectedModel.fillable.map((f) => (
                    <span key={f} style={{ fontSize: "10.5px", background: "rgba(255, 255, 255, 0.06)", border: "1px solid rgba(255, 255, 255, 0.1)", padding: "2px 6px", borderRadius: 4, color: "#e2e8f0", fontFamily: "monospace" }}>
                      {f}
                    </span>
                  ))}
                </div>
              )}

              {/* Action Toolbar: Search & Rows per page */}
              <div style={{ marginTop: 12, display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, flexWrap: "wrap" }}>
                <form onSubmit={handleSearchSubmit} style={{ display: "flex", gap: 8, flex: 1, maxWidth: 420 }}>
                  <div style={{ position: "relative", flex: 1, display: "flex", alignItems: "center" }}>
                    <input
                      type="text"
                      aria-label="Search records"
                      placeholder={`Search ${selectedModel.name} columns...`}
                      value={search}
                      onChange={(e) => setSearch(e.target.value)}
                      style={{
                        width: "100%",
                        padding: "7px 10px 7px 28px",
                        background: "rgba(7, 10, 18, 0.8)",
                        border: "1px solid rgba(255, 255, 255, 0.12)",
                        borderRadius: 7,
                        color: "var(--text-main)",
                        fontSize: "12px",
                      }}
                    />
                    <span style={{ position: "absolute", left: 9, color: "var(--text-dim)", fontSize: "12px" }}>🔍</span>
                  </div>
                  <button type="submit" className="action-btn small primary">
                    Filter
                  </button>
                  {search && (
                    <button
                      type="button"
                      className="action-btn small"
                      onClick={() => {
                        setSearch("");
                        loadRecords("");
                      }}
                    >
                      Reset
                    </button>
                  )}
                </form>

                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <span style={{ fontSize: "11.5px", color: "var(--text-dim)" }}>Rows:</span>
                  <select
                    aria-label="Rows per page"
                    value={perPage}
                    onChange={(e) => {
                      setPerPage(Number(e.target.value));
                      setPage(1);
                    }}
                    style={{
                      background: "rgba(7, 10, 18, 0.8)",
                      border: "1px solid rgba(255, 255, 255, 0.12)",
                      borderRadius: 6,
                      color: "var(--text-main)",
                      padding: "4px 8px",
                      fontSize: "11.5px",
                    }}
                  >
                    <option value={10}>10</option>
                    <option value={25}>25</option>
                    <option value={50}>50</option>
                    <option value={100}>100</option>
                  </select>
                </div>
              </div>
            </div>

            {/* Table Area */}
            <div style={{ flex: 1, overflow: "auto" }}>
              {error && (
                <div style={{ padding: "12px 18px", background: "rgba(244, 63, 94, 0.14)", borderBottom: "1px solid rgba(244, 63, 94, 0.3)", color: "#fda4af", fontSize: "12.5px" }}>
                  {error}
                </div>
              )}

              {loadingRecords ? (
                <div style={{ padding: 48, textAlign: "center", color: "var(--text-dim)", fontSize: "13px" }}>
                  ⏳ Loading {selectedModel.name} data records...
                </div>
              ) : records.length === 0 ? (
                <div style={{ padding: 64, textAlign: "center", color: "var(--text-dim)" }}>
                  <div style={{ fontSize: "32px", marginBottom: 8 }}>🔍</div>
                  <div style={{ fontSize: "14px", fontWeight: 600, color: "#fff" }}>No records found</div>
                  <p style={{ fontSize: "12px", marginTop: 4 }}>Try clearing search criteria or selecting another model.</p>
                </div>
              ) : (
                <table className="record-table">
                  <thead>
                    <tr>
                      <th style={{ width: 85, position: "sticky", left: 0, zIndex: 2, background: "rgba(15, 23, 42, 0.95)" }}>
                        ACTIONS
                      </th>
                      {columns.map((col) => {
                        const isSorted = sortColumn === col;
                        return (
                          <th
                            key={col}
                            aria-sort={isSorted ? (sortDirection === "asc" ? "ascending" : "descending") : "none"}
                          >
                            <button className="record-sort" onClick={() => handleSort(col)}>
                              {col} {isSorted ? (sortDirection === "asc" ? "▲" : "▼") : ""}
                            </button>
                          </th>
                        );
                      })}
                    </tr>
                  </thead>
                  <tbody>
                    {records.map((r, rIdx) => {
                      const pk = selectedModel.primaryKey || "id";
                      const rowKey = r[pk] !== undefined ? String(r[pk]) : String(rIdx);
                      return (
                        <tr key={rowKey}>
                          <td style={{ position: "sticky", left: 0, zIndex: 1, background: "rgba(13, 19, 33, 0.95)", whiteSpace: "nowrap" }}>
                            <div style={{ display: "flex", gap: 5 }}>
                              <button
                                onClick={() => handleEditClick(r)}
                                className="btn-table-action edit"
                                disabled={loadingModels || !trusted}
                                title="Edit record"
                              >
                                ✏️
                              </button>
                              <button
                                onClick={() => handleDeleteClick(r)}
                                className="btn-table-action del"
                                disabled={loadingModels || !trusted}
                                title="Delete record"
                              >
                                🗑️
                              </button>
                            </div>
                          </td>
                          {columns.map((col) => {
                            const val = r[col];

                            if (val === null || val === undefined) {
                              return (
                                <td key={col}>
                                  <span className="db-badge-null">NULL</span>
                                </td>
                              );
                            }

                            if (typeof val === "boolean") {
                              return (
                                <td key={col}>
                                  <span className={`db-badge-bool ${val ? "true" : "false"}`}>
                                    {val ? "TRUE" : "FALSE"}
                                  </span>
                                </td>
                              );
                            }

                            if (typeof val === "object") {
                              return (
                                <td key={col} title={JSON.stringify(val, null, 2)}>
                                  <span className="db-badge-json">{JSON.stringify(val)}</span>
                                </td>
                              );
                            }

                            return (
                              <td
                                key={col}
                                style={{
                                  maxWidth: 260,
                                  overflow: "hidden",
                                  textOverflow: "ellipsis",
                                  whiteSpace: "nowrap",
                                }}
                                title={String(val)}
                              >
                                {col === pk ? <span className="font-mono" style={{ color: "#7dd3fc" }}>{String(val)}</span> : String(val)}
                              </td>
                            );
                          })}
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              )}
            </div>

            {/* Pagination Footer */}
            <div
              style={{
                padding: "10px 20px",
                borderTop: "1px solid rgba(255, 255, 255, 0.08)",
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                background: "rgba(12, 16, 28, 0.8)",
                fontSize: "12px",
              }}
            >
              <span style={{ color: "var(--text-muted)" }}>
                Showing page <strong>{page}</strong> of <strong>{Math.max(1, Math.ceil(total / perPage))}</strong> ({total.toLocaleString()} total)
              </span>
              <div style={{ display: "flex", gap: 8 }}>
                <button
                  className="action-btn small"
                  disabled={page <= 1 || loadingRecords}
                  onClick={() => setPage(page - 1)}
                >
                  ◀ Previous
                </button>
                <button
                  className="action-btn small"
                  disabled={page >= Math.ceil(total / perPage) || loadingRecords}
                  onClick={() => setPage(page + 1)}
                >
                  Next ▶
                </button>
              </div>
            </div>
          </>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", height: "100%", color: "var(--text-muted)", padding: 48 }}>
            <div style={{ fontSize: "40px", marginBottom: 12 }}>🗄️</div>
            <div style={{ fontSize: "16px", fontWeight: 700, color: "#fff" }}>Select an Eloquent Model</div>
            <p style={{ fontSize: "12.5px", color: "var(--text-dim)", marginTop: 4, maxWidth: 360, textAlign: "center" }}>
              Choose a discovered entity from the sidebar to browse table attributes, search records, and edit data.
            </p>
          </div>
        )}
      </main>

      {/* Edit Record Modal */}
      {editingRecord && selectedModel && (
        <div className="modal-backdrop">
          <div className="modal" style={{ maxWidth: 580 }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
              <div>
                <h2>
                  Edit {selectedModel.name} #{editingRecord[selectedModel.primaryKey || "id"]}
                </h2>
                <span style={{ fontSize: "11px", color: "var(--text-dim)", fontFamily: "monospace" }}>
                  Table: {selectedModel.table}
                </span>
              </div>
              <button
                onClick={() => setEditingRecord(null)}
                aria-label="Close record editor"
                style={{ background: "transparent", border: "none", color: "var(--text-dim)", fontSize: "16px", cursor: "pointer" }}
              >
                ✕
              </button>
            </div>

            <div style={{ maxHeight: "60vh", overflowY: "auto", paddingRight: 6 }}>
              {editError && (
                <div style={{ background: "rgba(244, 63, 94, 0.15)", border: "1px solid rgba(244, 63, 94, 0.35)", borderRadius: 8, padding: "8px 14px", color: "#fda4af", fontSize: "12px", marginBottom: 14 }}>
                  {editError}
                </div>
              )}

              <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
                {columns.map((col) => {
                  const pk = selectedModel.primaryKey || "id";
                  const isPk = col === pk;
                  const isTimestamp = col === "created_at" || col === "updated_at";
                  const val = editFormData[col];
                  const strVal = formatRecordValue(val);

                  return (
                    <div key={col}>
                      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 4 }}>
                        <label style={{ fontSize: "11.5px", fontWeight: 600, color: "#cbd5e1", fontFamily: "monospace" }}>
                          {col}
                        </label>
                        {isPk ? (
                          <span style={{ fontSize: "10px", color: "#38bdf8", background: "rgba(56, 189, 248, 0.12)", padding: "1px 6px", borderRadius: 4 }}>
                            PRIMARY KEY (READ-ONLY)
                          </span>
                        ) : isTimestamp ? (
                          <span style={{ fontSize: "10px", color: "var(--text-dim)" }}>TIMESTAMP (AUTO)</span>
                        ) : null}
                      </div>

                      <input
                        aria-label={col}
                        type="text"
                        disabled={isPk || isTimestamp || savingRecord || val === null}
                        value={strVal}
                        onChange={(e) => {
                          setEditFormData({
                            ...editFormData,
                            [col]: e.target.value,
                          });
                        }}
                        style={{
                          width: "100%",
                          boxSizing: "border-box",
                          padding: "8px 12px",
                          background: isPk || isTimestamp ? "rgba(255,255,255,0.03)" : "rgba(7, 10, 18, 0.8)",
                          border: "1px solid rgba(255, 255, 255, 0.12)",
                          borderRadius: 7,
                          color: isPk || isTimestamp ? "var(--text-dim)" : "#fff",
                          fontSize: "12.5px",
                        }}
                      />

                      {!isPk && !isTimestamp && (
                        <label style={{ display: "inline-flex", alignItems: "center", gap: 6, marginTop: 4, fontSize: "11px", color: "var(--text-dim)", cursor: "pointer" }}>
                          <input
                            type="checkbox"
                            aria-label={`${col}: set NULL`}
                            checked={val === null}
                            disabled={savingRecord}
                            onChange={(e) =>
                              setEditFormData({ ...editFormData, [col]: e.target.checked ? null : "" })
                            }
                          />
                          Set column value to NULL
                        </label>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>

            <div style={{ display: "flex", justifyContent: "flex-end", gap: 10, marginTop: 20, paddingTop: 14, borderTop: "1px solid rgba(255, 255, 255, 0.08)" }}>
              <button
                className="btn"
                onClick={() => setEditingRecord(null)}
                disabled={savingRecord}
              >
                Cancel
              </button>
              <button
                className="btn primary"
                onClick={handleSaveEdit}
                disabled={savingRecord}
              >
                {savingRecord ? "Saving..." : "Save Changes"}
              </button>
            </div>
          </div>
        </div>
      )}
        </div>
      )}
    </div>
  );
};
