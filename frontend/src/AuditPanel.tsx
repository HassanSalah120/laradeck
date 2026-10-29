import { useEffect, useState, useMemo } from 'react';
import {
  AuditFullStackBestPractices,
  CancelAudit,
  OpenAuditBrowser,
  CloseAuditBrowser,
  AuditBrowserPage,
  PreviewAuditSetup,
  ApplyAuditSetup,
  GetAuditRules,
  AuditProject,
  RunPint,
} from '../wailsjs/go/main/App';
import { quality, workbench } from '../wailsjs/go/models';

export type AuditPanelProps = {
  project?: workbench.Project;
  trusted: boolean;
  report?: quality.AuditReport;
  browser?: quality.BrowserSessionStatus;
  reload: () => Promise<void>;
  auditItems?: quality.ConfigAuditItem[];
  pintReport?: quality.QualityReport | null;
  onRunPint?: () => Promise<void> | void;
  onRefreshConfigAudit?: () => Promise<void> | void;
};

type SubTab = 'static' | 'config' | 'pint' | 'browser' | 'rules' | 'setup';

const KIND_LABELS: Record<string, string> = {
  correctness: 'Defects & Security',
  security: 'Defects & Security',
  performance: 'Performance',
  convention: 'Optional Conventions',
};

export default function AuditPanel({
  project,
  trusted,
  report: propReport,
  browser,
  reload,
  auditItems: propAuditItems,
  pintReport: propPintReport,
  onRunPint,
  onRefreshConfigAudit,
}: AuditPanelProps) {
  // Navigation & Sub-Tabs
  const [subTab, setSubTab] = useState<SubTab>('static');

  // Audit Options
  const [options, setOptions] = useState({
    includeConventions: false,
    includePerformanceAdvice: false,
    gitHistory: false,
    gitleaksBaseline: '',
    semgrepConfig: '',
    semgrepWsl: '',
  });
  const [showSettings, setShowSettings] = useState(false);

  // Execution & Diagnostics State
  const [pending, setPending] = useState('');
  const [error, setError] = useState('');
  const [copiedType, setCopiedType] = useState<string | null>(null);
  const [copiedFindingIdx, setCopiedFindingIdx] = useState<number | null>(null);
  const [copiedFixId, setCopiedFixId] = useState<string | null>(null);
  const [copiedGroupPath, setCopiedGroupPath] = useState<string | null>(null);

  // Grouping & Inspection State
  const [groupBy, setGroupBy] = useState<'file' | 'rule' | 'flat'>('file');
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [expandedFindings, setExpandedFindings] = useState<Record<string, boolean>>({});
  const [showEnginesDetail, setShowEnginesDetail] = useState(false);

  // Backend snapshots own both completed reports and live progress.
  const activeReport = propReport;

  // Filter & Search
  const [filterGroup, setFilterGroup] = useState<string>('All');
  const [searchQuery, setSearchQuery] = useState('');
  const [expandedCoverage, setExpandedCoverage] = useState(false);

  // False Positive / Dismissal State
  const [falsePositives, setFalsePositives] = useState<Record<string, { citation: string; markedAt: string }>>(() => {
    try {
      const saved = localStorage.getItem(`laradeck_audit_fp_${project?.id || 'default'}`);
      return saved ? JSON.parse(saved) : {};
    } catch {
      return {};
    }
  });

  const [activeFpModalItem, setActiveFpModalItem] = useState<{
    id: string;
    key: string;
    rule: string;
    file: string;
    line: number;
    defaultCitation?: string;
  } | null>(null);
  const [fpCitationInput, setFpCitationInput] = useState('');

  useEffect(() => {
    try {
      const saved = localStorage.getItem(`laradeck_audit_fp_${project?.id || 'default'}`);
      setFalsePositives(saved ? JSON.parse(saved) : {});
    } catch {
      setFalsePositives({});
    }
  }, [project?.id]);

  const markAsFalsePositive = (key: string, citation: string) => {
    const updated = {
      ...falsePositives,
      [key]: {
        citation: citation.trim() || 'Reviewed false positive',
        markedAt: new Date().toISOString(),
      },
    };
    setFalsePositives(updated);
    try {
      localStorage.setItem(`laradeck_audit_fp_${project?.id || 'default'}`, JSON.stringify(updated));
    } catch {
      // ignore
    }
    setActiveFpModalItem(null);
  };

  const unmarkFalsePositive = (key: string) => {
    const updated = { ...falsePositives };
    delete updated[key];
    setFalsePositives(updated);
    try {
      localStorage.setItem(`laradeck_audit_fp_${project?.id || 'default'}`, JSON.stringify(updated));
    } catch {
      // ignore
    }
  };

  // Config Audit & Pint state (fallback if not supplied via props)
  const [localConfigItems, setLocalConfigItems] = useState<quality.ConfigAuditItem[]>([]);
  const [localPintReport, setLocalPintReport] = useState<quality.QualityReport | null>(null);
  const [pintRunning, setPintRunning] = useState(false);
  const [configRunning, setConfigRunning] = useState(false);

  // Browser audit state
  const [browserUrl, setBrowserUrl] = useState('');

  // Rules Catalog
  const [catalog, setCatalog] = useState<quality.RulePolicy[]>([]);
  const [catalogSearch, setCatalogSearch] = useState('');

  useEffect(() => {
    setBrowserUrl(project?.url || 'http://localhost');
    setError('');
  }, [project?.id, project?.url]);

  useEffect(() => {
    GetAuditRules().then(setCatalog).catch(() => {});
  }, []);

  const configItems = propAuditItems && propAuditItems.length > 0 ? propAuditItems : localConfigItems;
  const pintReport = propPintReport || localPintReport;

  // Run async action with loader & error handling
  const runAction = async (label: string, action: () => Promise<unknown>) => {
    setPending(label);
    setError('');
    try {
      await action();
    } catch (e) {
      setError(String(e));
    } finally {
      setPending('');
      await reload();
    }
  };

  const isAuditRunning = !!pending || !!activeReport?.running;

  // Compute counts
  const allFindings = useMemo(() => activeReport?.findings || [], [activeReport]);

  useEffect(() => {
    if (!project?.id || !allFindings.length) return;
    const storageKey=`laradeck_audit_fp_${project.id}`;
    let saved: Record<string, {citation:string;markedAt:string}>;
    try { saved=JSON.parse(localStorage.getItem(storageKey)||'{}'); } catch { return; }
    let changed=false;
    for (const f of allFindings) {
      const legacy=`${f.file}:${f.line}:${f.rule}`;
      if (f.fingerprint && saved[legacy]) { if (!saved[f.fingerprint]) saved[f.fingerprint]=saved[legacy]; delete saved[legacy]; changed=true; }
    }
    if (changed) { localStorage.setItem(storageKey,JSON.stringify(saved)); setFalsePositives(saved); }
  }, [allFindings,project?.id]);

  const fpCount = useMemo(() => {
    return allFindings.filter((f) => {
      const key = (f.fingerprint || `${f.file}:${f.line}:${f.rule}`);
      return !!falsePositives[key];
    }).length;
  }, [allFindings, falsePositives]);

  const activeFindings = useMemo(
    () => allFindings.filter((f) => !falsePositives[(f.fingerprint || `${f.file}:${f.line}:${f.rule}`)]),
    [allFindings, falsePositives]
  );

  const errorCount = useMemo(
    () => activeFindings.filter((f) => f.severity === 'error' || f.kind === 'correctness' || f.kind === 'security').length,
    [activeFindings]
  );
  const warnCount = useMemo(
    () => activeFindings.filter((f) => f.severity === 'warning' || f.kind === 'performance').length,
    [activeFindings]
  );
  const convCount = useMemo(
    () => activeFindings.filter((f) => f.kind === 'convention' || f.severity === 'info').length,
    [activeFindings]
  );

  // Filter findings by active group and search query
  const filteredFindings = useMemo(() => {
    return allFindings.filter((f) => {
      const key = (f.fingerprint || `${f.file}:${f.line}:${f.rule}`);
      const isFp = !!falsePositives[key];

      // If user is explicitly viewing False Positives:
      if (filterGroup === 'False Positives') {
        if (!isFp) return false;
      } else {
        // By default in other filters, hide findings marked as false positive!
        if (isFp) return false;

        if (filterGroup === 'Defects & Security') {
          if (f.severity !== 'error' && f.kind !== 'correctness' && f.kind !== 'security') return false;
        } else if (filterGroup === 'Performance') {
          if (f.kind !== 'performance') return false;
        } else if (filterGroup === 'Optional Conventions') {
          if (f.kind !== 'convention') return false;
        }
      }

      // Search query filter
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        const inRule = (f.rule || '').toLowerCase().includes(q);
        const inFile = (f.file || '').toLowerCase().includes(q);
        const inMsg = (f.message || '').toLowerCase().includes(q);
        const inTool = (f.tool || '').toLowerCase().includes(q);
        const inCitation = (falsePositives[key]?.citation || '').toLowerCase().includes(q);
        if (!inRule && !inFile && !inMsg && !inTool && !inCitation) return false;
      }

      return true;
    });
  }, [allFindings, filterGroup, searchQuery, falsePositives]);

  const completedAnalyzers = useMemo(
    () => (activeReport?.analyzers || []).filter((a) => a.status === 'completed'),
    [activeReport]
  );
  const failedAnalyzers = useMemo(
    () => (activeReport?.analyzers || []).filter((a) => a.status === 'failed'),
    [activeReport]
  );
  const skippedAnalyzers = useMemo(
    () => (activeReport?.analyzers || []).filter((a) => a.status !== 'completed' && a.status !== 'failed'),
    [activeReport]
  );

  // Copy report to clipboard
  const handleExport = async (format: 'markdown' | 'json') => {
    if (!activeReport) return;
    let text = '';
    if (format === 'json') {
      text = JSON.stringify(activeReport, null, 2);
    } else {
      text = [
        `# LaraDeck Quality & Security Audit: ${project?.name || 'Project'}`,
        `Generated: ${new Date().toISOString()}`,
        `Completed analyzers: ${completedAnalyzers.length}; Incomplete/Skipped: ${skippedAnalyzers.length + failedAnalyzers.length}`,
        '',
        '## Analyzers Summary',
        ...activeReport.analyzers.map(
          (a) => `- **${a.name}**: ${a.status.toUpperCase()} (${a.durationSeconds?.toFixed(1) || 0}s) · ${a.coverage}`
        ),
        '',
        `## Findings (${activeReport.findings.length})`,
        ...activeReport.findings.map(
          (f) =>
            `\n### [${f.severity.toUpperCase()}] ${f.rule}\n` +
            `* **Location:** \`${f.file || 'Rendered page'}:${f.line || ''}\`\n` +
            `* **Tool:** ${f.tools?.join(', ') || f.tool} | **Category:** ${KIND_LABELS[f.kind] || f.kind}\n` +
            `* **Message:** ${f.message}\n` +
            (f.fixGuidance ? `* **Recommended Fix:** ${f.fixGuidance}\n` : '') +
            (f.githubRef ? `* **Documentation:** ${f.githubRef}\n` : '')
        ),
      ].join('\n');
    }

    try {
      await navigator.clipboard.writeText(text);
      setCopiedType(format);
      setTimeout(() => setCopiedType(null), 2500);
    } catch {
      setError('Unable to copy to clipboard.');
    }
  };

  const copyPath = async (filePath: string, line: number, idx: number) => {
    try {
      await navigator.clipboard.writeText(line ? `${filePath}:${line}` : filePath);
      setCopiedFindingIdx(idx);
      setTimeout(() => setCopiedFindingIdx(null), 2000);
    } catch {
      // ignore
    }
  };

  const copyFix = async (fixText: string, id: string) => {
    try {
      await navigator.clipboard.writeText(fixText);
      setCopiedFixId(id);
      setTimeout(() => setCopiedFixId(null), 2000);
    } catch {
      // ignore
    }
  };

  const copyGroupPath = async (filePath: string) => {
    try {
      await navigator.clipboard.writeText(filePath);
      setCopiedGroupPath(filePath);
      setTimeout(() => setCopiedGroupPath(null), 2000);
    } catch {
      // ignore
    }
  };

  interface FindingItem {
    finding: quality.CodeBestPracticeFinding;
    originalIndex: number;
    id: string;
  }

  interface FileGroup {
    file: string;
    dir: string;
    name: string;
    items: FindingItem[];
    errorCount: number;
    warnCount: number;
    convCount: number;
  }

  const fileGroups = useMemo<FileGroup[]>(() => {
    const map = new Map<string, FileGroup>();

    filteredFindings.forEach((f, idx) => {
      const file = f.file || 'Rendered DOM / Page';
      if (!map.has(file)) {
        const lastSlash = file.lastIndexOf('/');
        const dir = lastSlash >= 0 ? file.slice(0, lastSlash + 1) : '';
        const name = lastSlash >= 0 ? file.slice(lastSlash + 1) : file;

        map.set(file, {
          file,
          dir,
          name,
          items: [],
          errorCount: 0,
          warnCount: 0,
          convCount: 0,
        });
      }

      const group = map.get(file)!;
      const id = `${f.file}:${f.line}:${f.rule}:${idx}`;
      group.items.push({ finding: f, originalIndex: idx, id });

      if (f.severity === 'error' || f.kind === 'correctness' || f.kind === 'security') {
        group.errorCount++;
      } else if (f.severity === 'warning' || f.kind === 'performance') {
        group.warnCount++;
      } else {
        group.convCount++;
      }
    });

    return Array.from(map.values()).sort((a, b) => {
      if (a.errorCount !== b.errorCount) return b.errorCount - a.errorCount;
      if (a.items.length !== b.items.length) return b.items.length - a.items.length;
      return a.file.localeCompare(b.file);
    });
  }, [filteredFindings]);

  interface RuleGroup {
    rule: string;
    category: string;
    severity: string;
    items: FindingItem[];
    errorCount: number;
    warnCount: number;
    convCount: number;
  }

  const ruleGroups = useMemo<RuleGroup[]>(() => {
    const map = new Map<string, RuleGroup>();

    filteredFindings.forEach((f, idx) => {
      const rule = f.rule || 'Unknown Rule';
      if (!map.has(rule)) {
        map.set(rule, {
          rule,
          category: KIND_LABELS[f.kind] || f.kind,
          severity: f.severity,
          items: [],
          errorCount: 0,
          warnCount: 0,
          convCount: 0,
        });
      }

      const group = map.get(rule)!;
      const id = `${f.file}:${f.line}:${f.rule}:${idx}`;
      group.items.push({ finding: f, originalIndex: idx, id });

      if (f.severity === 'error' || f.kind === 'correctness' || f.kind === 'security') {
        group.errorCount++;
      } else if (f.severity === 'warning' || f.kind === 'performance') {
        group.warnCount++;
      } else {
        group.convCount++;
      }
    });

    return Array.from(map.values()).sort((a, b) => {
      if (a.errorCount !== b.errorCount) return b.errorCount - a.errorCount;
      return b.items.length - a.items.length;
    });
  }, [filteredFindings]);

  const toggleGroup = (key: string) => {
    setCollapsedGroups((prev) => ({
      ...prev,
      [key]: !prev[key],
    }));
  };

  const toggleFindingDetail = (id: string) => {
    setExpandedFindings((prev) => ({
      ...prev,
      [id]: !prev[id],
    }));
  };

  const toggleAllDetails = () => {
    const allExpanded = filteredFindings.length > 0 && filteredFindings.every((f, idx) => {
      const id = `${f.file}:${f.line}:${f.rule}:${idx}`;
      return !!expandedFindings[id];
    });

    const next: Record<string, boolean> = {};
    filteredFindings.forEach((f, idx) => {
      const id = `${f.file}:${f.line}:${f.rule}:${idx}`;
      next[id] = !allExpanded;
    });
    setExpandedFindings(next);
  };

  const handleRunConfigAudit = async () => {
    if (!project) return;
    if (onRefreshConfigAudit) {
      await onRefreshConfigAudit();
      return;
    }
    setConfigRunning(true);
    try {
      const items = await AuditProject(project.id);
      setLocalConfigItems(Array.isArray(items) ? items : []);
    } catch (e) {
      setError(String(e));
    } finally {
      setConfigRunning(false);
    }
  };

  const handleRunPint = async () => {
    if (!project) return;
    if (onRunPint) {
      await onRunPint();
      return;
    }
    setPintRunning(true);
    try {
      const rep = await RunPint(project.id, true);
      setLocalPintReport(rep);
    } catch (e) {
      setError(String(e));
    } finally {
      setPintRunning(false);
    }
  };

  // Has active custom settings
  const hasCustomSettings =
    options.includeConventions ||
    options.includePerformanceAdvice ||
    options.gitHistory ||
    !!options.gitleaksBaseline ||
    !!options.semgrepConfig ||
    !!options.semgrepWsl;

  const renderFindingRow = (item: FindingItem) => {
    const f = item.finding;
    const isExpanded = !!expandedFindings[item.id];
    const findingKey = (f.fingerprint || `${f.file}:${f.line}:${f.rule}`);
    const isFp = !!falsePositives[findingKey];
    const isError = !isFp && (f.severity === 'error' || f.kind === 'correctness' || f.kind === 'security');
    const isWarn = !isFp && (f.severity === 'warning' || f.kind === 'performance');
    const severityClass = isFp ? 'finding-fp' : isError ? 'finding-error' : isWarn ? 'finding-warning' : 'finding-info';

    return (
      <div key={item.id} className="finding-row-wrapper">
        <div
          className={`finding-compact-row ${severityClass} ${isExpanded ? 'row-expanded' : ''}`}
          onClick={() => toggleFindingDetail(item.id)}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              toggleFindingDetail(item.id);
            }
          }}
        >
          <div className="row-left">
            <span className={`finding-severity-badge ${severityClass}`}>
              {isFp ? 'FP' : f.severity === 'error' ? 'ERR' : f.severity === 'warning' ? 'WARN' : 'INFO'}
            </span>

            {isFp && (
              <span className="count-badge badge-fp" title="Marked as false positive">
                🛡️ FP
              </span>
            )}

            {groupBy !== 'file' && f.file && (
              <code className="row-file" title={f.file}>
                {f.file.split('/').slice(-2).join('/')}
                {f.line > 0 ? `:${f.line}` : ''}
              </code>
            )}

            {groupBy === 'file' && f.line > 0 && (
              <span className="row-line" title={`Line ${f.line}`}>
                :{f.line}
              </span>
            )}

            <code className="row-rule">{f.rule}</code>
            <span className="row-tool-tag">{f.tools?.join(', ') || f.tool}</span>
            <span className="row-message-preview" title={f.message}>
              {f.message}
            </span>
          </div>

          <div className="row-right">
            <span className="row-category-pill">{KIND_LABELS[f.kind] || f.kind}</span>
            <button
              className="btn-row-expand"
              onClick={(e) => {
                e.stopPropagation();
                toggleFindingDetail(item.id);
              }}
              aria-label={isExpanded ? 'Collapse details' : 'Expand details'}
            >
              {isExpanded ? '▲' : '▼'}
            </button>
          </div>
        </div>

        {isExpanded && (
          <div className="finding-expanded-detail">
            <div className="detail-meta-bar">
              <div className="detail-path-group">
                <span className="location-icon">📁</span>
                <code className="detail-path">
                  {f.file ? `${f.file}:${f.line}` : 'Rendered DOM / Page'}
                </code>
                {f.file && (
                  <button
                    className="btn-copy-path"
                    onClick={(e) => {
                      e.stopPropagation();
                      copyPath(f.file, f.line, item.originalIndex);
                    }}
                    title="Copy file path and line number"
                  >
                    {copiedFindingIdx === item.originalIndex ? '✓ Copied' : '📋 Copy Path'}
                  </button>
                )}
              </div>

              <div className="detail-source-group">
                <span className="detail-engine-tag">Engine: {f.tools?.join(', ') || f.tool}</span>
                <span className="detail-certainty-tag">
                  {f.certainty === 'detected' ? '✓ Analyzer Detected' : '🔍 Needs Review'}
                </span>
              </div>
            </div>

            <div className="detail-message-box">
              <p>{f.message}</p>
            </div>

            {f.snippet && (
              <div className="finding-snippet-block">
                <div className="snippet-header">Code Context</div>
                <pre><code>{f.snippet}</code></pre>
              </div>
            )}

            {f.fixGuidance && (
              <div className="finding-guidance-box">
                <div className="guidance-header">
                  <div className="guidance-title">
                    <span className="guidance-icon">💡</span>
                    <strong>Recommended Fix</strong>
                  </div>
                  <button
                    className="btn-copy-fix"
                    onClick={(e) => {
                      e.stopPropagation();
                      copyFix(f.fixGuidance!, item.id);
                    }}
                    title="Copy recommended remediation to clipboard"
                  >
                    {copiedFixId === item.id ? '✓ Copied Fix' : '📋 Copy Fix'}
                  </button>
                </div>
                <p className="guidance-text">{f.fixGuidance}</p>
              </div>
            )}

            <div className="finding-footer-row">
              {isFp ? (
                <div className="finding-fp-status">
                  <span className="fp-badge">🛡️ Marked False Positive</span>
                  <span className="fp-citation-text">
                    Citing: <code>{falsePositives[findingKey]?.citation || 'Reviewed'}</code>
                  </span>
                  <button
                    type="button"
                    className="btn-tiny secondary btn-restore-fp"
                    onClick={(e) => {
                      e.stopPropagation();
                      unmarkFalsePositive(findingKey);
                    }}
                    title="Restore finding to active audit issues"
                  >
                    ↺ Restore
                  </button>
                </div>
              ) : (
                <button
                  type="button"
                  className="btn-mark-fp"
                  onClick={(e) => {
                    e.stopPropagation();
                    const defaultCit =
                      f.rule === 'MultiWriteTransactionRequired'
                        ? 'app/Http/Middleware/RecordAdminActivity.php:21'
                        : '';
                    setFpCitationInput(defaultCit);
                    setActiveFpModalItem({
                      id: item.id,
                      key: findingKey,
                      rule: f.rule,
                      file: f.file,
                      line: f.line,
                      defaultCitation: defaultCit,
                    });
                  }}
                  title="Mark finding as false positive and record architectural citation"
                >
                  🛡️ Mark False Positive
                </button>
              )}

              {f.githubRef && f.githubRef.startsWith('https://') && (
                <a
                  href={f.githubRef}
                  target="_blank"
                  rel="noreferrer"
                  className="finding-doc-link"
                  onClick={(e) => e.stopPropagation()}
                >
                  <span>📖 Official Rule Documentation</span>
                  <small>↗</small>
                </a>
              )}
            </div>
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="audit-dashboard" aria-label="LaraDeck Quality & Audit">
      {/* Top Hero Control Bar */}
      <div className="audit-hero-card">
        <div className="audit-hero-left">
          <div className="audit-hero-icon">🛡️</div>
          <div>
            <div className="audit-hero-title-row">
              <h2>Code Quality & Security Mission Control</h2>
              {project && <span className="audit-project-badge">{project.name}</span>}
            </div>
            <p className="audit-hero-desc">
              Continuous multi-engine static analysis, Larastan type checking, ESLint rules, and Laravel best practices.
            </p>
          </div>
        </div>

        <div className="audit-hero-actions">
          <button
            className={`btn-audit-primary ${isAuditRunning ? 'running' : ''}`}
            disabled={!project || !trusted || isAuditRunning}
            onClick={() =>
              project &&
              runAction('Running Full-Stack Audit', () =>
                AuditFullStackBestPractices(project.id, options)
              )
            }
            title={!trusted ? 'Trust project to run analyzers' : 'Execute all configured analyzers'}
          >
            {isAuditRunning ? (
              <>
                <span className="spinner-icon">↻</span>
                <span>{pending || 'Analyzing Codebase...'}</span>
              </>
            ) : (
              <>
                <span>▶</span>
                <span>Run Full Audit</span>
              </>
            )}
          </button>

          {isAuditRunning && (
            <button
              className="btn-audit-cancel"
              onClick={() => project && CancelAudit(project.id)}
              title="Stop running analyzer processes"
            >
              ⏹ Cancel
            </button>
          )}

          <button
            className={`btn-audit-settings-toggle ${showSettings ? 'active' : ''} ${hasCustomSettings ? 'has-active' : ''}`}
            onClick={() => setShowSettings(!showSettings)}
            title="Configure analyzer rules, baselines, and optional scans"
          >
            <span>⚙️</span>
            <span>Settings</span>
            {hasCustomSettings && <span className="settings-active-dot" title="Custom scan settings active" />}
          </button>

          {activeReport && (
            <div className="audit-export-group">
              <button
                className="btn-audit-ghost"
                onClick={() => handleExport('markdown')}
                title="Copy markdown audit report to clipboard"
              >
                {copiedType === 'markdown' ? '✓ Report Copied' : '📋 Copy Report'}
              </button>
              <button
                className="btn-audit-ghost"
                onClick={() => handleExport('json')}
                title="Copy raw audit report JSON"
              >
                {copiedType === 'json' ? '✓ JSON Copied' : 'JSON'}
              </button>
            </div>
          )}
        </div>
      </div>

      {/* Untrusted Warning */}
      {!trusted && (
        <div className="audit-untrusted-warning">
          <span className="warning-icon">⚠️</span>
          <div>
            <strong>Session Untrusted for {project?.name || 'this project'}</strong>
            <p>
              To run Larastan, ESLint, Pint, or open the local audit browser, trust this project in the top bar.
            </p>
          </div>
        </div>
      )}

      {/* Error Alert */}
      {error && (
        <div className="audit-error-alert" role="alert">
          <span>❌</span>
          <p>{error}</p>
          <button className="btn-close-error" onClick={() => setError('')}>✕</button>
        </div>
      )}

      {/* Collapsible Scan Settings Drawer */}
      {showSettings && (
        <div className="audit-settings-drawer">
          <div className="audit-settings-drawer-header">
            <h4>
              <span>⚙️</span> Scan Configuration & Engine Options
            </h4>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              {hasCustomSettings && (
                <button
                  className="btn-tiny"
                  onClick={() =>
                    setOptions({
                      includeConventions: false,
                      includePerformanceAdvice: false,
                      gitHistory: false,
                      gitleaksBaseline: '',
                      semgrepConfig: '',
                      semgrepWsl: '',
                    })
                  }
                >
                  Reset Defaults
                </button>
              )}
              <button className="btn-tiny" onClick={() => setShowSettings(false)}>
                ✕ Close
              </button>
            </div>
          </div>

          <div className="audit-settings-grid">
            <div className="audit-settings-col">
              <h5>Analysis Depth & Filters</h5>
              <label className="audit-checkbox-label">
                <input
                  type="checkbox"
                  checked={options.includeConventions}
                  disabled={isAuditRunning}
                  onChange={(e) => setOptions({ ...options, includeConventions: e.target.checked })}
                />
                <div>
                  <strong>Include Optional Conventions</strong>
                  <small>Architectural style, naming conventions, and advisory clean code guidelines.</small>
                </div>
              </label>

              <label className="audit-checkbox-label">
                <input
                  type="checkbox"
                  checked={options.includePerformanceAdvice}
                  disabled={isAuditRunning}
                  onChange={(e) => setOptions({ ...options, includePerformanceAdvice: e.target.checked })}
                />
                <div>
                  <strong>Include Performance Advice</strong>
                  <small>Query performance, N+1 patterns, and cache recommendations.</small>
                </div>
              </label>

              <label className="audit-checkbox-label">
                <input
                  type="checkbox"
                  checked={options.gitHistory}
                  disabled={isAuditRunning}
                  onChange={(e) => setOptions({ ...options, gitHistory: e.target.checked })}
                />
                <div>
                  <strong>Deep Git Commit Scan (Gitleaks)</strong>
                  <small>Scan full commit history for leaked credentials and API tokens.</small>
                </div>
              </label>
            </div>

            <div className="audit-settings-col">
              <h5>Custom Rule Paths & Baselines</h5>
              <div className="audit-input-field">
                <label>Gitleaks Baseline File (Optional)</label>
                <input
                  type="text"
                  value={options.gitleaksBaseline}
                  disabled={isAuditRunning}
                  placeholder="gitleaks-baseline.json"
                  onChange={(e) => setOptions({ ...options, gitleaksBaseline: e.target.value })}
                />
                <small>Ignore previously acknowledged baseline findings.</small>
              </div>

              <div className="audit-input-field">
                <label>Semgrep Rule Configuration (Optional)</label>
                <input
                  type="text"
                  value={options.semgrepConfig}
                  disabled={isAuditRunning}
                  placeholder="rules/security.yaml or p/ci"
                  onChange={(e) => setOptions({ ...options, semgrepConfig: e.target.value })}
                />
                <small>Local YAML rule file or Semgrep registry pack identifier.</small>
              </div>

              <div className="audit-input-field">
                <label>WSL Distribution for Semgrep (Optional)</label>
                <input
                  type="text"
                  value={options.semgrepWsl}
                  disabled={isAuditRunning}
                  placeholder="Ubuntu (Leave empty for native Windows binary)"
                  onChange={(e) => setOptions({ ...options, semgrepWsl: e.target.value })}
                />
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Modern Sub-Tab Navigation Bar */}
      <div className="audit-subnav-bar">
        <button
          className={`audit-subnav-btn ${subTab === 'static' ? 'active' : ''}`}
          onClick={() => setSubTab('static')}
        >
          <span>🛡️</span>
          <span>Static Analysis & Findings</span>
          {allFindings.length > 0 && (
            <span className={`subnav-badge ${errorCount > 0 ? 'badge-err' : 'badge-warn'}`}>
              {allFindings.length}
            </span>
          )}
        </button>

        <button
          className={`audit-subnav-btn ${subTab === 'config' ? 'active' : ''}`}
          onClick={() => {
            setSubTab('config');
            if (configItems.length === 0) handleRunConfigAudit();
          }}
        >
          <span>⚙️</span>
          <span>Configuration Health</span>
          {configItems.length > 0 && <span className="subnav-badge">{configItems.length}</span>}
        </button>

        <button
          className={`audit-subnav-btn ${subTab === 'pint' ? 'active' : ''}`}
          onClick={() => setSubTab('pint')}
        >
          <span>🎨</span>
          <span>Code Style (Pint)</span>
          {pintReport && (
            <span className={`subnav-badge ${pintReport.totalWarns > 0 ? 'badge-warn' : 'badge-good'}`}>
              {pintReport.totalWarns}
            </span>
          )}
        </button>

        <button
          className={`audit-subnav-btn ${subTab === 'browser' ? 'active' : ''}`}
          onClick={() => setSubTab('browser')}
        >
          <span>🌐</span>
          <span>Browser Accessibility</span>
          {browser?.running && <span className="subnav-badge badge-good">Active</span>}
        </button>

        <button
          className={`audit-subnav-btn ${subTab === 'rules' ? 'active' : ''}`}
          onClick={() => setSubTab('rules')}
        >
          <span>📖</span>
          <span>Rule Policy Catalog</span>
          <span className="subnav-badge">{catalog.length}</span>
        </button>

        <button
          className={`audit-subnav-btn ${subTab === 'setup' ? 'active' : ''}`}
          onClick={() => setSubTab('setup')}
        >
          <span>🛠️</span>
          <span>Tooling & Setup</span>
        </button>
      </div>

      {/* ========================================================================= */}
      {/* SUB-TAB 1: STATIC ANALYSIS & FINDINGS                                     */}
      {/* ========================================================================= */}
      {subTab === 'static' && (
        <div className="audit-subtab-pane">
          {/* Compact Analyzer Engine Status Ribbon */}
          {activeReport && (
            <div className="audit-engine-ribbon">
              <div className="engine-ribbon-left">
                <span className="engine-ribbon-title">Engines:</span>
                <div className="engine-ribbon-chips">
                  {(activeReport.analyzers || []).map((analyzer) => {
                    const isCompleted = analyzer.status === 'completed';
                    const isFailed = analyzer.status === 'failed';
                    const icon =
                      analyzer.id === 'builtin'
                        ? '⚡'
                        : analyzer.id === 'phpstan'
                        ? '🐘'
                        : analyzer.id === 'eslint'
                        ? '⬢'
                        : analyzer.id === 'gitleaks'
                        ? '🔑'
                        : analyzer.id === 'semgrep'
                        ? '🎯'
                        : '🌐';

                    const displayName = analyzer.name
                      .replace(' / Larastan', '')
                      .replace(' (optional)', '')
                      .replace(' browser accessibility', '')
                      .replace(' source heuristics', '');

                    return (
                      <div
                        key={analyzer.id}
                        className={`engine-chip ${
                          isCompleted ? 'chip-completed' : isFailed ? 'chip-failed' : 'chip-skipped'
                        }`}
                        title={`${analyzer.name} (${analyzer.status}): ${analyzer.coverage || analyzer.message || ''}`}
                      >
                        <span className="chip-icon">{icon}</span>
                        <span className="chip-name">{displayName}</span>
                        {isCompleted ? (
                          <span className="chip-time">{analyzer.durationSeconds?.toFixed(1) || '0.0'}s</span>
                        ) : (
                          <span className="chip-status">{analyzer.status}</span>
                        )}
                      </div>
                    );
                  })}
                </div>
              </div>

              <button
                className="btn-ribbon-toggle"
                onClick={() => setShowEnginesDetail(!showEnginesDetail)}
                title="Toggle analyzer engine coverage and message details"
              >
                {showEnginesDetail ? '▲ Hide Engine Cards' : '🔬 Engine Details ▾'}
              </button>
            </div>
          )}

          {/* Collapsible Deep Engine Cards */}
          {showEnginesDetail && activeReport && (
            <div className="audit-engines-section">
              <div className="audit-engines-header">
                <h4>
                  <span>🔬</span> Active Analyzer Engines ({(activeReport.analyzers || []).length})
                </h4>
                <button
                  className="btn-tiny"
                  onClick={() => setExpandedCoverage(!expandedCoverage)}
                  title="Toggle engine coverage details"
                >
                  {expandedCoverage ? '▲ Hide Coverage Details' : '▼ Show Coverage Details'}
                </button>
              </div>

              <div className="audit-engines-grid">
                {(activeReport.analyzers || []).map((analyzer) => {
                  const isCompleted = analyzer.status === 'completed';
                  const isFailed = analyzer.status === 'failed';
                  const isSkipped = analyzer.status === 'skipped' || analyzer.status === 'missing';

                  return (
                    <div
                      key={analyzer.id}
                      className={`engine-card ${
                        isCompleted ? 'engine-completed' : isFailed ? 'engine-failed' : 'engine-skipped'
                      }`}
                    >
                      <div className="engine-card-header">
                        <div className="engine-name-group">
                          <span className="engine-icon">
                            {analyzer.id === 'builtin'
                              ? '⚡'
                              : analyzer.id === 'phpstan'
                              ? '🐘'
                              : analyzer.id === 'eslint'
                              ? '⬢'
                              : analyzer.id === 'gitleaks'
                              ? '🔑'
                              : analyzer.id === 'semgrep'
                              ? '🎯'
                              : '🌐'}
                          </span>
                          <div>
                            <strong>{analyzer.name}</strong>
                            <small className="engine-version">
                              {analyzer.version || (isSkipped ? 'Not configured' : 'Active')}
                            </small>
                          </div>
                        </div>

                        <span
                          className={`engine-status-badge ${
                            isCompleted ? 'badge-good' : isFailed ? 'badge-error' : 'badge-neutral'
                          }`}
                        >
                          {analyzer.status}
                        </span>
                      </div>

                      <div className="engine-card-meta">
                        <span>⏱️ {analyzer.durationSeconds?.toFixed(2) || '0.00'}s</span>
                        {analyzer.configuration && (
                          <code className="engine-config-tag" title="Configuration file">
                            {analyzer.configuration}
                          </code>
                        )}
                      </div>

                      {expandedCoverage && (
                        <div className="engine-card-expanded">
                          <p className="engine-coverage-text">{analyzer.coverage}</p>
                          {analyzer.message && (
                            <p className="engine-message-text">{analyzer.message}</p>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* Findings Filter Bar, Search, and Grouping Controls */}
          {activeReport && (
            <div className="audit-filter-deck">
              <div className="audit-filter-tabs">
                {[
                  { id: 'All', label: 'Active Issues', count: activeFindings.length },
                  { id: 'Defects & Security', label: 'Defects & Errors', count: errorCount, icon: '🛑' },
                  { id: 'Performance', label: 'Performance', count: warnCount, icon: '⚡' },
                  { id: 'Optional Conventions', label: 'Conventions', count: convCount, icon: '💡' },
                  ...(fpCount > 0
                    ? [{ id: 'False Positives', label: 'False Positives', count: fpCount, icon: '🛡️' }]
                    : []),
                ].map((item) => (
                  <button
                    key={item.id}
                    className={`filter-pill-btn ${filterGroup === item.id ? 'active' : ''}`}
                    onClick={() => setFilterGroup(item.id)}
                  >
                    {item.icon && <span>{item.icon}</span>}
                    <span>{item.label}</span>
                    <span className="pill-count">{item.count}</span>
                  </button>
                ))}
              </div>

              <div className="audit-toolbar-right">
                <div className="audit-search-box">
                  <span className="search-icon">🔍</span>
                  <input
                    type="text"
                    placeholder="Filter by rule, file, or message..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                  />
                  {searchQuery && (
                    <button className="search-clear-btn" onClick={() => setSearchQuery('')}>
                      ✕
                    </button>
                  )}
                </div>

                <div className="group-by-toggle" title="Group issues by File, Rule, or Flat List">
                  <button
                    className={`btn-group-by ${groupBy === 'file' ? 'active' : ''}`}
                    onClick={() => setGroupBy('file')}
                  >
                    📁 By File
                  </button>
                  <button
                    className={`btn-group-by ${groupBy === 'rule' ? 'active' : ''}`}
                    onClick={() => setGroupBy('rule')}
                  >
                    ⚙️ By Rule
                  </button>
                  <button
                    className={`btn-group-by ${groupBy === 'flat' ? 'active' : ''}`}
                    onClick={() => setGroupBy('flat')}
                  >
                    ☰ Flat
                  </button>
                </div>

                {allFindings.length > 0 && (
                  <button
                    className="btn-toggle-all"
                    onClick={toggleAllDetails}
                    title="Toggle expanded details for all issues"
                  >
                    ↕ Toggle Details
                  </button>
                )}
              </div>
            </div>
          )}

          {/* Organized Findings Deck */}
          {activeReport && (
            <div className="audit-findings-list">
              {/* Grouped by File */}
              {groupBy === 'file' &&
                fileGroups.map((group) => {
                  const isCollapsed = !!collapsedGroups[group.file];
                  return (
                    <div
                      key={group.file}
                      className={`file-group-card ${
                        group.errorCount > 0 ? 'has-error' : group.warnCount > 0 ? 'has-warning' : ''
                      }`}
                    >
                      <div
                        className={`file-group-header ${!isCollapsed ? 'is-open' : ''}`}
                        onClick={() => toggleGroup(group.file)}
                      >
                        <div className="file-group-title">
                          <span>📁</span>
                          {group.dir && <span className="file-group-dir">{group.dir}</span>}
                          <span className="file-group-name">{group.name}</span>

                          <div className="file-group-counts">
                            {group.errorCount > 0 && (
                              <span className="count-badge badge-error" title={`${group.errorCount} errors`}>
                                {group.errorCount} {group.errorCount === 1 ? 'error' : 'errors'}
                              </span>
                            )}
                            {group.warnCount > 0 && (
                              <span className="count-badge badge-warn" title={`${group.warnCount} warnings`}>
                                {group.warnCount} {group.warnCount === 1 ? 'warning' : 'warnings'}
                              </span>
                            )}
                            <span className="count-badge badge-total" title="Total issues in file">
                              {group.items.length} {group.items.length === 1 ? 'issue' : 'issues'}
                            </span>
                          </div>
                        </div>

                        <div className="file-group-actions">
                          {group.file !== 'Rendered DOM / Page' && (
                            <button
                              className="btn-header-copy"
                              onClick={(e) => {
                                e.stopPropagation();
                                copyGroupPath(group.file);
                              }}
                              title="Copy relative file path"
                            >
                              {copiedGroupPath === group.file ? '✓ Copied' : '📋 Copy Path'}
                            </button>
                          )}
                          <span className="file-group-chevron">{isCollapsed ? '▼' : '▲'}</span>
                        </div>
                      </div>

                      {!isCollapsed && (
                        <div className="file-group-items">
                          {group.items.map((item) => renderFindingRow(item))}
                        </div>
                      )}
                    </div>
                  );
                })}

              {/* Grouped by Rule */}
              {groupBy === 'rule' &&
                ruleGroups.map((group) => {
                  const isCollapsed = !!collapsedGroups[group.rule];
                  return (
                    <div
                      key={group.rule}
                      className={`file-group-card ${
                        group.errorCount > 0 ? 'has-error' : group.warnCount > 0 ? 'has-warning' : ''
                      }`}
                    >
                      <div
                        className={`file-group-header ${!isCollapsed ? 'is-open' : ''}`}
                        onClick={() => toggleGroup(group.rule)}
                      >
                        <div className="file-group-title">
                          <span>⚙️</span>
                          <span className="file-group-name">{group.rule}</span>
                          <span className="row-category-pill">{group.category}</span>

                          <div className="file-group-counts">
                            {group.errorCount > 0 && (
                              <span className="count-badge badge-error">
                                {group.errorCount} {group.errorCount === 1 ? 'error' : 'errors'}
                              </span>
                            )}
                            {group.warnCount > 0 && (
                              <span className="count-badge badge-warn">
                                {group.warnCount} {group.warnCount === 1 ? 'warning' : 'warnings'}
                              </span>
                            )}
                            <span className="count-badge badge-total">
                              {group.items.length} {group.items.length === 1 ? 'issue' : 'issues'}
                            </span>
                          </div>
                        </div>

                        <div className="file-group-actions">
                          <span className="file-group-chevron">{isCollapsed ? '▼' : '▲'}</span>
                        </div>
                      </div>

                      {!isCollapsed && (
                        <div className="file-group-items">
                          {group.items.map((item) => renderFindingRow(item))}
                        </div>
                      )}
                    </div>
                  );
                })}

              {/* Flat View */}
              {groupBy === 'flat' && (
                <div className="file-group-card">
                  <div className="file-group-items">
                    {filteredFindings.map((f, idx) =>
                      renderFindingRow({
                        finding: f,
                        originalIndex: idx,
                        id: `${f.file}:${f.line}:${f.rule}:${idx}`,
                      })
                    )}
                  </div>
                </div>
              )}

              {/* No matching findings for current filter */}
              {filteredFindings.length === 0 && allFindings.length > 0 && (
                <div className="audit-empty-findings">
                  <div className="empty-icon">🔍</div>
                  <h4>No issues match your current filter</h4>
                  <p>Try switching to "All Issues" or clearing your search term.</p>
                  <button
                    className="btn-tiny primary"
                    onClick={() => {
                      setFilterGroup('All');
                      setSearchQuery('');
                    }}
                  >
                    Reset Filter
                  </button>
                </div>
              )}

              {/* Clean Codebase State */}
              {allFindings.length === 0 && (
                <div className="audit-clean-state">
                  <div className="clean-icon">🛡️✨</div>
                  <h3>{activeReport.running ? 'Audit in progress' : 'No findings in completed checks'}</h3>
                  <p>
                    {completedAnalyzers.length} completed checks; {failedAnalyzers.length + skippedAnalyzers.length} failed, missing, skipped or running. Coverage is limited to the checks shown above; this does not certify security or complete accessibility.
                  </p>
                  <button
                    className="btn-tiny"
                    onClick={() => project && runAction('Refreshing audit', () => AuditFullStackBestPractices(project.id, options))}
                  >
                    ↻ Run Verification Check
                  </button>
                </div>
              )}
            </div>
          )}

          {/* Initial State (Before First Audit Run) */}
          {!activeReport && (
            <div className="audit-initial-state">
              <div className="initial-icon">⚡</div>
              <h3>Continuous Full-Stack Code Quality</h3>
              <p>
                Run your first static audit to analyze PHP files with Larastan/PHPStan, check React & Vue hooks with ESLint, inspect credentials with Gitleaks, and enforce framework best practices.
              </p>
              <button
                className="btn-audit-primary"
                disabled={!project || !trusted}
                onClick={() =>
                  project &&
                  runAction('Running Full-Stack Audit', () =>
                    AuditFullStackBestPractices(project.id, options)
                  )
                }
              >
                ▶ Start Static Code Audit
              </button>
            </div>
          )}
        </div>
      )}

      {/* ========================================================================= */}
      {/* SUB-TAB 2: CONFIGURATION HEALTH                                           */}
      {/* ========================================================================= */}
      {subTab === 'config' && (
        <div className="audit-subtab-pane">
          <div className="workbench-card">
            <div className="card-header-flex">
              <div>
                <h3>Configuration & Environment Security Audit</h3>
                <p>
                  Automated checks for production security, sensitive .env exposures, error handling, session cookies, and PHP runtime directives.
                </p>
              </div>
              <button
                className="btn-action primary"
                disabled={configRunning || !trusted}
                onClick={handleRunConfigAudit}
              >
                {configRunning ? 'Checking...' : '↻ Refresh Config Audit'}
              </button>
            </div>

            <div className="config-audit-table-wrap">
              <table className="workbench-table">
                <thead>
                  <tr>
                    <th>Category</th>
                    <th>Configuration Directive</th>
                    <th>Current Value</th>
                    <th>Health Assessment</th>
                    <th>Remediation Guidance</th>
                  </tr>
                </thead>
                <tbody>
                  {configItems.map((item, idx) => {
                    const isGood = item.status === 'good';
                    const isWarn = item.status === 'warning';
                    return (
                      <tr key={idx}>
                        <td>
                          <span className="workbench-badge badge-info">{item.category}</span>
                        </td>
                        <td>
                          <strong className="config-key">{item.key}</strong>
                        </td>
                        <td>
                          <code className="config-value">{item.value || '(empty)'}</code>
                        </td>
                        <td>
                          <span
                            className={`workbench-badge ${
                              isGood ? 'badge-good' : isWarn ? 'badge-warn' : 'badge-info'
                            }`}
                          >
                            {item.message}
                          </span>
                        </td>
                        <td>
                          <span className="config-guidance">{item.fixGuidance}</span>
                        </td>
                      </tr>
                    );
                  })}
                  {configItems.length === 0 && !configRunning && (
                    <tr>
                      <td colSpan={5} className="empty-text">
                        No configuration audit records loaded. Click "Refresh Config Audit" to evaluate your environment.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* SUB-TAB 3: LARAVEL PINT (CODE STYLE)                                      */}
      {/* ========================================================================= */}
      {subTab === 'pint' && (
        <div className="audit-subtab-pane">
          <div className="workbench-card">
            <div className="card-header-flex">
              <div>
                <h3>Laravel Pint — PHP Code Style & Standards</h3>
                <p>
                  Opinionated PHP code style fixer built on PHP-CS-Fixer. Ensures consistent code formatting across your team without bikeshedding.
                </p>
              </div>
              <button
                className="btn-action primary"
                disabled={pintRunning || !trusted}
                onClick={handleRunPint}
              >
                {pintRunning ? 'Running Pint...' : '▶ Check Code Style (Pint)'}
              </button>
            </div>

            {pintReport ? (
              <div className="pint-results-deck">
                <div className={`audit-scorecard ${pintReport.totalWarns === 0 ? 'scorecard-clean' : 'scorecard-issues'}`}>
                  <div className="scorecard-main">
                    <div className="scorecard-icon">
                      {pintReport.totalWarns === 0 ? '✨' : '🎨'}
                    </div>
                    <div>
                      <h3>
                        {pintReport.totalWarns === 0
                          ? 'Zero Code Style Violations Detected'
                          : `${pintReport.totalWarns} Style Violations Found`}
                      </h3>
                      <p>Checked in {pintReport.durationSeconds.toFixed(1)} seconds using Pint configuration.</p>
                    </div>
                  </div>
                </div>

                {pintReport.rawOutput && (
                  <div className="pint-output-box">
                    <div className="output-header">Pint Execution Output</div>
                    <pre className="installation-output">{pintReport.rawOutput}</pre>
                  </div>
                )}
              </div>
            ) : (
              <div className="audit-initial-state">
                <div className="initial-icon">🎨</div>
                <h4>Evaluate Your Codebase with Laravel Pint</h4>
                <p>Run Pint in dry-run inspection mode to identify formatting discrepancies before committing.</p>
                <button
                  className="btn-tiny primary"
                  disabled={!trusted}
                  onClick={handleRunPint}
                >
                  Run Pint Inspection
                </button>
              </div>
            )}
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* SUB-TAB 4: BROWSER ACCESSIBILITY (AXE-CORE)                               */}
      {/* ========================================================================= */}
      {subTab === 'browser' && (
        <div className="audit-subtab-pane">
          <div className="workbench-card">
            <h3>🌐 Browser Accessibility & DOM Audit (axe-core)</h3>
            <p>
              Audits rendered HTML, aria labels, color contrast, and keyboard navigation directly inside a temporary local browser session.
            </p>

            <div className="browser-session-status-card">
              <span className={`status-dot ${browser?.running ? 'active' : 'inactive'}`} />
              <div>
                <strong>
                  {browser?.running ? 'Temporary Browser Session Active' : 'No Browser Session Running'}
                </strong>
                <p>
                  {browser?.message ||
                    'Closing the session completely removes its temporary profile; cookies, tokens, and screenshots are never persisted.'}
                </p>
              </div>
            </div>

            <div className="browser-controls-grid">
              <div className="browser-url-input-wrap">
                <label>Target Page URL to Audit</label>
                <input
                  type="text"
                  value={browserUrl}
                  disabled={isAuditRunning}
                  placeholder={project?.url || 'http://localhost'}
                  onChange={(e) => setBrowserUrl(e.target.value)}
                />
              </div>

              <div className="browser-buttons-row">
                <button
                  className="btn-action"
                  disabled={!trusted || !project || isAuditRunning || !!browser?.running}
                  onClick={() =>
                    project &&
                    runAction('Opening Temporary Browser', () => OpenAuditBrowser(project.id))
                  }
                  title="Launch an isolated Chrome/Chromium session"
                >
                  🚀 Open Temporary Browser
                </button>

                <button
                  className="btn-action primary"
                  disabled={!trusted || !project || isAuditRunning || !browser?.running}
                  onClick={() =>
                    project &&
                    runAction('Auditing Rendered Page', () =>
                      AuditBrowserPage(project.id, browserUrl)
                    )
                  }
                  title="Inject axe-core and analyze current DOM"
                >
                  🔍 Audit Selected Page
                </button>

                <button
                  className="btn-action"
                  disabled={!project || !browser?.running}
                  onClick={async () => {
                    if (!project) return;
                    try {
                      await CloseAuditBrowser(project.id);
                    } catch (e) {
                      setError(String(e));
                    } finally {
                      await reload();
                    }
                  }}
                  title="Terminate browser process and wipe session profile"
                >
                  ✕ Close Session
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* SUB-TAB 5: RULE POLICY CATALOG                                            */}
      {/* ========================================================================= */}
      {subTab === 'rules' && (
        <div className="audit-subtab-pane">
          <div className="workbench-card">
            <div className="card-header-flex">
              <div>
                <h3>Built-In Rule Policy Directory ({catalog.length} Rules)</h3>
                <p>
                  All active static checks, their owners (Built-in heuristic vs. Analyzer-owned), kind, and optionality.
                </p>
              </div>
              <div className="catalog-search-wrap">
                <input
                  type="text"
                  placeholder="Filter rules by name or owner..."
                  value={catalogSearch}
                  onChange={(e) => setCatalogSearch(e.target.value)}
                  style={{ width: '220px', padding: '6px 10px', fontSize: '12px' }}
                />
              </div>
            </div>

            <div className="catalog-rules-grid">
              {catalog
                .filter((r) => {
                  if (!catalogSearch) return true;
                  const q = catalogSearch.toLowerCase();
                  return (
                    r.id.toLowerCase().includes(q) ||
                    r.kind.toLowerCase().includes(q) ||
                    r.owner.toLowerCase().includes(q)
                  );
                })
                .map((rule) => (
                  <div className="catalog-rule-card" key={rule.id}>
                    <div className="rule-card-header">
                      <code>{rule.id}</code>
                      {rule.optional ? (
                        <span className="workbench-badge badge-warn">Optional</span>
                      ) : (
                        <span className="workbench-badge badge-info">Standard</span>
                      )}
                    </div>
                    <div className="rule-card-meta">
                      <span>Owner: <strong>{rule.owner}</strong></span>
                      <span>Category: <strong>{KIND_LABELS[rule.kind] || rule.kind}</strong></span>
                    </div>
                  </div>
                ))}
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* SUB-TAB 6: TOOLING SETUP                                                  */}
      {/* ========================================================================= */}
      {subTab === 'setup' && (
        <div className="audit-subtab-pane">
          <AuditSetupPanel
            project={project}
            trusted={trusted}
            installing={isAuditRunning}
            reload={reload}
          />
        </div>
      )}

      {/* False Positive Citation Modal */}
      {activeFpModalItem && (
        <div className="fp-modal-backdrop" onClick={() => setActiveFpModalItem(null)}>
          <div className="fp-modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="fp-modal-header">
              <div className="fp-modal-title">
                <span>🛡️</span>
                <h3>Mark as False Positive</h3>
              </div>
              <button
                type="button"
                className="btn-close-modal"
                onClick={() => setActiveFpModalItem(null)}
              >
                ✕
              </button>
            </div>

            <div className="fp-modal-body">
              <div className="fp-modal-context">
                <div className="fp-context-item">
                  <span className="label">Rule:</span>
                  <code className="val">{activeFpModalItem.rule}</code>
                </div>
                <div className="fp-context-item">
                  <span className="label">Target:</span>
                  <code className="val">
                    {activeFpModalItem.file}:{activeFpModalItem.line}
                  </code>
                </div>
              </div>

              <div className="fp-citation-field">
                <label htmlFor="fp-citation-input">
                  <strong>Architectural Citation / Rationale:</strong>
                  <small>
                    Document the middleware, database transaction wrapper, or regression test proving this finding is safe.
                  </small>
                </label>
                <input
                  id="fp-citation-input"
                  type="text"
                  className="input-citation"
                  placeholder="e.g. app/Http/Middleware/RecordAdminActivity.php:21"
                  value={fpCitationInput}
                  onChange={(e) => setFpCitationInput(e.target.value)}
                  autoFocus
                />
              </div>

              <div className="fp-quick-citations">
                <span className="quick-label">Quick Suggestions:</span>
                <div className="quick-pills">
                  {[
                    'app/Http/Middleware/RecordAdminActivity.php:21',
                    'Wrapped by Transactional Middleware',
                    'Atomic multi-write handled by database boundary',
                    'Protected by framework queue transaction',
                  ].map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      className="pill-quick-cit"
                      onClick={() => setFpCitationInput(preset)}
                    >
                      + {preset}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            <div className="fp-modal-actions">
              <button
                type="button"
                className="btn-action secondary"
                onClick={() => setActiveFpModalItem(null)}
              >
                Cancel
              </button>
              <button
                type="button"
                className="btn-action primary btn-confirm-fp"
                onClick={() => markAsFalsePositive(activeFpModalItem.key, fpCitationInput)}
              >
                ✓ Confirm False Positive
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

// Setup Sub-Component (Also used in Settings tab)
type SetupProps = {
  project?: workbench.Project;
  trusted: boolean;
  installing: boolean;
  reload: () => Promise<void>;
};

export function AuditSetupPanel({ project, trusted, installing, reload }: SetupProps) {
  const [plan, setPlan] = useState<quality.AuditSetupPlan | null>(null);
  const [manager, setManager] = useState('');
  const [pending, setPending] = useState(false);
  const [message, setMessage] = useState('');

  useEffect(() => {
    setPlan(null);
    setMessage('');
  }, [project?.id]);

  const preview = async (kind: string) => {
    if (!project) return;
    setPending(true);
    setMessage('');
    try {
      setPlan(await PreviewAuditSetup(project.id, kind, manager));
    } catch (e) {
      setMessage(String(e));
    } finally {
      setPending(false);
    }
  };

  return (
    <section className="workbench-card" aria-label="Audit tool setup">
      <div className="card-header-flex">
        <div>
          <h3>Analyzer & Tooling Starter Setup</h3>
          <p>
            Preview dependencies, scripts, and starter configuration files before making changes to your project repository.
          </p>
        </div>
        <div className="package-manager-select-wrap">
          <label style={{ fontSize: '12px', marginRight: '6px' }}>Package Manager:</label>
          <select
            value={manager}
            onChange={(e) => {
              setManager(e.target.value);
              setPlan(null);
            }}
            disabled={pending || installing}
          >
            <option value="">Auto-detect from lockfile</option>
            {['npm', 'pnpm', 'yarn', 'bun'].map((val) => (
              <option key={val} value={val}>
                {val}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="audit-setup-buttons-row">
        {[
          { kind: 'larastan', label: '🐘 Preview Larastan Setup', desc: 'Adds Larastan & generates phpstan.neon' },
          { kind: 'eslint', label: '⬢ Preview ESLint Setup', desc: 'Adds eslint-plugin-react-hooks & config' },
          { kind: 'browser', label: '🌐 Preview Browser Runtime', desc: 'Provisions private Chrome driver' },
        ].map((item) => (
          <button
            key={item.kind}
            className="btn-setup-card"
            disabled={!project || pending || installing}
            onClick={() => preview(item.kind)}
          >
            <strong>{item.label}</strong>
            <small>{item.desc}</small>
          </button>
        ))}
      </div>

      {!project && <p className="empty-text">Select a project in the sidebar first.</p>}
      {message && <div className="audit-message-pill" role="status">{message}</div>}

      {plan && (
        <div className="setup-preview-drawer" aria-label="Setup preview">
          <div className="preview-header">
            <h4>Setup Plan Preview ({plan.kind})</h4>
            <p>{plan.message}</p>
          </div>

          <div className="preview-commands-list">
            <h5>Commands to Execute:</h5>
            {plan.commands.map((cmd, i) => (
              <pre key={i} className="installation-output">
                <code>
                  {cmd.executable} {cmd.args.map((a) => JSON.stringify(a)).join(' ')}
                </code>
                {'\n'}
                <small style={{ color: 'var(--text-dim)' }}>Directory: {cmd.directory}</small>
              </pre>
            ))}
          </div>

          <div className="preview-files-list">
            <h5>Configuration Files to Generate:</h5>
            {plan.files.length === 0 && (
              <p className="empty-text">No new configuration files will be created.</p>
            )}
            {plan.files.map((file) => (
              <details key={file.path} className="setup-file-details">
                <summary>📄 {file.path}</summary>
                <pre className="installation-output">{file.content}</pre>
              </details>
            ))}
          </div>

          <div className="setup-preview-actions">
            {!trusted && (
              <p className="err" style={{ fontSize: '12px' }}>
                Trust this project before applying setup changes.
              </p>
            )}
            <button
              className="btn-action primary"
              disabled={!trusted || pending || installing}
              onClick={async () => {
                setPending(true);
                setMessage('');
                try {
                  await ApplyAuditSetup(plan.id);
                  setPlan(null);
                  setMessage('✓ Setup successfully applied. Run a static audit to verify coverage.');
                } catch (e) {
                  setMessage(String(e));
                } finally {
                  setPending(false);
                  await reload();
                }
              }}
            >
              ✓ Apply Reviewed Setup
            </button>
            <button
              className="btn-action secondary"
              disabled={pending}
              onClick={() => setPlan(null)}
            >
              Dismiss Preview
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
