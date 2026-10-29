package workbench

import (
	"context"
	"fmt"
	"strings"
	"time"

	"herdworkbench/internal/quality"
)

func (b *Backend) auditOperation(id string) (context.Context, func(), error) {
	if _, err := b.project(id, true); err != nil {
		return nil, nil, err
	}
	parent, finish, err := b.beginOperation()
	if err != nil {
		return nil, nil, err
	}
	b.mu.Lock()
	if b.auditCancels[id] != nil {
		b.mu.Unlock()
		finish()
		return nil, nil, fmt.Errorf("An audit is already running for this project")
	}
	ctx, cancel := context.WithCancel(parent)
	b.auditCancels[id] = cancel
	b.mu.Unlock()
	return ctx, func() {
		cancel()
		b.mu.Lock()
		delete(b.auditCancels, id)
		b.mu.Unlock()
		finish()
		b.emit()
	}, nil
}
func (b *Backend) publishAudit(id string, report quality.AuditReport) {
	report.ProjectID = id
	b.mu.Lock()
	b.auditReports[id] = report.Clone()
	b.mu.Unlock()
	b.emit()
}
func (b *Backend) AuditFullStackBestPractices(id string, options quality.AuditOptions) (quality.AuditReport, error) {
	ctx, done, err := b.auditOperation(id)
	if err != nil {
		return quality.AuditReport{}, err
	}
	defer done()
	project, err := b.project(id, true)
	if err != nil {
		return quality.AuditReport{}, err
	}
	report := quality.RunAudit(ctx, project.PHPBinary, quality.DetectProjectContext(project.Path), options, func(report quality.AuditReport) { b.publishAudit(id, report) })
	report.ProjectID = id
	report.ComparisonKey = companionAuditKey(project, options, report)
	b.publishAudit(id, report)
	if err := b.persistAudit(project, report); err != nil {
		return report, fmt.Errorf("Audit completed but history could not be saved: %w", err)
	}
	return report, nil
}
func (b *Backend) CancelAudit(id string) {
	b.mu.Lock()
	cancel := b.auditCancels[id]
	b.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (b *Backend) PreviewAuditSetup(id, kind, manager string) (quality.AuditSetupPlan, error) {
	project, err := b.project(id, false)
	if err != nil {
		return quality.AuditSetupPlan{}, err
	}
	plan, err := quality.PreviewAuditSetup(project.Path, project.PHPBinary, kind, manager)
	if err != nil {
		return plan, err
	}
	plan.ProjectID = id
	plan.ID = quality.SetupFingerprint(plan)
	b.mu.Lock()
	b.auditSetups[plan.ID] = plan
	b.mu.Unlock()
	return plan, nil
}
func (b *Backend) ApplyAuditSetup(previewID string) (err error) {
	b.mu.Lock()
	plan, ok := b.auditSetups[previewID]
	b.mu.Unlock()
	if !ok {
		return fmt.Errorf("Preview expired; create a fresh setup preview")
	}
	if _, err := b.project(plan.ProjectID, true); err != nil {
		return err
	}
	parent, done, err := b.beginOperation()
	if err != nil {
		return err
	}
	defer done()
	if !b.dependencyInstallMu.TryLock() {
		return fmt.Errorf("Another installation is running")
	}
	defer b.dependencyInstallMu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	b.mu.Lock()
	delete(b.auditSetups, previewID)
	b.installCancel = cancel
	b.installation = DependencyInstallation{ID: plan.Kind, Name: plan.Kind + " audit setup", ProjectID: plan.ProjectID, Running: true, State: "installing", StartedAt: time.Now().UTC().Format(time.RFC3339), Message: "Applying reviewed setup; existing configuration is preserved"}
	b.mu.Unlock()
	b.emit()
	defer func() {
		cancelled := ctx.Err() != nil
		cancel()
		b.mu.Lock()
		b.installCancel = nil
		b.installation.Running = false
		b.installation.State = "completed"
		b.installation.Message = "Audit setup finished; run the selected analyzer to verify coverage"
		if err != nil {
			b.installation.State = "failed"
			b.installation.Message = quality.SanitizeAuditText(err.Error())
			if cancelled {
				b.installation.State = "cancelled"
			}
		}
		b.mu.Unlock()
		b.emit()
	}()
	return quality.ApplyAuditSetup(ctx, plan, func(message string) {
		b.mu.Lock()
		b.installation.Output = append(b.installation.Output, quality.SanitizeAuditText(message))
		if len(b.installation.Output) > 20 {
			b.installation.Output = b.installation.Output[len(b.installation.Output)-20:]
		}
		b.mu.Unlock()
		b.emit()
	})
}
func (b *Backend) OpenAuditBrowser(id string) (quality.BrowserSessionStatus, error) {
	project, err := b.project(id, true)
	if err != nil {
		return quality.BrowserSessionStatus{}, err
	}
	parent, done, err := b.beginOperation()
	if err != nil {
		return quality.BrowserSessionStatus{}, err
	}
	defer done()
	// Serialize creation with other app-owned process starts and shutdown.
	b.startMu.Lock()
	defer b.startMu.Unlock()
	b.mu.Lock()
	previous := b.auditBrowsers[id]
	b.mu.Unlock()
	if previous != nil && previous.Status().Running {
		return previous.Status(), nil
	}
	session, err := quality.StartBrowserSession(parent, project.URL, false)
	if err != nil {
		return quality.BrowserSessionStatus{}, err
	}
	b.mu.Lock()
	b.auditBrowsers[id] = session
	b.mu.Unlock()
	go func() { <-session.Done(); b.emit() }()
	b.emit()
	return session.Status(), nil
}
func (b *Backend) CloseAuditBrowser(id string) {
	b.mu.Lock()
	session := b.auditBrowsers[id]
	b.mu.Unlock()
	if session != nil {
		session.Close()
	}
	b.emit()
}
func (b *Backend) AuditBrowserPage(id, address string) (quality.AuditReport, error) {
	ctx, done, err := b.auditOperation(id)
	if err != nil {
		return quality.AuditReport{}, err
	}
	defer done()
	b.mu.Lock()
	session := b.auditBrowsers[id]
	report := b.auditReports[id].Clone()
	b.mu.Unlock()
	if session == nil || !session.Status().Running {
		return report, fmt.Errorf("Open the temporary audit browser first")
	}
	stop := context.AfterFunc(ctx, session.Close)
	defer stop()
	report.Running = true
	report.StartedAt = time.Now().UTC().Format(time.RFC3339)
	report.FinishedAt = ""
	b.publishAudit(id, report)
	result, findings := session.Audit(ctx, address)
	if ctx.Err() != nil {
		result.Status = "cancelled"
		result.Message = "Browser audit cancelled; temporary session closed"
	}
	// Keep the latest browser result, together with completed static engine results.
	kept := report.Analyzers[:0]
	for _, item := range report.Analyzers {
		if item.ID != "axe" {
			kept = append(kept, item)
		}
	}
	report.Analyzers = append(kept, result)
	retained := report.Findings[:0]
	for _, item := range report.Findings {
		if !strings.EqualFold(item.Tool, "axe-core") {
			retained = append(retained, item)
		}
	}
	report.Findings = append(retained, findings...)
	for i := range report.Findings {
		report.Findings[i].Message = quality.SanitizeAuditText(report.Findings[i].Message)
	}
	report.Running = false
	report.FinishedAt = time.Now().UTC().Format(time.RFC3339)
	report.ProjectID = id
	b.publishAudit(id, report)
	return report, nil
}
