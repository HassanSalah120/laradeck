package activity

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type TraceNodeType string

const (
	NodeRequest    TraceNodeType = "request"
	NodeMiddleware TraceNodeType = "middleware"
	NodeController TraceNodeType = "controller"
	NodeDatabase   TraceNodeType = "database"
	NodeEvent      TraceNodeType = "event"
	NodeListener   TraceNodeType = "listener"
	NodeJob        TraceNodeType = "job"
	NodeView       TraceNodeType = "view"
	NodeCache      TraceNodeType = "cache"
)

type TraceNode struct {
	ID          string        `json:"id"`
	Type        TraceNodeType `json:"type"`
	Label       string        `json:"label"`
	Detail      string        `json:"detail,omitempty"`
	StartMs     float64       `json:"startMs"`
	DurationMs  float64       `json:"durationMs"`
	IsFramework bool          `json:"isFramework"`
	Children    []TraceNode   `json:"children,omitempty"`
}

type ActivityTrace struct {
	ID          string      `json:"id"`
	ProjectID   string      `json:"projectId"`
	Type        string      `json:"type"` // "http", "command", "job"
	Summary     string      `json:"summary"`
	Status      int         `json:"status"`
	TotalTimeMs float64     `json:"totalTimeMs"`
	MemoryMB    float64     `json:"memoryMb"`
	Timestamp   string      `json:"timestamp"`
	RootNode    TraceNode   `json:"rootNode"`
}

type RawActivityInput struct {
	Method     string
	URI        string
	Status     int
	Duration   float64
	Memory     float64
	Controller string
	Middleware []string
	Queries    []RawQuery
	Events     []RawEvent
	Views      []RawView
}

type RawQuery struct {
	Query      string  `json:"query"`
	Duration   float64 `json:"duration"`
	StartMs    float64 `json:"startMs"`
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
}

type RawEvent struct {
	Name       string   `json:"name"`
	Duration   float64  `json:"duration"`
	StartMs    float64  `json:"startMs"`
	Listeners  []string `json:"listeners,omitempty"`
}

type RawView struct {
	Name       string  `json:"name"`
	Duration   float64 `json:"duration"`
	StartMs    float64 `json:"startMs"`
}

// BuildActivityTrace generates a flamegraph-ready hierarchical execution tree
func BuildActivityTrace(id, projectID string, input RawActivityInput) ActivityTrace {
	root := TraceNode{
		ID:         fmt.Sprintf("%s_root", id),
		Type:       NodeRequest,
		Label:      fmt.Sprintf("%s %s", input.Method, input.URI),
		Detail:     fmt.Sprintf("HTTP %d (%.1f ms)", input.Status, input.Duration),
		StartMs:    0,
		DurationMs: input.Duration,
		Children:   make([]TraceNode, 0),
	}

	// 1. Add Middleware nodes
	if len(input.Middleware) > 0 {
		midDuration := 0.5
		if input.Duration > 5.0 {
			midDuration = input.Duration * 0.05
		}
		mwChildren := make([]TraceNode, 0, len(input.Middleware))
		for i, mw := range input.Middleware {
			mwChildren = append(mwChildren, TraceNode{
				ID:         fmt.Sprintf("%s_mw_%d", id, i),
				Type:       NodeMiddleware,
				Label:      mw,
				StartMs:    float64(i) * (midDuration / float64(len(input.Middleware))),
				DurationMs: midDuration / float64(len(input.Middleware)),
			})
		}
		root.Children = append(root.Children, TraceNode{
			ID:         fmt.Sprintf("%s_middleware_group", id),
			Type:       NodeMiddleware,
			Label:      "Middleware Pipeline",
			StartMs:    0,
			DurationMs: midDuration,
			Children:   mwChildren,
		})
	}

	// 2. Add Controller node
	ctrlLabel := input.Controller
	if ctrlLabel == "" {
		ctrlLabel = "Closure Route"
	}
	ctrlNode := TraceNode{
		ID:         fmt.Sprintf("%s_ctrl", id),
		Type:       NodeController,
		Label:      ctrlLabel,
		StartMs:    0.5,
		DurationMs: input.Duration - 0.5,
		Children:   make([]TraceNode, 0),
	}

	// Add Queries inside Controller
	for i, q := range input.Queries {
		qNode := TraceNode{
			ID:         fmt.Sprintf("%s_q_%d", id, i),
			Type:       NodeDatabase,
			Label:      summarizeSQL(q.Query),
			Detail:     q.Query,
			StartMs:    q.StartMs,
			DurationMs: q.Duration,
		}
		ctrlNode.Children = append(ctrlNode.Children, qNode)
	}

	// Add Events inside Controller
	for i, ev := range input.Events {
		isEvFramework := strings.HasPrefix(ev.Name, "Illuminate\\") || strings.HasPrefix(ev.Name, "Laravel\\") || strings.HasPrefix(ev.Name, "Symfony\\")
		evChildren := make([]TraceNode, 0, len(ev.Listeners))
		for li, l := range ev.Listeners {
			isLisFramework := strings.HasPrefix(l, "Illuminate\\") || strings.HasPrefix(l, "Laravel\\") || strings.HasPrefix(l, "Symfony\\")
			evChildren = append(evChildren, TraceNode{
				ID:          fmt.Sprintf("%s_ev_%d_lis_%d", id, i, li),
				Type:        NodeListener,
				Label:       l,
				StartMs:     ev.StartMs,
				DurationMs:  ev.Duration / float64(len(ev.Listeners)),
				IsFramework: isLisFramework,
			})
		}
		evNode := TraceNode{
			ID:          fmt.Sprintf("%s_ev_%d", id, i),
			Type:        NodeEvent,
			Label:       ev.Name,
			StartMs:     ev.StartMs,
			DurationMs:  ev.Duration,
			IsFramework: isEvFramework,
			Children:    evChildren,
		}
		ctrlNode.Children = append(ctrlNode.Children, evNode)
	}

	// Add Views
	for i, v := range input.Views {
		ctrlNode.Children = append(ctrlNode.Children, TraceNode{
			ID:         fmt.Sprintf("%s_v_%d", id, i),
			Type:       NodeView,
			Label:      v.Name,
			StartMs:    v.StartMs,
			DurationMs: v.Duration,
		})
	}

	// Sort children by StartMs
	sort.Slice(ctrlNode.Children, func(i, j int) bool {
		return ctrlNode.Children[i].StartMs < ctrlNode.Children[j].StartMs
	})

	root.Children = append(root.Children, ctrlNode)

	return ActivityTrace{
		ID:          id,
		ProjectID:   projectID,
		Type:        "http",
		Summary:     fmt.Sprintf("%s %s", input.Method, input.URI),
		Status:      input.Status,
		TotalTimeMs: input.Duration,
		MemoryMB:    input.Memory,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		RootNode:    root,
	}
}

func summarizeSQL(sql string) string {
	sql = strings.TrimSpace(sql)
	if len(sql) > 60 {
		return sql[:57] + "..."
	}
	return sql
}

// ExportTraceJSON exports trace as formatted JSON
func ExportTraceJSON(t ActivityTrace) ([]byte, error) {
	return json.MarshalIndent(t, "", "  ")
}

// ExportTraceHTML creates an interactive standalone HTML trace report with visual timeline bars
func ExportTraceHTML(t ActivityTrace) string {
	jsonData, _ := json.Marshal(t)
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>LaraDeck Activity Trace: %s</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #0b132b; color: #e0e1dd; padding: 24px; margin: 0; }
  .header { background: #1c2541; border-radius: 10px; padding: 24px; margin-bottom: 24px; border: 1px solid #3a506b; }
  h1 { margin: 0 0 12px 0; font-size: 22px; color: #ffffff; }
  .badges { display: flex; gap: 8px; margin-bottom: 12px; }
  .badge { padding: 4px 10px; border-radius: 6px; font-size: 13px; font-weight: bold; background: #3a506b; color: #fff; }
  .badge-success { background: #10b981; }
  .badge-purple { background: #8b5cf6; }
  .badge-cyan { background: #06b6d4; }
  .timeline-card { background: #1c2541; border-radius: 10px; padding: 20px; border: 1px solid #3a506b; }
  .node-row { display: flex; align-items: center; padding: 8px 12px; border-radius: 6px; margin: 4px 0; background: #0f172a; border-left: 4px solid #3b82f6; }
  .node-row.framework { opacity: 0.65; border-left-color: #64748b; }
  .node-row.database { border-left-color: #f59e0b; }
  .node-row.event { border-left-color: #ec4899; }
  .node-row.listener { border-left-color: #a855f7; }
  .node-type { font-size: 11px; text-transform: uppercase; font-weight: bold; width: 90px; color: #94a3b8; }
  .node-label { flex: 1; font-family: monospace; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .node-time { font-size: 12px; color: #38bdf8; font-weight: bold; margin-left: 12px; width: 80px; text-align: right; }
  .bar-container { width: 140px; background: #334155; height: 6px; border-radius: 3px; margin-left: 12px; overflow: hidden; }
  .bar-fill { height: 100%%; background: #10b981; border-radius: 3px; }
  pre { background: #090d16; padding: 14px; border-radius: 8px; overflow-x: auto; font-size: 12px; border: 1px solid #1e293b; color: #94a3b8; }
</style>
</head>
<body>
<div class="header">
  <h1>LaraDeck Activity Timeline: %s</h1>
  <div class="badges">
    <span class="badge">HTTP %d</span>
    <span class="badge badge-success">%.2f ms</span>
    <span class="badge badge-purple">%.2f MB</span>
    <span class="badge badge-cyan">%s</span>
  </div>
  <small style="color:#94a3b8;">Captured at %s</small>
</div>

<div class="timeline-card">
  <h3>Hierarchical Execution Flow</h3>
  <div id="tree"></div>
</div>

<div class="timeline-card" style="margin-top:24px;">
  <h3>Raw Execution Payload</h3>
  <pre id="json"></pre>
</div>

<script>
const trace = %s;
document.getElementById('json').textContent = JSON.stringify(trace, null, 2);

function renderNodes(nodes, container, depth, totalMs) {
  for (const n of nodes) {
    const row = document.createElement('div');
    row.className = 'node-row ' + n.type + (n.isFramework ? ' framework' : '');
    row.style.marginLeft = (depth * 20) + 'px';

    const typeDiv = document.createElement('div');
    typeDiv.className = 'node-type';
    typeDiv.textContent = n.type;

    const labelDiv = document.createElement('div');
    labelDiv.className = 'node-label';
    labelDiv.textContent = n.label + (n.isFramework ? ' (Framework)' : '');
    labelDiv.title = n.detail || n.label;

    const timeDiv = document.createElement('div');
    timeDiv.className = 'node-time';
    timeDiv.textContent = (n.durationMs ? n.durationMs.toFixed(1) : '0.0') + ' ms';

    const barContainer = document.createElement('div');
    barContainer.className = 'bar-container';
    const bar = document.createElement('div');
    bar.className = 'bar-fill';
    const pct = totalMs > 0 ? Math.min(100, Math.max(2, (n.durationMs / totalMs) * 100)) : 100;
    bar.style.width = pct + '%%';
    barContainer.appendChild(bar);

    row.appendChild(typeDiv);
    row.appendChild(labelDiv);
    row.appendChild(timeDiv);
    row.appendChild(barContainer);
    container.appendChild(row);

    if (n.children && n.children.length > 0) {
      renderNodes(n.children, container, depth + 1, totalMs);
    }
  }
}

const treeContainer = document.getElementById('tree');
if (trace.rootNode) {
  renderNodes([trace.rootNode], treeContainer, 0, trace.totalTimeMs || 1);
}
</script>
</body>
</html>`, t.Summary, t.Summary, t.Status, t.TotalTimeMs, t.MemoryMB, t.Type, t.Timestamp, string(jsonData))
}
