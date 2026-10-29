import React, { useState, useEffect } from "react";
import {
  ListRuntimes,
  InstallPHP,
  ToggleExtension,
  GetComposerStatus,
  InstallComposer,
  AddBinToUserPath,
  IsolateHerdSite,
} from "../wailsjs/go/main/App";
import { runtimes } from "../wailsjs/go/models";

interface PhpRuntimePanelProps {
  projectId?: string;
  projectName?: string;
}

export const PhpRuntimePanel: React.FC<PhpRuntimePanelProps> = ({
  projectId,
  projectName,
}) => {
  const [runtimeList, setRuntimeList] = useState<runtimes.RuntimeInfo[]>([]);
  const [composer, setComposer] = useState<runtimes.ComposerStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [installingVer, setInstallingVer] = useState<string | null>(null);
  const [busyComposer, setBusyComposer] = useState(false);
  const [expandedRuntime, setExpandedRuntime] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    loadData();
  }, [projectId]);

  const loadData = async () => {
    setLoading(true);
    try {
      const [rts, comp] = await Promise.all([
        ListRuntimes(),
        GetComposerStatus(),
      ]);
      setRuntimeList(rts || []);
      setComposer(comp);
    } catch (err: any) {
      console.error("Failed to load runtimes:", err);
    } finally {
      setLoading(false);
    }
  };

  const showNotice = (msg: string) => {
    setNotice(msg);
    setTimeout(() => setNotice(null), 4000);
  };

  const handleInstallPHP = async (ver: string) => {
    setInstallingVer(ver);
    try {
      await InstallPHP(ver);
      showNotice(`PHP ${ver} successfully installed and configured.`);
      await loadData();
    } catch (err: any) {
      alert("Failed to install PHP: " + (err?.message || String(err)));
    } finally {
      setInstallingVer(null);
    }
  };

  const handleToggleExtension = async (
    iniPath: string,
    extName: string,
    currentEnabled: boolean
  ) => {
    try {
      await ToggleExtension(iniPath, extName, !currentEnabled);
      showNotice(`${extName} ${!currentEnabled ? "enabled" : "disabled"}`);
      await loadData();
    } catch (err: any) {
      alert("Failed to toggle extension: " + (err?.message || String(err)));
    }
  };

  const handleInstallComposer = async () => {
    setBusyComposer(true);
    try {
      await InstallComposer();
      showNotice("Composer successfully installed with CLI shims.");
      await loadData();
    } catch (err: any) {
      alert("Failed to install Composer: " + (err?.message || String(err)));
    } finally {
      setBusyComposer(false);
    }
  };

  const handleAddToPath = async () => {
    try {
      await AddBinToUserPath();
      showNotice("~/.laradeck/bin successfully added to User PATH.");
      await loadData();
    } catch (err: any) {
      alert("Failed to update PATH: " + (err?.message || String(err)));
    }
  };

  const handleIsolateSite = async (ver: string) => {
    if (!projectId) return;
    try {
      await IsolateHerdSite(projectId, ver);
      showNotice(`Project isolated to PHP ${ver} in Herd.`);
    } catch (err: any) {
      alert("Herd isolate failed: " + (err?.message || String(err)));
    }
  };

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        background: "#0d1117",
        color: "#c9d1d9",
        overflowY: "auto",
        padding: "1.5rem",
        gap: "1.5rem",
      }}
    >
      {/* Header */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          borderBottom: "1px solid #30363d",
          paddingBottom: "1rem",
        }}
      >
        <div>
          <h2 style={{ margin: "0 0 4px 0", fontSize: "1.3rem", color: "#e6edf3" }}>
            🐘 PHP Versions &amp; Composer Runtime Manager
          </h2>
          <p style={{ margin: 0, fontSize: "0.85rem", color: "#8b949e" }}>
            Manage standalone official Windows PHP runtimes, inspect Herd PHP versions, and configure extensions without manual INI editing.
          </p>
        </div>
        <div style={{ display: "flex", gap: "10px", alignItems: "center" }}>
          {notice && (
            <span
              style={{
                fontSize: "0.8rem",
                background: "#23863633",
                color: "#3fb950",
                border: "1px solid #238636",
                borderRadius: "6px",
                padding: "4px 10px",
              }}
            >
              ✓ {notice}
            </span>
          )}
          <button
            onClick={loadData}
            disabled={loading}
            style={{
              padding: "6px 12px",
              background: "#21262d",
              color: "#c9d1d9",
              border: "1px solid #30363d",
              borderRadius: "6px",
              cursor: "pointer",
            }}
          >
            ↻ Refresh
          </button>
        </div>
      </div>

      {/* Composer Section */}
      <div
        style={{
          background: "#161b22",
          border: "1px solid #30363d",
          borderRadius: "8px",
          padding: "1rem 1.25rem",
        }}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            flexWrap: "wrap",
            gap: "10px",
          }}
        >
          <div>
            <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
              <h3 style={{ margin: 0, fontSize: "1rem", color: "#e6edf3" }}>
                🎼 Composer Dependency Manager
              </h3>
              <span
                style={{
                  fontSize: "0.75rem",
                  padding: "2px 8px",
                  borderRadius: "10px",
                  fontWeight: 600,
                  background: composer?.installed ? "#23863622" : "#f8514922",
                  color: composer?.installed ? "#3fb950" : "#f85149",
                  border: `1px solid ${composer?.installed ? "#238636" : "#f85149"}`,
                }}
              >
                {composer?.installed ? "● INSTALLED" : "○ NOT INSTALLED"}
              </span>
            </div>
            <div style={{ fontSize: "0.8rem", color: "#8b949e", marginTop: "4px" }}>
              {composer?.installed ? (
                <>
                  Version: <strong style={{ color: "#79c0ff" }}>{composer.version}</strong> • Path: <code>{composer.path}</code>
                </>
              ) : (
                "Standalone Composer executable and shell shims not installed in ~/.laradeck/bin."
              )}
            </div>
          </div>

          <div style={{ display: "flex", gap: "8px" }}>
            {!composer?.installed ? (
              <button
                onClick={handleInstallComposer}
                disabled={busyComposer}
                style={{
                  padding: "6px 14px",
                  background: "#238636",
                  color: "#fff",
                  border: "none",
                  borderRadius: "6px",
                  cursor: "pointer",
                  fontWeight: 600,
                }}
              >
                {busyComposer ? "Installing..." : "Install Composer"}
              </button>
            ) : (
              <>
                <button
                  onClick={handleInstallComposer}
                  disabled={busyComposer}
                  style={{
                    padding: "4px 10px",
                    background: "#21262d",
                    color: "#c9d1d9",
                    border: "1px solid #30363d",
                    borderRadius: "6px",
                    cursor: "pointer",
                    fontSize: "0.8rem",
                  }}
                >
                  Update Composer
                </button>
                {!composer.inPath && (
                  <button
                    onClick={handleAddToPath}
                    style={{
                      padding: "4px 10px",
                      background: "#1f6feb22",
                      color: "#58a6ff",
                      border: "1px solid #1f6feb",
                      borderRadius: "6px",
                      cursor: "pointer",
                      fontSize: "0.8rem",
                    }}
                  >
                    + Add ~/.laradeck/bin to PATH
                  </button>
                )}
              </>
            )}
          </div>
        </div>
      </div>

      {/* Runtimes List */}
      <div>
        <h3 style={{ margin: "0 0 12px 0", fontSize: "1.1rem", color: "#e6edf3" }}>
          Installed &amp; Available PHP Runtimes
        </h3>
        <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
          {runtimeList.map((rt) => {
            const isExpanded = expandedRuntime === rt.id;
            return (
              <div
                key={rt.id}
                style={{
                  background: "#161b22",
                  border: "1px solid #30363d",
                  borderRadius: "8px",
                  overflow: "hidden",
                }}
              >
                {/* Runtime Summary Row */}
                <div
                  style={{
                    padding: "1rem 1.25rem",
                    display: "flex",
                    justifyContent: "space-between",
                    alignItems: "center",
                    flexWrap: "wrap",
                    gap: "10px",
                  }}
                >
                  <div>
                    <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
                      <span style={{ fontSize: "1.1rem" }}>
                        {rt.type === "herd" ? "🐎" : "📦"}
                      </span>
                      <strong style={{ fontSize: "0.95rem", color: "#f0f6fc" }}>
                        {rt.name}
                      </strong>
                      <span
                        style={{
                          fontSize: "0.75rem",
                          padding: "2px 8px",
                          borderRadius: "10px",
                          background: rt.installed ? "#23863622" : "#8b949e22",
                          color: rt.installed ? "#3fb950" : "#8b949e",
                          border: `1px solid ${rt.installed ? "#238636" : "#30363d"}`,
                        }}
                      >
                        {rt.installed ? "INSTALLED" : "AVAILABLE"}
                      </span>
                    </div>
                    {rt.installed && (
                      <div
                        style={{
                          fontSize: "0.75rem",
                          color: "#8b949e",
                          marginTop: "6px",
                          display: "flex",
                          flexDirection: "column",
                          gap: "2px",
                        }}
                      >
                        <div>
                          Binary: <code>{rt.binaryPath}</code>
                        </div>
                        {rt.iniPath && (
                          <div>
                            Config: <code>{rt.iniPath}</code>
                          </div>
                        )}
                      </div>
                    )}
                  </div>

                  <div style={{ display: "flex", gap: "8px", alignItems: "center" }}>
                    {rt.installed ? (
                      <>
                        {rt.iniPath && (
                          <button
                            onClick={() =>
                              setExpandedRuntime(isExpanded ? null : rt.id)
                            }
                            style={{
                              padding: "4px 10px",
                              background: isExpanded ? "#30363d" : "#21262d",
                              color: "#c9d1d9",
                              border: "1px solid #30363d",
                              borderRadius: "6px",
                              cursor: "pointer",
                              fontSize: "0.8rem",
                            }}
                          >
                            {isExpanded ? "Hide Extensions ▲" : "Manage Extensions ▼"}
                          </button>
                        )}
                        {rt.type === "herd" && projectId && (
                          <button
                            onClick={() => handleIsolateSite(rt.version)}
                            style={{
                              padding: "4px 10px",
                              background: "#23863622",
                              color: "#3fb950",
                              border: "1px solid #238636",
                              borderRadius: "6px",
                              cursor: "pointer",
                              fontSize: "0.8rem",
                            }}
                          >
                            Use for {projectName || "Active Project"}
                          </button>
                        )}
                      </>
                    ) : (
                      <button
                        onClick={() =>
                          handleInstallPHP(rt.id.replace("standalone-", ""))
                        }
                        disabled={installingVer === rt.id.replace("standalone-", "")}
                        style={{
                          padding: "6px 14px",
                          background: "#238636",
                          color: "#fff",
                          border: "none",
                          borderRadius: "6px",
                          cursor: "pointer",
                          fontWeight: 600,
                          fontSize: "0.85rem",
                        }}
                      >
                        {installingVer === rt.id.replace("standalone-", "")
                          ? "Downloading & Installing..."
                          : `Install PHP ${rt.version}`}
                      </button>
                    )}
                  </div>
                </div>

                {/* Extensions Management Panel */}
                {isExpanded && rt.extensions && rt.extensions.length > 0 && (
                  <div
                    style={{
                      background: "#0d1117",
                      borderTop: "1px solid #30363d",
                      padding: "1rem 1.25rem",
                    }}
                  >
                    <div style={{ marginBottom: "10px" }}>
                      <strong style={{ fontSize: "0.85rem", color: "#e6edf3" }}>
                        🧩 Extension Modules for {rt.name}
                      </strong>
                      <span
                        style={{
                          fontSize: "0.75rem",
                          color: "#8b949e",
                          marginLeft: "10px",
                        }}
                      >
                        Click a module toggle to enable or disable in php.ini
                      </span>
                    </div>

                    <div
                      style={{
                        display: "grid",
                        gridTemplateColumns:
                          "repeat(auto-fill, minmax(280px, 1fr))",
                        gap: "8px",
                      }}
                    >
                      {rt.extensions.map((ext) => (
                        <div
                          key={ext.name}
                          style={{
                            display: "flex",
                            justifyContent: "space-between",
                            alignItems: "center",
                            background: "#161b22",
                            border: `1px solid ${
                              ext.enabled ? "#238636" : "#30363d"
                            }`,
                            borderRadius: "6px",
                            padding: "8px 10px",
                          }}
                        >
                          <div>
                            <strong
                              style={{
                                color: ext.enabled ? "#7ee787" : "#c9d1d9",
                                fontSize: "0.85rem",
                              }}
                            >
                              {ext.name}
                            </strong>
                            {ext.description && (
                              <div
                                style={{
                                  fontSize: "0.7rem",
                                  color: "#8b949e",
                                  marginTop: "2px",
                                }}
                              >
                                {ext.description}
                              </div>
                            )}
                          </div>

                          <button
                            onClick={() =>
                              handleToggleExtension(
                                rt.iniPath,
                                ext.name,
                                ext.enabled
                              )
                            }
                            style={{
                              padding: "4px 8px",
                              fontSize: "0.75rem",
                              fontWeight: 600,
                              borderRadius: "4px",
                              cursor: "pointer",
                              border: "none",
                              background: ext.enabled ? "#238636" : "#21262d",
                              color: ext.enabled ? "#fff" : "#8b949e",
                            }}
                          >
                            {ext.enabled ? "ON" : "OFF"}
                          </button>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
};

export default PhpRuntimePanel;
