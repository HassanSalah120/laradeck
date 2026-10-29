package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type WebhookRouteInspection struct {
	URI              string `json:"uri"`
	Name             string `json:"name"`
	Action           string `json:"action"`
	Provider         string `json:"provider"`         // "stripe", "github", "shopify", "generic"
	CsrfBlocked      bool   `json:"csrfBlocked"`      // true if not excluded
	CsrfFix          string `json:"csrfFix"`          // code snippet to copy
	AuthBlocked      bool   `json:"authBlocked"`      // true if trapped behind auth
	SecretConfigured bool   `json:"secretConfigured"` // true if secret is set in .env
	SecretKeyName    string `json:"secretKeyName"`
}

type WebhookDelivery struct {
	ID         string              `json:"id"`
	EndpointID string              `json:"endpointId"`
	Method     string              `json:"method"`
	ClientIP   string              `json:"clientIp"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
	ReceivedAt string              `json:"receivedAt"`
}

type WebhookReplayResult struct {
	DeliveryID string  `json:"deliveryId"`
	TargetURL  string  `json:"targetUrl"`
	StatusCode int     `json:"statusCode"`
	DurationMs float64 `json:"durationMs"`
	Response   string  `json:"response"`
	Error      string  `json:"error,omitempty"`
}

type WebhookManager struct {
	mu          sync.RWMutex
	deliveries  []WebhookDelivery
	maxItems    int
	onDelivery  func(WebhookDelivery)
}

func NewWebhookManager(maxItems int, onDelivery func(WebhookDelivery)) *WebhookManager {
	if maxItems <= 0 {
		maxItems = 100
	}
	return &WebhookManager{
		maxItems:   maxItems,
		deliveries: make([]WebhookDelivery, 0, maxItems),
		onDelivery: onDelivery,
	}
}

// Ingest captures an incoming webhook delivery
func (wm *WebhookManager) Ingest(endpointID, method, clientIP string, headers map[string][]string, body string) WebhookDelivery {
	del := WebhookDelivery{
		ID:         fmt.Sprintf("wh_%d", time.Now().UnixNano()),
		EndpointID: endpointID,
		Method:     method,
		ClientIP:   clientIP,
		Headers:    headers,
		Body:       body,
		ReceivedAt: time.Now().UTC().Format(time.RFC3339),
	}

	wm.mu.Lock()
	if len(wm.deliveries) >= wm.maxItems {
		wm.deliveries = wm.deliveries[1:]
	}
	wm.deliveries = append(wm.deliveries, del)
	cb := wm.onDelivery
	wm.mu.Unlock()

	if cb != nil {
		cb(del)
	}

	return del
}

func (wm *WebhookManager) GetDeliveries(endpointID string) []WebhookDelivery {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	res := make([]WebhookDelivery, 0, len(wm.deliveries))
	for _, d := range wm.deliveries {
		if endpointID == "" || d.EndpointID == endpointID {
			res = append(res, d)
		}
	}
	return res
}

func (wm *WebhookManager) Clear(endpointID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if endpointID == "" {
		wm.deliveries = make([]WebhookDelivery, 0, wm.maxItems)
		return
	}

	filtered := make([]WebhookDelivery, 0, len(wm.deliveries))
	for _, d := range wm.deliveries {
		if d.EndpointID != endpointID {
			filtered = append(filtered, d)
		}
	}
	wm.deliveries = filtered
}

// Replay dispatches a previously captured webhook delivery to a target local route
func (wm *WebhookManager) Replay(ctx context.Context, deliveryID, targetURL string) (*WebhookReplayResult, error) {
	wm.mu.RLock()
	var found *WebhookDelivery
	for _, d := range wm.deliveries {
		if d.ID == deliveryID {
			item := d
			found = &item
			break
		}
	}
	wm.mu.RUnlock()

	if found == nil {
		return nil, fmt.Errorf("webhook delivery %s not found", deliveryID)
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, found.Method, targetURL, bytes.NewBufferString(found.Body))
	if err != nil {
		return nil, err
	}

	for k, vv := range found.Headers {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	dur := float64(time.Since(start).Milliseconds())

	if err != nil {
		return &WebhookReplayResult{
			DeliveryID: deliveryID,
			TargetURL:  targetURL,
			StatusCode: 0,
			DurationMs: dur,
			Error:      err.Error(),
		}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	return &WebhookReplayResult{
		DeliveryID: deliveryID,
		TargetURL:  targetURL,
		StatusCode: resp.StatusCode,
		DurationMs: dur,
		Response:   string(respBody),
	}, nil
}

// ComputeSignature calculates the webhook signature for various third-party providers
func ComputeSignature(provider, secret, payload string, timestamp int64) (headerKey, headerVal string) {
	if timestamp <= 0 {
		timestamp = time.Now().Unix()
	}

	switch provider {
	case "stripe":
		signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(signedPayload))
		sig := hex.EncodeToString(mac.Sum(nil))
		return "Stripe-Signature", fmt.Sprintf("t=%d,v1=%s", timestamp, sig)

	case "github":
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		sig := hex.EncodeToString(mac.Sum(nil))
		return "X-Hub-Signature-256", fmt.Sprintf("sha256=%s", sig)

	case "shopify":
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		return "X-Shopify-Hmac-Sha256", sig

	default: // generic HMAC-SHA256
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		sig := hex.EncodeToString(mac.Sum(nil))
		return "X-Signature-SHA256", sig
	}
}

// DispatchSignedTestEvent sends a template test webhook directly to target application
func DispatchSignedTestEvent(ctx context.Context, targetURL, provider, secret, payload string) (*WebhookReplayResult, error) {
	if payload == "" {
		payload = GetDefaultPayloadTemplate(provider)
	}

	headerK, headerV := ComputeSignature(provider, secret, payload, time.Now().Unix())

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewBufferString(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerK, headerV)
	req.Header.Set("User-Agent", fmt.Sprintf("LaraDeck-Webhook-Workbench/%s", provider))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	dur := float64(time.Since(start).Milliseconds())

	if err != nil {
		return &WebhookReplayResult{
			TargetURL:  targetURL,
			StatusCode: 0,
			DurationMs: dur,
			Error:      err.Error(),
		}, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	return &WebhookReplayResult{
		TargetURL:  targetURL,
		StatusCode: resp.StatusCode,
		DurationMs: dur,
		Response:   string(respBody),
	}, nil
}

// GetDefaultPayloadTemplate returns realistic test webhook JSON templates
func GetDefaultPayloadTemplate(provider string) string {
	switch provider {
	case "stripe":
		return `{
  "id": "evt_test_payment_succeeded_123",
  "object": "event",
  "type": "payment_intent.succeeded",
  "data": {
    "object": {
      "id": "pi_3MtwBwLkdIwHu7ix28a3tqPa",
      "amount": 4900,
      "currency": "usd",
      "status": "succeeded"
    }
  }
}`
	case "github":
		return `{
  "ref": "refs/heads/main",
  "repository": {
    "name": "laradeck",
    "full_name": "developer/laradeck"
  },
  "pusher": {
    "name": "octocat"
  }
}`
	case "shopify":
		return `{
  "id": 820982911946154508,
  "email": "jon@example.com",
  "total_price": "199.00",
  "currency": "USD",
  "financial_status": "paid"
}`
	default:
		obj := map[string]interface{}{
			"event":     "ping",
			"timestamp": time.Now().Unix(),
			"message":   "Test event from LaraDeck Webhook Workbench",
		}
		data, _ := json.MarshalIndent(obj, "", "  ")
		return string(data)
	}
}

// ExplainStatusCode returns a plain English diagnostic explanation for webhook HTTP response codes
func ExplainStatusCode(code int) string {
	switch code {
	case 200, 201, 202, 204:
		return "Delivery acknowledged successfully by your application (2xx OK)."
	case 301, 302, 307, 308:
		return "Redirect received. Webhook providers never follow login redirects; this route is likely trapped inside the auth middleware group."
	case 401, 403:
		return "Unauthorized or Forbidden. The cryptographic signature did not match the secret in .env or provider access was denied."
	case 404:
		return "Not Found. The webhook route is not registered or points to a non-existent URL."
	case 405:
		return "Method Not Allowed. Webhooks must accept HTTP POST requests."
	case 419:
		return "CSRF Token Mismatch (419). Laravel blocked the delivery because CSRF validation is active for this route. Exclude it in bootstrap/app.php."
	case 500:
		return "Internal Server Error. An unhandled exception was thrown inside your webhook handler or controller. Check the Debugger tab for stack trace."
	case 502, 503, 504:
		return "Gateway or Server Timeout. PHP did not respond in time or crashed while processing the webhook."
	default:
		if code == 0 {
			return "Connection Failed. Your local site could not be reached (offline or port closed)."
		}
		return fmt.Sprintf("Application answered with HTTP status %d.", code)
	}
}

// ValidateDeliverySignature checks an incoming webhook delivery signature against the configured secret
func ValidateDeliverySignature(provider, secret, payload, headerSig string) string {
	if secret == "" {
		return "not_checked"
	}
	if headerSig == "" {
		return "not_signed"
	}

	switch provider {
	case "stripe":
		// Stripe format: t=123456,v1=abcdef...
		parts := strings.Split(headerSig, ",")
		var timestampStr, signature string
		for _, part := range parts {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 {
				if kv[0] == "t" {
					timestampStr = kv[1]
				} else if kv[0] == "v1" {
					signature = kv[1]
				}
			}
		}
		if timestampStr == "" || signature == "" {
			return "mismatch"
		}

		signedPayload := fmt.Sprintf("%s.%s", timestampStr, payload)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(signedPayload))
		expected := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(signature), []byte(expected)) {
			return "valid"
		}
		return "mismatch"

	case "github":
		// GitHub format: sha256=abcdef...
		sig := strings.TrimPrefix(headerSig, "sha256=")
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		expected := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return "valid"
		}
		return "mismatch"

	case "shopify":
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(headerSig), []byte(expected)) {
			return "valid"
		}
		return "mismatch"

	default:
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(payload))
		expected := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(headerSig), []byte(expected)) {
			return "valid"
		}
		return "mismatch"
	}
}

// InspectWebhookRoutes scans project routes and identifies potential webhook handlers with diagnostics
func InspectWebhookRoutes(projectPath string, routes []struct{ Method, URI, Name, Action string }) []WebhookRouteInspection {
	var inspections []WebhookRouteInspection

	// Read bootstrap/app.php or VerifyCsrfToken to check for CSRF exemptions
	appPhpContent := ""
	if data, err := os.ReadFile(filepath.Join(projectPath, "bootstrap", "app.php")); err == nil {
		appPhpContent = string(data)
	}

	// Read .env to check configured secrets
	envContent := ""
	if data, err := os.ReadFile(filepath.Join(projectPath, ".env")); err == nil {
		envContent = string(data)
	}

	specificProviders := []string{"stripe", "github", "shopify", "paddle", "lemonsqueezy", "svix", "slack"}
	generalKeywords := []string{"webhook", "hook", "callback"}

	for _, r := range routes {
		if r.Method != "POST" && !strings.Contains(r.Method, "POST") {
			continue
		}

		lowerURI := strings.ToLower(r.URI)
		lowerName := strings.ToLower(r.Name)
		lowerAction := strings.ToLower(r.Action)

		isWebhook := false
		provider := "generic"

		// Check specific providers first
		for _, sp := range specificProviders {
			if strings.Contains(lowerURI, sp) || strings.Contains(lowerName, sp) || strings.Contains(lowerAction, sp) {
				isWebhook = true
				provider = sp
				break
			}
		}

		// If not a specific provider, check general keywords
		if !isWebhook {
			for _, kw := range generalKeywords {
				if strings.Contains(lowerURI, kw) || strings.Contains(lowerName, kw) || strings.Contains(lowerAction, kw) {
					isWebhook = true
					break
				}
			}
		}

		if !isWebhook {
			continue
		}

		// Check CSRF exemption
		cleanURI := strings.TrimPrefix(r.URI, "/")
		csrfBlocked := true
		if strings.Contains(appPhpContent, cleanURI) || strings.Contains(appPhpContent, cleanURI+"/*") || strings.Contains(appPhpContent, "validateCsrfTokens(except: [") && strings.Contains(appPhpContent, strings.Split(cleanURI, "/")[0]) {
			csrfBlocked = false
		}

		csrfFix := fmt.Sprintf(`->withMiddleware(function (Middleware $middleware) {
    $middleware->validateCsrfTokens(except: [
        '%s',
    ]);
})`, cleanURI)

		// Check Auth middleware
		authBlocked := strings.Contains(lowerAction, "auth") || strings.Contains(r.Name, "auth.")

		// Check secret configuration
		secretKey := "WEBHOOK_SECRET"
		if provider != "generic" {
			secretKey = strings.ToUpper(provider) + "_WEBHOOK_SECRET"
		}
		secretConfigured := strings.Contains(envContent, secretKey+"=") && !strings.Contains(envContent, secretKey+"=\n")

		inspections = append(inspections, WebhookRouteInspection{
			URI:              r.URI,
			Name:             r.Name,
			Action:           r.Action,
			Provider:         provider,
			CsrfBlocked:      csrfBlocked,
			CsrfFix:          csrfFix,
			AuthBlocked:      authBlocked,
			SecretConfigured: secretConfigured,
			SecretKeyName:    secretKey,
		})
	}

	return inspections
}
