import React, { useState } from "react";
import { telemetry } from "../wailsjs/go/models";

interface WaterfallTimelineProps {
  flamegraph: telemetry.WaterfallFlamegraph | null;
  loading?: boolean;
}

const CATEGORY_COLORS: Record<string, { bg: string; border: string; text: string }> = {
  boot: { bg: "#8957e5", border: "#a371f7", text: "#fff" },
  middleware: { bg: "#1f6feb", border: "#388bfd", text: "#fff" },
  controller: { bg: "#238636", border: "#2ea043", text: "#fff" },
  view: { bg: "#3fb950", border: "#56d364", text: "#0d1117" },
  database: { bg: "#d29922", border: "#e3b341", text: "#0d1117" },
  cache: { bg: "#db61a2", border: "#f778ba", text: "#fff" },
  event: { bg: "#a371f7", border: "#bc8cff", text: "#fff" },
  other: { bg: "#484f58", border: "#6e7681", text: "#fff" },
};

export const WaterfallTimeline: React.FC<WaterfallTimelineProps> = ({
  flamegraph,
  loading = false,
}) => {
  const [selectedSegment, setSelectedSegment] = useState<telemetry.WaterfallSegment | null>(null);
  const [activeCategory, setActiveCategory] = useState<string | null>(null);

  if (loading) {
    return (
      <div style={{ padding: "1.5rem", textAlign: "center", color: "#8b949e", fontSize: "0.85rem" }}>
        Generating waterfall flamegraph...
      </div>
    );
  }

  if (!flamegraph || !flamegraph.segments || flamegraph.segments.length === 0) {
    return (
      <div style={{ padding: "1.5rem", textAlign: "center", color: "#8b949e", fontSize: "0.85rem" }}>
        Select a telemetry capture request to inspect its execution timeline.
      </div>
    );
  }

  const filteredSegments = activeCategory
    ? flamegraph.segments.filter((s) => s.category === activeCategory)
    : flamegraph.segments;

  return (
    <div
      style={{
        background: "#161b22",
        border: "1px solid #30363d",
        borderRadius: "8px",
        overflow: "hidden",
      }}
    >
      {/* Header with Breakdown */}
      <div
        style={{
          padding: "12px 16px",
          background: "#0d1117",
          borderBottom: "1px solid #30363d",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          flexWrap: "wrap",
          gap: "10px",
        }}
      >
        <div>
          <span style={{ fontWeight: 700, fontSize: "0.9rem", color: "#e6edf3" }}>
            ⏱️ Request Waterfall Breakdown
          </span>
          <span style={{ fontSize: "0.75rem", color: "#8b949e", marginLeft: "8px" }}>
            Total: <strong>{flamegraph.totalDuration}ms</strong>
          </span>
        </div>

        {/* Category Filter Pills */}
        <div style={{ display: "flex", alignItems: "center", gap: "6px", flexWrap: "wrap" }}>
          <button
            onClick={() => setActiveCategory(null)}
            style={{
              padding: "2px 8px",
              fontSize: "0.7rem",
              borderRadius: "10px",
              border: !activeCategory ? "1px solid #58a6ff" : "1px solid #30363d",
              background: !activeCategory ? "#1f6feb33" : "transparent",
              color: !activeCategory ? "#58a6ff" : "#8b949e",
              cursor: "pointer",
            }}
          >
            All
          </button>
          {Object.entries(flamegraph.breakdown || {}).map(([cat, dur]) => {
            const colors = CATEGORY_COLORS[cat] || CATEGORY_COLORS.other;
            const isSelected = activeCategory === cat;
            return (
              <button
                key={cat}
                onClick={() => setActiveCategory(isSelected ? null : cat)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "4px",
                  padding: "2px 8px",
                  fontSize: "0.7rem",
                  borderRadius: "10px",
                  border: isSelected ? `1px solid ${colors.border}` : "1px solid #30363d",
                  background: isSelected ? `${colors.bg}44` : "transparent",
                  color: isSelected ? colors.border : "#8b949e",
                  cursor: "pointer",
                }}
              >
                <span
                  style={{
                    width: "8px",
                    height: "8px",
                    borderRadius: "50%",
                    background: colors.bg,
                  }}
                />
                <span style={{ textTransform: "capitalize" }}>{cat}:</span>
                <strong>{dur}ms</strong>
              </button>
            );
          })}
        </div>
      </div>

      {/* Flamegraph Bar Chart Area */}
      <div style={{ padding: "14px 16px", maxHeight: "360px", overflowY: "auto" }}>
        <div style={{ display: "flex", flexDirection: "column", gap: "6px" }}>
          {filteredSegments.map((seg) => {
            const colors = CATEGORY_COLORS[seg.category] || CATEGORY_COLORS.other;
            const barWidth = Math.max(2, Math.min(100, seg.percentage));
            const leftOffset = flamegraph.totalDuration > 0
              ? Math.max(0, Math.min(95, (seg.startMs / flamegraph.totalDuration) * 100))
              : 0;

            const isSelected = selectedSegment?.id === seg.id;

            return (
              <div
                key={seg.id}
                onClick={() => setSelectedSegment(isSelected ? null : seg)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "10px",
                  padding: "4px 8px",
                  background: isSelected ? "#21262d" : "transparent",
                  borderRadius: "4px",
                  cursor: "pointer",
                  fontSize: "0.75rem",
                }}
              >
                {/* Category label */}
                <span
                  style={{
                    width: "70px",
                    textTransform: "uppercase",
                    fontSize: "0.65rem",
                    fontWeight: 700,
                    color: colors.border,
                    flexShrink: 0,
                  }}
                >
                  {seg.category}
                </span>

                {/* Timeline Bar Track */}
                <div
                  style={{
                    flex: 1,
                    height: "22px",
                    background: "#0d1117",
                    borderRadius: "4px",
                    position: "relative",
                    overflow: "hidden",
                    border: "1px solid #21262d",
                  }}
                >
                  <div
                    style={{
                      position: "absolute",
                      left: `${leftOffset}%`,
                      width: `${barWidth}%`,
                      height: "100%",
                      background: colors.bg,
                      borderRadius: "3px",
                      opacity: 0.85,
                      transition: "width 0.2s, left 0.2s",
                    }}
                  />
                  <div
                    style={{
                      position: "absolute",
                      left: "8px",
                      top: "2px",
                      right: "8px",
                      whiteSpace: "nowrap",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      color: "#e6edf3",
                      fontWeight: 500,
                      pointerEvents: "none",
                    }}
                  >
                    {seg.label}
                  </div>
                </div>

                {/* Duration */}
                <span
                  style={{
                    width: "60px",
                    textAlign: "right",
                    color: seg.status === "slow" ? "#f85149" : seg.status === "warning" ? "#d29922" : "#8b949e",
                    fontWeight: seg.status === "slow" ? 700 : 500,
                    flexShrink: 0,
                  }}
                >
                  {seg.durationMs}ms
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Selected Segment Details Footer */}
      {selectedSegment && (
        <div
          style={{
            padding: "10px 16px",
            background: "#0d1117",
            borderTop: "1px solid #30363d",
            fontSize: "0.75rem",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
          }}
        >
          <div style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis" }}>
            <div style={{ fontWeight: 600, color: "#58a6ff", marginBottom: "2px" }}>
              {selectedSegment.category.toUpperCase()} • {selectedSegment.durationMs}ms ({selectedSegment.percentage}%)
            </div>
            <code style={{ color: "#c9d1d9", wordBreak: "break-all" }}>
              {selectedSegment.details || selectedSegment.label}
            </code>
          </div>
          <button
            onClick={() => setSelectedSegment(null)}
            style={{
              background: "none",
              border: "none",
              color: "#8b949e",
              cursor: "pointer",
              marginLeft: "12px",
            }}
          >
            ✕
          </button>
        </div>
      )}
    </div>
  );
};
