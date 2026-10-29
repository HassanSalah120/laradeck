package webhooks

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebhookManagerIngestAndReplay(t *testing.T) {
	var lastReceivedBody string
	var lastReceivedHeader string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		lastReceivedBody = string(body)
		lastReceivedHeader = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer ts.Close()

	received := make(chan WebhookDelivery, 1)
	wm := NewWebhookManager(10, func(d WebhookDelivery) {
		received <- d
	})

	headers := map[string][]string{
		"X-Custom": {"test-value"},
	}
	del := wm.Ingest("stripe-ep", "POST", "127.0.0.1", headers, `{"event":"charge"}`)

	if del.EndpointID != "stripe-ep" {
		t.Fatalf("expected endpoint stripe-ep, got %s", del.EndpointID)
	}

	deliveries := wm.GetDeliveries("stripe-ep")
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}

	// Replay
	res, err := wm.Replay(context.Background(), del.ID, ts.URL)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
	if lastReceivedBody != `{"event":"charge"}` {
		t.Fatalf("expected body to match, got %s", lastReceivedBody)
	}
	if lastReceivedHeader != "test-value" {
		t.Fatalf("expected header to match, got %s", lastReceivedHeader)
	}
}

func TestComputeSignature(t *testing.T) {
	k, v := ComputeSignature("stripe", "whsec_test", `{"id":1}`, 1600000000)
	if k != "Stripe-Signature" || !strings.Contains(v, "t=1600000000,v1=") {
		t.Fatalf("unexpected stripe signature: %s: %s", k, v)
	}

	k, v = ComputeSignature("github", "secret", `{"ref":"main"}`, 0)
	if k != "X-Hub-Signature-256" || !strings.HasPrefix(v, "sha256=") {
		t.Fatalf("unexpected github signature: %s: %s", k, v)
	}
}

func TestWebhookDiagnosticsAndValidation(t *testing.T) {
	// 1. Test Status Code Explanations
	exp419 := ExplainStatusCode(419)
	if !strings.Contains(exp419, "CSRF Token Mismatch") {
		t.Fatalf("unexpected explanation for 419: %s", exp419)
	}

	exp302 := ExplainStatusCode(302)
	if !strings.Contains(exp302, "auth middleware") {
		t.Fatalf("unexpected explanation for 302: %s", exp302)
	}

	// 2. Test Signature Validation
	payload := `{"id":"evt_123"}`
	secret := "whsec_test_secret"
	headerK, headerV := ComputeSignature("stripe", secret, payload, 1600000000)
	if headerK != "Stripe-Signature" {
		t.Fatalf("unexpected header: %s", headerK)
	}

	val := ValidateDeliverySignature("stripe", secret, payload, headerV)
	if val != "valid" {
		t.Fatalf("expected valid signature, got %s", val)
	}

	valBad := ValidateDeliverySignature("stripe", "wrong_secret", payload, headerV)
	if valBad != "mismatch" {
		t.Fatalf("expected signature mismatch, got %s", valBad)
	}

	// 3. Test Route Inspection
	tmp := t.TempDir()
	routes := []struct{ Method, URI, Name, Action string }{
		{Method: "POST", URI: "/stripe/webhook", Name: "stripe.webhook", Action: "StripeController@handle"},
		{Method: "GET", URI: "/about", Name: "about", Action: "PageController@about"},
	}

	inspections := InspectWebhookRoutes(tmp, routes)
	if len(inspections) != 1 {
		t.Fatalf("expected 1 webhook route inspection, got %d", len(inspections))
	}
	if inspections[0].Provider != "stripe" || !inspections[0].CsrfBlocked {
		t.Fatalf("unexpected inspection result: %+v", inspections[0])
	}
}
