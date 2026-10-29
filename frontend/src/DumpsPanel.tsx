import React, { useState, useEffect } from 'react';
import {
  IsDebuggerAgentActive,
  ToggleDebuggerAgent,
  GetDumpsInHtml,
  SetDumpsInHtml,
  GetTelemetryFrames,
  ClearTelemetryFrames,
} from '../wailsjs/go/main/App';
import { telemetry } from '../wailsjs/go/models';

export type DumpPayload = telemetry.DumpPayload;
export type RequestTelemetryFrame = telemetry.RequestTelemetryFrame;

interface DumpsPanelProps {
  projectId?: string;
  dumps: DumpPayload[];
  onClear: () => void;
  onRefresh?: () => void;
  helperSnippet?: string;
}

export const DumpsPanel: React.FC<DumpsPanelProps> = ({
  projectId,
  dumps,
  onClear,
  onRefresh,
  helperSnippet,
}) => {
  const [activeSubTab, setActiveSubTab] = useState<'dumps' | 'telemetry'>('dumps');
  const [filter, setFilter] = useState('');
  const [showHelper, setShowHelper] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // Agent & HTML Dumps state
  const [isAgentActive, setIsAgentActive] = useState<boolean>(false);
  const [dumpsInHtml, setDumpsInHtmlState] = useState<boolean>(false);
  const [agentLoading, setAgentLoading] = useState<boolean>(false);
  const [frames, setFrames] = useState<RequestTelemetryFrame[]>([]);
  const [expandedFrameId, setExpandedFrameId] = useState<string | null>(null);

  useEffect(() => {
    if (projectId) {
      loadAgentState();
    }
  }, [projectId]);

  const loadAgentState = async () => {
    if (!projectId) return;
    try {
      const [agentActive, htmlMode, capturedFrames] = await Promise.all([
        IsDebuggerAgentActive(projectId),
        GetDumpsInHtml(),
        GetTelemetryFrames(projectId),
      ]);
      setIsAgentActive(Boolean(agentActive));
      setDumpsInHtmlState(Boolean(htmlMode));
      setFrames(capturedFrames || []);
    } catch (err) {
      console.error('Failed to load debugger agent state', err);
    }
  };

  const handleToggleAgent = async () => {
    if (!projectId) return;
    setAgentLoading(true);
    try {
      await ToggleDebuggerAgent(projectId, !isAgentActive);
      setIsAgentActive(!isAgentActive);
      await loadAgentState();
    } catch (err: any) {
      alert('Failed to toggle debugger agent: ' + (err.message || String(err)));
    } finally {
      setAgentLoading(false);
    }
  };

  const handleToggleDumpsInHtml = async () => {
    const next = !dumpsInHtml;
    try {
      await SetDumpsInHtml(next);
      setDumpsInHtmlState(next);
    } catch (err: any) {
      alert('Failed to set dumps in HTML: ' + (err.message || String(err)));
    }
  };

  const handleClearFrames = async () => {
    if (!projectId) return;
    try {
      await ClearTelemetryFrames(projectId);
      setFrames([]);
    } catch (err: any) {
      console.error('Failed to clear telemetry frames', err);
    }
  };

  const filteredDumps = dumps.filter(
    (d) =>
      d.file?.toLowerCase().includes(filter.toLowerCase()) ||
      d.payload?.toLowerCase().includes(filter.toLowerCase())
  );

  const filteredFrames = frames.filter(
    (f) =>
      f.url?.toLowerCase().includes(filter.toLowerCase()) ||
      f.method?.toLowerCase().includes(filter.toLowerCase())
  );

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  return (
    <div className="space-y-4">
      {/* Zero-Package Agent & Settings Toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-900/80 p-4 rounded-xl border border-slate-800 shadow-sm">
        <div className="flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-2">
            <span className="text-sm font-bold text-slate-200">Zero-Package Debugger Agent:</span>
            <button
              onClick={handleToggleAgent}
              disabled={agentLoading || !projectId}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition-all shadow-sm ${
                isAgentActive
                  ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 hover:bg-emerald-500/30'
                  : 'bg-slate-800 text-slate-400 border border-slate-700 hover:bg-slate-700 hover:text-slate-200'
              } disabled:opacity-50`}
            >
              <span
                className={`w-2 h-2 rounded-full ${
                  isAgentActive ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'
                }`}
              />
              {agentLoading ? 'Updating .user.ini...' : isAgentActive ? 'Agent Attached' : 'Attach Agent'}
            </button>
            {isAgentActive && (
              <span className="text-[11px] font-mono text-emerald-400/90 hidden sm:inline">
                (auto_prepend_file in public/.user.ini)
              </span>
            )}
          </div>

          {/* Dumps in HTML toggle */}
          <div className="flex items-center gap-2 pl-3 border-l border-slate-800">
            <label className="text-xs text-slate-300 font-medium flex items-center gap-2 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={dumpsInHtml}
                onChange={handleToggleDumpsInHtml}
                className="rounded bg-slate-950 border-slate-700 text-indigo-600 focus:ring-0 cursor-pointer"
              />
              <span>Render Dumps in HTML Response</span>
            </label>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {helperSnippet && (
            <button
              onClick={() => setShowHelper(!showHelper)}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
            >
              {showHelper ? 'Hide Helper Code' : 'Dump Helper Snippet'}
            </button>
          )}
          {onRefresh && (
            <button
              onClick={() => {
                onRefresh();
                loadAgentState();
              }}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
            >
              Refresh
            </button>
          )}
        </div>
      </div>

      {/* Helper snippet modal / accordion */}
      {showHelper && helperSnippet && (
        <div className="p-4 bg-slate-950 border border-indigo-500/30 rounded-xl space-y-2">
          <div className="flex justify-between items-center text-xs text-indigo-400 font-semibold">
            <span>Add this helper to bootstrap/app.php or composer autoload if not using the zero-package agent</span>
            <button
              onClick={() => copyToClipboard(helperSnippet, 'helper')}
              className="px-2 py-1 bg-indigo-600/30 rounded text-indigo-200 hover:bg-indigo-600/50"
            >
              {copiedId === 'helper' ? 'Copied' : 'Copy PHP'}
            </button>
          </div>
          <pre className="text-xs font-mono text-emerald-400 p-3 bg-slate-900 rounded overflow-x-auto max-h-48">
            {helperSnippet}
          </pre>
        </div>
      )}

      {/* Sub tabs: Dumps vs Request Frames */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800">
        <div className="flex gap-1.5">
          <button
            onClick={() => setActiveSubTab('dumps')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeSubTab === 'dumps'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            In-App Dumps ({dumps.length})
          </button>
          <button
            onClick={() => setActiveSubTab('telemetry')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeSubTab === 'telemetry'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Request Telemetry Frames ({frames.length})
          </button>
        </div>

        <div className="flex items-center gap-2">
          <input
            type="text"
            placeholder={activeSubTab === 'dumps' ? 'Filter by file or variable...' : 'Filter by URI or method...'}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            className="px-3 py-1.5 text-xs bg-slate-950/80 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 focus:outline-none focus:border-indigo-500 w-56"
          />

          {activeSubTab === 'dumps' ? (
            <button
              onClick={onClear}
              disabled={dumps.length === 0}
              className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-rose-500/20 text-rose-300 hover:bg-rose-500/30 border border-rose-500/30 disabled:opacity-50"
            >
              Clear Dumps
            </button>
          ) : (
            <button
              onClick={handleClearFrames}
              disabled={frames.length === 0}
              className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-rose-500/20 text-rose-300 hover:bg-rose-500/30 border border-rose-500/30 disabled:opacity-50"
            >
              Clear Frames
            </button>
          )}
        </div>
      </div>

      {/* Dumps Stream */}
      {activeSubTab === 'dumps' && (
        <>
          {filteredDumps.length === 0 ? (
            <div className="text-center py-16 bg-slate-900/40 rounded-xl border border-slate-800/80">
              <h4 className="text-base font-semibold text-slate-300">No In-App Dumps Captured Yet</h4>
              <p className="text-xs text-slate-500 max-w-md mx-auto mt-1">
                Calls to <code className="text-indigo-400 font-mono">dump($var)</code> will appear here automatically when the zero-package agent is attached.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {filteredDumps.map((d) => (
                <div
                  key={d.id}
                  className="bg-slate-900/70 border border-slate-800 rounded-xl p-4 transition-all hover:border-slate-700"
                >
                  <div className="flex flex-wrap items-center justify-between text-xs text-slate-400 mb-2 border-b border-slate-800/80 pb-2">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-indigo-400 font-semibold">
                        {d.file || 'Unknown file'}
                        {d.line > 0 && `:${d.line}`}
                      </span>
                      {d.projectId && (
                        <span className="px-1.5 py-0.5 rounded bg-slate-800 text-[10px] text-slate-400">
                          {d.projectId}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="text-[11px] text-slate-500">
                        {d.createdAt ? new Date(d.createdAt).toLocaleTimeString() : ''}
                      </span>
                      <button
                        onClick={() => copyToClipboard(d.payload, d.id)}
                        className="text-slate-400 hover:text-slate-200 text-xs"
                      >
                        {copiedId === d.id ? 'Copied' : 'Copy'}
                      </button>
                    </div>
                  </div>
                  <pre className="text-xs font-mono text-slate-200 bg-slate-950 p-3 rounded-lg overflow-x-auto whitespace-pre-wrap leading-relaxed">
                    {d.payload}
                  </pre>
                </div>
              ))}
            </div>
          )}
        </>
      )}

      {/* Telemetry Frames Stream */}
      {activeSubTab === 'telemetry' && (
        <>
          {filteredFrames.length === 0 ? (
            <div className="text-center py-16 bg-slate-900/40 rounded-xl border border-slate-800/80">
              <h4 className="text-base font-semibold text-slate-300">No Request Telemetry Frames Captured</h4>
              <p className="text-xs text-slate-500 max-w-md mx-auto mt-1">
                Attach the Zero-Package Debugger Agent and make HTTP requests to your site to capture full request-lifecycle telemetry, SQL queries, logs, and memory stats.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {filteredFrames.map((frame) => {
                const isExpanded = expandedFrameId === frame.id;
                const statusColor =
                  frame.status >= 500
                    ? 'bg-rose-500/20 text-rose-300 border-rose-500/40'
                    : frame.status >= 400
                    ? 'bg-amber-500/20 text-amber-300 border-amber-500/40'
                    : 'bg-emerald-500/20 text-emerald-300 border-emerald-500/40';

                return (
                  <div
                    key={frame.id}
                    className="bg-slate-900/70 border border-slate-800 rounded-xl overflow-hidden transition-all hover:border-slate-700"
                  >
                    <div
                      onClick={() => setExpandedFrameId(isExpanded ? null : frame.id)}
                      className="p-4 flex flex-wrap items-center justify-between gap-3 cursor-pointer select-none"
                    >
                      <div className="flex items-center gap-3">
                        <span className={`px-2 py-0.5 rounded text-[11px] font-mono font-bold border ${statusColor}`}>
                          {frame.status}
                        </span>
                        <span className="px-2 py-0.5 rounded text-[10px] font-mono font-bold bg-slate-800 text-slate-300">
                          {frame.method}
                        </span>
                        <span className="text-xs font-mono font-medium text-slate-200 truncate max-w-md">
                          {frame.url}
                        </span>
                      </div>

                      <div className="flex items-center gap-4 text-xs font-mono text-slate-400">
                        <span>{frame.durationMs.toFixed(1)}ms</span>
                        <span>{(frame.memoryPeakBytes / 1024 / 1024).toFixed(1)}MB</span>
                        <span className="text-indigo-400 font-semibold">{frame.queries?.length || 0} queries</span>
                        {frame.dumps && frame.dumps.length > 0 && (
                          <span className="text-amber-400 font-semibold">{frame.dumps.length} dumps</span>
                        )}
                        <span className="text-slate-500 text-[11px]">
                          {frame.createdAt ? new Date(frame.createdAt).toLocaleTimeString() : ''}
                        </span>
                        <span className="text-slate-500 text-xs">{isExpanded ? '▲' : '▼'}</span>
                      </div>
                    </div>

                    {isExpanded && (
                      <div className="p-4 bg-slate-950/80 border-t border-slate-800/80 space-y-4">
                        {/* Queries */}
                        {frame.queries && frame.queries.length > 0 && (
                          <div className="space-y-2">
                            <h5 className="text-xs font-bold text-slate-400 uppercase tracking-wider">
                              Executed SQL Queries ({frame.queries.length})
                            </h5>
                            <div className="space-y-1.5 max-h-48 overflow-y-auto pr-1">
                              {frame.queries.map((q, idx) => (
                                <div
                                  key={idx}
                                  className="p-2.5 rounded-lg bg-slate-900 border border-slate-800/80 flex items-start justify-between gap-2 text-xs font-mono"
                                >
                                  <div className="text-slate-300 break-all">{q.sql}</div>
                                  <div className="text-indigo-400 whitespace-nowrap pl-2 font-semibold">
                                    {q.timeMs.toFixed(1)}ms
                                  </div>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Logs */}
                        {frame.logs && frame.logs.length > 0 && (
                          <div className="space-y-2">
                            <h5 className="text-xs font-bold text-slate-400 uppercase tracking-wider">
                              Logged Messages ({frame.logs.length})
                            </h5>
                            <div className="space-y-1.5 max-h-40 overflow-y-auto pr-1">
                              {frame.logs.map((l, idx) => (
                                <div
                                  key={idx}
                                  className="p-2 rounded bg-slate-900 border border-slate-800 flex items-center gap-2 text-xs font-mono"
                                >
                                  <span className="px-1.5 py-0.5 rounded text-[10px] font-bold bg-slate-800 text-amber-300 uppercase">
                                    {l.level}
                                  </span>
                                  <span className="text-slate-300 truncate">{l.message}</span>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Request Dumps */}
                        {frame.dumps && frame.dumps.length > 0 && (
                          <div className="space-y-2">
                            <h5 className="text-xs font-bold text-slate-400 uppercase tracking-wider">
                              Dumps In This Request ({frame.dumps.length})
                            </h5>
                            <div className="space-y-2">
                              {frame.dumps.map((d) => (
                                <div key={d.id} className="p-3 bg-slate-900 border border-slate-800 rounded-lg">
                                  <div className="text-xs text-indigo-400 font-mono mb-1">
                                    {d.file}:{d.line}
                                  </div>
                                  <pre className="text-xs font-mono text-slate-200 whitespace-pre-wrap">
                                    {d.payload}
                                  </pre>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </>
      )}
    </div>
  );
};
