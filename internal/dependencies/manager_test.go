package dependencies

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mockDownload(t *testing.T, manager *Manager, badChecksum bool) {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	file, _ := writer.Create("nested/mailpit.exe")
	file.Write([]byte("MZtest executable bytes"))
	unrelated, _ := writer.Create("../../outside.txt")
	unrelated.Write([]byte("must not escape"))
	writer.Close()
	sum := sha256.Sum256(archive.Bytes())
	digest := hex.EncodeToString(sum[:])
	if badChecksum {
		digest = strings.Repeat("0", 64)
	}
	metadata, _ := json.Marshal(map[string]any{"tag_name": "v1.2.3", "assets": []map[string]string{{"name": "mailpit-windows-amd64.zip", "browser_download_url": "https://github.com/axllent/mailpit/releases/download/v1.2.3/mailpit-windows-amd64.zip", "digest": "sha256:" + digest}}})
	manager.client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		data := archive.Bytes()
		if req.URL.Host == "api.github.com" {
			data = metadata
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), ContentLength: int64(len(data)), Header: http.Header{}}, nil
	})
}
func TestInstallVerifiesAndPersistsPrivateBinary(t *testing.T) {
	root := t.TempDir()
	manager := NewManager(root, nil)
	mockDownload(t, manager, false)
	installed, err := manager.Install(context.Background(), "mailpit")
	if err != nil {
		t.Fatal(err)
	}
	if installed.State != "installed" || installed.Version != "v1.2.3" {
		t.Fatal(installed)
	}
	resolved, err := NewManager(root, nil).Resolve("mailpit")
	if err != nil || resolved != installed.Path {
		t.Fatalf("new manager did not resolve installed binary: %v", err)
	}
	if !strings.HasPrefix(resolved, root+string(filepath.Separator)) {
		t.Fatal("installation left private root")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside.txt")); err == nil {
		t.Fatal("archive wrote outside managed root")
	}
}
func TestBadChecksumPreservesExistingTool(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(t.TempDir(), "mailpit.exe")
	os.WriteFile(existing, []byte("MZold"), 0700)
	manager := NewManager(root, nil)
	if err := manager.SetBinary("mailpit", existing); err != nil {
		t.Fatal(err)
	}
	mockDownload(t, manager, true)
	if _, err := manager.Install(context.Background(), "mailpit"); err == nil {
		t.Fatal("unverified binary installed")
	}
	resolved, err := manager.Resolve("mailpit")
	if err != nil || resolved != existing {
		t.Fatal("failed installation replaced existing configuration")
	}
}
func TestUnknownToolAndUnsafeRedirectAreRejected(t *testing.T) {
	manager := NewManager(t.TempDir(), nil)
	if _, err := manager.Install(context.Background(), "unknown"); err == nil {
		t.Fatal("unknown tool accepted")
	}
	request, _ := http.NewRequest("GET", "https://untrusted.example/binary.exe", nil)
	if err := manager.client.CheckRedirect(request, nil); err == nil {
		t.Fatal("unapproved redirect accepted")
	}
	request, _ = http.NewRequest("GET", "http://github.com/file.exe", nil)
	if err := manager.client.CheckRedirect(request, nil); err == nil {
		t.Fatal("HTTP downgrade accepted")
	}
}
func TestProjectPackagesUseInstalledMetadata(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "vendor", "composer")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "installed.json"), []byte(`{"packages":[{"name":"itsgoingd/clockwork","version":"v5.3.5"}]}`), 0600)
	for _, status := range ProjectStatus(root) {
		if status.ID == "clockwork" && (status.State != "incomplete" || status.Version != "v5.3.5") {
			t.Fatal(status)
		}
	}
	for _, status := range ProjectStatus("") {
		if status.CanInstall || status.State != "select_project" {
			t.Fatal("package can install without a selected project")
		}
	}
}

func TestLaravelTestRunnerNeedsCollision(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "vendor", "composer")
	os.MkdirAll(dir, 0700)
	for _, collision := range []bool{false, true} {
		packages := []map[string]string{{"name": "phpunit/phpunit", "version": "12.0.0"}}
		if collision {
			packages = append(packages, map[string]string{"name": "nunomaduro/collision", "version": "8.0.0"})
		}
		data, _ := json.Marshal(map[string]any{"packages": packages})
		os.WriteFile(filepath.Join(dir, "installed.json"), data, 0600)
		expected := "incomplete"
		if collision {
			expected = "installed"
		}
		for _, status := range ProjectStatus(root) {
			if status.ID == "phpunit" && status.State != expected {
				t.Fatalf("collision=%v: %s", collision, status.State)
			}
		}
	}
}

func TestDownloadRetriesTransientTimeout(t *testing.T) {
	manager := NewManager(t.TempDir(), nil)
	attempts := 0
	manager.client.Transport = transportFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return nil, context.DeadlineExceeded
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ok"))}, nil
	})
	response, err := manager.get(context.Background(), "https://getcomposer.org/versions")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if attempts != 2 {
		t.Fatalf("attempts=%d", attempts)
	}
}

func TestClockworkReadinessNeedsAutoloadAndDiscovery(t *testing.T) {
	root := t.TempDir()
	vendor := filepath.Join(root, "vendor", "composer")
	os.MkdirAll(vendor, 0700)
	os.WriteFile(filepath.Join(vendor, "installed.json"), []byte(`{"packages":[{"name":"itsgoingd/clockwork","version":"v5.3.5"}]}`), 0600)
	for step := 0; step < 3; step++ {
		if step == 1 {
			os.WriteFile(filepath.Join(vendor, "autoload_psr4.php"), []byte("Clockwork"), 0600)
		}
		if step == 2 {
			os.MkdirAll(filepath.Join(root, "bootstrap", "cache"), 0700)
			os.WriteFile(filepath.Join(root, "bootstrap", "cache", "packages.php"), []byte("itsgoingd/clockwork"), 0600)
		}
		for _, status := range ProjectStatus(root) {
			if status.ID == "clockwork" {
				expected := "incomplete"
				if step == 2 {
					expected = "installed"
				}
				if status.State != expected {
					t.Fatalf("stage=%d expected=%s got=%s", step, expected, status.State)
				}
			}
		}
	}
}
