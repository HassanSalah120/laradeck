package workbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"herdworkbench/internal/runner"
)

type Project struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	URL             string `json:"url"`
	PHP             string `json:"php"`
	PHPBinary       string `json:"-"`
	Framework       string `json:"framework"`
	Clockwork       string `json:"clockwork"`
	VendorPresent   bool   `json:"vendorPresent"`
	DependencyState string `json:"dependencyState"`
	Secured         bool   `json:"secured"`
}
type Herd struct {
	PHP  string
	Phar string
}

func FindHerd() (Herd, error) {
	h := Herd{}
	batch, err := exec.LookPath("herd.bat")
	if err != nil {
		return h, errors.New("Herd was not found in PATH. Start Herd and reopen Workbench")
	}
	root := filepath.Dir(batch)
	h.Phar = filepath.Join(root, "herd.phar")
	candidates, _ := filepath.Glob(filepath.Join(root, "php*", "php.exe"))
	sort.Sort(sort.Reverse(sort.StringSlice(candidates)))
	if len(candidates) == 0 {
		return h, errors.New("No Herd PHP executable found")
	}
	h.PHP = candidates[0]
	return h, nil
}
func (h Herd) command(dir string, args ...string) ([]byte, error) {
	return runner.Capture(runner.Spec{Executable: h.PHP, Args: append([]string{h.Phar}, args...), Directory: dir}, 20*time.Second)
}
func (h Herd) Discover() ([]Project, error) {
	raw, err := h.command("", "sites", "--json", "--no-interaction")
	if err != nil {
		return nil, err
	}
	// Decode only these fields: Herd's complete objects can include environment secrets.
	var sites []struct {
		Site    string `json:"site"`
		Path    string `json:"path"`
		URL     string `json:"url"`
		PHP     string `json:"phpVersion"`
		Secured any    `json:"secured"`
	}
	if json.Unmarshal(raw, &sites) != nil {
		return nil, errors.New("Herd returned invalid site JSON")
	}
	projects := make([]Project, 0, len(sites))
	for _, site := range sites {
		if site.Path == "" {
			continue
		}
		digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(site.Path))))
		sec := false
		if b, ok := site.Secured.(bool); ok {
			sec = b
		} else if s, ok := site.Secured.(string); ok && strings.EqualFold(s, "yes") {
			sec = true
		}
		p := Project{ID: hex.EncodeToString(digest[:8]), Name: site.Site, Path: site.Path, URL: site.URL, PHP: site.PHP, Secured: sec}
		php, err := h.command(site.Path, "which-php")
		if err == nil {
			p.PHPBinary = strings.TrimSpace(string(php))
		}
		InspectDependencies(&p)
		projects = append(projects, p)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}

func (h Herd) Secure(projectPath string) error {
	_, err := h.command(projectPath, "secure")
	return err
}

func (h Herd) Unsecure(projectPath string) error {
	_, err := h.command(projectPath, "unsecure")
	return err
}

type packageItem struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func InspectDependencies(p *Project) {
	p.Framework = "unknown"
	p.Clockwork = "not installed"
	p.DependencyState = "Lockfile unavailable"
	raw, err := os.ReadFile(filepath.Join(p.Path, "composer.lock"))
	if err != nil {
		return
	}
	var lock struct {
		Packages []packageItem `json:"packages"`
		Dev      []packageItem `json:"packages-dev"`
	}
	if json.Unmarshal(raw, &lock) != nil {
		p.DependencyState = "Unreadable lockfile"
		return
	}
	expected := map[string]string{}
	for _, pkg := range append(lock.Packages, lock.Dev...) {
		expected[pkg.Name] = pkg.Version
	}
	if v := expected["laravel/framework"]; v != "" {
		p.Framework = v
	}
	if v := expected["itsgoingd/clockwork"]; v != "" {
		p.Clockwork = v + " (locked)"
	}
	_, err = os.Stat(filepath.Join(p.Path, "vendor", "autoload.php"))
	p.VendorPresent = err == nil
	p.DependencyState = "Dependencies not installed"
	if !p.VendorPresent {
		return
	}
	raw, err = os.ReadFile(filepath.Join(p.Path, "vendor", "composer", "installed.json"))
	if err != nil {
		p.DependencyState = "Installed dependencies not verified"
		return
	}
	var installed struct {
		Packages []packageItem `json:"packages"`
	}
	if json.Unmarshal(raw, &installed) != nil {
		p.DependencyState = "Installed dependencies not verified"
		return
	}
	actual := map[string]string{}
	for _, pkg := range installed.Packages {
		actual[pkg.Name] = pkg.Version
	}
	p.DependencyState = "Installed metadata matches"
	for _, name := range []string{"laravel/framework", "itsgoingd/clockwork"} {
		if expected[name] != actual[name] {
			p.DependencyState = "Lockfile/vendor mismatch"
		}
	}
	if v := actual["itsgoingd/clockwork"]; v != "" {
		p.Clockwork = v
	} else {
		p.Clockwork = "not installed"
	}
}

type Capability struct {
	Command string `json:"command"`
	JSON    bool   `json:"json"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

func Capabilities(p Project) []Capability {
	out := []Capability{}
	for _, name := range []string{"route:list", "about", "queue:failed", "channel:list"} {
		c := Capability{Command: name, Status: "unknown"}
		data, err := runner.Capture(runner.Spec{Executable: p.PHPBinary, Directory: p.Path, Args: []string{"artisan", "help", name, "--format=json", "--no-ansi", "--no-interaction"}}, 15*time.Second)
		if err != nil {
			c.Reason = err.Error()
			out = append(out, c)
			continue
		}
		var help struct {
			Definition struct {
				Options map[string]json.RawMessage `json:"options"`
			} `json:"definition"`
		}
		if json.Unmarshal(data, &help) != nil || help.Definition.Options == nil {
			c.Reason = "Help did not return a valid option definition"
		} else {
			_, c.JSON = help.Definition.Options["json"]
			c.Status = "verified"
			if !c.JSON {
				c.Reason = "This installed command has no JSON flag"
			}
		}
		out = append(out, c)
	}
	return out
}

type Route struct {
	Method string `json:"method"`
	URI    string `json:"uri"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

func Routes(p Project) ([]Route, error) {
	data, err := runner.Capture(runner.Spec{Executable: p.PHPBinary, Directory: p.Path, Args: []string{"artisan", "route:list", "--json", "--no-ansi", "--no-interaction"}}, 20*time.Second)
	if err != nil {
		return nil, err
	}
	var routes []Route
	if json.Unmarshal(data, &routes) != nil {
		return nil, fmt.Errorf("The installed route:list did not return valid JSON")
	}
	return routes, nil
}
