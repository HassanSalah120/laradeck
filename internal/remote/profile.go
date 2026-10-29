package remote

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type RemoteProfile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	User        string    `json:"user"`
	AuthType    string    `json:"authType"` // "key", "agent", "password"
	KeyPath     string    `json:"keyPath,omitempty"`
	RemotePath  string    `json:"remotePath"`
	Environment string    `json:"environment"` // "production", "staging", "development"
	ReadOnly    bool      `json:"readOnly"`
	CreatedAt   time.Time `json:"createdAt"`
	LastTested  time.Time `json:"lastTested,omitempty"`
	Status      string    `json:"status,omitempty"` // "reachable", "unreachable", "untested"
	LatencyMs   float64   `json:"latencyMs,omitempty"`
	RemotePHP   string    `json:"remotePhp,omitempty"`
	RemoteVer   string    `json:"remoteVer,omitempty"`
}

type RemoteTestResult struct {
	Reachable bool    `json:"reachable"`
	LatencyMs float64 `json:"latencyMs"`
	PHP       string  `json:"php,omitempty"`
	Laravel   string  `json:"laravel,omitempty"`
	Error     string  `json:"error,omitempty"`
}

func GenerateProfileID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "rem_" + hex.EncodeToString(b)
}

func ValidateProfile(p RemoteProfile) error {
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("Profile name is required")
	}
	if strings.TrimSpace(p.Host) == "" {
		return errors.New("Remote host is required")
	}
	if strings.TrimSpace(p.User) == "" {
		return errors.New("Remote SSH username is required")
	}
	if strings.TrimSpace(p.RemotePath) == "" {
		return errors.New("Remote project path is required")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return errors.New("Valid SSH port (1-65535) is required")
	}
	return nil
}
