package workbench

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/recipes"
	"herdworkbench/internal/runner"
	"herdworkbench/internal/storage"
)

// Installation state is separate from package presence: Composer can still be
// running scripts or checks after vendor metadata reports a package installed.
type DependencyInstallation struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"projectId"`
	Name      string   `json:"name"`
	Running   bool     `json:"running"`
	State     string   `json:"state"`
	StartedAt string   `json:"startedAt"`
	Message   string   `json:"message"`
	Output    []string `json:"output"`
}

func (b *Backend) CancelDependencyInstall() error {
	b.mu.Lock()
	cancel := b.installCancel
	if cancel != nil {
		b.installation.Message = "Stopping installation; waiting for the owned process tree to exit"
	}
	b.mu.Unlock()
	if cancel != nil {
		cancel()
		b.emit()
	}
	return nil
}
func (b *Backend) installOutput(events []runner.Event) {
	output := []string{}
	for _, event := range events {
		if event.Text != "" && (event.Kind == "stdout" || event.Kind == "stderr" || event.Kind == "error") {
			text := storage.RedactSensitive(event.Text)
			if len(text) > 2000 {
				text = text[:2000] + " ..."
			}
			output = append(output, text)
		}
	}
	if len(output) > 20 {
		output = output[len(output)-20:]
	}
	b.mu.Lock()
	changed := strings.Join(output, "\n") != strings.Join(b.installation.Output, "\n")
	b.installation.Output = output
	b.mu.Unlock()
	if changed {
		b.emit()
	}
}

func (b *Backend) DependencyStatus(projectID string) []dependencies.Status {
	result := b.dependencies.List()
	b.mu.Lock()
	var selected Project
	for _, p := range b.projects {
		if p.ID == projectID {
			selected = p
			break
		}
	}
	trusted := b.trusted[projectID]
	b.mu.Unlock()
	project := dependencies.ProjectStatus(selected.Path)
	for i := range project {
		if !trusted {
			project[i].CanInstall = false
			if selected.Path != "" {
				project[i].Message = "Trust this project before installing development packages"
			}
		}
	}
	return append(result, project...)
}
func (b *Backend) SetDependencyBinary(id, path string) error {
	if err := b.dependencies.SetBinary(id, path); err != nil {
		return err
	}
	b.emit()
	return nil
}
func (b *Backend) InstallDependency(id, projectID string) (result dependencies.Status, err error) {
	ctx, done, err := b.beginOperation()
	if err != nil {
		return result, err
	}
	defer done()
	if !b.dependencyInstallMu.TryLock() {
		return result, errors.New("another dependency installation is running")
	}
	defer b.dependencyInstallMu.Unlock()
	spec, err := dependencies.Lookup(id)
	if err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	ctx, cancel := context.WithCancel(ctx)
	b.mu.Lock()
	b.installCancel = cancel
	b.installation = DependencyInstallation{ID: id, ProjectID: projectID, Name: spec.Name, Running: true, State: "installing", StartedAt: time.Now().UTC().Format(time.RFC3339), Message: "Installing; package files may appear before Composer scripts and checks finish", Output: []string{}}
	b.mu.Unlock()
	b.emit()
	defer func() {
		cancel()
		b.mu.Lock()
		b.installCancel = nil
		b.installation.Running = false
		b.installation.State = "completed"
		b.installation.Message = "Installation completed"
		if err != nil {
			b.installation.State = "failed"
			b.installation.Message = storage.RedactSensitive(err.Error())
			if errors.Is(err, context.Canceled) {
				b.installation.State = "cancelled"
				b.installation.Message = "Installation cancelled. Composer may have changed project files; recheck dependencies before retrying."
			}
		} else if result.State == "installer_opened" {
			b.installation.Message = result.Message
		}
		b.mu.Unlock()
		b.emit()
	}()
	if spec.Scope == "app" {
		return b.dependencies.Install(ctx, id)
	}
	if spec.Scope == "system" {
		return b.dependencies.OpenSystemInstaller(id)
	}
	var project Project
	if spec.Scope == "project" {
		project, err = b.project(projectID, true)
		if err != nil {
			return result, err
		}
	} else {
		herd, findErr := FindHerd()
		if findErr != nil {
			return result, findErr
		}
		project = Project{PHPBinary: herd.PHP, Path: storage.AppDataDir()}
	}
	b.dependencies.Report(id, "installing", "Installing Composer package; wait for Composer to finish")
	defer func() {
		if err != nil {
			b.dependencies.Report(id, "failed", storage.RedactSensitive(err.Error()))
		}
	}()
	args := []string{"require", "--dev", spec.Package, "--no-interaction", "--prefer-dist"}
	if id == "clockwork" {
		for _, status := range dependencies.ProjectStatus(project.Path) {
			if status.ID == id && status.Version != "" {
				if status.State == "installed" {
					return status, nil
				}
				args = []string{"dump-autoload", "--no-scripts", "--no-interaction"}
				break
			}
		}
	}
	if id == "phpunit" {
		args = append(args, "nunomaduro/collision")
	}
	if spec.Scope == "global" {
		args = []string{"global", "require", spec.Package, "--no-interaction", "--prefer-dist"}
	}
	if spec.Scope == "project" {
		// Preserve the two project manifests, but never copy auth.json or environment secrets.
		for _, name := range []string{"composer.json", "composer.lock"} {
			path := filepath.Join(project.Path, name)
			bytes, readErr := os.ReadFile(path)
			if os.IsNotExist(readErr) {
				continue
			}
			if readErr != nil {
				return result, readErr
			}
			if err := os.WriteFile(fmt.Sprintf("%s.workbench-backup.%d", path, time.Now().UnixNano()), bytes, 0600); err != nil {
				return result, fmt.Errorf("could not back up %s: %w", name, err)
			}
		}
	}
	command, err := recipes.ComposerSpec(project.PHPBinary, project.Path, args)
	if err != nil {
		return result, err
	}
	execution, err := runner.CaptureExecContextWithProgress(ctx, command, 5*time.Minute, b.installOutput)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return result, err
		}
		detail := ""
		if execution != nil {
			detail = storage.RedactSensitive(execution.Combined)
			if len(detail) > 12000 {
				detail = detail[len(detail)-12000:]
			}
		}
		return result, fmt.Errorf("Composer did not finish: %w. Package files may already exist; review the last output before retrying. %s", err, detail)
	}
	if execution.ExitCode != 0 {
		return result, fmt.Errorf("Composer exited with code %d: %s", execution.ExitCode, storage.RedactSensitive(execution.Combined))
	}
	if id == "clockwork" {
		// Rebuild Laravel's generated package list without booting the application or
		// executing unrelated project Composer hooks. Respect dont-discover entries.
		code := `$root=getcwd();require $root.'/vendor/autoload.php';$path=$root.'/bootstrap/cache/packages.php';if(is_file($path)&&!copy($path,$path.'.workbench-backup.'.bin2hex(random_bytes(6)))){fwrite(STDERR,'Cannot back up package manifest');exit(1);}$manifest=new Illuminate\Foundation\PackageManifest(new Illuminate\Filesystem\Filesystem,$root,$path);$manifest->build();echo "Laravel package discovery refreshed\n";`
		discovery, discoveryErr := runner.CaptureExecContextWithProgress(ctx, runner.Spec{Executable: project.PHPBinary, Directory: project.Path, Args: []string{"-r", code}}, 30*time.Second, b.installOutput)
		if discoveryErr != nil {
			return result, fmt.Errorf("Clockwork package discovery did not finish: %w", discoveryErr)
		}
		if discovery.ExitCode != 0 {
			return result, fmt.Errorf("Clockwork package discovery failed: %s", storage.RedactSensitive(discovery.Combined))
		}
	}
	if spec.Scope == "global" {
		path, err := b.dependencies.Resolve(id)
		if err != nil {
			return result, errors.New("Composer completed, but Expose binary was not found; check Composer's global bin directory")
		}
		b.dependencies.Report(id, "installed", "Composer global package installed")
		return dependencies.Status{Spec: spec, State: "installed", Path: path, Message: "Composer global package installed", CanInstall: true}, nil
	}
	for _, status := range dependencies.ProjectStatus(project.Path) {
		if status.ID == id {
			result = status
			break
		}
	}
	if result.State != "installed" {
		return result, fmt.Errorf("Composer finished, but package setup is incomplete: %s", result.Message)
	}
	InspectDependencies(&project)
	b.mu.Lock()
	for i := range b.projects {
		if b.projects[i].ID == projectID {
			b.projects[i] = project
		}
	}
	selected := b.selected == projectID
	for key := range b.caps {
		if strings.HasPrefix(key, projectID) {
			delete(b.caps, key)
		}
	}
	b.mu.Unlock()
	if selected {
		b.clock.Select(project)
	}
	b.dependencies.Report(id, "installed", "Development package installed in selected project")
	b.emit()
	return result, nil
}
