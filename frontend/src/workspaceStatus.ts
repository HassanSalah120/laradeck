export type StatusTone = "good" | "warn" | "danger" | "info" | "neutral";
export type VisualStatus = { tone: StatusTone; label: string; detail: string };

// Release generations, verified against official release pages on 2026-10-18.
// This is deliberately not an assertion about patch freshness or end of support.
export const releaseReference = { laravel: 13, php: [8, 5], checked: "2026-10-18" } as const;

export function versionStatus(kind: "Laravel" | "PHP", value: string): VisualStatus {
  const label = (value || "").trim().replace(/^v/i, "") || "Unknown";
  const match = /^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?$/i.exec((value || "").trim());
  if (!match || (kind === "PHP" && !match[2])) {
    return { tone: "neutral", label, detail: `${kind}: release generation is unknown or not a stable version.` };
  }
  const major = Number(match[1]), minor = Number(match[2] || 0);
  const age = kind === "Laravel" ? releaseReference.laravel - major
    : major === releaseReference.php[0] ? releaseReference.php[1] - minor
    : major < releaseReference.php[0] ? 2 : -1;
  const branch = kind === "Laravel" ? `${releaseReference.laravel}.x` : releaseReference.php.join(".");
  if (age < 0) return { tone: "neutral", label, detail: `${kind} ${label}: newer than the verified ${branch} reference; release status not checked.` };
  const generation = age === 0 ? "Latest reference branch" : age === 1 ? "Previous release branch" : "Older release branch";
  return { tone: age === 0 ? "good" : age === 1 ? "warn" : "danger", label,
    detail: `${kind} ${label} · ${generation}. Reference: ${branch}, checked ${releaseReference.checked}. Patch freshness and security-support status are not inferred.` };
}

export function auditAgeStatus(timestamp: string, now = Date.now()): VisualStatus {
  if (!timestamp) return { tone: "neutral", label: "Not run", detail: "No completed audit is recorded." };
  const time = Date.parse(timestamp);
  if (!Number.isFinite(time) || time > now + 60_000) return { tone: "neutral", label: "Unknown date", detail: "The recorded audit date cannot be reliably interpreted." };
  const days = Math.max(0, (now - time) / 86_400_000);
  return { tone: days < 1 ? "good" : days < 7 ? "warn" : "danger", label: days < 1 ? "Today" : `${Math.floor(days)}d ago`,
    detail: `Last completed audit: ${new Date(time).toLocaleString()}. Green: under 24 hours; yellow: under 7 days; red: 7 days or older. Recency does not imply a clean result.` };
}

export function coverageStatus(coverage: string): VisualStatus {
  if (coverage === "Running") return { tone: "info", label: "Running", detail: "Audit is in progress; coverage is not final." };
  const match = /^(\d+)\/(\d+) completed$/.exec(coverage);
  if (!match || Number(match[2]) === 0) return { tone: "neutral", label: "Not checked", detail: "No completed analyzer coverage is available." };
  const complete = Number(match[1]), total = Number(match[2]);
  if (complete > total) return { tone: "neutral", label: "Unknown", detail: "Analyzer counts are inconsistent." };
  return { tone: complete === total ? "good" : complete === 0 ? "danger" : "warn", label: `${complete}/${total}`,
    detail: `${complete} of ${total} configured analyzers completed. Missing, failed and skipped analyzers do not count as coverage. Completed coverage is not a security certification.` };
}
