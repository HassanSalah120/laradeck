import React, { useState, useEffect } from "react";
import {
  ListRemoteProfiles,
  SaveRemoteProfile,
  DeleteRemoteProfile,
  TestRemoteConnection,
  AssessRemoteCommandSafety,
} from "../wailsjs/go/main/App";
import { remote } from "../wailsjs/go/models";

export const RemoteProfilesPanel: React.FC = () => {
  const [profiles, setProfiles] = useState<remote.RemoteProfile[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  // Testing connection state
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<Record<string, remote.RemoteTestResult>>({});

  // Edit / Create modal state
  const [editingProfile, setEditingProfile] = useState<remote.RemoteProfile | null>(null);
  const [formData, setFormData] = useState<Partial<remote.RemoteProfile>>({
    name: "",
    host: "",
    port: 22,
    user: "forge",
    authType: "key",
    keyPath: "~/.ssh/id_rsa",
    remotePath: "/var/www/my-app",
    environment: "production",
    readOnly: true,
  });

  // Safety test sandbox state
  const [safetyCmd, setSafetyCmd] = useState<string>("php artisan migrate:fresh");
  const [safetyToken, setSafetyToken] = useState<string>("");
  const [safetyAssessment, setSafetyAssessment] = useState<remote.SafetyAssessment | null>(null);

  useEffect(() => {
    loadProfiles();
  }, []);

  const loadProfiles = async () => {
    setLoading(true);
    try {
      const list = await ListRemoteProfiles();
      setProfiles(list || []);
    } catch (err: any) {
      setError(err?.message || String(err));
    } finally {
      setLoading(false);
    }
  };

  const handleTestConnection = async (id: string) => {
    setTestingId(id);
    try {
      const res = await TestRemoteConnection(id);
      setTestResult((prev) => ({ ...prev, [id]: res }));
      loadProfiles();
    } catch (err: any) {
      setTestResult((prev) => ({
        ...prev,
        [id]: { reachable: false, latencyMs: 0, error: err?.message || String(err) },
      }));
    } finally {
      setTestingId(null);
    }
  };

  const handleSave = async () => {
    try {
      await SaveRemoteProfile(formData as remote.RemoteProfile);
      setEditingProfile(null);
      loadProfiles();
    } catch (err: any) {
      alert("Failed to save profile: " + (err?.message || String(err)));
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm("Are you sure you want to delete this remote profile?")) return;
    try {
      await DeleteRemoteProfile(id);
      loadProfiles();
    } catch (err: any) {
      alert("Failed to delete profile: " + (err?.message || String(err)));
    }
  };

  const handleCheckSafety = async (profileId: string) => {
    try {
      const res = await AssessRemoteCommandSafety(profileId, safetyCmd, safetyToken);
      setSafetyAssessment(res);
    } catch (err: any) {
      alert("Safety check error: " + (err?.message || String(err)));
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
      }}
    >
      {/* Top Banner */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: "1.5rem",
        }}
      >
        <div>
          <h2 style={{ margin: 0, fontSize: "1.3rem", color: "#e6edf3", display: "flex", alignItems: "center", gap: "8px" }}>
            🛡️ Remote SSH Profiles &amp; Production Safety Vault
          </h2>
          <p style={{ margin: "4px 0 0 0", fontSize: "0.85rem", color: "#8b949e" }}>
            Safely orchestrate staging and production Laravel servers with strict read-only interlocks.
          </p>
        </div>

        <button
          onClick={() => {
            setFormData({
              name: "",
              host: "",
              port: 22,
              user: "forge",
              authType: "key",
              keyPath: "~/.ssh/id_rsa",
              remotePath: "/var/www/my-app",
              environment: "production",
              readOnly: true,
            });
            setEditingProfile({} as remote.RemoteProfile);
          }}
          style={{
            background: "#238636",
            color: "#fff",
            border: "none",
            borderRadius: "6px",
            padding: "8px 16px",
            fontSize: "0.85rem",
            fontWeight: 600,
            cursor: "pointer",
          }}
        >
          + Add Remote Profile
        </button>
      </div>

      {error && (
        <div style={{ padding: "10px 14px", background: "#3d1314", color: "#f85149", borderRadius: "6px", marginBottom: "1rem" }}>
          {error}
        </div>
      )}

      {/* Profiles Grid */}
      {profiles.length === 0 ? (
        <div
          style={{
            padding: "3rem",
            textAlign: "center",
            background: "#161b22",
            border: "1px dashed #30363d",
            borderRadius: "8px",
          }}
        >
          <div style={{ fontSize: "2rem", marginBottom: "0.5rem" }}>🌐</div>
          <div style={{ fontWeight: 600, color: "#e6edf3" }}>No Remote Profiles Configured</div>
          <p style={{ color: "#8b949e", fontSize: "0.85rem", maxWidth: "420px", margin: "6px auto 14px auto" }}>
            Configure remote SSH connections to your VPS, Forge, or Vapor servers to inspect logs and telemetry with read-only safety guardrails.
          </p>
        </div>
      ) : (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(360px, 1fr))", gap: "1rem", marginBottom: "2rem" }}>
          {profiles.map((p) => {
            const isProd = p.environment?.toLowerCase() === "production";
            const isStaging = p.environment?.toLowerCase() === "staging";
            const res = testResult[p.id];

            return (
              <div
                key={p.id}
                style={{
                  background: "#161b22",
                  border: isProd ? "1px solid #da363366" : "1px solid #30363d",
                  borderRadius: "8px",
                  padding: "1.25rem",
                  boxShadow: "0 4px 12px rgba(0,0,0,0.3)",
                }}
              >
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "8px" }}>
                  <div>
                    <h3 style={{ margin: 0, fontSize: "1rem", color: "#e6edf3" }}>{p.name}</h3>
                    <code style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                      {p.user}@{p.host}:{p.port}
                    </code>
                  </div>

                  <div style={{ display: "flex", gap: "6px" }}>
                    <span
                      style={{
                        fontSize: "0.65rem",
                        fontWeight: 700,
                        textTransform: "uppercase",
                        padding: "2px 8px",
                        borderRadius: "10px",
                        background: isProd ? "#da363322" : isStaging ? "#d2992222" : "#23863622",
                        color: isProd ? "#f85149" : isStaging ? "#f0883e" : "#3fb950",
                        border: `1px solid ${isProd ? "#da363355" : isStaging ? "#d2992255" : "#23863655"}`,
                      }}
                    >
                      {p.environment}
                    </span>
                    {p.readOnly && (
                      <span
                        style={{
                          fontSize: "0.65rem",
                          fontWeight: 700,
                          padding: "2px 6px",
                          borderRadius: "10px",
                          background: "#1f6feb22",
                          color: "#58a6ff",
                          border: "1px solid #1f6feb55",
                        }}
                      >
                        🔒 READ-ONLY
                      </span>
                    )}
                  </div>
                </div>

                <div style={{ fontSize: "0.8rem", color: "#8b949e", marginBottom: "12px" }}>
                  Path: <code>{p.remotePath}</code>
                </div>

                {/* Connection Status & Results */}
                <div
                  style={{
                    background: "#0d1117",
                    borderRadius: "6px",
                    padding: "8px 12px",
                    marginBottom: "12px",
                    fontSize: "0.75rem",
                  }}
                >
                  <div style={{ display: "flex", justifyContent: "space-between", marginBottom: "4px" }}>
                    <span>Status:</span>
                    <strong style={{ color: p.status === "reachable" ? "#7ee787" : p.status === "unreachable" ? "#f85149" : "#8b949e" }}>
                      {p.status ? p.status.toUpperCase() : "UNTESTED"}
                      {p.latencyMs ? ` (${p.latencyMs}ms)` : ""}
                    </strong>
                  </div>
                  {p.remotePhp && <div>PHP: <code>{p.remotePhp}</code></div>}
                  {p.remoteVer && <div>Artisan: <code>{p.remoteVer}</code></div>}
                  {res?.error && <div style={{ color: "#f85149", marginTop: "4px" }}>⚠️ {res.error}</div>}
                </div>

                {/* Actions */}
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <button
                    onClick={() => handleTestConnection(p.id)}
                    disabled={testingId === p.id}
                    style={{
                      background: "#21262d",
                      color: "#58a6ff",
                      border: "1px solid #30363d",
                      padding: "4px 10px",
                      borderRadius: "6px",
                      fontSize: "0.75rem",
                      cursor: "pointer",
                      fontWeight: 600,
                    }}
                  >
                    {testingId === p.id ? "Probing..." : "⚡ Probe Connection"}
                  </button>

                  <div style={{ display: "flex", gap: "6px" }}>
                    <button
                      onClick={() => {
                        setFormData(p);
                        setEditingProfile(p);
                      }}
                      style={{
                        background: "#21262d",
                        color: "#c9d1d9",
                        border: "1px solid #30363d",
                        padding: "4px 10px",
                        borderRadius: "6px",
                        fontSize: "0.75rem",
                        cursor: "pointer",
                      }}
                    >
                      Edit
                    </button>
                    <button
                      onClick={() => handleDelete(p.id)}
                      style={{
                        background: "#21262d",
                        color: "#f85149",
                        border: "1px solid #30363d",
                        padding: "4px 8px",
                        borderRadius: "6px",
                        fontSize: "0.75rem",
                        cursor: "pointer",
                      }}
                    >
                      Delete
                    </button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Safety Interlock Sandbox Tester */}
      <div
        style={{
          background: "#161b22",
          border: "1px solid #30363d",
          borderRadius: "8px",
          padding: "1.25rem",
        }}
      >
        <h3 style={{ margin: "0 0 8px 0", fontSize: "0.95rem", color: "#f0883e" }}>
          🛡️ Production Safety Interlock Sandbox
        </h3>
        <p style={{ margin: "0 0 12px 0", fontSize: "0.8rem", color: "#8b949e" }}>
          Verify how the LaraDeck safety engine intercepts destructive commands before they ever execute.
        </p>

        <div style={{ display: "flex", gap: "8px", flexWrap: "wrap", marginBottom: "10px" }}>
          <input
            type="text"
            value={safetyCmd}
            onChange={(e) => setSafetyCmd(e.target.value)}
            placeholder="Command (e.g. php artisan migrate:fresh)"
            style={{
              flex: 1,
              minWidth: "240px",
              padding: "6px 10px",
              background: "#0d1117",
              border: "1px solid #30363d",
              borderRadius: "6px",
              color: "#c9d1d9",
              fontSize: "0.85rem",
              fontFamily: "monospace",
            }}
          />
          <input
            type="text"
            value={safetyToken}
            onChange={(e) => setSafetyToken(e.target.value)}
            placeholder="Confirmation Token"
            style={{
              width: "200px",
              padding: "6px 10px",
              background: "#0d1117",
              border: "1px solid #30363d",
              borderRadius: "6px",
              color: "#c9d1d9",
              fontSize: "0.85rem",
              fontFamily: "monospace",
            }}
          />
          <button
            onClick={() => {
              if (profiles.length > 0) handleCheckSafety(profiles[0].id);
              else alert("Create a remote profile first to test safety.");
            }}
            style={{
              padding: "6px 16px",
              background: "#1f6feb",
              color: "#fff",
              border: "none",
              borderRadius: "6px",
              cursor: "pointer",
              fontWeight: 600,
              fontSize: "0.85rem",
            }}
          >
            Test Interlock
          </button>
        </div>

        {safetyAssessment && (
          <div
            style={{
              padding: "10px 14px",
              borderRadius: "6px",
              background: safetyAssessment.allowed ? "#122619" : "#3d1314",
              border: `1px solid ${safetyAssessment.allowed ? "#238636" : "#da3633"}`,
              fontSize: "0.85rem",
              color: safetyAssessment.allowed ? "#7ee787" : "#ffa198",
            }}
          >
            <strong>{safetyAssessment.allowed ? "✅ PASS: Command Allowed" : "⛔ INTERCEPTED: Command Blocked"}</strong>
            <div style={{ marginTop: "4px" }}>{safetyAssessment.reason}</div>
            {safetyAssessment.requiresToken && (
              <div style={{ marginTop: "6px", fontSize: "0.75rem", color: "#e3b341" }}>
                🔑 Required Unlock Token: <code>{safetyAssessment.expectedToken}</code>
              </div>
            )}
          </div>
        )}
      </div>

      {/* Edit / Create Modal */}
      {editingProfile && (
        <div
          style={{
            position: "fixed",
            inset: 0,
            background: "rgba(0,0,0,0.75)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            zIndex: 1000,
          }}
        >
          <div
            style={{
              background: "#161b22",
              border: "1px solid #30363d",
              borderRadius: "10px",
              width: "500px",
              padding: "1.5rem",
              boxShadow: "0 16px 32px rgba(0,0,0,0.7)",
            }}
          >
            <h3 style={{ margin: "0 0 1rem 0", color: "#e6edf3" }}>
              {editingProfile.id ? "Edit Remote Profile" : "New Remote Profile"}
            </h3>

            <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
              <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                Profile Name:
                <input
                  type="text"
                  value={formData.name || ""}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  placeholder="Production Web Server"
                  style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                />
              </label>

              <div style={{ display: "grid", gridTemplateColumns: "2fr 1fr", gap: "10px" }}>
                <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                  Host / IP:
                  <input
                    type="text"
                    value={formData.host || ""}
                    onChange={(e) => setFormData({ ...formData, host: e.target.value })}
                    placeholder="192.168.1.10 or app.com"
                    style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                  />
                </label>
                <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                  SSH Port:
                  <input
                    type="number"
                    value={formData.port || 22}
                    onChange={(e) => setFormData({ ...formData, port: parseInt(e.target.value) || 22 })}
                    style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                  />
                </label>
              </div>

              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "10px" }}>
                <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                  SSH User:
                  <input
                    type="text"
                    value={formData.user || "forge"}
                    onChange={(e) => setFormData({ ...formData, user: e.target.value })}
                    style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                  />
                </label>
                <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                  Environment:
                  <select
                    value={formData.environment || "production"}
                    onChange={(e) => setFormData({ ...formData, environment: e.target.value })}
                    style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                  >
                    <option value="production">Production</option>
                    <option value="staging">Staging</option>
                    <option value="development">Development</option>
                  </select>
                </label>
              </div>

              <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                Remote Project Root Path:
                <input
                  type="text"
                  value={formData.remotePath || ""}
                  onChange={(e) => setFormData({ ...formData, remotePath: e.target.value })}
                  placeholder="/var/www/my-laravel-app"
                  style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                />
              </label>

              <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                SSH Key Path (optional if using ssh-agent):
                <input
                  type="text"
                  value={formData.keyPath || ""}
                  onChange={(e) => setFormData({ ...formData, keyPath: e.target.value })}
                  placeholder="~/.ssh/id_rsa"
                  style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                />
              </label>

              <label style={{ display: "flex", alignItems: "center", gap: "8px", marginTop: "8px", cursor: "pointer" }}>
                <input
                  type="checkbox"
                  checked={formData.readOnly ?? true}
                  onChange={(e) => setFormData({ ...formData, readOnly: e.target.checked })}
                />
                <span style={{ fontSize: "0.85rem", color: "#e6edf3", fontWeight: 600 }}>
                  Enforce Read-Only Mode (Blocks all destructive mutations)
                </span>
              </label>
            </div>

            <div style={{ display: "flex", justifyContent: "flex-end", gap: "8px", marginTop: "1.5rem" }}>
              <button
                onClick={() => setEditingProfile(null)}
                style={{ padding: "6px 14px", background: "#21262d", color: "#c9d1d9", border: "1px solid #30363d", borderRadius: "6px", cursor: "pointer" }}
              >
                Cancel
              </button>
              <button
                onClick={handleSave}
                style={{ padding: "6px 16px", background: "#238636", color: "#fff", border: "none", borderRadius: "6px", cursor: "pointer", fontWeight: 600 }}
              >
                Save Profile
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
