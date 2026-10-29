package dependencies

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"herdworkbench/internal/runner"
	"herdworkbench/internal/storage"
)

const maxDownload = int64(512 * 1024 * 1024)

type Manager struct {
	root      string
	client    *http.Client
	mu        sync.Mutex
	installMu sync.Mutex
	states    map[string]Status
	notify    func()
}

func NewManager(root string, notify func()) *Manager {
	if root == "" {
		root = filepath.Join(storage.AppDataDir(), "tools")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 30 * time.Second
	return &Manager{root: root, states: map[string]Status{}, notify: notify, client: &http.Client{Transport: transport, Timeout: 5 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		host := req.URL.Hostname()
		if req.URL.Scheme != "https" || !(host == "github.com" || host == "api.github.com" || strings.HasSuffix(host, ".githubusercontent.com") || host == "getcomposer.org") {
			return errors.New("tool download redirected outside an approved HTTPS host")
		}
		if len(via) >= 10 {
			return errors.New("too many download redirects")
		}
		return nil
	}}}
}
func (m *Manager) update(id, state, message string, progress float64) {
	m.mu.Lock()
	m.states[id] = Status{State: state, Message: message, Progress: progress}
	m.mu.Unlock()
	if m.notify != nil {
		m.notify()
	}
}
func (m *Manager) SetBinary(id, binary string) error {
	spec, err := Lookup(id)
	if err != nil {
		return err
	}
	if spec.Scope == "project" {
		return errors.New("project packages must be installed with Composer")
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	if info, err := os.Stat(absolute); err != nil || info.IsDir() {
		return errors.New("choose an existing binary file")
	}
	extension := strings.ToLower(filepath.Ext(absolute))
	if extension != ".exe" && !(id == "composer" && extension == ".phar") && !(id == "herd" && extension == ".bat") && !(id == "expose" && (extension == ".bat" || filepath.Base(absolute) == "expose")) {
		return errors.New("choose an executable (.exe), Composer PHAR or Herd launcher")
	}
	if err := m.saveRecord(id, Record{Path: absolute, Version: "custom", Custom: true}); err != nil {
		return err
	}
	m.update(id, "installed", "Configured binary", 1)
	return nil
}
func (m *Manager) saveRecord(id string, record Record) error {
	dir := filepath.Join(m.root, id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, "current-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return storage.ReplaceFileAtomic(name, filepath.Join(dir, "current.json"))
}
func (m *Manager) get(ctx context.Context, address string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "HerdWorkbench-DependencySetup/1.0")
	for attempt := 0; attempt < 3; attempt++ {
		response, requestErr := m.client.Do(req)
		var timeout net.Error
		retry := requestErr != nil && errors.As(requestErr, &timeout) && timeout.Timeout()
		if requestErr == nil {
			if response.StatusCode == http.StatusOK {
				return response, nil
			}
			response.Body.Close()
			requestErr = fmt.Errorf("download source returned HTTP %d", response.StatusCode)
			retry = response.StatusCode == 502 || response.StatusCode == 503 || response.StatusCode == 504
		}
		if !retry || attempt == 2 || ctx.Err() != nil {
			return nil, requestErr
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	return nil, errors.New("download attempts exhausted")
}

type asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}

func (m *Manager) release(ctx context.Context, spec Spec) (string, asset, error) {
	if spec.ID == "composer" {
		response, err := m.get(ctx, "https://getcomposer.org/versions")
		if err != nil {
			return "", asset{}, err
		}
		defer response.Body.Close()
		var versions struct {
			Stable []struct {
				Version string `json:"version"`
				Path    string `json:"path"`
			} `json:"stable"`
		}
		if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&versions); err != nil || len(versions.Stable) == 0 {
			return "", asset{}, errors.New("invalid Composer version metadata")
		}
		version := versions.Stable[0]
		if !regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(version.Version) || version.Path != "/download/"+version.Version+"/composer.phar" {
			return "", asset{}, errors.New("invalid Composer release path")
		}
		address := "https://getcomposer.org" + version.Path
		checksum, err := m.get(ctx, address+".sha256sum")
		if err != nil {
			return "", asset{}, err
		}
		defer checksum.Body.Close()
		text, err := io.ReadAll(io.LimitReader(checksum.Body, 1024))
		fields := strings.Fields(string(text))
		if err != nil || len(fields) == 0 {
			return "", asset{}, errors.New("Composer checksum unavailable")
		}
		return version.Version, asset{Name: "composer.phar", URL: address, Digest: "sha256:" + fields[0]}, nil
	}
	response, err := m.get(ctx, "https://api.github.com/repos/"+spec.Repository+"/releases/latest")
	if err != nil {
		return "", asset{}, err
	}
	defer response.Body.Close()
	var release struct {
		Tag    string  `json:"tag_name"`
		Assets []asset `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&release); err != nil {
		return "", asset{}, err
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9._-]+$`).MatchString(release.Tag) || release.Tag == ".." {
		return "", asset{}, errors.New("invalid release version")
	}
	pattern := regexp.MustCompile(spec.AssetPattern)
	for _, item := range release.Assets {
		if pattern.MatchString(item.Name) {
			if !strings.HasPrefix(item.URL, "https://github.com/"+spec.Repository+"/releases/download/") {
				return "", asset{}, errors.New("release asset URL does not match the approved publisher")
			}
			return release.Tag, item, nil
		}
	}
	return "", asset{}, errors.New("publisher has no matching Windows amd64 release asset")
}
func (m *Manager) Install(ctx context.Context, id string) (result Status, err error) {
	spec, err := Lookup(id)
	if err != nil {
		return result, err
	}
	if spec.Scope != "app" {
		return result, errors.New("this dependency requires its project or system installer")
	}
	if !m.installMu.TryLock() {
		return result, errors.New("another portable tool installation is running")
	}
	defer m.installMu.Unlock()
	defer func() {
		if err != nil {
			m.update(id, "failed", err.Error(), 0)
		}
	}()
	m.update(id, "installing", "Checking official release and checksum", 0)
	version, item, err := m.release(ctx, spec)
	if err != nil {
		return result, err
	}
	expected := strings.TrimPrefix(item.Digest, "sha256:")
	if !strings.HasPrefix(item.Digest, "sha256:") || len(expected) != 64 {
		return result, errors.New("publisher did not provide a SHA-256 digest; installation refused")
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return result, errors.New("publisher checksum is invalid")
	}
	if err := os.MkdirAll(m.root, 0700); err != nil {
		return result, err
	}
	download, err := os.CreateTemp(m.root, "download-*.tmp")
	if err != nil {
		return result, err
	}
	downloadPath := download.Name()
	defer os.Remove(downloadPath)
	defer download.Close()
	response, err := m.get(ctx, item.URL)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.ContentLength > maxDownload {
		return result, errors.New("tool download exceeds 512 MiB")
	}
	hasher := sha256.New()
	reader := &progressReader{Reader: io.LimitReader(response.Body, maxDownload+1), total: response.ContentLength, notify: func(progress float64) { m.update(id, "installing", "Downloading "+item.Name, progress*.9) }}
	count, err := io.Copy(io.MultiWriter(download, hasher), reader)
	if err != nil {
		return result, err
	}
	if count > maxDownload {
		return result, errors.New("tool download exceeds 512 MiB")
	}
	if !strings.EqualFold(hex.EncodeToString(hasher.Sum(nil)), expected) {
		return result, errors.New("SHA-256 verification failed; downloaded binary was not installed")
	}
	if err := download.Close(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	m.update(id, "installing", "Checksum verified; installing binary", .95)
	dir := filepath.Join(m.root, id, version+"-"+fmt.Sprint(time.Now().UnixNano()))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return result, err
	}
	destination := filepath.Join(dir, spec.Executable)
	if strings.HasSuffix(strings.ToLower(item.Name), ".zip") {
		err = extractBinary(downloadPath, destination, spec.Executable)
	} else {
		err = copyBinary(downloadPath, destination)
	}
	if err != nil {
		_ = os.Remove(destination)
		_ = os.Remove(dir)
		return result, err
	}
	if spec.ID != "composer" {
		file, err := os.Open(destination)
		if err != nil {
			return result, err
		}
		var magic [2]byte
		_, readErr := io.ReadFull(file, magic[:])
		file.Close()
		if readErr != nil || string(magic[:]) != "MZ" {
			os.Remove(destination)
			os.Remove(dir)
			return result, errors.New("download is not a Windows executable")
		}
	}
	if err := m.saveRecord(id, Record{Version: version, Path: destination}); err != nil {
		return result, err
	}
	m.update(id, "installed", "Installed and ready to use", 1)
	return Status{Spec: spec, State: "installed", Version: version, Path: destination, Progress: 1, CanInstall: true, Message: "Installed and ready to use"}, nil
}

type progressReader struct {
	io.Reader
	read, total int64
	last        time.Time
	notify      func(float64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.read += int64(n)
	if time.Since(r.last) > 200*time.Millisecond {
		r.last = time.Now()
		progress := 0.0
		if r.total > 0 {
			progress = float64(r.read) / float64(r.total)
		}
		r.notify(progress)
	}
	return n, err
}
func copyBinary(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	return writeBinary(input, destination)
}
func writeBinary(input io.Reader, destination string) error {
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		return err
	}
	count, err := io.Copy(output, io.LimitReader(input, maxDownload+1))
	closeErr := output.Close()
	if err != nil {
		return err
	}
	if count > maxDownload {
		return errors.New("extracted binary exceeds 512 MiB")
	}
	return closeErr
}
func extractBinary(archive, destination, executable string) error {
	file, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	var selected *zip.File
	for _, entry := range file.File {
		if strings.EqualFold(path.Base(strings.ReplaceAll(entry.Name, "\\", "/")), executable) {
			if selected != nil {
				return errors.New("archive has ambiguous executable entries")
			}
			selected = entry
		}
	}
	if selected == nil {
		return errors.New("expected executable is missing from archive")
	}
	if selected.Mode()&os.ModeSymlink != 0 || selected.FileInfo().IsDir() || selected.UncompressedSize64 > uint64(maxDownload) {
		return errors.New("unsafe archive executable entry")
	}
	reader, err := selected.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	return writeBinary(reader, destination)
}
func (m *Manager) OpenSystemInstaller(id string) (Status, error) {
	spec, err := Lookup(id)
	if err != nil {
		return Status{}, err
	}
	if spec.Scope != "system" || spec.WingetID == "" {
		return Status{}, errors.New("this dependency has no system installer")
	}
	if _, err := url.ParseRequestURI(spec.Website); err != nil {
		return Status{}, err
	}
	if err := runner.LaunchSystemInstaller(spec.WingetID); err != nil {
		return Status{}, err
	}
	message := "Installer opened in a separate console. Complete the vendor installer, then recheck. This is not an installation-success result."
	m.update(id, "installer_opened", message, 0)
	return Status{Spec: spec, State: "installer_opened", CanInstall: true, Message: message}, nil
}

func (m *Manager) Report(id, state, message string) { m.update(id, state, message, 0) }
