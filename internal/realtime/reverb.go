package realtime

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type WebSocketFrame struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Direction string    `json:"direction"` // "incoming" or "outgoing"
	Event     string    `json:"event"`
	Channel   string    `json:"channel,omitempty"`
	Data      string    `json:"data"`
	Size      int       `json:"size"`
}

type ChannelInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "public", "private", "presence"
	Subscribers int    `json:"subscribers"`
	LastEvent   string `json:"lastEvent,omitempty"`
	LastSeen    string `json:"lastSeen,omitempty"`
}

type ReverbConfig struct {
	AppKey    string `json:"appKey"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Scheme    string `json:"scheme"`
	Driver    string `json:"driver"`
	IsRunning bool   `json:"isRunning"`
}

type EventSimulationSpec struct {
	Channel string `json:"channel"`
	Event   string `json:"event"`
	Payload string `json:"payload"`
}

type SimulationResult struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Frame   WebSocketFrame `json:"frame"`
}

type Inspector struct {
	mu          sync.RWMutex
	frames      []WebSocketFrame
	channels    map[string]*ChannelInfo
	maxFrames   int
}

func NewInspector() *Inspector {
	return &Inspector{
		frames:    make([]WebSocketFrame, 0, 200),
		channels:  make(map[string]*ChannelInfo),
		maxFrames: 200,
	}
}

// DetectConfig reads .env in the project directory to detect Reverb or Pusher settings
func DetectConfig(envPath string) ReverbConfig {
	cfg := ReverbConfig{
		Host:   "127.0.0.1",
		Port:   8080,
		Scheme: "http",
		Driver: "reverb",
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		return cfg
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		k := strings.TrimSpace(parts[0])
		v := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch k {
		case "REVERB_APP_KEY":
			cfg.AppKey = v
		case "REVERB_HOST":
			if v != "" {
				cfg.Host = v
			}
		case "REVERB_PORT":
			if p, err := strconv.Atoi(v); err == nil && p > 0 {
				cfg.Port = p
			}
		case "REVERB_SCHEME":
			if v != "" {
				cfg.Scheme = v
			}
		case "PUSHER_APP_KEY":
			if cfg.AppKey == "" {
				cfg.AppKey = v
				cfg.Driver = "pusher"
			}
		case "BROADCAST_CONNECTION", "BROADCAST_DRIVER":
			if v != "" {
				cfg.Driver = v
			}
		}
	}

	// Check if port is listening
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err == nil {
		cfg.IsRunning = true
		_ = conn.Close()
	}

	return cfg
}

func (ins *Inspector) RecordFrame(direction, event, channel, data string) WebSocketFrame {
	ins.mu.Lock()
	defer ins.mu.Unlock()

	b := make([]byte, 6)
	_, _ = rand.Read(b)
	frame := WebSocketFrame{
		ID:        "ws_" + hex.EncodeToString(b),
		Timestamp: time.Now(),
		Direction: direction,
		Event:     event,
		Channel:   channel,
		Data:      data,
		Size:      len(data),
	}

	ins.frames = append(ins.frames, frame)
	if len(ins.frames) > ins.maxFrames {
		ins.frames = ins.frames[len(ins.frames)-ins.maxFrames:]
	}

	if channel != "" {
		chType := "public"
		if strings.HasPrefix(channel, "presence-") {
			chType = "presence"
		} else if strings.HasPrefix(channel, "private-") {
			chType = "private"
		}

		ch, exists := ins.channels[channel]
		if !exists {
			ch = &ChannelInfo{
				Name:        channel,
				Type:        chType,
				Subscribers: 0,
			}
			ins.channels[channel] = ch
		}
		ch.LastEvent = event
		ch.LastSeen = time.Now().Format("15:04:05")
	}

	return frame
}

func (ins *Inspector) GetFrames() []WebSocketFrame {
	ins.mu.RLock()
	defer ins.mu.RUnlock()
	res := make([]WebSocketFrame, len(ins.frames))
	copy(res, ins.frames)
	return res
}

func (ins *Inspector) GetChannels() []ChannelInfo {
	ins.mu.RLock()
	defer ins.mu.RUnlock()
	res := make([]ChannelInfo, 0, len(ins.channels))
	for _, ch := range ins.channels {
		res = append(res, *ch)
	}
	return res
}

func (ins *Inspector) SimulateEvent(spec EventSimulationSpec) SimulationResult {
	if strings.TrimSpace(spec.Channel) == "" {
		return SimulationResult{Success: false, Message: "Channel name is required"}
	}
	if strings.TrimSpace(spec.Event) == "" {
		return SimulationResult{Success: false, Message: "Event name is required"}
	}
	payload := spec.Payload
	if strings.TrimSpace(payload) == "" {
		payload = "{}"
	}

	frame := ins.RecordFrame("incoming", spec.Event, spec.Channel, payload)
	return SimulationResult{
		Success: true,
		Message: fmt.Sprintf("Event '%s' recorded in local simulation buffer for channel '%s' (simulation mode)", spec.Event, spec.Channel),
		Frame:   frame,
	}
}

func (ins *Inspector) ClearFrames() {
	ins.mu.Lock()
	defer ins.mu.Unlock()
	ins.frames = make([]WebSocketFrame, 0, ins.maxFrames)
}
