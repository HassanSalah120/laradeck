package wizard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"herdworkbench/internal/recipes"
	"herdworkbench/internal/runner"
	"herdworkbench/internal/services"
)

type StarterFlavors string

const (
	StarterLaravelStandard StarterFlavors = "laravel"
	StarterLivewireBreeze  StarterFlavors = "livewire"
	StarterInertiaReact    StarterFlavors = "react"
	StarterInertiaVue      StarterFlavors = "vue"
	StarterFilament        StarterFlavors = "filament"
	StarterAPI             StarterFlavors = "api"
)

type WizardSpec struct {
	Directory           string   `json:"directory"`
	Name                string   `json:"name"`
	Starter             string   `json:"starter"` // laravel, livewire, react, vue, filament, api
	DatabaseEngine      string   `json:"databaseEngine"` // sqlite, mysql, postgres
	DatabaseName        string   `json:"databaseName"`
	PHPBinary           string   `json:"phpBinary"`
	SelectedPackages    []string `json:"selectedPackages"` // pest, telescope, pulse, horizon, pennant
	GitInit             bool     `json:"gitInit"`
	AutoSyncEnvServices bool     `json:"autoSyncEnvServices"`
	LinkWithHerd        bool     `json:"linkWithHerd"`
}

type WizardStepResult struct {
	Step    string `json:"step"`
	Success bool   `json:"success"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

type WizardResult struct {
	ProjectPath string             `json:"projectPath"`
	Success     bool               `json:"success"`
	Steps       []WizardStepResult `json:"steps"`
	DurationMs  int64              `json:"durationMs"`
}

// CreateProject executes the complete end-to-end Laravel project onboarding pipeline
func CreateProject(ctx context.Context, spec WizardSpec) (*WizardResult, error) {
	if spec.Directory == "" || spec.Name == "" {
		return nil, errors.New("destination directory and project name are required")
	}

	validName := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)
	if !validName.MatchString(spec.Name) {
		return nil, errors.New("project name must contain only letters, numbers, dashes, and underscores")
	}

	targetDir := filepath.Join(spec.Directory, spec.Name)
	if entries, err := os.ReadDir(targetDir); err == nil && len(entries) > 0 {
		return nil, fmt.Errorf("target directory %s already exists and is not empty", targetDir)
	}
	_ = os.MkdirAll(targetDir, 0755)

	startTime := time.Now()
	res := &WizardResult{
		ProjectPath: targetDir,
		Success:     true,
		Steps:       make([]WizardStepResult, 0),
	}

	recordStep := func(stepName string, success bool, out string, err error) {
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		res.Steps = append(res.Steps, WizardStepResult{
			Step:    stepName,
			Success: success,
			Output:  strings.TrimSpace(out),
			Error:   errStr,
		})
		if !success {
			res.Success = false
		}
	}

	php := spec.PHPBinary
	if php == "" {
		php = "php"
	}

	// Step 1: composer create-project
	compSpec, err := recipes.ComposerSpec(php, targetDir, []string{"create-project", "laravel/laravel:^12.0", ".", "--prefer-dist", "--no-interaction"})
	if err != nil {
		recordStep("Composer Setup", false, "", err)
		res.DurationMs = time.Since(startTime).Milliseconds()
		return res, err
	}

	out, err := runner.CaptureContext(ctx, compSpec, 300*time.Second)
	if err != nil {
		recordStep("Create Project", false, string(out), err)
		res.DurationMs = time.Since(startTime).Milliseconds()
		return res, fmt.Errorf("composer create-project failed: %w", err)
	}
	recordStep("Create Project", true, "Laravel 12 skeleton scaffolded successfully", nil)

	// Step 2: Database Provisioning & .env configuration
	dbEngine := strings.ToLower(spec.DatabaseEngine)
	if dbEngine == "" {
		dbEngine = "sqlite"
	}
	dbName := spec.DatabaseName
	if dbName == "" {
		dbName = strings.ReplaceAll(spec.Name, "-", "_")
	}

	envOpts := services.EnvSyncOptions{
		DatabaseEngine:       dbEngine,
		DatabaseName:         dbName,
		ConfigureRedis:       spec.AutoSyncEnvServices,
		ConfigureMailpit:     spec.AutoSyncEnvServices,
		ConfigureMeilisearch: spec.AutoSyncEnvServices,
	}
	envPath := filepath.Join(targetDir, ".env")
	if _, syncErr := services.ApplyServicesToEnv(envPath, envOpts); syncErr != nil {
		recordStep("Configure .env", false, "", syncErr)
	} else {
		recordStep("Configure .env", true, fmt.Sprintf("Set database to %s and synchronized services", dbEngine), nil)
	}

	if dbEngine != "sqlite" {
		if provErr := services.AutoProvisionDatabase(ctx, dbEngine, dbName, php, targetDir); provErr != nil {
			recordStep("Provision Database", false, "", provErr)
		} else {
			recordStep("Provision Database", true, fmt.Sprintf("Provisioned %s database %s", dbEngine, dbName), nil)
		}
	}

	// Step 3: Starter flavor setup
	switch strings.ToLower(spec.Starter) {
	case "api":
		apiOut, apiErr := runner.CaptureContext(ctx, runner.Spec{
			Executable: php,
			Args:       []string{"artisan", "install:api", "--no-interaction"},
			Directory:  targetDir,
		}, 60*time.Second)
		recordStep("Setup API", apiErr == nil, string(apiOut), apiErr)

	case "livewire", "breeze":
		breezeSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "laravel/breeze", "--dev", "--no-interaction"})
		if bOut, bErr := runner.CaptureContext(ctx, breezeSpec, 180*time.Second); bErr == nil {
			instOut, instErr := runner.CaptureContext(ctx, runner.Spec{
				Executable: php,
				Args:       []string{"artisan", "breeze:install", "livewire", "--no-interaction"},
				Directory:  targetDir,
			}, 120*time.Second)
			recordStep("Install Livewire", instErr == nil, string(instOut), instErr)
		} else {
			recordStep("Install Livewire", false, string(bOut), bErr)
		}

	case "react":
		breezeSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "laravel/breeze", "--dev", "--no-interaction"})
		if bOut, bErr := runner.CaptureContext(ctx, breezeSpec, 180*time.Second); bErr == nil {
			instOut, instErr := runner.CaptureContext(ctx, runner.Spec{
				Executable: php,
				Args:       []string{"artisan", "breeze:install", "react", "--typescript", "--no-interaction"},
				Directory:  targetDir,
			}, 120*time.Second)
			recordStep("Install Inertia React", instErr == nil, string(instOut), instErr)
		} else {
			recordStep("Install Inertia React", false, string(bOut), bErr)
		}

	case "vue":
		breezeSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "laravel/breeze", "--dev", "--no-interaction"})
		if bOut, bErr := runner.CaptureContext(ctx, breezeSpec, 180*time.Second); bErr == nil {
			instOut, instErr := runner.CaptureContext(ctx, runner.Spec{
				Executable: php,
				Args:       []string{"artisan", "breeze:install", "vue", "--typescript", "--no-interaction"},
				Directory:  targetDir,
			}, 120*time.Second)
			recordStep("Install Inertia Vue", instErr == nil, string(instOut), instErr)
		} else {
			recordStep("Install Inertia Vue", false, string(bOut), bErr)
		}

	case "filament":
		filSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "filament/filament:^3.2", "-W", "--no-interaction"})
		if fOut, fErr := runner.CaptureContext(ctx, filSpec, 180*time.Second); fErr == nil {
			instOut, instErr := runner.CaptureContext(ctx, runner.Spec{
				Executable: php,
				Args:       []string{"artisan", "filament:install", "--panels", "--no-interaction"},
				Directory:  targetDir,
			}, 120*time.Second)
			recordStep("Install Filament", instErr == nil, string(instOut), instErr)
		} else {
			recordStep("Install Filament", false, string(fOut), fErr)
		}
	}

	// Step 4: Install selected package presets
	for _, pkg := range spec.SelectedPackages {
		switch strings.ToLower(pkg) {
		case "pest":
			pSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "pestphp/pest", "pestphp/pest-plugin-laravel", "--dev", "--no-interaction"})
			if pOut, pErr := runner.CaptureContext(ctx, pSpec, 180*time.Second); pErr == nil {
				_, _ = runner.CaptureContext(ctx, runner.Spec{
					Executable: php,
					Args:       []string{"artisan", "pest:install", "--no-interaction"},
					Directory:  targetDir,
				}, 60*time.Second)
				recordStep("Install Pest 3", true, "Configured Pest testing framework", nil)
			} else {
				recordStep("Install Pest 3", false, string(pOut), pErr)
			}
		case "telescope":
			tSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "laravel/telescope", "--dev", "--no-interaction"})
			if tOut, tErr := runner.CaptureContext(ctx, tSpec, 180*time.Second); tErr == nil {
				_, _ = runner.CaptureContext(ctx, runner.Spec{
					Executable: php,
					Args:       []string{"artisan", "telescope:install", "--no-interaction"},
					Directory:  targetDir,
				}, 60*time.Second)
				recordStep("Install Telescope", true, "Configured Laravel Telescope", nil)
			} else {
				recordStep("Install Telescope", false, string(tOut), tErr)
			}
		case "pulse":
			puSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "laravel/pulse", "--no-interaction"})
			if puOut, puErr := runner.CaptureContext(ctx, puSpec, 180*time.Second); puErr == nil {
				recordStep("Install Pulse", true, "Configured Laravel Pulse", nil)
			} else {
				recordStep("Install Pulse", false, string(puOut), puErr)
			}
		case "horizon":
			hSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "laravel/horizon", "--no-interaction"})
			if hOut, hErr := runner.CaptureContext(ctx, hSpec, 180*time.Second); hErr == nil {
				_, _ = runner.CaptureContext(ctx, runner.Spec{
					Executable: php,
					Args:       []string{"artisan", "horizon:install", "--no-interaction"},
					Directory:  targetDir,
				}, 60*time.Second)
				recordStep("Install Horizon", true, "Configured Laravel Horizon", nil)
			} else {
				recordStep("Install Horizon", false, string(hOut), hErr)
			}
		case "pennant":
			peSpec, _ := recipes.ComposerSpec(php, targetDir, []string{"require", "laravel/pennant", "--no-interaction"})
			if peOut, peErr := runner.CaptureContext(ctx, peSpec, 180*time.Second); peErr == nil {
				recordStep("Install Pennant", true, "Configured Feature Flags (Pennant)", nil)
			} else {
				recordStep("Install Pennant", false, string(peOut), peErr)
			}
		}
	}

	// Step 5: Run migrations
	migOut, migErr := runner.CaptureContext(ctx, runner.Spec{
		Executable: php,
		Args:       []string{"artisan", "migrate", "--force", "--no-interaction"},
		Directory:  targetDir,
	}, 60*time.Second)
	if migErr == nil {
		recordStep("Database Migrations", true, "Database migrated successfully", nil)
	} else {
		recordStep("Database Migrations", false, string(migOut), migErr)
	}

	// Step 6: Git Init
	if spec.GitInit {
		gitOut, gitErr := runner.CaptureContext(ctx, runner.Spec{
			Executable: "git",
			Args:       []string{"init"},
			Directory:  targetDir,
		}, 10*time.Second)
		if gitErr == nil {
			_, _ = runner.CaptureContext(ctx, runner.Spec{
				Executable: "git",
				Args:       []string{"add", "."},
				Directory:  targetDir,
			}, 15*time.Second)
			_, _ = runner.CaptureContext(ctx, runner.Spec{
				Executable: "git",
				Args:       []string{"commit", "-m", "Initial commit from LaraDeck Wizard"},
				Directory:  targetDir,
			}, 15*time.Second)
			recordStep("Git Repository", true, "Initialized Git repository with initial commit", nil)
		} else {
			recordStep("Git Repository", false, string(gitOut), gitErr)
		}
	}

	// Step 7: Link with Herd if requested
	if spec.LinkWithHerd {
		if herdBin, err := exec.LookPath("herd"); err == nil {
			cmd := exec.CommandContext(ctx, herdBin, "link", spec.Name)
			cmd.Dir = targetDir
			cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
			if hOut, err := cmd.CombinedOutput(); err == nil {
				recordStep("Herd Link", true, fmt.Sprintf("Linked site with Herd as http://%s.test", spec.Name), nil)
			} else {
				recordStep("Herd Link", false, string(hOut), err)
			}
		}
	}

	res.DurationMs = time.Since(startTime).Milliseconds()
	return res, nil
}
