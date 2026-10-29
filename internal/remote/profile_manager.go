package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ProfileManager struct {
	mu       sync.RWMutex
	filePath string
	profiles []RemoteProfile
}

func NewProfileManager(storageDir string) (*ProfileManager, error) {
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, err
	}
	fp := filepath.Join(storageDir, "remote_profiles.json")
	pm := &ProfileManager{
		filePath: fp,
		profiles: make([]RemoteProfile, 0),
	}
	pm.load()
	return pm, nil
}

func (pm *ProfileManager) load() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	data, err := os.ReadFile(pm.filePath)
	if err != nil {
		return
	}
	var loaded []RemoteProfile
	if err := json.Unmarshal(data, &loaded); err == nil {
		pm.profiles = loaded
	}
}

func (pm *ProfileManager) save() error {
	data, err := json.MarshalIndent(pm.profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pm.filePath, data, 0644)
}

func (pm *ProfileManager) ListProfiles() []RemoteProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	res := make([]RemoteProfile, len(pm.profiles))
	copy(res, pm.profiles)
	return res
}

func (pm *ProfileManager) SaveProfile(p RemoteProfile) (RemoteProfile, error) {
	if err := ValidateProfile(p); err != nil {
		return RemoteProfile{}, err
	}

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if p.ID == "" {
		p.ID = GenerateProfileID()
		p.CreatedAt = time.Now()
		if p.Status == "" {
			p.Status = "untested"
		}
		pm.profiles = append(pm.profiles, p)
	} else {
		found := false
		for i, existing := range pm.profiles {
			if existing.ID == p.ID {
				p.CreatedAt = existing.CreatedAt
				pm.profiles[i] = p
				found = true
				break
			}
		}
		if !found {
			p.CreatedAt = time.Now()
			pm.profiles = append(pm.profiles, p)
		}
	}

	if err := pm.save(); err != nil {
		return RemoteProfile{}, err
	}
	return p, nil
}

func (pm *ProfileManager) DeleteProfile(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	idx := -1
	for i, p := range pm.profiles {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		return errors.New("Profile not found")
	}

	pm.profiles = append(pm.profiles[:idx], pm.profiles[idx+1:]...)
	return pm.save()
}

func (pm *ProfileManager) TestConnection(ctx context.Context, id string) (RemoteTestResult, error) {
	pm.mu.RLock()
	var target *RemoteProfile
	for _, p := range pm.profiles {
		if p.ID == id {
			cpy := p
			target = &cpy
			break
		}
	}
	pm.mu.RUnlock()

	if target == nil {
		return RemoteTestResult{Reachable: false, Error: "Profile not found"}, errors.New("Profile not found")
	}

	start := time.Now()
	// Build SSH command
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=accept-new",
		"-p", fmt.Sprint(target.Port),
	}
	if target.KeyPath != "" {
		args = append(args, "-i", target.KeyPath)
	}
	remoteCmd := fmt.Sprintf("cd %s && php -v | head -n 1 && php artisan --version 2>/dev/null || echo 'No Artisan'", target.RemotePath)
	args = append(args, fmt.Sprintf("%s@%s", target.User, target.Host), remoteCmd)

	cmd := exec.CommandContext(ctx, "ssh", args...)
	out, err := cmd.CombinedOutput()
	latency := float64(time.Since(start).Milliseconds())

	if err != nil {
		errMsg := strings.TrimSpace(string(out))
		if errMsg == "" {
			errMsg = err.Error()
		}
		pm.updateStatus(id, "unreachable", latency, "", "")
		return RemoteTestResult{
			Reachable: false,
			LatencyMs: latency,
			Error:     errMsg,
		}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	phpVer := ""
	artisanVer := ""
	if len(lines) > 0 {
		phpVer = lines[0]
	}
	if len(lines) > 1 {
		artisanVer = lines[1]
	}

	pm.updateStatus(id, "reachable", latency, phpVer, artisanVer)

	return RemoteTestResult{
		Reachable: true,
		LatencyMs: latency,
		PHP:       phpVer,
		Laravel:   artisanVer,
	}, nil
}

func (pm *ProfileManager) updateStatus(id, status string, latency float64, php, laravel string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for i, p := range pm.profiles {
		if p.ID == id {
			pm.profiles[i].Status = status
			pm.profiles[i].LatencyMs = latency
			pm.profiles[i].RemotePHP = php
			pm.profiles[i].RemoteVer = laravel
			pm.profiles[i].LastTested = time.Now()
			_ = pm.save()
			break
		}
	}
}
