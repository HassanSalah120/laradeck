import React, { useState } from 'react';
import {
  ExecuteCheckupFix,
  GenerateAuthCredentials,
} from '../wailsjs/go/main/App';
import { performance } from '../wailsjs/go/models';

export type CheckupItem = performance.CheckupItem;
export type CheckupReport = performance.CheckupReport;
export type LoadTestSpec = performance.LoadTestSpec;
export type LoadTestResult = performance.LoadTestResult;
export type LoadTestPoint = performance.LoadTestPoint;

interface PerformancePanelProps {
  projectId?: string;
  checkup: CheckupReport | null;
  defaultUrl: string;
  onRunCheckup: () => Promise<void>;
  onApplyFix: (fixAction: string) => Promise<void>;
  onRunLoadTest: (spec: LoadTestSpec) => Promise<LoadTestResult>;
}

export const PerformancePanel: React.FC<PerformancePanelProps> = ({
  projectId,
  checkup,
  defaultUrl,
  onRunCheckup,
  onApplyFix,
  onRunLoadTest,
}) => {
  const [activeTab, setActiveTab] = useState<'checkup' | 'loadtest'>('checkup');
  const [isScanning, setIsScanning] = useState(false);
  const [fixingAction, setFixingAction] = useState<string | null>(null);

  // Load test state
  const [loadUrl, setLoadUrl] = useState(defaultUrl || 'http://127.0.0.1:8000');
  const [concurrency, setConcurrency] = useState(5);
  const [durationSec, setDurationSec] = useState(5);
  const [sessionCookies, setSessionCookies] = useState('');
  const [authHeaders, setAuthHeaders] = useState<Record<string, string>>({});
  const [authIdentifier, setAuthIdentifier] = useState('1');
  const [isGeneratingAuth, setIsGeneratingAuth] = useState(false);
  const [authStatusMessage, setAuthStatusMessage] = useState<string | null>(null);

  const [isRunningLoad, setIsRunningLoad] = useState(false);
  const [loadResult, setLoadResult] = useState<LoadTestResult | null>(null);

  const handleScan = async () => {
    setIsScanning(true);
    try {
      await onRunCheckup();
    } finally {
      setIsScanning(false);
    }
  };

  const handleFix = async (item: CheckupItem) => {
    const fixKey = item.fixAction || item.id;
    setFixingAction(fixKey);
    try {
      if (projectId && item.fixAction) {
        await ExecuteCheckupFix(projectId, item.fixAction);
      } else if (item.fixAction) {
        await onApplyFix(item.fixAction);
      }
      await onRunCheckup();
    } catch (err: any) {
      alert('Failed to execute fix: ' + (err.message || String(err)));
    } finally {
      setFixingAction(null);
    }
  };

  const handleGenerateAuth = async () => {
    if (!projectId) return;
    setIsGeneratingAuth(true);
    setAuthStatusMessage(null);
    try {
      const creds = await GenerateAuthCredentials(projectId, authIdentifier);
      if (creds.type === 'cookie' && creds.name && creds.value) {
        const cookieStr = `${creds.name}=${creds.value}`;
        setSessionCookies(cookieStr);
        setAuthStatusMessage(`Generated session cookie: ${cookieStr.slice(0, 30)}...`);
      } else if (creds.token) {
        setAuthHeaders({ Authorization: `Bearer ${creds.token}` });
        setAuthStatusMessage(`Generated Sanctum token: ${creds.token.slice(0, 20)}...`);
      } else {
        setAuthStatusMessage(`Auth credentials ready for user: ${creds.name || authIdentifier}`);
      }
    } catch (e: any) {
      setAuthStatusMessage('Error: ' + (e.message || String(e)));
    } finally {
      setIsGeneratingAuth(false);
    }
  };

  const handleStartLoadTest = async () => {
    setIsRunningLoad(true);
    try {
      const res = await onRunLoadTest({
        url: loadUrl,
        method: 'GET',
        concurrency,
        durationSec,
        maxRequests: 0,
        headers: authHeaders,
        cookies: sessionCookies,
        body: '',
      });
      setLoadResult(res);
    } finally {
      setIsRunningLoad(false);
    }
  };

  return (
    <div className="space-y-4">
      {/* Sub Tabs */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800">
        <div className="flex gap-1.5">
          <button
            onClick={() => setActiveTab('checkup')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'checkup'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Project Checkup &amp; 1-Click Optimizations
          </button>
          <button
            onClick={() => setActiveTab('loadtest')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'loadtest'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Native Authenticated Load Tester
          </button>
        </div>

        {activeTab === 'checkup' && (
          <button
            onClick={handleScan}
            disabled={isScanning}
            className="px-3.5 py-1.5 text-xs font-semibold rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-50 shadow"
          >
            {isScanning ? 'Scanning...' : 'Run Checkup'}
          </button>
        )}
      </div>

      {/* Tab: Checkup */}
      {activeTab === 'checkup' && (
        <div className="space-y-4">
          {checkup && (
            <div className="flex flex-wrap items-center justify-between gap-4 bg-slate-900/60 border border-slate-800 rounded-xl p-4">
              <div>
                <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider">
                  Health &amp; Optimization Score
                </h4>
                <div className="text-2xl font-extrabold text-slate-100 mt-1">
                  <span
                    className={
                      checkup.score >= 80
                        ? 'text-emerald-400'
                        : checkup.score >= 50
                        ? 'text-amber-400'
                        : 'text-rose-400'
                    }
                  >
                    {checkup.score}
                  </span>{' '}
                  / 100
                </div>
              </div>
              <div className="text-xs text-slate-500">
                Scanned: {new Date(checkup.scannedAt).toLocaleTimeString()}
              </div>
            </div>
          )}

          {checkup?.items && checkup.items.length > 0 ? (
            <div className="space-y-3">
              {checkup.items.map((item) => {
                const canFix = Boolean(item.fixAction);
                const isFixing = fixingAction === item.fixAction;

                return (
                  <div
                    key={item.id}
                    className="flex flex-wrap items-center justify-between gap-3 bg-slate-900/70 border border-slate-800 rounded-xl p-4 transition-all hover:border-slate-700"
                  >
                    <div className="flex items-start gap-3">
                      <span
                        className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase mt-0.5 shrink-0 ${
                          item.status === 'ok'
                            ? 'bg-emerald-500/20 text-emerald-300'
                            : item.status === 'error'
                            ? 'bg-rose-500/20 text-rose-300'
                            : 'bg-amber-500/20 text-amber-300'
                        }`}
                      >
                        {item.status}
                      </span>
                      <div>
                        <div className="flex items-center gap-2">
                          <h5 className="text-xs font-bold text-slate-200">{item.title}</h5>
                          {item.category && (
                            <span className="text-[10px] px-1.5 py-0.2 rounded bg-slate-800 text-slate-400 uppercase">
                              {item.category}
                            </span>
                          )}
                        </div>
                        <p className="text-xs text-slate-400 mt-0.5">{item.message}</p>
                      </div>
                    </div>

                    {canFix && item.status !== 'ok' && (
                      <button
                        onClick={() => handleFix(item)}
                        disabled={isFixing}
                        className="px-3.5 py-1.5 text-xs font-semibold rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white shadow disabled:opacity-50 shrink-0"
                      >
                        {isFixing ? 'Applying Fix...' : `1-Click Fix`}
                      </button>
                    )}
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="text-center py-16 bg-slate-900/40 rounded-xl border border-slate-800">
              <h4 className="text-sm font-semibold text-slate-300">Run a Performance Checkup</h4>
              <p className="text-xs text-slate-500 mt-1 max-w-sm mx-auto">
                Audits OPcache, route caching, config caching, event caching, composer autoloader optimization, profiler overhead, and database drivers with one-click fixes.
              </p>
              <button
                onClick={handleScan}
                disabled={isScanning}
                className="mt-4 px-4 py-2 text-xs font-semibold rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white shadow"
              >
                {isScanning ? 'Scanning...' : 'Start Health Checkup'}
              </button>
            </div>
          )}
        </div>
      )}

      {/* Tab: Native Load Testing */}
      {activeTab === 'loadtest' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 space-y-4">
          <div>
            <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider">
              Native High-Performance HTTP Load Tester
            </h4>
            <p className="text-xs text-slate-400 mt-0.5">
              Benchmark route throughput and response time percentiles under concurrent virtual user traffic without third-party dependencies.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div className="md:col-span-2">
              <label className="block text-xs font-semibold text-slate-400 mb-1">Target Route URL</label>
              <input
                type="text"
                value={loadUrl}
                onChange={(e) => setLoadUrl(e.target.value)}
                className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
              />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-400 mb-1">Concurrency (VUs)</label>
              <input
                type="number"
                min={1}
                max={50}
                value={concurrency}
                onChange={(e) => setConcurrency(parseInt(e.target.value) || 1)}
                className="w-full px-3 py-2 text-xs bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-semibold text-slate-400 mb-1">Duration (Seconds)</label>
              <input
                type="number"
                min={1}
                max={60}
                value={durationSec}
                onChange={(e) => setDurationSec(parseInt(e.target.value) || 5)}
                className="w-full px-3 py-2 text-xs bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
              />
            </div>

            {/* Authenticated User Selector */}
            <div>
              <label className="block text-xs font-semibold text-slate-400 mb-1">
                Authenticate as User (ID or Email)
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  placeholder="e.g. 1 or test@example.com"
                  value={authIdentifier}
                  onChange={(e) => setAuthIdentifier(e.target.value)}
                  className="flex-1 px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
                />
                <button
                  type="button"
                  onClick={handleGenerateAuth}
                  disabled={isGeneratingAuth || !projectId}
                  className="px-3 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold disabled:opacity-50 shrink-0"
                >
                  {isGeneratingAuth ? 'Auth...' : 'Issue Auth Session'}
                </button>
              </div>
            </div>
          </div>

          {authStatusMessage && (
            <div className="p-2.5 rounded-lg bg-slate-950 border border-indigo-500/30 text-xs font-mono text-indigo-300">
              {authStatusMessage}
            </div>
          )}

          {sessionCookies && (
            <div>
              <label className="block text-xs font-semibold text-slate-400 mb-1">
                Active Session Cookie Header
              </label>
              <input
                type="text"
                value={sessionCookies}
                onChange={(e) => setSessionCookies(e.target.value)}
                className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
              />
            </div>
          )}

          <button
            onClick={handleStartLoadTest}
            disabled={isRunningLoad || !loadUrl}
            className="w-full py-2.5 px-4 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-md disabled:opacity-50 transition-all"
          >
            {isRunningLoad ? 'Running Concurrent Benchmark...' : 'Start Load Test'}
          </button>

          {loadResult && (
            <div className="bg-slate-950 p-4 rounded-xl border border-indigo-500/30 space-y-4">
              <h5 className="text-xs font-bold text-indigo-400 uppercase">Load Test Summary</h5>

              {/* Redirect Warning Alert */}
              {loadResult.redirectWarning && (
                <div className="p-3 rounded-lg bg-amber-950/40 border border-amber-500/40 text-amber-200 text-xs space-y-1">
                  <div className="font-bold flex items-center gap-1.5">
                    ⚠ Redirect Alert Detected (HTTP 302/301)
                  </div>
                  <p>
                    More than 50% of benchmark requests were redirected. You are likely load-testing an unauthenticated route redirecting to /login. Use the Authenticate as User tool above to simulate authenticated sessions.
                  </p>
                </div>
              )}

              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 text-xs text-slate-300">
                <div className="bg-slate-900 p-2.5 rounded-lg border border-slate-800">
                  <div className="text-slate-500">Total Requests</div>
                  <div className="text-lg font-bold text-slate-100">{loadResult.totalRequests}</div>
                </div>
                <div className="bg-slate-900 p-2.5 rounded-lg border border-slate-800">
                  <div className="text-slate-500">Throughput</div>
                  <div className="text-lg font-bold text-emerald-400">{loadResult.rps.toFixed(1)} req/sec</div>
                </div>
                <div className="bg-slate-900 p-2.5 rounded-lg border border-slate-800">
                  <div className="text-slate-500">P50 Latency</div>
                  <div className="text-lg font-bold text-indigo-400">{loadResult.p50Ms.toFixed(1)} ms</div>
                </div>
                <div className="bg-slate-900 p-2.5 rounded-lg border border-slate-800">
                  <div className="text-slate-500">P99 Latency</div>
                  <div className="text-lg font-bold text-purple-400">{loadResult.p99Ms.toFixed(1)} ms</div>
                </div>
              </div>

              {/* Time-Series Chart */}
              {loadResult.timeSeries && loadResult.timeSeries.length > 0 && (
                <div className="p-3 bg-slate-900 rounded-lg border border-slate-800 space-y-2">
                  <div className="flex items-center justify-between text-xs text-slate-400">
                    <span className="font-bold uppercase tracking-wider">
                      Sampling Timeline (Throughput &amp; Latency)
                    </span>
                    <span>{loadResult.timeSeries.length} intervals</span>
                  </div>

                  <div className="h-28 flex items-end gap-1 pt-2">
                    {(() => {
                      const maxRps = Math.max(...loadResult.timeSeries.map((p: LoadTestPoint) => p.rps), 1);
                      return loadResult.timeSeries.map((point: LoadTestPoint, idx: number) => {
                        const heightPct = Math.max((point.rps / maxRps) * 100, 4);
                        return (
                          <div
                            key={idx}
                            className="flex-1 flex flex-col items-center justify-end group relative h-full"
                          >
                            <div
                              style={{ height: `${heightPct}%` }}
                              className="w-full rounded-t transition-all bg-indigo-500/80 group-hover:bg-indigo-400"
                            />
                            {/* Tooltip on hover */}
                            <div className="absolute bottom-full mb-1 hidden group-hover:flex flex-col bg-slate-950 border border-slate-700 text-[10px] font-mono text-slate-200 p-1.5 rounded z-10 whitespace-nowrap shadow-xl">
                              <span>Time: {point.elapsedSec}s</span>
                              <span>RPS: {point.rps.toFixed(1)}</span>
                              <span>Avg Latency: {point.avgMs.toFixed(1)}ms</span>
                              <span>Requests: {point.requestsCount}</span>
                            </div>
                          </div>
                        );
                      });
                    })()}
                  </div>
                  <div className="flex justify-between text-[10px] text-slate-500 font-mono">
                    <span>0.0s</span>
                    <span>Throughput (RPS) over Duration</span>
                    <span>{durationSec}.0s</span>
                  </div>
                </div>
              )}

              <div className="text-xs text-slate-400 flex flex-wrap gap-4 pt-2 border-t border-slate-800">
                <span>Successful: <strong className="text-emerald-400">{loadResult.successful}</strong></span>
                <span>Failed: <strong className="text-rose-400">{loadResult.failed}</strong></span>
                <span>Min: {loadResult.minMs.toFixed(1)} ms</span>
                <span>Max: {loadResult.maxMs.toFixed(1)} ms</span>
                <span>Avg: {loadResult.avgMs.toFixed(1)} ms</span>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
