package architecture

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildArchitectureCanvas(t *testing.T) {
	tmp := t.TempDir()

	// 1. Controller with raw DB call
	ctrlDir := filepath.Join(tmp, "app", "Http", "Controllers")
	if err := os.MkdirAll(ctrlDir, 0755); err != nil {
		t.Fatal(err)
	}
	ctrlCode := `<?php
namespace App\Http\Controllers;
use Illuminate\Support\Facades\DB;

class OrderController {
    public function index() {
        return DB::select('SELECT * FROM orders');
    }
}
`
	if err := os.WriteFile(filepath.Join(ctrlDir, "OrderController.php"), []byte(ctrlCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Action
	actionDir := filepath.Join(tmp, "app", "Actions")
	if err := os.MkdirAll(actionDir, 0755); err != nil {
		t.Fatal(err)
	}
	actionCode := `<?php
namespace App\Actions;

class ProcessOrderAction {
    public function execute($orderId) {
        return true;
    }
}
`
	if err := os.WriteFile(filepath.Join(actionDir, "ProcessOrderAction.php"), []byte(actionCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Model with HTTP call
	modelDir := filepath.Join(tmp, "app", "Models")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		t.Fatal(err)
	}
	modelCode := `<?php
namespace App\Models;
use Illuminate\Database\Eloquent\Model;
use Illuminate\Support\Facades\Http;

class User extends Model {
    public function syncRemote() {
        Http::post('https://api.example.com/sync');
    }
}
`
	if err := os.WriteFile(filepath.Join(modelDir, "User.php"), []byte(modelCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Job
	jobDir := filepath.Join(tmp, "app", "Jobs")
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		t.Fatal(err)
	}
	jobCode := `<?php
namespace App\Jobs;

class SendReceiptEmail {
    public function handle() {}
}
`
	if err := os.WriteFile(filepath.Join(jobDir, "SendReceiptEmail.php"), []byte(jobCode), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	rep := BuildArchitectureCanvas(ctx, "test-proj", tmp)

	if rep.TotalComponents != 4 {
		t.Fatalf("expected 4 components, got %d", rep.TotalComponents)
	}

	if len(rep.Layers["1"]) != 1 || rep.Layers["1"][0].Name != "OrderController" {
		t.Errorf("expected OrderController in Layer 1")
	}
	if len(rep.Layers["2"]) != 1 || rep.Layers["2"][0].Name != "ProcessOrderAction" {
		t.Errorf("expected ProcessOrderAction in Layer 2")
	}
	if len(rep.Layers["3"]) != 1 || rep.Layers["3"][0].Name != "SendReceiptEmail" {
		t.Errorf("expected SendReceiptEmail in Layer 3")
	}
	if len(rep.Layers["4"]) != 1 || rep.Layers["4"][0].Name != "User" {
		t.Errorf("expected User in Layer 4")
	}

	// Boundary Violations check
	if len(rep.Violations) < 2 {
		t.Fatalf("expected at least 2 violations, got %d", len(rep.Violations))
	}

	hasDbWarn := false
	hasHttpErr := false
	for _, v := range rep.Violations {
		if v.RuleID == "controller-direct-db" {
			hasDbWarn = true
		}
		if v.RuleID == "model-http-call" {
			hasHttpErr = true
		}
	}

	if !hasDbWarn {
		t.Errorf("missing controller-direct-db violation")
	}
	if !hasHttpErr {
		t.Errorf("missing model-http-call violation")
	}

	// Check Mermaid
	if !strings.Contains(rep.MermaidCode, "graph TD") || !strings.Contains(rep.MermaidCode, "OrderController") {
		t.Errorf("mermaid code does not contain expected graph definition: %s", rep.MermaidCode)
	}
}
