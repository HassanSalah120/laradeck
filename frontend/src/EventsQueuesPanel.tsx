import React, { useState, useEffect } from 'react';
import {
  IntrospectEvent,
  IntrospectJob,
  RunJobNow,
  DispatchEvent,
} from '../wailsjs/go/main/App';
import { actions } from '../wailsjs/go/models';

export type EventListenerItem = actions.EventListenerItem;
export type FailedJobItem = actions.FailedJobItem;
export type QueueWorkerStatus = actions.QueueWorkerStatus;
export type EventIntrospection = actions.EventIntrospection;
export type JobIntrospection = actions.JobIntrospection;
export type JobRunResult = actions.JobRunResult;
export type ParameterInfo = actions.ParameterInfo;

interface EventsQueuesPanelProps {
  projectId?: string;
  events: EventListenerItem[];
  failedJobs: FailedJobItem[];
  workerStatus: QueueWorkerStatus;
  onDispatchEvent?: (eventClass: string, payloadJSON: string) => Promise<{ success: boolean; output: string[]; error?: string }>;
  onStartWorker: (queue: string) => Promise<void>;
  onStopWorker: () => Promise<void>;
  onRetryJob: (id: string) => Promise<void>;
  onRetryAllJobs: () => Promise<void>;
  onRefresh: () => void;
}

export const EventsQueuesPanel: React.FC<EventsQueuesPanelProps> = ({
  projectId,
  events,
  failedJobs,
  workerStatus,
  onDispatchEvent,
  onStartWorker,
  onStopWorker,
  onRetryJob,
  onRetryAllJobs,
  onRefresh,
}) => {
  const [activeTab, setActiveTab] = useState<'events' | 'jobs' | 'supervisor' | 'failed'>('events');

  // Event Dispatcher State
  const [selectedEvent, setSelectedEvent] = useState<string>('');
  const [eventIntrospection, setEventIntrospection] = useState<EventIntrospection | null>(null);
  const [eventPayloadMode, setEventPayloadMode] = useState<'form' | 'raw'>('form');
  const [eventFormValues, setEventFormValues] = useState<Record<string, any>>({});
  const [rawEventPayload, setRawEventPayload] = useState<string>('{}');
  const [isDispatching, setIsDispatching] = useState(false);
  const [dispatchResult, setDispatchResult] = useState<{ success: boolean; output: string[]; error?: string } | null>(null);

  // Job Inspector & Runner State
  const [jobSearch, setJobSearch] = useState<string>('App\\Jobs\\');
  const [selectedJob, setSelectedJob] = useState<string>('');
  const [jobIntrospection, setJobIntrospection] = useState<JobIntrospection | null>(null);
  const [jobFormValues, setJobFormValues] = useState<Record<string, any>>({});
  const [jobPayloadMode, setJobPayloadMode] = useState<'form' | 'raw'>('form');
  const [rawJobPayload, setRawJobPayload] = useState<string>('{}');
  const [isRunningJob, setIsRunningJob] = useState(false);
  const [jobResult, setJobResult] = useState<JobRunResult | null>(null);

  // Supervisor State
  const [workerQueue, setWorkerQueue] = useState('default');

  // Failed Jobs State
  const [searchFailed, setSearchFailed] = useState('');

  // When event selection changes, introspect
  useEffect(() => {
    if (selectedEvent && projectId) {
      IntrospectEvent(projectId, selectedEvent)
        .then((intro) => {
          setEventIntrospection(intro);
          const initialForm: Record<string, any> = {};
          intro.parameters?.forEach((p) => {
            initialForm[p.name] = p.defaultValue ?? (p.type === 'int' ? 0 : p.type === 'bool' ? false : '');
          });
          setEventFormValues(initialForm);
          setRawEventPayload(JSON.stringify(initialForm, null, 2));
        })
        .catch(() => {
          setEventIntrospection(null);
        });
    }
  }, [selectedEvent, projectId]);

  // When job is inspected
  const handleInspectJob = async (jobClass: string) => {
    if (!jobClass || !projectId) return;
    setSelectedJob(jobClass);
    try {
      const intro = await IntrospectJob(projectId, jobClass);
      setJobIntrospection(intro);
      const initialForm: Record<string, any> = {};
      intro.parameters?.forEach((p) => {
        initialForm[p.name] = p.defaultValue ?? (p.type === 'int' ? 0 : p.type === 'bool' ? false : '');
      });
      setJobFormValues(initialForm);
      setRawJobPayload(JSON.stringify(initialForm, null, 2));
    } catch (e: any) {
      alert('Failed to introspect job: ' + (e.message || String(e)));
    }
  };

  const handleDispatch = async () => {
    if (!selectedEvent || !projectId) return;
    setIsDispatching(true);
    setDispatchResult(null);
    try {
      const payload =
        eventPayloadMode === 'form' ? JSON.stringify(eventFormValues) : rawEventPayload;
      if (onDispatchEvent) {
        const res = await onDispatchEvent(selectedEvent, payload);
        setDispatchResult(res);
      } else {
        const res = await DispatchEvent(projectId, selectedEvent, payload);
        setDispatchResult({
          success: res.success,
          output: res.output || [],
          error: res.error,
        });
      }
    } catch (err: any) {
      setDispatchResult({ success: false, output: [], error: err.message || 'Dispatch failed' });
    } finally {
      setIsDispatching(false);
    }
  };

  const handleRunJob = async (execution: 'sync' | 'queued') => {
    if (!selectedJob || !projectId) return;
    setIsRunningJob(true);
    setJobResult(null);
    try {
      const payload =
        jobPayloadMode === 'form' ? JSON.stringify(jobFormValues) : rawJobPayload;
      if (execution === 'sync') {
        const res = await RunJobNow(projectId, selectedJob, payload);
        setJobResult(res);
      } else {
        // Dispatch to queue
        const res = await DispatchEvent(
          projectId,
          selectedJob,
          payload
        );
        setJobResult({
          success: res.success,
          output: res.output || ['Job pushed to queue successfully.'],
          durationMs: res.durationMs,
          queries: 0,
          error: res.error,
        });
      }
    } catch (e: any) {
      setJobResult({
        success: false,
        output: [],
        error: e.message || String(e),
        durationMs: 0,
        queries: 0,
      });
    } finally {
      setIsRunningJob(false);
    }
  };

  const filteredFailed = failedJobs.filter(
    (j) =>
      j.class?.toLowerCase().includes(searchFailed.toLowerCase()) ||
      j.exception?.toLowerCase().includes(searchFailed.toLowerCase()) ||
      j.id?.includes(searchFailed)
  );

  return (
    <div className="space-y-4">
      {/* Top Subtabs Navigation */}
      <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-900/60 p-2.5 rounded-xl border border-slate-800">
        <div className="flex flex-wrap gap-1.5">
          <button
            onClick={() => setActiveTab('events')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'events'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Events &amp; Dispatcher ({events.length})
          </button>
          <button
            onClick={() => setActiveTab('jobs')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'jobs'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Jobs Workbench (Run Now / Queue)
          </button>
          <button
            onClick={() => setActiveTab('supervisor')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'supervisor'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Queue Supervisor
            {workerStatus.running && (
              <span className="ml-1.5 w-2 h-2 rounded-full bg-emerald-400 inline-block animate-pulse" />
            )}
          </button>
          <button
            onClick={() => setActiveTab('failed')}
            className={`px-3.5 py-1.5 rounded-lg text-xs font-semibold transition-all ${
              activeTab === 'failed'
                ? 'bg-indigo-600 text-white shadow'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800'
            }`}
          >
            Failed Jobs ({failedJobs.length})
          </button>
        </div>

        <button
          onClick={onRefresh}
          className="px-3 py-1.5 text-xs font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700"
        >
          Refresh
        </button>
      </div>

      {/* Events Subtab */}
      {activeTab === 'events' && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Registered Events List */}
          <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-4 space-y-3">
            <h4 className="text-xs font-bold text-slate-400 uppercase tracking-wider">
              Registered Events &amp; Listeners
            </h4>
            {events.length === 0 ? (
              <p className="text-xs text-slate-500 py-8 text-center">No registered events detected in project.</p>
            ) : (
              <div className="space-y-2 max-h-[500px] overflow-y-auto pr-1">
                {events.map((ev) => (
                  <div
                    key={ev.event}
                    onClick={() => setSelectedEvent(ev.event)}
                    className={`p-3 rounded-lg border cursor-pointer transition-all ${
                      selectedEvent === ev.event
                        ? 'bg-indigo-950/40 border-indigo-500/60'
                        : 'bg-slate-950/60 border-slate-800/80 hover:border-slate-700'
                    }`}
                  >
                    <div className="flex items-center justify-between text-xs font-semibold text-slate-200">
                      <span className="font-mono text-indigo-300 truncate">{ev.event}</span>
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-400 shrink-0">
                        {ev.listeners.length} listener{ev.listeners.length !== 1 ? 's' : ''}
                      </span>
                    </div>
                    {ev.listeners.length > 0 && (
                      <div className="mt-1.5 space-y-0.5 pl-2 border-l border-slate-800">
                        {ev.listeners.map((l, i) => (
                          <div key={i} className="text-[11px] font-mono text-slate-400 truncate">
                            ↳ {l}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Interactive Event Dispatcher Form */}
          <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 space-y-4">
            <div className="flex items-center justify-between">
              <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider">
                Event Dispatcher &amp; Introspection
              </h4>
              <div className="flex rounded-lg bg-slate-950 p-0.5 border border-slate-800">
                <button
                  onClick={() => setEventPayloadMode('form')}
                  className={`px-2.5 py-1 text-[11px] font-medium rounded ${
                    eventPayloadMode === 'form' ? 'bg-indigo-600 text-white' : 'text-slate-400'
                  }`}
                >
                  Parameter Form
                </button>
                <button
                  onClick={() => setEventPayloadMode('raw')}
                  className={`px-2.5 py-1 text-[11px] font-medium rounded ${
                    eventPayloadMode === 'raw' ? 'bg-indigo-600 text-white' : 'text-slate-400'
                  }`}
                >
                  Raw PHP Expression
                </button>
              </div>
            </div>

            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-slate-400 mb-1">Target Event Class</label>
                <input
                  type="text"
                  placeholder="e.g. App\Events\OrderPlaced"
                  value={selectedEvent}
                  onChange={(e) => setSelectedEvent(e.target.value)}
                  className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-indigo-500"
                />
              </div>

              {/* Dispatched From Scanned Badges */}
              {eventIntrospection?.dispatchedFrom && eventIntrospection.dispatchedFrom.length > 0 && (
                <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800 space-y-1">
                  <span className="text-[11px] font-semibold text-slate-400">Dispatched from code:</span>
                  <div className="flex flex-wrap gap-1.5 mt-1">
                    {eventIntrospection.dispatchedFrom.map((loc, idx) => (
                      <span
                        key={idx}
                        className="px-2 py-0.5 rounded bg-indigo-950/60 border border-indigo-500/30 font-mono text-[10px] text-indigo-300"
                      >
                        {loc}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Form vs Raw Inputs */}
              {eventPayloadMode === 'form' ? (
                <div className="space-y-2">
                  <span className="text-xs font-medium text-slate-400">Constructor Parameters:</span>
                  {eventIntrospection?.parameters && eventIntrospection.parameters.length > 0 ? (
                    <div className="space-y-2 bg-slate-950 p-3 rounded-lg border border-slate-800">
                      {eventIntrospection.parameters.map((prop) => (
                        <div key={prop.name} className="flex items-center justify-between gap-3 text-xs">
                          <label className="font-mono text-slate-300 w-1/3 truncate">
                            ${prop.name}
                            <span className="text-[10px] text-slate-500 ml-1">({prop.type})</span>
                          </label>
                          <input
                            type="text"
                            value={eventFormValues[prop.name] ?? ''}
                            onChange={(e) =>
                              setEventFormValues({ ...eventFormValues, [prop.name]: e.target.value })
                            }
                            placeholder={prop.hasDefault ? String(prop.defaultValue ?? '') : 'required'}
                            className="w-2/3 px-2 py-1 text-xs font-mono bg-slate-900 border border-slate-700 rounded text-slate-200 focus:outline-none focus:border-indigo-500"
                          />
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-[11px] text-slate-500 italic bg-slate-950 p-2.5 rounded border border-slate-800">
                      No constructor parameters detected for this event class.
                    </p>
                  )}
                </div>
              ) : (
                <div>
                  <label className="block text-xs font-medium text-slate-400 mb-1">
                    Raw PHP Expression or JSON
                  </label>
                  <textarea
                    rows={5}
                    value={rawEventPayload}
                    onChange={(e) => setRawEventPayload(e.target.value)}
                    placeholder='{"orderId": 42, "user": "User::first()"}'
                    className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-indigo-500"
                  />
                </div>
              )}

              {/* Registered Listeners preview */}
              {eventIntrospection?.listeners && (
                <div className="text-xs text-slate-400">
                  <span className="font-semibold text-slate-300">Registered Listeners to Trigger: </span>
                  {eventIntrospection.listeners.length > 0
                    ? eventIntrospection.listeners.join(', ')
                    : 'None'}
                </div>
              )}

              <button
                onClick={handleDispatch}
                disabled={!selectedEvent || isDispatching}
                className="w-full py-2 px-4 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow-md disabled:opacity-50 transition-all"
              >
                {isDispatching ? 'Dispatching Event...' : 'Dispatch Event Now'}
              </button>

              {dispatchResult && (
                <div
                  className={`p-3 rounded-lg border text-xs font-mono space-y-1 ${
                    dispatchResult.success
                      ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-200'
                      : 'bg-rose-950/40 border-rose-500/40 text-rose-200'
                  }`}
                >
                  <div className="font-bold flex items-center gap-1.5">
                    {dispatchResult.success ? '✓ Dispatched Successfully' : '✕ Dispatch Failed'}
                  </div>
                  {dispatchResult.error && <div>{dispatchResult.error}</div>}
                  {dispatchResult.output && dispatchResult.output.length > 0 && (
                    <pre className="text-[11px] whitespace-pre-wrap opacity-90 max-h-32 overflow-y-auto">
                      {dispatchResult.output.join('\n')}
                    </pre>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Jobs Subtab */}
      {activeTab === 'jobs' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 space-y-4">
          <div>
            <h4 className="text-xs font-bold text-indigo-400 uppercase tracking-wider">
              Jobs Pro Workbench (Handle Inspection &amp; Execution)
            </h4>
            <p className="text-xs text-slate-400 mt-0.5">
              Inspect queued job handles, configuration parameters, and run jobs immediately or push to the queue.
            </p>
          </div>

          <div className="flex gap-2">
            <input
              type="text"
              value={jobSearch}
              onChange={(e) => setJobSearch(e.target.value)}
              placeholder="e.g. App\Jobs\ProcessPodcast"
              className="flex-1 px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-indigo-500"
            />
            <button
              onClick={() => handleInspectJob(jobSearch)}
              className="px-4 py-2 text-xs font-semibold rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white"
            >
              Inspect Job
            </button>
          </div>

          {jobIntrospection && (
            <div className="space-y-4 pt-2 border-t border-slate-800">
              {/* Job Metadata Badges */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-2 text-xs bg-slate-950 p-3 rounded-lg border border-slate-800">
                <div>
                  <span className="text-slate-500">Queue:</span>{' '}
                  <strong className="text-indigo-400">{jobIntrospection.queue || 'default'}</strong>
                </div>
                <div>
                  <span className="text-slate-500">Connection:</span>{' '}
                  <strong className="text-slate-200">{jobIntrospection.connection || 'default'}</strong>
                </div>
                <div>
                  <span className="text-slate-500">Tries:</span>{' '}
                  <strong className="text-emerald-400">{jobIntrospection.tries || 'Default'}</strong>
                </div>
                <div>
                  <span className="text-slate-500">Timeout:</span>{' '}
                  <strong className="text-purple-400">
                    {jobIntrospection.timeout ? `${jobIntrospection.timeout}s` : 'Default'}
                  </strong>
                </div>
              </div>

              {jobIntrospection.schedule && (
                <div className="p-2.5 rounded-lg bg-indigo-950/40 border border-indigo-500/30 text-xs font-mono text-indigo-300">
                  Scheduled Task Cron: <strong>{jobIntrospection.schedule}</strong>
                </div>
              )}

              {/* Job Parameters Form */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-slate-300">Job Constructor Arguments:</span>
                  <div className="flex rounded-lg bg-slate-950 p-0.5 border border-slate-800">
                    <button
                      onClick={() => setJobPayloadMode('form')}
                      className={`px-2 py-0.5 text-[10px] font-medium rounded ${
                        jobPayloadMode === 'form' ? 'bg-indigo-600 text-white' : 'text-slate-400'
                      }`}
                    >
                      Form
                    </button>
                    <button
                      onClick={() => setJobPayloadMode('raw')}
                      className={`px-2 py-0.5 text-[10px] font-medium rounded ${
                        jobPayloadMode === 'raw' ? 'bg-indigo-600 text-white' : 'text-slate-400'
                      }`}
                    >
                      Raw
                    </button>
                  </div>
                </div>

                {jobPayloadMode === 'form' ? (
                  <div className="space-y-2 bg-slate-950 p-3 rounded-lg border border-slate-800">
                    {jobIntrospection.parameters && jobIntrospection.parameters.length > 0 ? (
                      jobIntrospection.parameters.map((p) => (
                        <div key={p.name} className="flex items-center justify-between gap-3 text-xs">
                          <label className="font-mono text-slate-300 w-1/3 truncate">
                            ${p.name}
                            <span className="text-[10px] text-slate-500 ml-1">({p.type})</span>
                          </label>
                          <input
                            type="text"
                            value={jobFormValues[p.name] ?? ''}
                            onChange={(e) =>
                              setJobFormValues({ ...jobFormValues, [p.name]: e.target.value })
                            }
                            placeholder={p.hasDefault ? String(p.defaultValue ?? '') : 'required'}
                            className="w-2/3 px-2 py-1 text-xs font-mono bg-slate-900 border border-slate-700 rounded text-slate-200 focus:outline-none focus:border-indigo-500"
                          />
                        </div>
                      ))
                    ) : (
                      <p className="text-[11px] text-slate-500 italic">No constructor properties required.</p>
                    )}
                  </div>
                ) : (
                  <textarea
                    rows={4}
                    value={rawJobPayload}
                    onChange={(e) => setRawJobPayload(e.target.value)}
                    className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200"
                  />
                )}
              </div>

              {/* Execution Actions */}
              <div className="flex gap-3">
                <button
                  onClick={() => handleRunJob('sync')}
                  disabled={isRunningJob}
                  className="flex-1 py-2 px-4 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow disabled:opacity-50"
                >
                  {isRunningJob ? 'Running...' : 'Run Now (Synchronous Execution)'}
                </button>
                <button
                  onClick={() => handleRunJob('queued')}
                  disabled={isRunningJob}
                  className="flex-1 py-2 px-4 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow disabled:opacity-50"
                >
                  {isRunningJob ? 'Queuing...' : 'Queue It (Dispatch to Worker)'}
                </button>
              </div>

              {/* Job Result */}
              {jobResult && (
                <div
                  className={`p-3 rounded-lg border text-xs font-mono space-y-1.5 ${
                    jobResult.success
                      ? 'bg-emerald-950/40 border-emerald-500/40 text-emerald-200'
                      : 'bg-rose-950/40 border-rose-500/40 text-rose-200'
                  }`}
                >
                  <div className="font-bold flex items-center justify-between">
                    <span>{jobResult.success ? '✓ Job Executed Successfully' : '✕ Job Failed'}</span>
                    <span className="text-[10px] opacity-80">
                      {jobResult.durationMs}ms • {jobResult.queries} queries
                    </span>
                  </div>
                  {jobResult.error && <div>{jobResult.error}</div>}
                  {jobResult.output && jobResult.output.length > 0 && (
                    <pre className="text-[11px] whitespace-pre-wrap opacity-90 max-h-40 overflow-y-auto">
                      {jobResult.output.join('\n')}
                    </pre>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Queue Supervisor Subtab */}
      {activeTab === 'supervisor' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-xl p-5 space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-800 pb-4">
            <div>
              <h4 className="text-xs font-bold text-slate-200 uppercase tracking-wider">
                Local Queue Worker Supervisor
              </h4>
              <p className="text-xs text-slate-400 mt-0.5">
                Manage background <code>artisan queue:listen</code> processes with real-time log output.
              </p>
            </div>

            <div className="flex items-center gap-3">
              <span
                className={`px-3 py-1 rounded-full text-xs font-bold ${
                  workerStatus.running ? 'bg-emerald-500/20 text-emerald-400' : 'bg-slate-800 text-slate-400'
                }`}
              >
                {workerStatus.running ? `Running (PID ${workerStatus.pid})` : 'Stopped'}
              </span>

              {workerStatus.running ? (
                <button
                  onClick={onStopWorker}
                  className="px-3.5 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white font-semibold text-xs shadow"
                >
                  Stop Worker
                </button>
              ) : (
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    value={workerQueue}
                    onChange={(e) => setWorkerQueue(e.target.value)}
                    placeholder="Queue name"
                    className="px-2.5 py-1 text-xs bg-slate-950 border border-slate-700 rounded-lg text-slate-200 w-28"
                  />
                  <button
                    onClick={() => onStartWorker(workerQueue)}
                    className="px-3.5 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white font-semibold text-xs shadow"
                  >
                    Start Worker
                  </button>
                </div>
              )}
            </div>
          </div>

          <div className="space-y-2">
            <h5 className="text-xs font-bold text-slate-400 uppercase tracking-wider">Worker Log Stream</h5>
            <div className="bg-slate-950 border border-slate-800 rounded-xl p-4 font-mono text-xs text-slate-300 max-h-72 overflow-y-auto space-y-1">
              {workerStatus.output && workerStatus.output.length > 0 ? (
                workerStatus.output.map((line, idx) => <div key={idx}>{line}</div>)
              ) : (
                <div className="text-slate-600 italic">No output logged yet.</div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Failed Jobs Subtab */}
      {activeTab === 'failed' && (
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-3 bg-slate-900/60 p-3 rounded-xl border border-slate-800">
            <input
              type="text"
              placeholder="Filter by job class or exception message..."
              value={searchFailed}
              onChange={(e) => setSearchFailed(e.target.value)}
              className="px-3 py-1.5 text-xs bg-slate-950 border border-slate-700 rounded-lg text-slate-200 placeholder-slate-500 w-72"
            />

            <button
              onClick={onRetryAllJobs}
              disabled={failedJobs.length === 0}
              className="px-3.5 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-xs shadow disabled:opacity-50"
            >
              Retry All Failed Jobs
            </button>
          </div>

          {filteredFailed.length === 0 ? (
            <div className="text-center py-16 bg-slate-900/40 rounded-xl border border-slate-800">
              <h4 className="text-sm font-semibold text-slate-300">No Failed Jobs</h4>
              <p className="text-xs text-slate-500 mt-1">All queued jobs completed cleanly.</p>
            </div>
          ) : (
            <div className="space-y-3">
              {filteredFailed.map((job) => (
                <div
                  key={job.id}
                  className="bg-slate-900/70 border border-slate-800 rounded-xl p-4 space-y-2"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
                    <span className="font-mono font-bold text-rose-400">{job.class}</span>
                    <div className="flex items-center gap-3">
                      <span className="text-slate-500 text-[11px]">{job.failedAt}</span>
                      <button
                        onClick={() => onRetryJob(job.id)}
                        className="px-2.5 py-1 rounded bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold"
                      >
                        Retry Job
                      </button>
                    </div>
                  </div>

                  <div className="text-xs text-slate-400 flex gap-4">
                    <span>Queue: <strong>{job.queue}</strong></span>
                    <span>Connection: <strong>{job.connection}</strong></span>
                    <span>ID: <code className="font-mono">{job.id}</code></span>
                  </div>

                  <pre className="text-xs font-mono text-rose-300/90 bg-slate-950 p-2.5 rounded-lg overflow-x-auto max-h-36 whitespace-pre-wrap">
                    {job.exception}
                  </pre>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
