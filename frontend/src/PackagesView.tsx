import { useState } from "react";

type Issue = { name: string; current: string; wanted: string; latest: string; severity: string; title: string };
type Check = { name: string; status: string; detail: string; issues: Issue[] };
type Entry = { name: string; group: string; requested: string; locked: string; installed: string };
type Manager = { present: boolean; checks: Check[]; packages: Entry[] };
export type PackageReport = { checkedAt: string; online: boolean; composer: Manager; npm: Manager };

function Badge({ value, tone = "neutral", detail }: { value: string; tone?: string; detail?: string }) {
  return <span className={`workspace-status status-${tone}`} title={detail}>{value}</span>;
}
function updateTone(current: string, latest: string) {
  const a = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(current), b = /^v?(\d+)\.(\d+)\.(\d+)$/.exec(latest);
  if (!a || !b) return "neutral";
  if (a.slice(1).every((v, i) => v === b[i + 1])) return "good";
  // Higher registry tags are an upgrade signal; older tags are not errors.
  for (let i = 1; i <= 3; i++) { if (+b[i] > +a[i]) return i === 1 ? "danger" : "warn"; if (+b[i] < +a[i]) return "neutral"; }
  return "neutral";
}
function ManagerReport({ name, manager }: { name: string; manager: Manager }) {
  const [filter, setFilter] = useState("");
  return <section className="workbench-card package-manager"><h3>{name}</h3>
    {!manager.present ? <p>No manifest in this project.</p> : <>
      <div className="package-checks">{manager.checks.map((check, i) => <div key={`${check.name}-${i}`} className="package-check">
        <div className="companion-row"><strong>{check.name}</strong><Badge value={check.status === "ready" ? "Passed" : check.status === "attention" ? "Needs attention" : check.status === "error" ? "Check failed" : "Not checked"} tone={check.status === "ready" ? "good" : check.status === "attention" ? "warn" : check.status === "error" ? "danger" : "neutral"}/></div>
        <p>{check.detail}</p>
        {!!check.issues?.length && <details><summary>{check.issues.length} affected packages / advisories</summary><div className="companion-table-scroll"><table className="workbench-table"><thead><tr><th>Package</th><th>Current / affected</th>{check.name === "outdated" ? <><th>Wanted</th><th>Latest</th></> : <><th>Severity</th><th>Details</th></>}</tr></thead><tbody>{check.issues.map((issue, n) => <tr key={`${issue.name}-${n}`}>
          <td>{issue.name}</td><td><code>{issue.current || "Unavailable"}</code></td>
          {check.name === "outdated" ? <><td><code>{issue.wanted || "—"}</code></td><td><Badge value={issue.latest || "Unknown"} tone={updateTone(issue.current, issue.latest)} detail="Green: matching version; yellow: newer minor/patch; red: newer major. A new major may require breaking changes; this is not a security rating."/></td></> : <><td><Badge value={issue.severity || "Unrated"} tone={["critical", "high"].includes(issue.severity) ? "danger" : issue.severity ? "warn" : "neutral"}/></td><td>{issue.title}</td></>}
        </tr>)}</tbody></table></div></details>}
      </div>)}</div>
      <details className="package-inventory"><summary>Package inventory ({manager.packages.length})</summary>
        <label>Filter {name} packages<input value={filter} onChange={e => setFilter(e.target.value)} placeholder="Package name"/></label>
        <p>{name === "Composer" ? "All locked packages, including transitive dependencies." : "Root direct dependencies. Nested workspaces and transitive packages are not listed here; audit covers the npm lockfile."}</p>
        <div className="companion-table-scroll"><table className="workbench-table"><thead><tr><th>Package</th><th>Scope</th><th>Requested</th><th>Locked</th><th>Installed</th></tr></thead><tbody>{manager.packages.filter(p => p.name.toLowerCase().includes(filter.toLowerCase())).map(p => <tr key={p.name}><td>{p.name}</td><td>{p.group}</td><td><code>{p.requested || "Transitive"}</code></td><td><code>{p.locked || "Unavailable"}</code></td><td><Badge value={p.installed || "Not detected"} tone={!p.installed ? "warn" : !p.locked ? "neutral" : p.installed.replace(/^v/, "") === p.locked.replace(/^v/, "") ? "good" : "warn"} detail="Installed metadata compared with the lockfile. Missing optional/dev packages may be intentional; package file integrity is not verified."/></td></tr>)}</tbody></table></div>
      </details>
    </>}
  </section>;
}
export default function PackagesView({ report }: { report: PackageReport | null }) {
  return report ? <><p>{report.checkedAt ? `Last check: ${new Date(report.checkedAt).toLocaleString()} · ${report.online ? "Registry checks requested" : "Local checks only"}` : "Static inventory only. Run checks to validate installation, updates and advisories."}</p>
    <ManagerReport key="composer" name="Composer" manager={report.composer}/><ManagerReport key="npm" name="npm" manager={report.npm}/></> : null;
}
