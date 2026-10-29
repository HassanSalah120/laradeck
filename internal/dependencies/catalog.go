// Package dependencies owns detection and explicit installation of supported developer tools.
package dependencies

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Spec struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Scope        string `json:"scope"`
	Executable   string `json:"executable"`
	Repository   string `json:"repository,omitempty"`
	AssetPattern string `json:"-"`
	WingetID     string `json:"wingetId,omitempty"`
	Package      string `json:"package,omitempty"`
	Website      string `json:"website"`
	Note         string `json:"note"`
}

type Status struct {
	Spec
	State      string  `json:"state"`
	Version    string  `json:"version"`
	Path       string  `json:"path"`
	Message    string  `json:"message"`
	CanInstall bool    `json:"canInstall"`
	Progress   float64 `json:"progress"`
}

var catalog = []Spec{
	{ID: "gitleaks", Name: "Gitleaks secret scanner", Scope: "app", Executable: "gitleaks.exe", Repository: "gitleaks/gitleaks", AssetPattern: `^gitleaks_[0-9.]+_windows_x64\.zip$`, Website: "https://github.com/gitleaks/gitleaks", Note: "Private verified binary; reports suspected secrets with values redacted. Git history scanning is opt-in."},
	{ID: "mailpit", Name: "Mailpit", Scope: "app", Executable: "mailpit.exe", Repository: "axllent/mailpit", AssetPattern: `^mailpit-windows-amd64\.zip$`, Website: "https://mailpit.axllent.org/", Note: "Private Workbench binary; SMTP 1025 and inbox 8025. Installation does not start the service."},
	{ID: "k6", Name: "k6 load testing", Scope: "app", Executable: "k6.exe", Repository: "grafana/k6", AssetPattern: `^k6-v.+-windows-amd64\.zip$`, Website: "https://grafana.com/docs/k6/", Note: "Private Workbench binary; no cloud account required for local tests."},
	{ID: "meilisearch", Name: "Meilisearch", Scope: "app", Executable: "meilisearch.exe", Repository: "meilisearch/meilisearch", AssetPattern: `^meilisearch-windows-amd64\.exe$`, Website: "https://www.meilisearch.com/", Note: "Private Workbench binary; installation does not start the engine."},
	{ID: "cloudflared", Name: "Cloudflare Tunnel CLI", Scope: "app", Executable: "cloudflared.exe", Repository: "cloudflare/cloudflared", AssetPattern: `^cloudflared-windows-amd64\.exe$`, Website: "https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/", Note: "Installing the CLI does not start a public tunnel."},
	{ID: "composer", Name: "Composer", Scope: "app", Executable: "composer.phar", Website: "https://getcomposer.org/download/", Note: "Uses existing Herd/Composer when available; otherwise downloads a verified private PHAR. Requires PHP from Herd."},
	{ID: "telescope", Name: "Laravel Telescope", Scope: "project", Package: "laravel/telescope", Website: "https://laravel.com/docs/telescope", Note: "Installs a development package. Provider/assets setup and migrations require separate review."},
	{ID: "pulse", Name: "Laravel Pulse", Scope: "project", Package: "laravel/pulse", Website: "https://laravel.com/docs/pulse", Note: "Installs a development package. Publishing configuration and database migrations are separate actions."},
	{ID: "clockwork", Name: "Clockwork", Scope: "project", Package: "itsgoingd/clockwork", Website: "https://underground.works/clockwork/", Note: "Composer development dependency in the selected project. Modifies composer.json/lock and can run project Composer scripts."},
	{ID: "pint", Name: "Laravel Pint", Scope: "project", Package: "laravel/pint", Website: "https://laravel.com/docs/pint", Note: "Composer development dependency in the selected project."},
	{ID: "phpstan", Name: "PHPStan", Scope: "project", Package: "phpstan/phpstan", Website: "https://phpstan.org/", Note: "Composer development dependency. Analysis still needs a project PHPStan configuration."},
	{ID: "phpunit", Name: "PHPUnit and Laravel test runner", Scope: "project", Package: "phpunit/phpunit", Website: "https://phpunit.de/", Note: "Installs PHPUnit and Collision as Composer development dependencies. Existing Pest projects are detected as having a test runner."},
	{ID: "node", Name: "Node.js LTS and npm", Scope: "system", Executable: "node.exe", WingetID: "OpenJS.NodeJS.LTS", Website: "https://nodejs.org/", Note: "Interactive system installer. Restart Workbench after installation to refresh PATH."},
	{ID: "git", Name: "Git", Scope: "system", Executable: "git.exe", WingetID: "Git.Git", Website: "https://git-scm.com/", Note: "Interactive system installer; may update PATH."},
	{ID: "postgres", Name: "PostgreSQL 18", Scope: "system", Executable: "postgres.exe", WingetID: "PostgreSQL.PostgreSQL.18", Website: "https://www.postgresql.org/download/windows/", Note: "May require administrator access and create a Windows service. Review credentials and port 5432 in the vendor installer."},
	{ID: "mysql", Name: "MySQL", Scope: "system", Executable: "mysqld.exe", WingetID: "Oracle.MySQL", Website: "https://dev.mysql.com/downloads/", Note: "May require administrator access and create a Windows service. Review credentials and port 3306."},
	{ID: "redis", Name: "Memurai Developer (Redis compatible)", Scope: "system", Executable: "memurai.exe", WingetID: "Memurai.MemuraiDeveloper", Website: "https://www.memurai.com/get-memurai", Note: "Native Windows Redis-compatible engine. Review vendor licensing, administrator prompts and any service on port 6379."},
	{ID: "docker", Name: "Docker Desktop", Scope: "system", Executable: "docker.exe", WingetID: "Docker.DockerDesktop", Website: "https://www.docker.com/products/docker-desktop/", Note: "Optional. Review vendor licensing, virtualization/WSL requirements and possible restart."},
	{ID: "ngrok", Name: "ngrok", Scope: "system", Executable: "ngrok.exe", WingetID: "Ngrok.Ngrok", Website: "https://ngrok.com/download", Note: "Interactive installer. Account/auth-token setup is separate; no tunnel is started."},
	{ID: "herd", Name: "Laravel Herd", Scope: "system", Executable: "herd.bat", WingetID: "BeyondCode.Herd", Website: "https://herd.laravel.com/windows", Note: "Interactive system installer providing local PHP and project serving."},
	{ID: "expose", Name: "Expose CLI", Scope: "global", Package: "beyondcode/expose", Executable: "expose.bat", Website: "https://expose.dev/", Note: "Composer global package in your user profile. Authentication is separate; no tunnel is started."},
}

func Catalog() []Spec { return append([]Spec(nil), catalog...) }
func Lookup(id string) (Spec, error) {
	for _, spec := range catalog {
		if spec.ID == id {
			return spec, nil
		}
	}
	return Spec{}, errors.New("unsupported dependency")
}

type Record struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	Custom  bool   `json:"custom"`
}

func (m *Manager) Resolve(id string) (string, error) {
	spec, err := Lookup(id)
	if err != nil {
		return "", err
	}
	var record Record
	data, _ := os.ReadFile(filepath.Join(m.root, id, "current.json"))
	if json.Unmarshal(data, &record) == nil && record.Path != "" {
		if !record.Custom {
			rel, err := filepath.Rel(filepath.Join(m.root, id), record.Path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
				return "", errors.New("managed tool path leaves its installation directory")
			}
		}
		if info, err := os.Stat(record.Path); err == nil && !info.IsDir() {
			return record.Path, nil
		}
	}
	if spec.Executable != "" {
		if path, err := exec.LookPath(spec.Executable); err == nil {
			return path, nil
		}
	}
	if id == "composer" {
		if executable, err := exec.LookPath("composer.exe"); err == nil {
			return executable, nil
		}
		if path, err := exec.LookPath("composer.bat"); err == nil {
			phar := filepath.Join(filepath.Dir(path), "composer.phar")
			if _, err := os.Stat(phar); err == nil {
				return phar, nil
			}
		}
	}
	programFiles := os.Getenv("ProgramFiles")
	patterns := map[string][]string{
		"postgres": {filepath.Join(programFiles, "PostgreSQL", "*", "bin", "postgres.exe")},
		"mysql":    {filepath.Join(programFiles, "MySQL", "*", "bin", "mysqld.exe")},
		"redis":    {filepath.Join(programFiles, "Memurai", "memurai.exe")},
		"node":     {filepath.Join(programFiles, "nodejs", "node.exe")},
		"git":      {filepath.Join(programFiles, "Git", "cmd", "git.exe")},
		"docker":   {filepath.Join(programFiles, "Docker", "Docker", "resources", "bin", "docker.exe")},
		"herd":     {filepath.Join(os.Getenv("USERPROFILE"), ".config", "herd", "bin", "herd.bat")},
		"expose":   {filepath.Join(os.Getenv("APPDATA"), "Composer", "vendor", "bin", "expose"), filepath.Join(os.Getenv("COMPOSER_HOME"), "vendor", "bin", "expose"), filepath.Join(os.Getenv("APPDATA"), "Composer", "vendor", "bin", "expose.bat")},
	}
	for _, pattern := range patterns[id] {
		matches, _ := filepath.Glob(pattern)
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		for _, path := range matches {
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				return path, nil
			}
		}
	}
	return "", errors.New(spec.Name + " is not installed; open Settings → Dependencies to install it or choose its binary")
}

func (m *Manager) List() []Status {
	result := []Status{}
	_, wingetErr := exec.LookPath("winget.exe")
	for _, spec := range catalog {
		status := Status{Spec: spec, State: "missing", CanInstall: true}
		if spec.Scope == "project" {
			continue
		}
		if spec.Scope == "system" && wingetErr != nil {
			status.CanInstall = false
			status.Message = "WinGet is unavailable. Install Windows App Installer or use the vendor website, then choose the installed binary."
		}
		if path, err := m.Resolve(spec.ID); err == nil {
			status.State = "installed"
			status.Path = path
			status.Message = "Executable detected"
			var record Record
			if bytes, err := os.ReadFile(filepath.Join(m.root, spec.ID, "current.json")); err == nil && json.Unmarshal(bytes, &record) == nil && record.Path == path {
				status.Version = record.Version
			}
		}
		m.mu.Lock()
		progress, ok := m.states[spec.ID]
		m.mu.Unlock()
		if ok && (progress.State == "installing" || status.State != "installed") {
			status.State = progress.State
			status.Message = progress.Message
			status.Progress = progress.Progress
		}
		result = append(result, status)
	}
	return result
}

func ProjectStatus(projectPath string) []Status {
	versions := map[string]string{}
	var installed struct {
		Packages []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"packages"`
	}
	if data, err := os.ReadFile(filepath.Join(projectPath, "vendor", "composer", "installed.json")); err == nil && json.Unmarshal(data, &installed) == nil {
		for _, pkg := range installed.Packages {
			versions[pkg.Name] = pkg.Version
		}
	}
	result := []Status{}
	for _, spec := range catalog {
		if spec.Scope != "project" {
			continue
		}
		s := Status{Spec: spec, State: "missing", CanInstall: projectPath != ""}
		if projectPath == "" {
			s.State = "select_project"
			s.Message = "Select a Laravel project first"
		} else if version := versions[spec.Package]; version != "" {
			s.State = "installed"
			s.Version = version
			s.Message = "Installed in this project's vendor directory"
		} else if spec.ID == "phpunit" && versions["pestphp/pest"] != "" {
			s.State = "installed"
			s.Version = versions["pestphp/pest"]
			s.Message = "Pest test runner detected"
		}
		if spec.ID == "clockwork" && s.State == "installed" {
			psr, _ := os.ReadFile(filepath.Join(projectPath, "vendor", "composer", "autoload_psr4.php"))
			classes, _ := os.ReadFile(filepath.Join(projectPath, "vendor", "composer", "autoload_classmap.php"))
			manifest, _ := os.ReadFile(filepath.Join(projectPath, "bootstrap", "cache", "packages.php"))
			if !strings.Contains(string(psr), "Clockwork") && !strings.Contains(string(classes), "Clockwork") {
				s.State = "incomplete"
				s.Message = "Clockwork files are present, but Composer's autoloader is incomplete. Repair setup to rebuild the autoloader and Laravel package discovery."
			} else if !strings.Contains(string(manifest), "itsgoingd/clockwork") {
				s.State = "incomplete"
				s.Message = "Clockwork files are present, but Laravel package discovery is incomplete or disabled. Repair setup; intentional dont-discover settings are preserved."
			}
		}
		if spec.ID == "phpunit" && s.State == "installed" && versions["nunomaduro/collision"] == "" {
			s.State = "incomplete"
			s.Message = "Test runner detected; install Collision to enable Laravel artisan test"
		}
		result = append(result, s)
	}
	return result
}
