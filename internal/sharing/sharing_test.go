package sharing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateHMACSignature(t *testing.T) {
	secret := "whsec_test_secret"
	payload := `{"event":"order.completed","amount":2500}`

	// GitHub signature
	githubSig := GenerateHMACSignature(secret, payload, "github")
	if !strings.HasPrefix(githubSig, "sha256=") {
		t.Errorf("expected sha256= prefix for github sig, got %s", githubSig)
	}

	// Stripe signature
	stripeSig := GenerateHMACSignature(secret, payload, "stripe")
	if !strings.HasPrefix(stripeSig, "t=") || !strings.Contains(stripeSig, ",v1=") {
		t.Errorf("expected t=...,v1=... format for stripe sig, got %s", stripeSig)
	}
}

func TestDispatchWebhook(t *testing.T) {
	var receivedSig string
	var receivedPayload string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get("Stripe-Signature")
		b := make([]byte, 1024)
		n, _ := r.Body.Read(b)
		receivedPayload = string(b[:n])

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"received":true}`))
	}))
	defer server.Close()

	scenario := WebhookScenario{
		ID:        "scen_1",
		Name:      "Test Stripe Webhook",
		Provider:  "stripe",
		TargetURL: server.URL,
		Secret:    "whsec_12345",
		Payload:   `{"type":"payment_intent.succeeded"}`,
	}

	res, err := DispatchWebhook(context.Background(), scenario)
	if err != nil {
		t.Fatalf("DispatchWebhook failed: %v", err)
	}
	if !res.Success || res.StatusCode != 200 {
		t.Errorf("expected success HTTP 200, got %d", res.StatusCode)
	}
	if receivedSig == "" || !strings.HasPrefix(receivedSig, "t=") {
		t.Errorf("server did not receive valid Stripe-Signature header: %s", receivedSig)
	}
	if !strings.Contains(receivedPayload, "payment_intent.succeeded") {
		t.Errorf("server did not receive expected payload: %s", receivedPayload)
	}
}

func TestDetectIntegrations(t *testing.T) {
	envKeys := map[string]string{
		"STRIPE_KEY":    "pk_test_12345",
		"STRIPE_SECRET": "sk_test_67890",
		"MAIL_MAILER":   "smtp",
		"AWS_BUCKET":    "my-project-assets",
	}

	items := DetectIntegrations(envKeys)
	if len(items) != 3 {
		t.Fatalf("expected 3 detected integrations, got %d", len(items))
	}

	var foundStripe bool
	for _, it := range items {
		if it.Name == "Stripe" && it.Environment == "test-mode" {
			foundStripe = true
		}
	}
	if !foundStripe {
		t.Errorf("Stripe test-mode not detected properly")
	}
}

func TestStartTunnelValidation(t *testing.T) {
	tm := NewTunnelManager()

	// 1. Rejects admin port
	_, err := tm.StartTunnel("ngrok", "http://127.0.0.1:8025")
	if err == nil || !strings.Contains(err.Error(), "administrative services") {
		t.Errorf("expected error rejecting administrative port 8025, got: %v", err)
	}

	// 2. Rejects invalid or empty provider
	_, err = tm.StartTunnel("invalid-provider", "http://127.0.0.1:8000")
	if err == nil {
		t.Errorf("expected error for unsupported tunnel provider, got nil")
	}

	// 3. Rejects non-local exposure targets (e.g. public domains)
	_, err = tm.StartTunnel("ngrok", "https://external-site.com")
	if err == nil || !strings.Contains(err.Error(), "exposure policy") {
		t.Errorf("expected error rejecting external non-local domain, got: %v", err)
	}

	// 4. Status is not running initially
	status := tm.GetStatus()
	if status.Running {
		t.Errorf("expected initial status Running=false, got true")
	}

	// 5. StopTunnel(false) on idle manager is safe
	if err := tm.StopTunnel(false); err != nil {
		t.Errorf("unexpected error stopping idle tunnel: %v", err)
	}
}
