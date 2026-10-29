package workbench

import (
	"herdworkbench/internal/quality"
	"herdworkbench/internal/runner"
	"testing"
)

func TestAuditTrustCancellationAndSnapshot(t *testing.T) {
	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	b.projects = []Project{{ID: "audit", Path: t.TempDir(), PHPBinary: "php.exe"}}
	if _, done, err := b.auditOperation("audit"); err == nil {
		done()
		t.Fatal("untrusted execution")
	}
	b.trusted["audit"] = true
	ctx, done, err := b.auditOperation("audit")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := b.auditOperation("audit"); err == nil {
		t.Fatal("overlapping audit")
	}
	b.CancelAudit("audit")
	if ctx.Err() == nil {
		t.Fatal("cancellation missing")
	}
	done()
	b.publishAudit("audit", quality.AuditReport{Findings: []quality.CodeBestPracticeFinding{{Tools: []string{"original"}}}})
	snapshot := b.Snapshot()
	r := snapshot.AuditReports["audit"]
	r.Findings[0].Tools[0] = "mutated"
	if b.auditReports["audit"].Findings[0].Tools[0] != "original" {
		t.Fatal("snapshot aliases backend")
	}
}
func TestAuditSetupFailureIsNotCancellation(t *testing.T) {
	b := NewBackend(func() {})
	t.Cleanup(b.Shutdown)
	b.projects = []Project{{ID: "audit", Path: t.TempDir(), PHPBinary: "php.exe"}}
	b.trusted["audit"] = true
	b.auditSetups["preview"] = quality.AuditSetupPlan{ProjectID: "audit", Kind: "eslint", Commands: []runner.Spec{{Executable: "missing-audit-executable.exe"}}}
	if err := b.ApplyAuditSetup("preview"); err == nil {
		t.Fatal("missing executable passed")
	}
	if b.installation.State != "failed" || b.installation.Running {
		t.Fatalf("wrong failure status: %+v", b.installation)
	}
}
