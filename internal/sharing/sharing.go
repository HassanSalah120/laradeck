package sharing

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/runner"
	wktesting "herdworkbench/internal/testing"
)

type TunnelStatus struct {
	Running     bool   `json:"running"`
	Provider    string `json:"provider"`
	LocalTarget string `json:"localTarget"`
	PublicURL   string `json:"publicUrl"`
	StartedAt   string `json:"startedAt"`
	PID         int    `json:"pid"`
	Error       string `json:"error,omitempty"`
}

type TunnelManager struct {
	mu            sync.Mutex
	activeProcess *runner.Process
	status        TunnelStatus
}

func NewTunnelManager() *TunnelManager {
	return &TunnelManager{
		status: TunnelStatus{Running: false},
	}
}

// AvailableProviders inspects installed CLIs for tunnel support.
func (tm *TunnelManager) AvailableProviders() []string {
	var providers []string
	if p, err := dependencies.NewManager("", nil).Resolve("cloudflared"); err == nil && p != "" {
		providers = append(providers, "cloudflared")
	}
	if p, err := dependencies.NewManager("", nil).Resolve("expose"); err == nil && p != "" {
		providers = append(providers, "expose")
	}
	if p, err := dependencies.NewManager("", nil).Resolve("ngrok"); err == nil && p != "" {
		providers = append(providers, "ngrok")
	}
	return providers
}

// StartTunnel starts a client preview tunnel to the local project URL.
func (tm *TunnelManager) StartTunnel(provider, localTarget string) (*TunnelStatus, error) {
	return tm.StartTunnelWithPHP(provider, localTarget, "")
}
func (tm *TunnelManager) StartTunnelWithPHP(provider, localTarget, phpBinary string) (*TunnelStatus, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.activeProcess != nil && tm.activeProcess.Running() {
		return nil, errors.New("A preview tunnel is already active")
	}

	// Validate target against approved local project exposure policy
	u, err := url.Parse(localTarget)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("Invalid project target URL: must start with http:// or https://")
	}
	host := strings.ToLower(u.Hostname())
	isAllowedLocal := host == "localhost" || host == "127.0.0.1" || host == "::1" ||
		strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".local")
	if !isAllowedLocal {
		return nil, fmt.Errorf("security guard: target host %q does not match approved local project exposure policy (*.test, *.local, localhost)", host)
	}
	port := u.Port()
	if port == "8025" || port == "1025" || port == "5432" || port == "3306" || port == "6379" || port == "7700" || port == "9200" {
		return nil, errors.New("Security guard: Tunnel cannot expose administrative services")
	}

	var spec runner.Spec
	var urlPattern *regexp.Regexp

	switch strings.ToLower(provider) {
	case "expose":
		exposeBin, err := dependencies.NewManager("", nil).Resolve("expose")
		if err != nil {
			return nil, errors.New("Expose CLI not found in PATH")
		}
		spec = runner.Spec{
			Executable: exposeBin,
			Args:       []string{"share", localTarget, "--no-interaction"},
		}
		if !strings.EqualFold(filepath.Ext(exposeBin), ".exe") {
			if strings.EqualFold(filepath.Ext(exposeBin), ".bat") {
				exposeBin = strings.TrimSuffix(exposeBin, filepath.Ext(exposeBin))
			}
			if _, err := os.Stat(exposeBin); err != nil {
				return nil, errors.New("Expose PHP entry point was not found")
			}
			if phpBinary == "" {
				phpBinary, err = exec.LookPath("php.exe")
				if err != nil {
					return nil, errors.New("resolve Herd PHP before starting Expose")
				}
			}
			spec.Executable = phpBinary
			spec.Args = append([]string{exposeBin}, spec.Args...)
		}
		urlPattern = regexp.MustCompile(`(?i)(https://[a-zA-Z0-9-]+\.sharedwithexpose\.com|Public URL:\s+(https://\S+))`)

	case "ngrok":
		ngrokBin, err := dependencies.NewManager("", nil).Resolve("ngrok")
		if err != nil {
			return nil, errors.New("ngrok CLI not found in PATH")
		}
		spec = runner.Spec{
			Executable: ngrokBin,
			Args:       []string{"http", localTarget, "--log=stdout"},
		}
		urlPattern = regexp.MustCompile(`(?i)url=(https://[a-zA-Z0-9-]+\.ngrok(?:-free)?\.(?:app|io))|(https://[a-zA-Z0-9-]+\.ngrok(?:-free)?\.(?:app|io))`)

	case "cloudflared", "cloudflare":
		cfBin, err := dependencies.NewManager("", nil).Resolve("cloudflared")
		if err != nil {
			return nil, errors.New("cloudflared CLI not found in PATH")
		}
		spec = runner.Spec{
			Executable: cfBin,
			Args:       []string{"tunnel", "--url", localTarget},
		}
		urlPattern = regexp.MustCompile(`(?i)(https://[a-zA-Z0-9-]+\.trycloudflare\.com)`)

	default:
		return nil, fmt.Errorf("unsupported or unconfigured tunnel provider %q; supported providers are cloudflared, ngrok, expose", provider)
	}

	proc, err := runner.Start(spec)
	if err != nil {
		return nil, fmt.Errorf("could not launch tunnel provider %s: %w", provider, err)
	}

	// Verify process health and wait to extract the real assigned public URL (Finding 12)
	var assignedURL string
	var startupErr strings.Builder
	deadline := time.Now().Add(4 * time.Second)

	for time.Now().Before(deadline) {
		if !proc.Running() {
			for _, e := range proc.Events() {
				if e.Kind == "stderr" || e.Kind == "stdout" {
					startupErr.WriteString(e.Text)
					startupErr.WriteString(" ")
				}
			}
			proc.Close()
			errText := strings.TrimSpace(startupErr.String())
			if errText == "" {
				errText = fmt.Sprintf("tunnel process exited prematurely with code %d", proc.ExitCode())
			}
			return nil, fmt.Errorf("tunnel provider failed to start: %s", errText)
		}

		for _, e := range proc.Events() {
			if e.Kind == "stdout" || e.Kind == "stderr" {
				if m := urlPattern.FindStringSubmatch(e.Text); len(m) > 1 {
					for _, candidate := range m[1:] {
						if strings.HasPrefix(candidate, "http") {
							assignedURL = candidate
							break
						}
					}
					if assignedURL != "" {
						break
					}
				}
			}
		}

		if assignedURL != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if assignedURL == "" {
		if !proc.Running() {
			proc.Close()
			return nil, errors.New("tunnel provider exited before establishing connection; check credentials/auth")
		}
		_ = proc.ForceStop()
		if proc.Wait(3 * time.Second) {
			_ = proc.Close()
		}
		return nil, errors.New("timeout waiting for tunnel provider to assign a verified public URL")
	}

	tm.activeProcess = proc
	tm.status = TunnelStatus{
		Running:     true,
		Provider:    provider,
		LocalTarget: localTarget,
		PublicURL:   assignedURL,
		PID:         proc.PID(),
		StartedAt:   time.Now().UTC().Format(time.RFC3339),
	}

	snapshot := tm.status
	return &snapshot, nil
}

// StopTunnel stops any running preview tunnel gracefully, or forces exit if requested.
func (tm *TunnelManager) StopTunnel(force bool) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.activeProcess != nil && tm.activeProcess.Running() {
		if force {
			if err := tm.activeProcess.ForceStop(); err != nil {
				return err
			}
			if !tm.activeProcess.Wait(3 * time.Second) {
				return errors.New("owned tunnel process tree has not exited")
			}
			_ = tm.activeProcess.Close()
			tm.activeProcess = nil
			tm.status = TunnelStatus{Running: false}
			return nil
		}

		_ = tm.activeProcess.RequestStop()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if !tm.activeProcess.Running() {
				_ = tm.activeProcess.Close()
				tm.activeProcess = nil
				tm.status = TunnelStatus{Running: false}
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
		return errors.New("tunnel process did not stop within timeout; use force to terminate")
	}

	if tm.activeProcess != nil {
		_ = tm.activeProcess.Close()
		tm.activeProcess = nil
	}
	tm.status = TunnelStatus{Running: false}
	return nil
}

func (tm *TunnelManager) GetStatus() TunnelStatus {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.activeProcess != nil && !tm.activeProcess.Running() && tm.status.Running && tm.status.Provider != "local-direct" {
		tm.status.Running = false
	}
	return tm.status
}

// WebhookScenario definition
type WebhookScenario struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Provider  string            `json:"provider"` // "stripe", "github", "custom"
	TargetURL string            `json:"targetUrl"`
	Secret    string            `json:"secret"`
	Headers   map[string]string `json:"headers"`
	Payload   string            `json:"payload"`
}

type WebhookDispatchResult struct {
	StatusCode      int               `json:"statusCode"`
	DurationMs      float64           `json:"durationMs"`
	ResponseHeaders map[string]string `json:"responseHeaders"`
	ResponseBody    string            `json:"responseBody"`
	ComputedSig     string            `json:"computedSignature"`
	SignatureValid  bool              `json:"signatureValid"`
	Success         bool              `json:"success"`
	Error           string            `json:"error,omitempty"`
}

// GenerateHMACSignature calculates HMAC-SHA256 signature for webhooks.
func GenerateHMACSignature(secret, payload, provider string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sigHex := hex.EncodeToString(mac.Sum(nil))

	switch strings.ToLower(provider) {
	case "stripe":
		timestamp := time.Now().Unix()
		signedPayload := fmt.Sprintf("%d.%s", timestamp, payload)
		stripeMac := hmac.New(sha256.New, []byte(secret))
		stripeMac.Write([]byte(signedPayload))
		return fmt.Sprintf("t=%d,v1=%s", timestamp, hex.EncodeToString(stripeMac.Sum(nil)))
	case "github":
		return "sha256=" + sigHex
	default:
		return sigHex
	}
}

// DispatchWebhook sends the test payload with signature headers.
func DispatchWebhook(ctx context.Context, s WebhookScenario) (*WebhookDispatchResult, error) {
	sig := GenerateHMACSignature(s.Secret, s.Payload, s.Provider)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.TargetURL, bytes.NewBufferString(s.Payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}

	// Set signature header based on provider
	switch strings.ToLower(s.Provider) {
	case "stripe":
		req.Header.Set("Stripe-Signature", sig)
	case "github":
		req.Header.Set("X-Hub-Signature-256", sig)
	default:
		req.Header.Set("X-Signature", sig)
	}

	client := &http.Client{Timeout: 10 * time.Second, Transport: wktesting.LocalTransport()}
	defer client.CloseIdleConnections()
	start := time.Now()
	resp, err := client.Do(req)
	dur := float64(time.Since(start).Microseconds()) / 1000.0

	if err != nil {
		return &WebhookDispatchResult{
			DurationMs:  dur,
			ComputedSig: sig,
			Success:     false,
			Error:       err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	respHdrs := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			respHdrs[k] = v[0]
		}
	}

	return &WebhookDispatchResult{
		StatusCode:      resp.StatusCode,
		DurationMs:      dur,
		ResponseHeaders: respHdrs,
		ResponseBody:    string(bodyBytes),
		ComputedSig:     sig,
		SignatureValid:  false, // A receiver response does not prove that it verified this signature.
		Success:         resp.StatusCode >= 200 && resp.StatusCode < 400,
	}, nil
}

type IntegrationItem struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DetectedIn  string `json:"detectedIn"`
	Status      string `json:"status"` // "configured", "unconfigured"
	Environment string `json:"environment"`
}

// DetectIntegrations inspects project environment keys for third-party integrations.
func DetectIntegrations(envKeys map[string]string) []IntegrationItem {
	var items []IntegrationItem

	// Stripe
	if envKeys["STRIPE_KEY"] != "" || envKeys["STRIPE_SECRET"] != "" {
		env := "live"
		if strings.HasPrefix(envKeys["STRIPE_KEY"], "pk_test_") || strings.HasPrefix(envKeys["STRIPE_SECRET"], "sk_test_") {
			env = "test-mode"
		}
		items = append(items, IntegrationItem{
			Name:        "Stripe",
			Type:        "Payments",
			DetectedIn:  ".env",
			Status:      "configured",
			Environment: env,
		})
	}

	// Mailgun / Postmark / Resend
	if mailer := envKeys["MAIL_MAILER"]; mailer != "" && mailer != "log" && mailer != "array" {
		items = append(items, IntegrationItem{
			Name:        "Mail Service (" + mailer + ")",
			Type:        "Email",
			DetectedIn:  "MAIL_MAILER",
			Status:      "configured",
			Environment: "active",
		})
	}

	// AWS S3
	if envKeys["AWS_ACCESS_KEY_ID"] != "" || envKeys["AWS_BUCKET"] != "" {
		items = append(items, IntegrationItem{
			Name:        "Amazon S3",
			Type:        "Storage",
			DetectedIn:  "AWS_BUCKET",
			Status:      "configured",
			Environment: "active",
		})
	}

	// Pusher
	if envKeys["PUSHER_APP_KEY"] != "" {
		items = append(items, IntegrationItem{
			Name:        "Pusher",
			Type:        "WebSockets",
			DetectedIn:  "PUSHER_APP_KEY",
			Status:      "configured",
			Environment: "active",
		})
	}

	return items
}
