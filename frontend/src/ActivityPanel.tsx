import React, { useState } from 'react';

export interface TraceNode {
  id: string;
  type: string;
  label: string;
  detail?: string;
  startMs: number;
  durationMs: number;
  isFramework?: boolean;
  children?: TraceNode[];
}

export interface ActivityTrace {
  id: string;
  projectId: string;
  type: string;
  summary: string;
  status: number;
  totalTimeMs: number;
  memoryMb: number;
  timestamp: string;
  rootNode: TraceNode;
}

interface ActivityPanelProps {
  trace: ActivityTrace | null;
  onExportHTML?: () => void;
  onExportJSON?: () => void;
  isLoading?: boolean;
}

export const ActivityPanel: React.FC<ActivityPanelProps> = ({
  trace,
  onExportHTML,
  onExportJSON,
  isLoading,
}) => {
  const [selectedNode, setSelectedNode] = useState<TraceNode | null>(null);
  const [hideFramework, setHideFramework] = useState(false);

  if (isLoading) {
    return (
      <div className="py-20 text-center text-slate-500">
        <div className="animate-spin inline-block w-8 h-8 border-4 border-indigo-500 border-t-transparent rounded-full mb-3"></div>
        <p className="text-sm font-medium">Profiling Activity Execution Tree...</p>
      </div>
    );
  }

  if (!trace) {
    return (
      <div className="text-center py-20 bg-slate-900/40 rounded-xl border border-slate-800">
        <h4 className="text-base font-semibold text-slate-300">No Activity Trace Selected</h4>
        <p className="text-xs text-slate-500 max-w-md mx-auto mt-1">
          Select an incoming HTTP request, queued job, or artisan command to inspect its microsecond hierarchical flamegraph.
        </p>
      </div>
    );
  }

  const getNodeColor = (type: string, isFw?: boolean) => {
    if (isFw) {
      return 'bg-slate-800/40 text-slate-400 border-slate-700/40 opacity-70';
    }
    switch (type) {
      case 'request':
        return 'bg-blue-600/30 text-blue-300 border-blue-500/50';
      case 'middleware':
        return 'bg-purple-600/30 text-purple-300 border-purple-500/50';
      case 'controller':
        return 'bg-emerald-600/30 text-emerald-300 border-emerald-500/50';
      case 'database':
        return 'bg-amber-600/30 text-amber-300 border-amber-500/50';
      case 'event':
        return 'bg-cyan-600/30 text-cyan-300 border-cyan-500/50';
      case 'listener':
        return 'bg-pink-600/30 text-pink-300 border-pink-500/50';
      case 'view':
        return 'bg-teal-600/30 text-teal-300 border-teal-500/50';
      default:
        return 'bg-slate-700/30 text-slate-300 border-slate-600/50';
    }
  };

  const renderFlameNode = (node: TraceNode, totalDuration: number, depth = 0): React.ReactNode => {
    if (hideFramework && node.isFramework && (!node.children || node.children.length === 0)) {
      return null;
    }

    const widthPct = Math.max((node.durationMs / totalDuration) * 100, 2);
    const leftPct = (node.startMs / totalDuration) * 100;

    return (
      <div key={node.id} className="space-y-1 my-1">
        <div className="relative h-7 bg-slate-950/60 rounded overflow-hidden">
          <div
            onClick={() => setSelectedNode(node)}
            style={{ left: `${leftPct}%`, width: `${widthPct}%` }}
            className={`absolute top-0 bottom-0 cursor-pointer rounded px-2 flex items-center justify-between text-[11px] font-mono border transition-all hover:brightness-125 ${getNodeColor(
              node.type,
              node.isFramework
            )}`}
          >
            <div className="flex items-center gap-1.5 truncate mr-1 font-semibold">
              {node.isFramework && (
                <span className="px-1 py-0.2 rounded bg-slate-800 text-[9px] text-slate-400">vendor</span>
              )}
              <span className="truncate">{node.label}</span>
            </div>
            <span className="shrink-0 opacity-80">{node.durationMs.toFixed(1)}ms</span>
          </div>
        </div>
        {node.children && node.children.length > 0 && (
          <div className="pl-3 border-l border-slate-800">
            {node.children.map((child) => renderFlameNode(child, totalDuration, depth + 1))}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="space-y-4">
      {/* Trace Header */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-900/60 p-4 rounded-xl border border-slate-800">
        <div>
          <div className="flex items-center gap-2">
            <span
              className={`px-2 py-0.5 text-xs font-bold rounded ${
                trace.status >= 400 ? 'bg-rose-500/20 text-rose-300' : 'bg-emerald-500/20 text-emerald-300'
              }`}
            >
              HTTP {trace.status}
            </span>
            <h3 className="text-sm font-semibold text-slate-200">{trace.summary}</h3>
          </div>
          <div className="flex items-center gap-4 text-xs text-slate-400 mt-1">
            <span>Duration: <strong className="text-indigo-400">{trace.totalTimeMs.toFixed(1)} ms</strong></span>
            <span>Memory: <strong className="text-purple-400">{trace.memoryMb.toFixed(1)} MB</strong></span>
            <span>Project: <strong>{trace.projectId}</strong></span>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => setHideFramework(!hideFramework)}
            className={`px-3 py-1.5 text-xs font-medium rounded-lg border transition-all ${
              hideFramework
                ? 'bg-indigo-600 text-white border-indigo-500'
                : 'bg-slate-800 hover:bg-slate-700 text-slate-300 border-slate-700'
            }`}
          >
            {hideFramework ? 'Showing App Only' : 'Filter Framework Spans'}
          </button>
          {onExportHTML && (
            <button
              onClick={onExportHTML}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
            >
              Export HTML Report
            </button>
          )}
          {onExportJSON && (
            <button
              onClick={onExportJSON}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
            >
              Export JSON
            </button>
          )}
        </div>
      </div>

      {/* Flamegraph Execution Tree */}
      <div className="bg-slate-900/40 p-4 rounded-xl border border-slate-800 space-y-2">
        <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider mb-2">
          Execution Flamegraph (Microsecond Chronology)
        </h4>
        <div className="overflow-x-auto">
          {renderFlameNode(trace.rootNode, Math.max(trace.totalTimeMs, 1))}
        </div>
      </div>

      {/* Selected Node Inspector */}
      {selectedNode && (
        <div className="bg-slate-900/70 p-4 rounded-xl border border-indigo-500/30 space-y-2">
          <div className="flex justify-between items-center">
            <h4 className="text-xs font-bold text-indigo-400 uppercase">Node Detail</h4>
            <button
              onClick={() => setSelectedNode(null)}
              className="text-xs text-slate-500 hover:text-slate-300"
            >
              Close ✕
            </button>
          </div>
          <div className="text-xs grid grid-cols-2 md:grid-cols-4 gap-2 text-slate-300">
            <div><span className="text-slate-500">Type:</span> <strong>{selectedNode.type}</strong></div>
            <div><span className="text-slate-500">Label:</span> <strong>{selectedNode.label}</strong></div>
            <div><span className="text-slate-500">Start:</span> <strong>{selectedNode.startMs.toFixed(2)} ms</strong></div>
            <div><span className="text-slate-500">Duration:</span> <strong>{selectedNode.durationMs.toFixed(2)} ms</strong></div>
          </div>
          {selectedNode.detail && (
            <pre className="text-xs font-mono text-slate-200 bg-slate-950 p-2.5 rounded-lg overflow-x-auto">
              {selectedNode.detail}
            </pre>
          )}
        </div>
      )}
    </div>
  );
};
