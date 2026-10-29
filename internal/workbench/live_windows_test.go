package workbench

import (
	"context"
	"crypto/sha256"
	"fmt"
	"herdworkbench/internal/actions"
	"herdworkbench/internal/mail"
	"herdworkbench/internal/quality"
	"herdworkbench/internal/recipes"
	wktesting "herdworkbench/internal/testing"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"herdworkbench/internal/runner"
)

// Opt-in only: this executes project code. Use a disposable fixture, never a user site.
func TestLiveDisposableFixture(t *testing.T) {
	fixture := os.Getenv("WORKBENCH_LIVE_FIXTURE")
	if fixture == "" {
		t.Skip("set WORKBENCH_LIVE_FIXTURE and WORKBENCH_RUNNER for the disposable Laravel fixture")
	}
	if filepath.Base(fixture) != "laravel-fixture" {
		t.Fatal("fixture must be the disposable laravel-fixture")
	}
	h, err := FindHerd()
	if err != nil {
		t.Fatal(err)
	}
	projects, err := h.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var p Project
	for _, site := range projects {
		if strings.EqualFold(filepath.Clean(site.Path), filepath.Clean(fixture)) {
			p = site
		}
	}
	if p.ID == "" || p.PHPBinary == "" {
		t.Fatal("fixture discovery or per-project PHP resolution failed")
	}
	t.Logf("Herd discovery: %d sites; fixture Laravel=%s Clockwork=%s PHP=%s", len(projects), p.Framework, p.Clockwork, p.PHP)
	b := NewBackend(nil)
	t.Cleanup(b.Shutdown)
	b.projects = projects
	if _, err = b.Start(p.ID, "queue-once"); err == nil {
		t.Fatal("untrusted project execution allowed")
	}
	wanted := map[string]bool{"route:list": true, "about": true, "queue:failed": false, "channel:list": false}
	for _, cap := range Capabilities(p) {
		if cap.Status != "verified" || cap.JSON != wanted[cap.Command] {
			t.Fatalf("unexpected installed capability: %+v", cap)
		}
		t.Logf("%s: json=%t", cap.Command, cap.JSON)
	}
	routes, err := Routes(p)
	if err != nil || len(routes) < 4 {
		t.Fatalf("routes=%d err=%v", len(routes), err)
	}
	client := http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		_, port, _ := net.SplitHostPort(address)
		return (&net.Dialer{}).DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
	}}}
	defer client.CloseIdleConnections()
	visit := func(path string) {
		resp, err := client.Get(strings.TrimRight(p.URL, "/") + path)
		if err != nil {
			t.Error(err)
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	visit("/spike")
	c := NewClockwork(nil)
	c.Select(p)
	s, err := c.Refresh(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range s.Requests {
		for _, q := range r.Queries {
			if strings.Contains(q.SQL, "workbench_probe") {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("real SQL probe was not captured")
	}
	dir := filepath.Join(p.Path, "storage", "clockwork")
	fingerprint := func() string {
		entries, _ := os.ReadDir(dir)
		var out strings.Builder
		for _, e := range entries {
			if !e.IsDir() {
				raw, _ := os.ReadFile(filepath.Join(dir, e.Name()))
				fmt.Fprintf(&out, "%s:%x;", e.Name(), sha256.Sum256(raw))
			}
		}
		return out.String()
	}
	before := fingerprint()
	if _, err = c.Refresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(600 * time.Millisecond)
	if before != fingerprint() {
		t.Fatal("metadata read changed Clockwork storage")
	}
	t.Log("Metadata read: zero Clockwork storage changes")
	if err = c.Watch(p); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	time.Sleep(100 * time.Millisecond)
	captureStart := float64(time.Now().UnixNano()) / 1e9
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); visit(fmt.Sprintf("/spike-parallel/%d", i)) }(i)
	}
	wg.Wait()
	visit("/spike-failure")
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		seen := 0
		failed := false
		for _, r := range c.State().Requests {
			if strings.HasPrefix(r.URI, "/spike-parallel/") && r.Time >= captureStart {
				seen++
			}
			if r.URI == "/spike-failure" && r.Status == 500 && r.Time >= captureStart && r.Exceptions > 0 {
				failed = true
			}
		}
		if seen >= 6 && failed {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	seen := 0
	failed := false
	for _, r := range c.State().Requests {
		if strings.HasPrefix(r.URI, "/spike-parallel/") && r.Time >= captureStart {
			seen++
		}
		if r.URI == "/spike-failure" && r.Status == 500 && r.Time >= captureStart && r.Exceptions > 0 {
			failed = true
		}
	}
	if seen < 6 || !failed {
		t.Fatalf("watch missed requests: parallel=%d failure=%t", seen, failed)
	}
	t.Logf("Real file watcher: %d parallel requests and HTTP 500 capture visible", seen)
	os.Remove(filepath.Join(p.Path, "storage", "app", "workbench-job-completed.txt"))
	// Dispatch only the fixture's own inert job; never issue queue:restart.
	_, err = runner.Capture(runner.Spec{Executable: p.PHPBinary, Directory: p.Path, Args: []string{"artisan", "tinker", "--execute=App\\Jobs\\WorkbenchProbe::dispatch();", "--no-interaction"}}, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.Capture(runner.Spec{Executable: p.PHPBinary, Directory: p.Path, Args: []string{"artisan", "queue:work", "--once", "--no-interaction", "--no-ansi"}}, 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(filepath.Join(p.Path, "storage", "app", "workbench-job-completed.txt"))
	if err != nil || string(marker) != "completed" {
		t.Fatal("fixture queue job did not complete")
	}
	t.Log("Queue --once: fixture job completed")
}

func TestLiveToolContracts(t *testing.T) {
	fixture := os.Getenv("WORKBENCH_LIVE_FIXTURE")
	if fixture == "" {
		t.Skip("set WORKBENCH_LIVE_FIXTURE for disposable integration")
	}
	h, err := FindHerd()
	if err != nil {
		t.Fatal(err)
	}
	sites, err := h.Discover()
	if err != nil {
		t.Fatal(err)
	}
	var p Project
	for _, site := range sites {
		if strings.EqualFold(filepath.Clean(site.Path), filepath.Clean(fixture)) {
			p = site
		}
	}
	if p.ID == "" {
		t.Fatal("fixture not found")
	}
	report, err := quality.RunPint(p.PHPBinary, p.Path, true)
	if err != nil || report == nil || report.TotalWarns == 0 {
		t.Fatalf("real Pint findings: report=%+v err=%v", report, err)
	}
	t.Logf("Real Pint parsed %d formatter findings", report.TotalWarns)
	tests, err := wktesting.NewTestCoordinator().RunProjectTests(p.PHPBinary, p.Path, "")
	if err != nil || !tests.Success || tests.Passed < 2 {
		t.Fatalf("real test suite: result=%+v err=%v", tests, err)
	}
	api, err := wktesting.NewTestCoordinator().ExecuteAPIRequest(context.Background(), wktesting.APIRequestSpec{URL: strings.TrimRight(p.URL, "/") + "/spike", Assertions: []wktesting.APIAssertion{{Type: "status_code", Value: "200"}}})
	if err != nil || !api.Passed {
		t.Fatalf("Herd API request: result=%+v err=%v", api, err)
	}
	t.Logf("Real Laravel tests: %d passed; API status=%d", tests.Passed, api.StatusCode)
	failurePath := filepath.Join(fixture, "tests", "Unit", "WorkbenchFailureProbeTest.php")
	failureFile, err := os.OpenFile(failurePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(failurePath) })
	_, writeErr := failureFile.WriteString(`<?php namespace Tests\Unit; class WorkbenchFailureProbeTest extends \PHPUnit\Framework\TestCase { public function test_failure_is_reported(): void { self::assertSame(1, 2, 'workbench expected failure'); } }`)
	_ = failureFile.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	failed, err := wktesting.NewTestCoordinator().RunProjectTests(p.PHPBinary, p.Path, "WorkbenchFailureProbeTest")
	if err != nil || failed.Success || failed.Failed != 1 || !strings.Contains(strings.Join(failed.Output, "\n"), "workbench expected failure") {
		t.Fatalf("failing PHP suite result=%+v err=%v", failed, err)
	}
	t.Log("Real failing PHP test: failure count, unsuccessful status and failure output preserved")
	action, actionErr := actions.NewActionsCoordinator().ExecuteCommand(p.PHPBinary, []string{"-r", `fwrite(STDERR, "expected command failure"); exit(7);`}, p.Path, 10*time.Second)
	if actionErr == nil || action.Success || action.ExitCode != 7 || !strings.Contains(strings.Join(action.Output, "\n"), "expected command failure") {
		t.Fatal("saved command lost actual exit code or stderr")
	}
	t.Log("Real saved-command execution: exit code 7 and stderr preserved")

}
func TestLiveMailpit(t *testing.T) {
	binary := os.Getenv("WORKBENCH_MAILPIT")
	if binary == "" {
		t.Skip("set WORKBENCH_MAILPIT for isolated native Mailpit integration")
	}
	freePort := func() int {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		listener.Close()
		return port
	}
	smtpPort, apiPort := freePort(), freePort()
	proc, err := runner.Start(runner.Spec{Executable: binary, Directory: t.TempDir(), Args: []string{"--smtp", fmt.Sprintf("127.0.0.1:%d", smtpPort), "--listen", fmt.Sprintf("127.0.0.1:%d", apiPort), "--database", "mailpit.db"}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = proc.RequestStop()
		if !proc.Wait(3 * time.Second) {
			_ = proc.ForceStop()
			proc.Wait(3 * time.Second)
		}
		_ = proc.Close()
	}()
	client := mail.NewMailpitClient(fmt.Sprintf("http://127.0.0.1:%d", apiPort))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok, _ := client.CheckHealth(context.Background()); ok {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	message := "From: sender@example.test\r\nTo: receiver@example.test\r\nSubject: Workbench live probe\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n<section>Safe text<script>alert(1)</script><img src=\"https://tracker.example.test/pixel\"></section>\r\n"
	if err := smtp.SendMail(fmt.Sprintf("127.0.0.1:%d", smtpPort), nil, "sender@example.test", []string{"receiver@example.test"}, []byte(message)); err != nil {
		t.Fatal(err)
	}
	mailbox, err := client.GetMessages(context.Background(), "subject:Workbench", 50, 0)
	if err != nil || len(mailbox.Messages) != 1 {
		t.Fatalf("search: mailbox=%+v err=%v", mailbox, err)
	}
	detail, err := client.GetMessage(context.Background(), mailbox.Messages[0].ID)
	if err != nil || strings.Contains(detail.SafeHTML, "<script") || strings.Contains(detail.SafeHTML, ` src="https://tracker`) || !strings.Contains(detail.SafeHTML, "Safe text") {
		t.Fatalf("mail detail: %+v err=%v", detail, err)
	}
	if len(detail.Headers) == 0 {
		t.Error("Mailpit headers missing")
	}
	if err := client.DeleteMessage(context.Background(), detail.ID); err != nil {
		t.Fatal(err)
	}
	mailbox, err = client.GetMessages(context.Background(), "", 50, 0)
	if err != nil || len(mailbox.Messages) != 0 {
		t.Fatal("message was not actually deleted")
	}
	t.Logf("Native Mailpit SMTP → search → sanitized detail → delete verified")
}

func TestLiveProjectWizard(t *testing.T) {
	root := os.Getenv("WORKBENCH_WIZARD_ROOT")
	if root == "" {
		t.Skip("set WORKBENCH_WIZARD_ROOT to create a disposable Laravel project")
	}
	h, err := FindHerd()
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("wizard-%d", time.Now().UnixNano())
	output, err := recipes.ExecuteProjectWizard(recipes.ProjectWizardSpec{Directory: root, Name: name, Starter: "none", Database: "sqlite", PHPBinary: h.PHP})
	if err != nil {
		t.Fatalf("wizard failed: %v\n%s", err, output)
	}
	project := filepath.Join(root, name)
	for _, file := range []string{"artisan", "composer.lock", "vendor/autoload.php", ".env"} {
		if _, err := os.Stat(filepath.Join(project, file)); err != nil {
			t.Fatal(err)
		}
	}
	if bytes, err := runner.Capture(runner.Spec{Executable: h.PHP, Args: []string{"artisan", "about", "--json"}, Directory: project}, 20*time.Second); err != nil || len(bytes) == 0 {
		t.Fatal("generated project cannot boot")
	}
	t.Log("Wizard created and booted a real disposable Laravel 12 project through Herd Composer PHAR")
}
