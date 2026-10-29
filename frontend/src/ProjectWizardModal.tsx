import React, { useState } from 'react';
import { wizard } from '../wailsjs/go/models';

export type WizardSpec = wizard.WizardSpec;
export type WizardResult = wizard.WizardResult;
export type WizardStepResult = wizard.WizardStepResult;

interface ProjectWizardModalProps {
  isOpen: boolean;
  onClose: () => void;
  defaultDirectory: string;
  onSuccess: (projectPath: string) => void;
  onCreateProject: (spec: WizardSpec) => Promise<WizardResult>;
}

export const ProjectWizardModal: React.FC<ProjectWizardModalProps> = ({
  isOpen,
  onClose,
  defaultDirectory,
  onSuccess,
  onCreateProject,
}) => {
  const [step, setStep] = useState<'form' | 'executing' | 'done'>('form');
  const [name, setName] = useState('');
  const [directory, setDirectory] = useState(defaultDirectory || 'C:\\Users\\hassa\\Herd');
  const [starter, setStarter] = useState('laravel');
  const [databaseEngine, setDatabaseEngine] = useState('mysql');
  const [databaseName, setDatabaseName] = useState('');
  const [selectedPackages, setSelectedPackages] = useState<string[]>(['pest']);
  const [gitInit, setGitInit] = useState(true);
  const [autoSyncEnvServices, setAutoSyncEnvServices] = useState(true);
  const [linkWithHerd, setLinkWithHerd] = useState(true);
  const [isExecuting, setIsExecuting] = useState(false);
  const [executionResult, setExecutionResult] = useState<WizardResult | null>(null);

  if (!isOpen) return null;

  const togglePackage = (pkg: string) => {
    if (selectedPackages.includes(pkg)) {
      setSelectedPackages(selectedPackages.filter((p) => p !== pkg));
    } else {
      setSelectedPackages([...selectedPackages, pkg]);
    }
  };

  const handleStartCreation = async () => {
    if (!name.trim() || !directory.trim()) return;
    setStep('executing');
    setIsExecuting(true);

    try {
      const res = await onCreateProject({
        directory,
        name: name.trim(),
        starter,
        databaseEngine,
        databaseName: databaseName || name.trim().replace(/-/g, '_'),
        phpBinary: "",
        selectedPackages,
        gitInit,
        autoSyncEnvServices,
        linkWithHerd,
      });
      setExecutionResult(res);
      setStep('done');
      if (res.success) {
        onSuccess(res.projectPath);
      }
    } catch (err: any) {
      setExecutionResult(
        wizard.WizardResult.createFrom({
          projectPath: '',
          success: false,
          steps: [{ step: 'Initialization', success: false, error: err.message }],
          durationMs: 0,
        })
      );
      setStep('done');
    } finally {
      setIsExecuting(false);
    }
  };

  const startersList = [
    { id: 'laravel', title: 'Laravel Standard', desc: 'Latest Laravel 12 skeleton with Blade & Tailwind' },
    { id: 'livewire', title: 'Livewire 3 + Breeze', desc: 'Full-stack reactive components with Volt & Alpine.js' },
    { id: 'react', title: 'Inertia.js + React', desc: 'Modern SPA with React, TypeScript & SSR support' },
    { id: 'vue', title: 'Inertia.js + Vue 3', desc: 'Single-page app with Vue 3 Composition API & TypeScript' },
    { id: 'filament', title: 'Filament Admin Panel', desc: 'Pre-installed Filament v3 dashboard & resource builders' },
    { id: 'api', title: 'Laravel API', desc: 'Headless REST API with Sanctum token authentication' },
  ];

  const packagesList = [
    { id: 'pest', label: 'Pest 3 (Testing)' },
    { id: 'telescope', label: 'Laravel Telescope (Diagnostics)' },
    { id: 'pulse', label: 'Laravel Pulse (Performance)' },
    { id: 'horizon', label: 'Laravel Horizon (Redis Queues)' },
    { id: 'pennant', label: 'Laravel Pennant (Feature Flags)' },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="bg-slate-900 border border-slate-700 rounded-2xl w-full max-w-3xl overflow-hidden shadow-2xl flex flex-col max-h-[90vh]">
        {/* Modal Header */}
        <div className="flex items-center justify-between p-4 border-b border-slate-800 bg-slate-950/60">
          <div>
            <h3 className="text-base font-bold text-slate-100 flex items-center gap-2">
              <span>🪄</span> New Laravel Project Wizard
            </h3>
            <p className="text-xs text-slate-400 mt-0.5">
              Scaffold, provision, configure, and register a fresh Laravel project in seconds.
            </p>
          </div>
          <button
            onClick={onClose}
            disabled={isExecuting}
            className="p-1 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 disabled:opacity-50"
          >
            ✕
          </button>
        </div>

        {/* Modal Body */}
        <div className="p-5 overflow-y-auto space-y-5 flex-1">
          {step === 'form' && (
            <>
              {/* Project Destination */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">Project Name</label>
                  <input
                    type="text"
                    placeholder="my-saas-app"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-indigo-500"
                  />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-slate-300 mb-1">Destination Directory</label>
                  <input
                    type="text"
                    value={directory}
                    onChange={(e) => setDirectory(e.target.value)}
                    className="w-full px-3 py-2 text-xs font-mono bg-slate-950 border border-slate-700 rounded-lg text-slate-200 focus:outline-none focus:border-indigo-500"
                  />
                </div>
              </div>

              {/* Starter Flavor */}
              <div>
                <label className="block text-xs font-bold text-slate-300 uppercase tracking-wider mb-2">
                  Select Starter Kit
                </label>
                <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2.5">
                  {startersList.map((s) => (
                    <div
                      key={s.id}
                      onClick={() => setStarter(s.id)}
                      className={`p-3 rounded-xl border cursor-pointer transition-all ${
                        starter === s.id
                          ? 'bg-indigo-950/40 border-indigo-500 shadow-md ring-1 ring-indigo-500'
                          : 'bg-slate-950/60 border-slate-800 hover:border-slate-700'
                      }`}
                    >
                      <div className="text-xs font-bold text-slate-200">{s.title}</div>
                      <div className="text-[11px] text-slate-400 mt-1 leading-snug">{s.desc}</div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Database Provisioning */}
              <div className="bg-slate-950/60 p-4 rounded-xl border border-slate-800 space-y-3">
                <label className="block text-xs font-bold text-slate-300 uppercase tracking-wider">
                  Database Auto-Provisioning
                </label>
                <div className="flex gap-4 text-xs text-slate-300">
                  {['mysql', 'postgres', 'sqlite'].map((engine) => (
                    <label key={engine} className="flex items-center gap-1.5 cursor-pointer">
                      <input
                        type="radio"
                        name="dbEngine"
                        value={engine}
                        checked={databaseEngine === engine}
                        onChange={() => setDatabaseEngine(engine)}
                        className="text-indigo-600 focus:ring-indigo-500"
                      />
                      <span className="capitalize font-semibold">{engine}</span>
                    </label>
                  ))}
                </div>
                {databaseEngine !== 'sqlite' && (
                  <input
                    type="text"
                    placeholder={`Database name (default: ${name ? name.replace(/-/g, '_') : 'my_project'})`}
                    value={databaseName}
                    onChange={(e) => setDatabaseName(e.target.value)}
                    className="w-full px-3 py-1.5 text-xs font-mono bg-slate-900 border border-slate-700 rounded-lg text-slate-200"
                  />
                )}
              </div>

              {/* Optional Package Bundles */}
              <div>
                <label className="block text-xs font-bold text-slate-300 uppercase tracking-wider mb-2">
                  Curated Package Bundles
                </label>
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
                  {packagesList.map((pkg) => (
                    <label
                      key={pkg.id}
                      className={`flex items-center gap-2 p-2 rounded-lg border cursor-pointer text-xs transition-all ${
                        selectedPackages.includes(pkg.id)
                          ? 'bg-indigo-950/30 border-indigo-500/60 text-slate-200 font-medium'
                          : 'bg-slate-950/40 border-slate-800 text-slate-400'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={selectedPackages.includes(pkg.id)}
                        onChange={() => togglePackage(pkg.id)}
                        className="text-indigo-600 rounded"
                      />
                      <span>{pkg.label}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Toggles */}
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 pt-2 border-t border-slate-800 text-xs text-slate-300">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={gitInit}
                    onChange={(e) => setGitInit(e.target.checked)}
                    className="text-indigo-600 rounded"
                  />
                  <span>Initialize Git Repository</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={autoSyncEnvServices}
                    onChange={(e) => setAutoSyncEnvServices(e.target.checked)}
                    className="text-indigo-600 rounded"
                  />
                  <span>Sync Services to .env</span>
                </label>
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={linkWithHerd}
                    onChange={(e) => setLinkWithHerd(e.target.checked)}
                    className="text-indigo-600 rounded"
                  />
                  <span>Register with Herd</span>
                </label>
              </div>
            </>
          )}

          {step === 'executing' && (
            <div className="py-16 text-center space-y-3">
              <div className="animate-spin inline-block w-10 h-10 border-4 border-indigo-500 border-t-transparent rounded-full"></div>
              <h4 className="text-base font-bold text-slate-100">Building Your Project...</h4>
              <p className="text-xs text-slate-400 max-w-md mx-auto">
                Running Composer create-project, provisioning database, configuring .env credentials, and running initial migrations.
              </p>
            </div>
          )}

          {step === 'done' && executionResult && (
            <div className="space-y-4">
              <div
                className={`p-4 rounded-xl border ${
                  executionResult.success
                    ? 'bg-emerald-950/30 border-emerald-500/40 text-emerald-300'
                    : 'bg-rose-950/30 border-rose-500/40 text-rose-300'
                }`}
              >
                <div className="text-sm font-bold flex items-center gap-2">
                  <span>{executionResult.success ? '🎉 Project Created Successfully!' : '✕ Creation Encountered Issues'}</span>
                </div>
                <div className="text-xs text-slate-400 mt-1">
                  Completed in {(executionResult.durationMs / 1000).toFixed(1)} seconds at {executionResult.projectPath}
                </div>
              </div>

              {/* Step Execution Log */}
              <div className="space-y-2">
                <h5 className="text-xs font-bold text-slate-400 uppercase">Execution Steps</h5>
                {executionResult.steps.map((s, idx) => (
                  <div
                    key={idx}
                    className="flex items-start gap-2.5 bg-slate-950 p-2.5 rounded-lg border border-slate-800 text-xs"
                  >
                    <span className={s.success ? 'text-emerald-400' : 'text-rose-400'}>
                      {s.success ? '✓' : '✕'}
                    </span>
                    <div className="flex-1">
                      <div className="font-semibold text-slate-200">{s.step}</div>
                      {s.output && <div className="text-[11px] text-slate-400 mt-0.5">{s.output}</div>}
                      {s.error && <div className="text-[11px] text-rose-400 mt-0.5 font-mono">{s.error}</div>}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="p-4 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
          <div className="text-xs text-slate-500 font-mono">
            {name ? `http://${name}.test` : 'Zero configuration'}
          </div>

          <div className="flex items-center gap-2">
            {step === 'form' && (
              <>
                <button
                  onClick={onClose}
                  className="px-4 py-2 text-xs font-medium rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
                >
                  Cancel
                </button>
                <button
                  onClick={handleStartCreation}
                  disabled={!name.trim()}
                  className="px-5 py-2 text-xs font-bold rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg disabled:opacity-50"
                >
                  🚀 Create Project
                </button>
              </>
            )}
            {step === 'done' && (
              <button
                onClick={onClose}
                className="px-5 py-2 text-xs font-bold rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white shadow"
              >
                Done
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
