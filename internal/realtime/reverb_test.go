package realtime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "reverb-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	envContent := `APP_NAME=Laravel
REVERB_APP_KEY=my-reverb-key
REVERB_HOST=127.0.0.1
REVERB_PORT=8080
REVERB_SCHEME=http
BROADCAST_CONNECTION=reverb
`
	envPath := filepath.Join(tmpDir, ".env")
	if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	cfg := DetectConfig(envPath)
	if cfg.AppKey != "my-reverb-key" {
		t.Errorf("expected app key 'my-reverb-key', got %q", cfg.AppKey)
	}
	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.Driver != "reverb" {
		t.Errorf("expected driver 'reverb', got %q", cfg.Driver)
	}
}

func TestInspectorFrameRecordingAndSimulation(t *testing.T) {
	ins := NewInspector()

	frame := ins.RecordFrame("incoming", "App\\Events\\OrderPlaced", "private-orders.1", `{"order_id": 42}`)
	if frame.ID == "" {
		t.Error("expected frame ID to be generated")
	}
	if frame.Event != "App\\Events\\OrderPlaced" {
		t.Errorf("unexpected event: %s", frame.Event)
	}

	channels := ins.GetChannels()
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].Name != "private-orders.1" {
		t.Errorf("expected channel 'private-orders.1', got %q", channels[0].Name)
	}
	if channels[0].Type != "private" {
		t.Errorf("expected type 'private', got %q", channels[0].Type)
	}

	// Test Simulation
	simRes := ins.SimulateEvent(EventSimulationSpec{
		Channel: "orders",
		Event:   "OrderShipped",
		Payload: `{"status": "in_transit"}`,
	})

	if !simRes.Success {
		t.Errorf("simulation failed: %s", simRes.Message)
	}

	frames := ins.GetFrames()
	if len(frames) != 2 {
		t.Errorf("expected 2 frames, got %d", len(frames))
	}

	ins.ClearFrames()
	if len(ins.GetFrames()) != 0 {
		t.Errorf("expected 0 frames after clear")
	}
}
