package activity

import (
	"strings"
	"testing"
)

func TestBuildActivityTrace(t *testing.T) {
	input := RawActivityInput{
		Method:     "POST",
		URI:        "/api/checkout",
		Status:     200,
		Duration:   85.5,
		Memory:     14.2,
		Controller: "CheckoutController@store",
		Middleware: []string{"auth:sanctum", "throttle:api"},
		Queries: []RawQuery{
			{Query: "SELECT * FROM users WHERE id = 1", Duration: 2.1, StartMs: 1.2},
			{Query: "UPDATE carts SET status = 'completed' WHERE id = 42", Duration: 5.4, StartMs: 15.0},
		},
		Events: []RawEvent{
			{Name: "OrderCreated", Duration: 12.0, StartMs: 25.0, Listeners: []string{"SendInvoiceNotification", "UpdateInventory"}},
		},
		Views: []RawView{
			{Name: "emails.invoice", Duration: 4.5, StartMs: 40.0},
		},
	}

	trace := BuildActivityTrace("trace_123", "proj_abc", input)
	if trace.ID != "trace_123" {
		t.Fatalf("expected trace id trace_123, got %s", trace.ID)
	}
	if len(trace.RootNode.Children) < 2 {
		t.Fatalf("expected at least 2 children under root (middleware and controller), got %d", len(trace.RootNode.Children))
	}

	// Verify JSON export
	jsonData, err := ExportTraceJSON(trace)
	if err != nil {
		t.Fatalf("export JSON failed: %v", err)
	}
	if !strings.Contains(string(jsonData), "CheckoutController@store") {
		t.Fatal("expected JSON to contain controller name")
	}

	// Verify HTML export
	html := ExportTraceHTML(trace)
	if !strings.Contains(html, "LaraDeck Activity Trace") {
		t.Fatal("expected HTML to contain title")
	}
}
