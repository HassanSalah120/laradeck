import React, { useState, useEffect } from 'react';
import {
  InspectWebhookRoutes,
  ExplainStatusCode,
  ValidateWebhookSignature,
} from '../wailsjs/go/main/App';
import { webhooks } from '../wailsjs/go/models';

export type WebhookDelivery = webhooks.WebhookDelivery;
export type WebhookReplayResult = webhooks.WebhookReplayResult;
export type WebhookRouteInspection = webhooks.WebhookRouteInspection;

interface WebhooksPanelProps {
  projectId?: string;
  deliveries: WebhookDelivery[];
  localSiteUrl: string;
  onClear: () => void;
  onReplay: (deliveryId: string, targetUrl: string) => Promise<WebhookReplayResult>;
  onDispatchSigned: (targetUrl: string, provider: string, secret: string, payload: string) => Promise<WebhookReplayResult>;
  onRefresh: () => void;
}

export const WebhooksPanel: React.FC<WebhooksPanelProps> = ({
  projectId,
  deliveries,
  localSiteUrl,
  onClear,
  onReplay,
  onDispatchSigned,
  onRefresh,
}) => {
  const [activeTab, setActiveTab] = useState<'inbox' | 'routes' | 'sender'>('inbox');
  const [selectedDelivery, setSelectedDelivery] = useState<WebhookDelivery | null>(null);
  const [replayTarget, setReplayTarget] = useState(
    localSiteUrl ? `${localSiteUrl}/api/webhooks/stripe` : 'http://127.0.0.1:8000/api/webhooks/stripe'
  );
  const [isReplaying, setIsReplaying] = useState(false);
  const [replayResult, setReplayResult] = useState<WebhookReplayResult | null>(null);
  const [copiedSnippet, setCopiedSnippet] = useState<string | null>(null);

  // Discovered routes state
  const [discoveredRoutes, setDiscoveredRoutes] = useState<WebhookRouteInspection[]>([]);
  const [loadingRoutes, setLoadingRoutes] = useState(false);

  // Test event dispatcher state
  const [testProvider, setTestProvider] = useState('stripe');
  const [testSecret, setTestSecret] = useState('whsec_test_secret_123');
  const [testTargetUrl, setTestTargetUrl] = useState(
    localSiteUrl ? `${localSiteUrl}/api/webhooks/stripe` : 'http://127.0.0.1:8000/api/webhooks/stripe'
  );
  const [testPayload, setTestPayload] = useState(
    '{\n  "type": "payment_intent.succeeded",\n  "data": {\n    "id": "pi_12345",\n    "amount": 2500\n  }\n}'
  );
  const [isSendingTest, setIsSendingTest] = useState(false);
  const [testResult, setTestResult] = useState<WebhookReplayResult | null>(null);

  // Signature validation state in inspector
  const [verifyProvider, setVerifyProvider] = useState('stripe');
  const [verifySecret, setVerifySecret] = useState('');
  const [sigCheckResult, setSigCheckResult] = useState<string | null>(null);

  useEffect(() => {
    if (projectId) {
      loadRoutes();
    }
  }, [projectId]);

  const loadRoutes = async () => {
    if (!projectId) return;
    setLoadingRoutes(true);
    try {
      const routes = await InspectWebhookRoutes(projectId);
      setDiscoveredRoutes(routes || []);
    } catch (err) {
      console.error('Failed to inspect webhook routes', err);
    } finally {
      setLoadingRoutes(false);
    }
  };

  const handleReplay = async () => {
    if (!selectedDelivery) return;
    setIsReplaying(true);
    setReplayResult(null);
    try {
      const res = await onReplay(selectedDelivery.id, replayTarget);
      setReplayResult(res);
    } catch (err: any) {
      setReplayResult({
        deliveryId: selectedDelivery.id,
        targetUrl: replayTarget,
        statusCode: 0,
        durationMs: 0,
        response: '',
        error: err.message || String(err),
      });
    } finally {
      setIsReplaying(false);
    }
  };

  const handleSendTest = async () => {
    setIsSendingTest(true);
    setTestResult(null);
    try {
      const res = await onDispatchSigned(testTargetUrl, testProvider, testSecret, testPayload);
      setTestResult(res);
    } catch (err: any) {
      setTestResult({
        deliveryId: '',
        targetUrl: testTargetUrl,
        statusCode: 0,
        durationMs: 0,
        response: '',
        error: err.message || String(err),
      });
    } finally {
      setIsSendingTest(false);
    }
  };

  const handleVerifyDeliverySig = async () => {
    if (!selectedDelivery || !verifySecret) return;
    try {
      // Find signature header
      let headerSig = '';
      for (const [key, vals] of Object.entries(selectedDelivery.headers || {})) {
        if (
          key.toLowerCase().includes('signature') ||
          key.toLowerCase().includes('hmac') ||
          key.toLowerCase().includes('hub-signature')
        ) {
          headerSig = vals[0] || '';
          break;
        }
      }
      const res = await ValidateWebhookSignature(
        verifyProvider,
        verifySecret,
        selectedDelivery.body,
        headerSig
      );
      setSigCheckResult(res);
    } catch (err: any) {
      setSigCheckResult('Error: ' + (err.message || String(err)));
    }
  };

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedSnippet(id);
    setTimeout(() => setCopiedSnippet(null), 2500);
  };

  return (
    <div className="space-y-4">
      {/* Top Header & Subtabs */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800">
        <div className="flex flex-wrap gap-1.5">
          <button
            onClick={() => setActiveTab('inbox')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'inbox'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Captured Deliveries ({deliveries.length})
          </button>
          <button
            onClick={() => {
              setActiveTab('routes');
              loadRoutes();
            }}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'routes'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Route Diagnostics ({discoveredRoutes.length})
          </button>
          <button
            onClick={() => setActiveTab('sender')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'sender'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Signed Test Dispatcher
          </button>
        </div>

        <div className="flex items-center gap-2">
          {activeTab === 'inbox' && (
            <button
              onClick={onClear}
              disabled={deliveries.length === 0}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-rose-500/20 text-rose-300 hover:bg-rose-500/30 border border-rose-500/30 disabled:opacity-50"
            >
              Clear Deliveries
            </button>
          )}
          <button
            onClick={() => {
              onRefresh();
              if (activeTab === 'routes') loadRoutes();
            }}
            className="px-3 py-1.5 text-xs font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
          >
            Refresh
          </button>
        </div>
      </div>

      {/* Tab: Inbound Deliveries */}
      {activeTab === 'inbox' && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Deliveries List */}
          <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-4 space-y-3">
            <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider">Inbound Deliveries</h4>
            {deliveries.length === 0 ? (
              <div className="text-center py-14 text-slate-500">
                <p className="text-xs font-semibold text-slate-400">No Webhook Deliveries Captured</p>
                <p className="text-[11px] text-slate-600 mt-1 max-w-xs mx-auto">
                  Incoming POST webhooks sent to this site or replayed from providers will be captured here.
                </p>
              </div>
            ) : (
              <div className="space-y-2 max-h-[500px] overflow-y-auto pr-1">
                {deliveries.map((del) => (
                  <div
                    key={del.id}
                    onClick={() => {
                      setSelectedDelivery(del);
                      setReplayResult(null);
                      setSigCheckResult(null);
                    }}
                    className={`p-3 rounded-lg border cursor-pointer transition-all ${
                      selectedDelivery?.id === del.id
                        ? 'bg-indigo-950/40 border-indigo-500/60'
                        : 'bg-slate-950/60 border-slate-800/80 hover:border-slate-700'
                    }`}
                  >
                    <div className="flex items-center justify-between text-xs font-semibold">
                      <div className="flex items-center gap-2">
                        <span className="px-1.5 py-0.5 rounded text-[10px] font-bold bg-blue-600/30 text-blue-300">
                          {del.method}
                        </span>
                        <span className="text-slate-200 font-mono text-[11px] truncate max-w-xs">
                          {del.endpointId}
                        </span>
                      </div>
                      <span className="text-[11px] text-slate-500">
                        {del.receivedAt ? new Date(del.receivedAt).toLocaleTimeString() : ''}
                      </span>
                    </div>
                    <div className="text-[11px] text-slate-400 truncate mt-1">IP: {del.clientIp}</div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Delivery Inspector & Replay */}
          <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 space-y-3">
            <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider">
              Delivery Details &amp; Replay
            </h4>
            {selectedDelivery ? (
              <div className="space-y-3">
                <div className="text-xs text-slate-300 grid grid-cols-2 gap-2 bg-slate-950 p-2.5 rounded-lg border border-slate-800 font-mono">
                  <div>ID: <strong className="text-indigo-400">{selectedDelivery.id}</strong></div>
                  <div>IP: <strong>{selectedDelivery.clientIp}</strong></div>
                </div>

                {/* Headers Inspector */}
                {selectedDelivery.headers && (
                  <div>
                    <label className="block text-[11px] font-semibold text-slate-400 mb-1">
                      Inbound HTTP Headers
                    </label>
                    <div className="p-2.5 bg-slate-950 rounded-lg border border-slate-800 font-mono text-[11px] space-y-0.5 max-h-32 overflow-y-auto">
                      {Object.entries(selectedDelivery.headers).map(([k, v]) => (
                        <div key={k} className="flex gap-2 text-slate-300">
                          <span className="text-indigo-300 shrink-0">{k}:</span>
                          <span className="truncate">{Array.isArray(v) ? v.join(', ') : String(v)}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                {/* Body */}
                <div>
                  <label className="block text-[11px] font-semibold text-slate-400 mb-1">Payload Body</label>
                  <pre className="text-xs font-mono text-slate-200 bg-slate-950 p-3 rounded-lg overflow-x-auto max-h-40 whitespace-pre-wrap">
                    {selectedDelivery.body}
                  </pre>
                </div>

                {/* Signature Verification Tool */}
                <div className="p-3 bg-slate-950 rounded-lg border border-slate-800 space-y-2">
                  <div className="flex items-center justify-between text-xs font-semibold text-slate-300">
                    <span>Signature Verification:</span>
                    <select
                      value={verifyProvider}
                      onChange={(e) => setVerifyProvider(e.target.value)}
                      className="px-2 py-0.5 text-xs bg-slate-900 border border-slate-700 rounded text-slate-200"
                    >
                      <option value="stripe">Stripe</option>
                      <option value="github">GitHub</option>
                      <option value="shopify">Shopify</option>
                      <option value="paddle">Paddle</option>
                    </select>
                  </div>
                  <div className="flex gap-2">
                    <input
                      type="text"
                      placeholder="Webhook signing secret (e.g. whsec_...)"
                      value={verifySecret}
                      onChange={(e) => setVerifySecret(e.target.value)}
                      className="flex-1 px-2.5 py-1 text-xs font-mono bg-slate-900 border border-slate-700 rounded text-slate-200"
                    />
                    <button
                      onClick={handleVerifyDeliverySig}
                      disabled={!verifySecret}
                      className="px-3 py-1 text-xs font-semibold rounded bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-50"
                    >
                      Verify
                    </button>
                  </div>
                  {sigCheckResult && (
                    <div
                      className={`text-xs font-mono p-2 rounded ${
                        sigCheckResult.includes('Valid')
                          ? 'bg-emerald-950/60 text-emerald-300 border border-emerald-500/40'
                          : 'bg-rose-950/60 text-rose-300 border border-rose-500/40'
                      }`}
                    >
                      {sigCheckResult}
                    </div>
                  )}
                </div>

                {/* Replay form */}
                <div className="pt-2 border-t border-slate-800 space-y-2">
                  <label className="block text-[11px] font-semibold text-slate-300">Replay Target Route</label>
                  <div className="flex gap-2">
                    <input
                      type="text"
                      value={replayTarget}
                      onChange={(e) => setReplayTarget(e.target.value)}
                      className="flex-1 px-3 py-1.5 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
                    />
                    <button
                      onClick={handleReplay}
                      disabled={isReplaying}
                      className="px-4 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow disabled:opacity-50"
                    >
                      {isReplaying ? 'Replaying...' : 'Replay Delivery'}
                    </button>
                  </div>
                </div>

                {/* Replay Result */}
                {replayResult && (
                  <div
                    className={`p-3 rounded-lg border text-xs font-mono space-y-1.5 ${
                      replayResult.statusCode >= 200 && replayResult.statusCode < 300
                        ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-200'
                        : 'bg-rose-950/40 border-rose-500/40 text-rose-200'
                    }`}
                  >
                    <div className="flex items-center justify-between font-bold">
                      <span>HTTP {replayResult.statusCode}</span>
                      <span>{replayResult.durationMs.toFixed(1)}ms</span>
                    </div>

                    {/* Plain English explanation */}
                    {replayResult.statusCode > 0 && (
                      <div className="text-[11px] text-slate-300 border-t border-slate-800 pt-1">
                        <strong>Diagnostics: </strong>
                        {ExplainStatusCode(replayResult.statusCode)}
                      </div>
                    )}

                    {replayResult.error && <div>{replayResult.error}</div>}
                    {replayResult.response && (
                      <pre className="text-[11px] opacity-90 max-h-32 overflow-y-auto whitespace-pre-wrap">
                        {replayResult.response}
                      </pre>
                    )}
                  </div>
                )}
              </div>
            ) : (
              <div className="py-20 text-center text-slate-500 text-xs">
                Select a captured delivery from the left to inspect headers, payload, verify signatures, or replay to local routes.
              </div>
            )}
          </div>
        </div>
      )}

      {/* Tab: Route Diagnostics */}
      {activeTab === 'routes' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider">
                Discovered Webhook Routes &amp; Pre-flight Diagnostics
              </h4>
              <p className="text-xs text-slate-400 mt-0.5">
                Automatically detects webhook controllers, checks for CSRF exclusions, warns against authentication traps, and audits environment secret keys.
              </p>
            </div>
            {loadingRoutes && <span className="text-xs text-slate-400">Scanning routes...</span>}
          </div>

          {discoveredRoutes.length === 0 ? (
            <div className="text-center py-16 text-slate-500">
              <h4 className="text-sm font-semibold text-slate-300">No Webhook Routes Detected</h4>
              <p className="text-xs text-slate-500 mt-1">
                Endpoints with names or paths containing stripe, webhook, github, or paddle will appear here.
              </p>
            </div>
          ) : (
            <div className="space-y-3">
              {discoveredRoutes.map((route, idx) => (
                <div
                  key={idx}
                  className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-3 hover:border-slate-700 transition-all"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <span className="px-2 py-0.5 rounded text-[10px] font-bold font-mono bg-indigo-950 text-indigo-300 border border-indigo-500/30 uppercase">
                        {route.provider}
                      </span>
                      <strong className="text-sm font-mono text-slate-200">{route.uri}</strong>
                    </div>
                    <span className="text-xs font-mono text-slate-400 truncate max-w-sm">
                      {route.action}
                    </span>
                  </div>

                  {/* Diagnostic Badges */}
                  <div className="flex flex-wrap gap-2 text-xs">
                    {/* CSRF Status */}
                    {route.csrfBlocked ? (
                      <span className="px-2.5 py-1 rounded bg-rose-500/20 text-rose-300 border border-rose-500/40 font-semibold">
                        CSRF Protection Active (Will Fail with 419)
                      </span>
                    ) : (
                      <span className="px-2.5 py-1 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 font-semibold">
                        CSRF Exempted
                      </span>
                    )}

                    {/* Auth Trap */}
                    {route.authBlocked ? (
                      <span className="px-2.5 py-1 rounded bg-amber-500/20 text-amber-300 border border-amber-500/40 font-semibold">
                        Auth Trap: Middleware Blocked (Will redirect 302 / 401)
                      </span>
                    ) : (
                      <span className="px-2.5 py-1 rounded bg-slate-800 text-slate-300 font-semibold">
                        Public Access (No Auth Trap)
                      </span>
                    )}

                    {/* Secret Key Status */}
                    {route.secretConfigured ? (
                      <span className="px-2.5 py-1 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/40 font-semibold">
                        Secret Key Configured ({route.secretKeyName})
                      </span>
                    ) : (
                      <span className="px-2.5 py-1 rounded bg-amber-500/20 text-amber-300 border border-amber-500/40 font-semibold">
                        Missing Secret Key ({route.secretKeyName})
                      </span>
                    )}
                  </div>

                  {/* 1-Click CSRF Exception Snippet Copy */}
                  {route.csrfBlocked && route.csrfFix && (
                    <div className="p-3 rounded-lg bg-slate-950 border border-rose-500/30 space-y-2">
                      <div className="flex items-center justify-between text-xs text-rose-300 font-semibold">
                        <span>Fix CSRF: Add exclusion to bootstrap/app.php:</span>
                        <button
                          onClick={() => copyToClipboard(route.csrfFix, `csrf-${idx}`)}
                          className="px-2.5 py-1 rounded bg-indigo-600 hover:bg-indigo-500 text-white text-[11px]"
                        >
                          {copiedSnippet === `csrf-${idx}` ? 'Copied' : 'Copy Snippet'}
                        </button>
                      </div>
                      <pre className="text-xs font-mono text-slate-300 overflow-x-auto">
                        {route.csrfFix}
                      </pre>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Tab: Signed Test Event Dispatcher */}
      {activeTab === 'sender' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 space-y-4">
          <div>
            <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider">
              Signed Webhook Test Dispatcher
            </h4>
            <p className="text-xs text-slate-400 mt-0.5">
              Generate HMAC-SHA256 signatures for Stripe, GitHub, Shopify, or Paddle and simulate live incoming webhook delivery to your local routes.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-semibold text-slate-400 mb-1">Target Endpoint URL</label>
              <input
                type="text"
                value={testTargetUrl}
                onChange={(e) => setTestTargetUrl(e.target.value)}
                className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
              />
            </div>
            <div>
              <label className="block text-xs font-semibold text-slate-400 mb-1">Provider Format</label>
              <select
                value={testProvider}
                onChange={(e) => setTestProvider(e.target.value)}
                className="w-full px-3 py-2 text-xs bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
              >
                <option value="stripe">Stripe (Stripe-Signature: t=...,v1=...)</option>
                <option value="github">GitHub (X-Hub-Signature-256: sha256=...)</option>
                <option value="shopify">Shopify (X-Shopify-Hmac-Sha256: base64)</option>
                <option value="paddle">Paddle (Paddle-Signature: ts=...;h1=...)</option>
              </select>
            </div>
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-400 mb-1">Signing Secret</label>
            <input
              type="text"
              value={testSecret}
              onChange={(e) => setTestSecret(e.target.value)}
              className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
            />
          </div>

          <div>
            <label className="block text-xs font-semibold text-slate-400 mb-1">JSON Payload</label>
            <textarea
              rows={6}
              value={testPayload}
              onChange={(e) => setTestPayload(e.target.value)}
              className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
            />
          </div>

          <button
            onClick={handleSendTest}
            disabled={isSendingTest || !testTargetUrl}
            className="w-full py-2.5 px-4 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-md disabled:opacity-50 transition-all"
          >
            {isSendingTest ? 'Signing & Dispatching Webhook...' : 'Sign & Dispatch Test Webhook'}
          </button>

          {testResult && (
            <div
              className={`p-4 rounded-xl border text-xs font-mono space-y-2 ${
                testResult.statusCode >= 200 && testResult.statusCode < 300
                  ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-200'
                  : 'bg-rose-950/40 border-rose-500/40 text-rose-200'
              }`}
            >
              <div className="flex items-center justify-between font-bold text-sm">
                <span>Response: HTTP {testResult.statusCode}</span>
                <span>{testResult.durationMs.toFixed(1)}ms</span>
              </div>

              {testResult.statusCode > 0 && (
                <div className="text-xs text-slate-300 border-t border-slate-800 pt-1.5">
                  <strong>Explanation: </strong>
                  {ExplainStatusCode(testResult.statusCode)}
                </div>
              )}

              {testResult.error && <div>Error: {testResult.error}</div>}
              {testResult.response && (
                <pre className="text-xs opacity-90 max-h-40 overflow-y-auto whitespace-pre-wrap">
                  {testResult.response}
                </pre>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
