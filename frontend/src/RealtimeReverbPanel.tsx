import React, { useState, useEffect } from "react";
import {
  GetReverbConfig,
  GetWebSocketFrames,
  GetWebSocketChannels,
  SimulateBroadcastEvent,
  ClearWebSocketFrames,
} from "../wailsjs/go/main/App";
import { realtime } from "../wailsjs/go/models";

interface RealtimeReverbPanelProps {
  projectId: string;
}

export const RealtimeReverbPanel: React.FC<RealtimeReverbPanelProps> = ({ projectId }) => {
  const [config, setConfig] = useState<realtime.ReverbConfig | null>(null);
  const [frames, setFrames] = useState<realtime.WebSocketFrame[]>([]);
  const [channels, setChannels] = useState<realtime.ChannelInfo[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [search, setSearch] = useState<string>("");

  // Event simulator form
  const [simChannel, setSimChannel] = useState<string>("orders");
  const [simEvent, setSimEvent] = useState<string>("OrderPlaced");
  const [simPayload, setSimPayload] = useState<string>('{\n  "order_id": 1042,\n  "amount": 89.50,\n  "status": "paid"\n}');
  const [simResult, setSimResult] = useState<string | null>(null);

  useEffect(() => {
    loadData();
    const timer = setInterval(pollFrames, 2000);
    return () => clearInterval(timer);
  }, [projectId]);

  const loadData = async () => {
    setLoading(true);
    try {
      if (projectId) {
        const cfg = await GetReverbConfig(projectId);
        setConfig(cfg);
      }
      await pollFrames();
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const pollFrames = async () => {
    try {
      const [f, ch] = await Promise.all([GetWebSocketFrames(), GetWebSocketChannels()]);
      setFrames(f || []);
      setChannels(ch || []);
    } catch (e) {
      // ignore poll errors
    }
  };

  const handleSimulate = async () => {
    try {
      const res = await SimulateBroadcastEvent({
        channel: simChannel,
        event: simEvent,
        payload: simPayload,
      });
      setSimResult(res.message);
      pollFrames();
      setTimeout(() => setSimResult(null), 3000);
    } catch (err: any) {
      alert("Simulation failed: " + (err?.message || String(err)));
    }
  };

  const handleClear = async () => {
    await ClearWebSocketFrames();
    pollFrames();
  };

  const filteredFrames = frames.filter(
    (f) =>
      f.event?.toLowerCase().includes(search.toLowerCase()) ||
      f.channel?.toLowerCase().includes(search.toLowerCase()) ||
      f.data?.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100%",
        background: "#0d1117",
        color: "#c9d1d9",
        overflow: "hidden",
      }}
    >
      {/* Header Bar */}
      <div
        style={{
          padding: "1rem 1.5rem",
          background: "#161b22",
          borderBottom: "1px solid #30363d",
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          flexWrap: "wrap",
          gap: "10px",
        }}
      >
        <div>
          <div style={{ display: "flex", alignItems: "center", gap: "10px" }}>
            <h2 style={{ margin: 0, fontSize: "1.2rem", color: "#e6edf3" }}>
              📡 WebSocket &amp; Laravel Reverb Inspector
            </h2>
            <span
              style={{
                fontSize: "0.75rem",
                padding: "2px 8px",
                borderRadius: "10px",
                fontWeight: 600,
                background: config?.isRunning ? "#23863622" : "#8b949e22",
                color: config?.isRunning ? "#3fb950" : "#8b949e",
                border: `1px solid ${config?.isRunning ? "#238636" : "#30363d"}`,
              }}
            >
              {config?.isRunning ? "● REVERB LISTENING" : "○ REVERB OFFLINE"}
            </span>
          </div>
          <div style={{ fontSize: "0.75rem", color: "#8b949e", marginTop: "4px" }}>
            Driver: <strong style={{ color: "#79c0ff" }}>{config?.driver || "reverb"}</strong> • App Key: <code>{config?.appKey || "None"}</code> • Host: <code>{config?.host}:{config?.port}</code>
          </div>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
          <input
            type="text"
            placeholder="Search frames..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{
              padding: "4px 10px",
              background: "#0d1117",
              border: "1px solid #30363d",
              borderRadius: "6px",
              color: "#c9d1d9",
              fontSize: "0.8rem",
              width: "180px",
            }}
          />
          <button
            onClick={handleClear}
            style={{
              padding: "4px 10px",
              background: "#21262d",
              color: "#f85149",
              border: "1px solid #30363d",
              borderRadius: "6px",
              cursor: "pointer",
              fontSize: "0.8rem",
            }}
          >
            Clear Frames
          </button>
        </div>
      </div>

      {/* Main Content: Split View */}
      <div style={{ flex: 1, display: "flex", minHeight: 0 }}>
        {/* Left Pane: Live WebSocket Frames Stream */}
        <div
          style={{
            flex: 2,
            borderRight: "1px solid #30363d",
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          <div
            style={{
              padding: "8px 14px",
              background: "#161b22",
              borderBottom: "1px solid #30363d",
              fontSize: "0.75rem",
              fontWeight: 600,
              color: "#8b949e",
              display: "flex",
              justifyContent: "space-between",
            }}
          >
            <span>PACKET STREAM ({filteredFrames.length})</span>
            <span>AUTO-POLLING EVERY 2s</span>
          </div>

          <div style={{ flex: 1, overflowY: "auto", padding: "10px" }}>
            {filteredFrames.length === 0 ? (
              <div style={{ padding: "3rem", textAlign: "center", color: "#8b949e", fontSize: "0.85rem" }}>
                <div>📡 Waiting for WebSocket activity...</div>
                <div style={{ fontSize: "0.75rem", marginTop: "4px" }}>
                  Connect your frontend with Laravel Echo or use the Broadcast Simulator on the right.
                </div>
              </div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                {filteredFrames.slice().reverse().map((f) => (
                  <div
                    key={f.id}
                    style={{
                      background: "#161b22",
                      border: "1px solid #30363d",
                      borderRadius: "6px",
                      padding: "10px 12px",
                      fontSize: "0.8rem",
                    }}
                  >
                    <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: "6px" }}>
                      <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                        <span
                          style={{
                            fontSize: "0.65rem",
                            fontWeight: 700,
                            padding: "1px 6px",
                            borderRadius: "4px",
                            background: f.direction === "incoming" ? "#23863633" : "#1f6feb33",
                            color: f.direction === "incoming" ? "#3fb950" : "#58a6ff",
                          }}
                        >
                          {f.direction.toUpperCase()}
                        </span>
                        <strong style={{ color: "#e6edf3" }}>{f.event}</strong>
                        {f.channel && (
                          <span style={{ fontSize: "0.75rem", color: "#e3b341", background: "#d2992222", padding: "1px 6px", borderRadius: "4px" }}>
                            #{f.channel}
                          </span>
                        )}
                      </div>
                      <span style={{ fontSize: "0.7rem", color: "#8b949e" }}>
                        {f.timestamp ? new Date(f.timestamp).toLocaleTimeString() : "Just now"}
                      </span>
                    </div>

                    <pre
                      style={{
                        margin: 0,
                        background: "#0d1117",
                        padding: "6px 8px",
                        borderRadius: "4px",
                        fontSize: "0.75rem",
                        color: "#7ee787",
                        overflowX: "auto",
                        fontFamily: "monospace",
                      }}
                    >
                      {f.data}
                    </pre>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right Pane: Channel Monitor & Event Broadcast Simulator */}
        <div style={{ flex: 1, display: "flex", flexDirection: "column", overflowY: "auto", background: "#161b22" }}>
          {/* Active Channels Box */}
          <div style={{ padding: "14px", borderBottom: "1px solid #30363d" }}>
            <h4 style={{ margin: "0 0 10px 0", fontSize: "0.85rem", color: "#e6edf3" }}>
              Active Channels ({channels.length})
            </h4>
            {channels.length === 0 ? (
              <p style={{ margin: 0, fontSize: "0.75rem", color: "#8b949e" }}>
                No active channels observed yet.
              </p>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "6px" }}>
                {channels.map((ch) => (
                  <div
                    key={ch.name}
                    style={{
                      display: "flex",
                      justifyContent: "space-between",
                      alignItems: "center",
                      padding: "6px 10px",
                      background: "#0d1117",
                      borderRadius: "6px",
                      fontSize: "0.75rem",
                    }}
                  >
                    <div>
                      <span style={{ color: "#58a6ff", fontWeight: 600 }}>#{ch.name}</span>
                      <span style={{ fontSize: "0.65rem", color: "#8b949e", marginLeft: "6px" }}>
                        ({ch.type})
                      </span>
                    </div>
                    <span style={{ color: "#7ee787" }}>● {ch.lastSeen}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Broadcast Event Simulator */}
          <div style={{ padding: "14px" }}>
            <h4 style={{ margin: "0 0 6px 0", fontSize: "0.85rem", color: "#f0883e" }}>
              ⚡ Broadcast Event Simulator
            </h4>
            <p style={{ margin: "0 0 12px 0", fontSize: "0.75rem", color: "#8b949e" }}>
              Simulate an event dispatch to test frontend listeners without touching backend PHP code.
            </p>

            <div style={{ display: "flex", flexDirection: "column", gap: "10px" }}>
              <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                Channel:
                <input
                  type="text"
                  value={simChannel}
                  onChange={(e) => setSimChannel(e.target.value)}
                  style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                />
              </label>

              <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                Event Name:
                <input
                  type="text"
                  value={simEvent}
                  onChange={(e) => setSimEvent(e.target.value)}
                  style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#fff", marginTop: "4px" }}
                />
              </label>

              <label style={{ fontSize: "0.75rem", color: "#8b949e" }}>
                JSON Payload:
                <textarea
                  rows={6}
                  value={simPayload}
                  onChange={(e) => setSimPayload(e.target.value)}
                  style={{ width: "100%", padding: "6px", background: "#0d1117", border: "1px solid #30363d", borderRadius: "4px", color: "#7ee787", fontFamily: "monospace", fontSize: "0.75rem", marginTop: "4px" }}
                />
              </label>

              <button
                onClick={handleSimulate}
                style={{
                  padding: "8px",
                  background: "#238636",
                  color: "#fff",
                  border: "none",
                  borderRadius: "6px",
                  fontWeight: 600,
                  fontSize: "0.8rem",
                  cursor: "pointer",
                }}
              >
                🚀 Dispatch Simulated Event
              </button>

              {simResult && (
                <div style={{ padding: "6px 10px", background: "#1b4721", color: "#7ee787", borderRadius: "4px", fontSize: "0.75rem" }}>
                  {simResult}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
