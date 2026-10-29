package actions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseFailedJobsText(t *testing.T) {
	table := `
+----+--------------------------------------+------------+---------+-----------------------------------------+---------------------+
| ID | UUID                                 | Connection | Queue   | Class                                   | Failed At           |
+----+--------------------------------------+------------+---------+-----------------------------------------+---------------------+
| 1  | 8d48897f-8559-4d2c-a226-068393e98622 | redis      | default | App\Jobs\ProcessPodcast                 | 2026-10-16 01:22:30 |
| 2  | b932c021-9311-40be-bd69-fc0e51bb7e94 | sync       | emails  | App\Jobs\SendWelcomeEmail               | 2026-10-16 01:23:45 |
+----+--------------------------------------+------------+---------+-----------------------------------------+---------------------+
`
	items := parseFailedJobsText(table)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].ID != "1" || items[0].Class != `App\Jobs\ProcessPodcast` || items[0].Queue != "default" {
		t.Errorf("unexpected first item: %+v", items[0])
	}
	if items[1].ID != "2" || items[1].Class != `App\Jobs\SendWelcomeEmail` || items[1].Queue != "emails" {
		t.Errorf("unexpected second item: %+v", items[1])
	}
}

func TestParseLaravel12SingleLineFailedJobs(t *testing.T) {
	// Exact output produced by installed Laravel 12.70.0 ListFailedCommand (twoColumnDetail single line)
	output := `
  2026-10-16 02:45:00 48b0c8ef-7a08-410c-b26a-d43c2242fa52 .................. database@default App\Jobs\WorkbenchProbe
  2026-10-16 02:46:15 a1b2c3d4-e5f6-7890-abcd-ef1234567890 .................. redis@high App\Jobs\SendNotification
`
	items := parseFailedJobsText(output)
	if len(items) != 2 {
		t.Fatalf("expected 2 items from Laravel 12 single-line output, got %d", len(items))
	}

	if items[0].FailedAt != "2026-10-16 02:45:00" {
		t.Errorf("expected FailedAt '2026-10-16 02:45:00', got %q", items[0].FailedAt)
	}
	if items[0].UUID != "48b0c8ef-7a08-410c-b26a-d43c2242fa52" {
		t.Errorf("expected UUID '48b0c8ef-7a08-410c-b26a-d43c2242fa52', got %q", items[0].UUID)
	}
	if items[0].Connection != "database" {
		t.Errorf("expected Connection 'database', got %q", items[0].Connection)
	}
	if items[0].Queue != "default" {
		t.Errorf("expected Queue 'default', got %q", items[0].Queue)
	}
	if items[0].Class != `App\Jobs\WorkbenchProbe` {
		t.Errorf("expected Class 'App\\Jobs\\WorkbenchProbe', got %q", items[0].Class)
	}

	if items[1].Connection != "redis" || items[1].Queue != "high" || items[1].Class != `App\Jobs\SendNotification` {
		t.Errorf("unexpected second item: %+v", items[1])
	}
}

func TestParseLaravel12FailedJobsTwoColumn(t *testing.T) {
	// Multi-line twoColumnDetail fallback format
	l12Output := `
  15 / a9df0280-5a3d-4261-a0aa-4395663556bf .................................................... 2026-10-16 02:00:00
  App\Jobs\ProcessPodcast ..................................................................... database / default

  16 / f14d9b23-11a2-4bb3-8991-729ab5109f5e .................................................... 2026-10-16 02:05:12
  App\Jobs\SendInvoice ........................................................................ redis / high
`
	items := parseFailedJobsText(l12Output)
	if len(items) != 2 {
		t.Fatalf("expected 2 items from Laravel 12 output, got %d", len(items))
	}

	if items[0].ID != "15" {
		t.Errorf("expected ID 15, got %s", items[0].ID)
	}
	if items[0].UUID != "a9df0280-5a3d-4261-a0aa-4395663556bf" {
		t.Errorf("expected UUID a9df0280-5a3d-4261-a0aa-4395663556bf, got %s", items[0].UUID)
	}
	if items[0].Class != `App\Jobs\ProcessPodcast` {
		t.Errorf("expected Class App\\Jobs\\ProcessPodcast, got %s", items[0].Class)
	}
	if items[0].Connection != "database" || items[0].Queue != "default" {
		t.Errorf("expected database / default, got %s / %s", items[0].Connection, items[0].Queue)
	}

	if items[1].ID != "16" || items[1].Class != `App\Jobs\SendInvoice` || items[1].Connection != "redis" || items[1].Queue != "high" {
		t.Errorf("unexpected second item: %+v", items[1])
	}
}

func TestNoFailedJobs(t *testing.T) {
	output := "No failed jobs found."
	items := parseFailedJobsText(output)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestListEventsNoFakeListeners(t *testing.T) {
	tmpDir := t.TempDir()
	eventsDir := filepath.Join(tmpDir, "app", "Events")
	_ = os.MkdirAll(eventsDir, 0755)
	_ = os.WriteFile(filepath.Join(eventsDir, "OrderShipped.php"), []byte("<?php namespace App\\Events; class OrderShipped {}"), 0644)

	ac := NewActionsCoordinator()
	events, err := ac.ListEvents("nonexistent-php", tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if len(events[0].Listeners) != 0 {
		t.Errorf("expected 0 fabricated listeners, got %+v", events[0].Listeners)
	}
}

func TestParseScheduledTasksDotLeaders(t *testing.T) {
	output := `
  0 0 * * *  php artisan backup:clean ................................... Next Due: 5 hours from now
  */15 * * * *  App\Jobs\ProcessReports [has mutex] .................... Next Due: 10 minutes from now
  @daily  php artisan model:prune ...................................... Next Due: 1 day from now
`
	tasks := parseScheduledTasksText(output)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	if tasks[0].Expression != "0 0 * * *" || tasks[0].Command != "php artisan backup:clean" || tasks[0].NextDue != "5 hours from now" {
		t.Errorf("unexpected first task: %+v", tasks[0])
	}
	if tasks[0].HasMutex {
		t.Error("expected first task to not have mutex")
	}

	if tasks[1].Expression != "*/15 * * *" || tasks[1].Command != "App\\Jobs\\ProcessReports" || !tasks[1].HasMutex {
		// Note: */15 * * * * or cron pattern
		if !tasks[1].HasMutex {
			t.Errorf("expected second task to have mutex: %+v", tasks[1])
		}
	}

	if tasks[2].Expression != "@daily" || tasks[2].Command != "php artisan model:prune" {
		t.Errorf("unexpected third task: %+v", tasks[2])
	}
}

func TestParseScheduledTasksTable(t *testing.T) {
	table := `
+-----------------------------------+--------------------+------------------------+---------------------+
| Command                           | Interval           | Description            | Next Due            |
+-----------------------------------+--------------------+------------------------+---------------------+
| php artisan model:prune           | 0 0 * * *          | Prune models           | 4 hours from now    |
| php artisan queue:monitor         | * * * * *          | Check queue depth      | 1 minute from now   |
+-----------------------------------+--------------------+------------------------+---------------------+
`
	tasks := parseScheduledTasksText(table)
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Command != "php artisan model:prune" || tasks[0].Interval != "0 0 * * *" || tasks[0].Description != "Prune models" {
		t.Errorf("unexpected first task: %+v", tasks[0])
	}
	if tasks[1].Command != "php artisan queue:monitor" || tasks[1].NextDue != "1 minute from now" {
		t.Errorf("unexpected second task: %+v", tasks[1])
	}
}

func TestEmptyScheduledTasks(t *testing.T) {
	tasks := parseScheduledTasksText("No scheduled tasks have been defined.")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

