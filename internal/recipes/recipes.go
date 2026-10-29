package recipes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"herdworkbench/internal/dependencies"
	"herdworkbench/internal/environment"
	"herdworkbench/internal/runner"
)

type StepType string

const (
	StepCommand    StepType = "command"
	StepWriteFile  StepType = "write_file"
	StepAppendFile StepType = "append_file"
	StepEnvSet     StepType = "env_set"
	StepVerify     StepType = "verify"
)

type RecipeStep struct {
	Type        StepType `json:"type"`
	Title       string   `json:"title"`
	Command     string   `json:"command,omitempty"`
	Args        []string `json:"args,omitempty"`
	TargetFile  string   `json:"targetFile,omitempty"`
	Content     string   `json:"content,omitempty"`
	EnvKey      string   `json:"envKey,omitempty"`
	EnvValue    string   `json:"envValue,omitempty"`
	VerifyCheck string   `json:"verifyCheck,omitempty"`
}

type Recipe struct {
	ID                   string       `json:"id"`
	Title                string       `json:"title"`
	Description          string       `json:"description"`
	Category             string       `json:"category"` // "starter", "package", "theme", "setup"
	Prerequisites        []string     `json:"prerequisites"`
	Steps                []RecipeStep `json:"steps"`
	RollbackInstructions string       `json:"rollbackInstructions"`
}

type RecipePreview struct {
	RecipeID      string   `json:"recipeId"`
	Title         string   `json:"title"`
	CommandsToRun []string `json:"commandsToRun"`
	FilesModified []string `json:"filesModified"`
	EnvKeysSet    []string `json:"envKeysSet"`
}

type StepExecutionResult struct {
	StepIndex int      `json:"stepIndex"`
	Title     string   `json:"title"`
	Success   bool     `json:"success"`
	Output    []string `json:"output"`
	Error     string   `json:"error,omitempty"`
}

type RecipeExecutionResult struct {
	RecipeID    string                `json:"recipeId"`
	Success     bool                  `json:"success"`
	StepResults []StepExecutionResult `json:"stepResults"`
	DurationMs  float64               `json:"durationMs"`
	Recovery    string                `json:"recoveryGuidance,omitempty"`
}

type ProjectWizardSpec struct {
	Directory string `json:"directory"`
	Name      string `json:"name"`
	Starter   string `json:"starter"`  // "none", "breeze", "api"
	Database  string `json:"database"` // "sqlite", "mysql", "postgres"
	PHPBinary string `json:"phpBinary"`
	GitInit   bool   `json:"gitInit"`
}

type RecipeCatalog struct {
	recipes map[string]Recipe
}

func NewRecipeCatalog() *RecipeCatalog {
	rc := &RecipeCatalog{recipes: make(map[string]Recipe)}
	rc.registerBundledRecipes()
	return rc
}

func (rc *RecipeCatalog) registerBundledRecipes() {
	// 1. Livewire 3
	rc.recipes["livewire-3"] = Recipe{
		ID:            "livewire-3",
		Title:         "Install Livewire 3",
		Description:   "Adds Livewire 3 to the project and publishes its configuration assets.",
		Category:      "package",
		Prerequisites: []string{"composer.json", "artisan"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Install Livewire via Composer",
				Command: "composer",
				Args:    []string{"require", "livewire/livewire:^3.0", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Publish Livewire configuration",
				Command: "artisan",
				Args:    []string{"livewire:publish", "--config", "--no-interaction"},
			},
			{
				Type:        StepVerify,
				Title:       "Verify Livewire installation",
				VerifyCheck: "vendor/livewire/livewire",
			},
		},
		RollbackInstructions: "Run 'composer remove livewire/livewire' and delete config/livewire.php if created.",
	}

	// 2. Filament v3 Admin Panel
	rc.recipes["filament-v3"] = Recipe{
		ID:            "filament-v3",
		Title:         "Install Filament v3 Admin Panel",
		Description:   "Installs Filament Admin Panel, publishes panel provider, and configures themes.",
		Category:      "theme",
		Prerequisites: []string{"composer.json", "artisan"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Filament Panel package",
				Command: "composer",
				Args:    []string{"require", "filament/filament:^3.2", "-W", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Install Filament Panel Provider",
				Command: "artisan",
				Args:    []string{"filament:install", "--panels", "--no-interaction"},
			},
			{
				Type:        StepVerify,
				Title:       "Verify Filament Provider",
				VerifyCheck: "app/Providers/Filament",
			},
		},
		RollbackInstructions: "Run 'composer remove filament/filament' and remove registered panel provider in bootstrap/providers.php or config/app.php.",
	}

	// 3. Horizon
	rc.recipes["horizon"] = Recipe{
		ID:            "horizon",
		Title:         "Install Laravel Horizon",
		Description:   "Installs Redis queue monitoring dashboard.",
		Category:      "package",
		Prerequisites: []string{"composer.json", "artisan"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Laravel Horizon",
				Command: "composer",
				Args:    []string{"require", "laravel/horizon", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Publish Horizon assets",
				Command: "artisan",
				Args:    []string{"horizon:install", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/horizon' and remove config/horizon.php.",
	}

	// 4. Production Optimization
	rc.recipes["production-optimize"] = Recipe{
		ID:            "production-optimize",
		Title:         "Production Readiness Optimization",
		Description:   "Caches configuration, routes, views, and events for maximum production speed.",
		Category:      "setup",
		Prerequisites: []string{"artisan"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Cache configuration",
				Command: "artisan",
				Args:    []string{"config:cache", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Cache routes",
				Command: "artisan",
				Args:    []string{"route:cache", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Cache views",
				Command: "artisan",
				Args:    []string{"view:cache", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Cache events",
				Command: "artisan",
				Args:    []string{"event:cache", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'php artisan optimize:clear' to flush all cached configuration and routes.",
	}

	// 5. Laravel Pulse
	rc.recipes["pulse"] = Recipe{
		ID:            "pulse",
		Title:         "Install Laravel Pulse",
		Description:   "At-a-glance application performance, slow queries, and system health dashboard.",
		Category:      "package",
		Prerequisites: []string{"composer.json", "artisan"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Laravel Pulse",
				Command: "composer",
				Args:    []string{"require", "laravel/pulse", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Publish Pulse assets & migrations",
				Command: "artisan",
				Args:    []string{"vendor:publish", "--provider=Laravel\\Pulse\\PulseServiceProvider", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/pulse' and delete config/pulse.php.",
	}

	// 6. Laravel Telescope
	rc.recipes["telescope"] = Recipe{
		ID:            "telescope",
		Title:         "Install Laravel Telescope",
		Description:   "Comprehensive local debugging assistant for requests, exceptions, database queries, and mail.",
		Category:      "package",
		Prerequisites: []string{"composer.json", "artisan"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Laravel Telescope for Development",
				Command: "composer",
				Args:    []string{"require", "laravel/telescope", "--dev", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Install Telescope assets and migrations",
				Command: "artisan",
				Args:    []string{"telescope:install", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/telescope --dev' and delete config/telescope.php.",
	}

	// 7. Laravel Sanctum
	rc.recipes["sanctum"] = Recipe{
		ID:            "sanctum",
		Title:         "Install Laravel Sanctum API Auth",
		Description:   "Featherweight authentication guard for SPAs, mobile applications, and token-based APIs.",
		Category:      "package",
		Prerequisites: []string{"composer.json", "artisan"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Laravel Sanctum",
				Command: "composer",
				Args:    []string{"require", "laravel/sanctum", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Publish Sanctum configuration & migrations",
				Command: "artisan",
				Args:    []string{"vendor:publish", "--provider=Laravel\\Sanctum\\SanctumServiceProvider", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/sanctum' and delete config/sanctum.php.",
	}

	// 8. Pest PHP Test Suite
	rc.recipes["pest"] = Recipe{
		ID:            "pest",
		Title:         "Install Pest PHP & Architecture Testing",
		Description:   "Delightful, expressive PHP testing framework with built-in architecture assertion rules.",
		Category:      "setup",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Pest and Pest Laravel Plugin",
				Command: "composer",
				Args:    []string{"require", "pestphp/pest", "pestphp/pest-plugin-laravel", "--dev", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Initialize Pest configuration",
				Command: "artisan",
				Args:    []string{"pest:install", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove pestphp/pest pestphp/pest-plugin-laravel --dev' and delete tests/Pest.php.",
	}

	// 9. Laravel Pint & Larastan
	rc.recipes["pint-larastan"] = Recipe{
		ID:            "pint-larastan",
		Title:         "Code Quality: Pint & Larastan",
		Description:   "Installs Laravel Pint zero-config code linter and Larastan strict static typing.",
		Category:      "setup",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Pint and Larastan",
				Command: "composer",
				Args:    []string{"require", "laravel/pint", "larastan/larastan", "--dev", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/pint larastan/larastan --dev'.",
	}

	// 10. Laravel Sail (Docker)
	rc.recipes["sail"] = Recipe{
		ID:            "sail",
		Title:         "Install Laravel Sail (Docker Compose)",
		Description:   "Lightweight CLI experience for managing Docker environment with MySQL, Redis, and Mailpit.",
		Category:      "setup",
		Prerequisites: []string{"composer.json", "artisan"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Laravel Sail",
				Command: "composer",
				Args:    []string{"require", "laravel/sail", "--dev", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Publish Sail docker-compose file",
				Command: "artisan",
				Args:    []string{"sail:install", "--with=mysql,redis,mailpit", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/sail --dev' and delete docker-compose.yml.",
	}

	// 11. Tailwind CSS & Inertia.js
	rc.recipes["tailwind-inertia"] = Recipe{
		ID:            "tailwind-inertia",
		Title:         "Tailwind CSS v4 & Inertia.js Setup",
		Description:   "Configures Vite for modern single-page apps with server-driven routing and utility CSS.",
		Category:      "theme",
		Prerequisites: []string{"composer.json", "package.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Inertia Laravel adapter",
				Command: "composer",
				Args:    []string{"require", "inertiajs/inertia-laravel", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove inertiajs/inertia-laravel'.",
	}

	// 12. Laravel Cashier (Stripe)
	rc.recipes["cashier"] = Recipe{
		ID:            "cashier",
		Title:         "Install Laravel Cashier (Stripe)",
		Description:   "Expressive, fluent interface to Stripe subscription billing services.",
		Category:      "package",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Laravel Cashier",
				Command: "composer",
				Args:    []string{"require", "laravel/cashier", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/cashier'.",
	}

	// 13. Laravel Reverb
	rc.recipes["reverb"] = Recipe{
		ID:            "reverb",
		Title:         "Install Laravel Reverb (WebSockets)",
		Description:   "Blazing-fast, first-party WebSocket server built directly for Laravel.",
		Category:      "package",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Install Laravel Reverb",
				Command: "php",
				Args:    []string{"artisan", "install:broadcasting", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/reverb' and delete config/reverb.php.",
	}

	// 14. Laravel Pennant
	rc.recipes["pennant"] = Recipe{
		ID:            "pennant",
		Title:         "Install Laravel Pennant (Feature Flags)",
		Description:   "Simple, lightweight feature flag manager for gradual rollout and A/B testing.",
		Category:      "package",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Laravel Pennant",
				Command: "composer",
				Args:    []string{"require", "laravel/pennant", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Publish Pennant Configuration",
				Command: "php",
				Args:    []string{"artisan", "vendor:publish", "--provider=Laravel\\Pennant\\PennantServiceProvider"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/pennant' and delete config/pennant.php.",
	}

	// 15. Laravel Scout with Meilisearch
	rc.recipes["scout-meilisearch"] = Recipe{
		ID:            "scout-meilisearch",
		Title:         "Install Laravel Scout & Meilisearch",
		Description:   "Sync Eloquent models with ultra-fast full-text search indexes in Meilisearch.",
		Category:      "package",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Scout & Meilisearch Driver",
				Command: "composer",
				Args:    []string{"require", "laravel/scout", "meilisearch/meilisearch-php", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Publish Scout Configuration",
				Command: "php",
				Args:    []string{"artisan", "vendor:publish", "--provider=Laravel\\Scout\\ScoutServiceProvider"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/scout meilisearch/meilisearch-php'.",
	}

	// 16. Laravel Socialite
	rc.recipes["socialite"] = Recipe{
		ID:            "socialite",
		Title:         "Install Laravel Socialite (OAuth)",
		Description:   "OAuth authentication with GitHub, Google, Twitter, Facebook, and more.",
		Category:      "package",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Laravel Socialite",
				Command: "composer",
				Args:    []string{"require", "laravel/socialite", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove laravel/socialite'.",
	}

	// 17. Spatie Permission
	rc.recipes["spatie-permission"] = Recipe{
		ID:            "spatie-permission",
		Title:         "Install Spatie Permission (RBAC)",
		Description:   "Industry standard Role & Permission authorization management for Eloquent models.",
		Category:      "package",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Spatie Permission",
				Command: "composer",
				Args:    []string{"require", "spatie/laravel-permission", "--no-interaction"},
			},
			{
				Type:    StepCommand,
				Title:   "Publish Permission Migrations",
				Command: "php",
				Args:    []string{"artisan", "vendor:publish", "--provider=Spatie\\Permission\\PermissionServiceProvider"},
			},
		},
		RollbackInstructions: "Run 'composer remove spatie/laravel-permission'.",
	}

	// 18. Sentry for Laravel
	rc.recipes["sentry"] = Recipe{
		ID:            "sentry",
		Title:         "Install Sentry Error Tracking & APM",
		Description:   "Real-time exception tracking, distributed tracing, and performance monitoring.",
		Category:      "package",
		Prerequisites: []string{"composer.json"},
		Steps: []RecipeStep{
			{
				Type:    StepCommand,
				Title:   "Require Sentry Laravel SDK",
				Command: "composer",
				Args:    []string{"require", "sentry/sentry-laravel", "--no-interaction"},
			},
		},
		RollbackInstructions: "Run 'composer remove sentry/sentry-laravel' and delete config/sentry.php.",
	}
}

func (rc *RecipeCatalog) ListRecipes() []Recipe {
	var list []Recipe
	for _, r := range rc.recipes {
		list = append(list, r)
	}
	order := map[string]int{
		"livewire-3":          1,
		"filament-v3":         2,
		"horizon":             3,
		"pulse":               4,
		"telescope":           5,
		"sanctum":             6,
		"pest":                7,
		"pint-larastan":       8,
		"sail":                9,
		"tailwind-inertia":    10,
		"cashier":             11,
		"production-optimize": 12,
	}
	sort.Slice(list, func(i, j int) bool {
		oi, okI := order[list[i].ID]
		if !okI {
			oi = 99
		}
		oj, okJ := order[list[j].ID]
		if !okJ {
			oj = 99
		}
		if oi != oj {
			return oi < oj
		}
		return list[i].ID < list[j].ID
	})
	return list
}

func (rc *RecipeCatalog) GetRecipe(id string) (Recipe, error) {
	r, ok := rc.recipes[id]
	if !ok {
		return Recipe{}, errors.New("Recipe not found")
	}
	return r, nil
}

func (rc *RecipeCatalog) PreviewRecipe(id string) (*RecipePreview, error) {
	r, err := rc.GetRecipe(id)
	if err != nil {
		return nil, err
	}

	preview := &RecipePreview{
		RecipeID: id,
		Title:    r.Title,
	}

	for _, step := range r.Steps {
		switch step.Type {
		case StepCommand:
			cmdStr := fmt.Sprintf("%s %s", step.Command, strings.Join(step.Args, " "))
			preview.CommandsToRun = append(preview.CommandsToRun, cmdStr)
		case StepWriteFile, StepAppendFile:
			preview.FilesModified = append(preview.FilesModified, step.TargetFile)
		case StepEnvSet:
			preview.EnvKeysSet = append(preview.EnvKeysSet, fmt.Sprintf("%s=%s", step.EnvKey, step.EnvValue))
		}
	}
	return preview, nil
}

// ExecuteRecipe runs each step sequentially, capturing real outputs and halting on error.
func (rc *RecipeCatalog) ExecuteRecipe(id, phpBinary, projectPath string) (*RecipeExecutionResult, error) {
	return rc.ExecuteRecipeContext(context.Background(), id, phpBinary, projectPath)
}

func (rc *RecipeCatalog) ExecuteRecipeContext(ctx context.Context, id, phpBinary, projectPath string) (*RecipeExecutionResult, error) {
	r, err := rc.GetRecipe(id)
	if err != nil {
		return nil, err
	}

	if runtime.GOOS == "windows" && id == "horizon" {
		return nil, errors.New("Horizon requires POSIX process control; run it inside WSL or Docker")
	}
	for _, prerequisite := range r.Prerequisites {
		if _, err := os.Stat(filepath.Join(projectPath, prerequisite)); err != nil {
			return nil, fmt.Errorf("recipe prerequisite %s: %w", prerequisite, err)
		}
	}
	start := time.Now()
	var stepResults []StepExecutionResult

	for idx, step := range r.Steps {
		if err := ctx.Err(); err != nil {
			return &RecipeExecutionResult{RecipeID: id, Success: false, StepResults: stepResults, Recovery: r.RollbackInstructions}, err
		}
		stepRes := StepExecutionResult{
			StepIndex: idx + 1,
			Title:     step.Title,
			Success:   true,
		}

		switch step.Type {
		case StepCommand:
			executable := phpBinary
			args := step.Args
			if step.Command == "artisan" {
				args = append([]string{"artisan"}, step.Args...)
			} else if step.Command == "composer" {
				spec, err := ComposerSpec(phpBinary, projectPath, step.Args)
				var comp []byte
				if err == nil {
					comp, err = runner.CaptureContext(ctx, spec, 120*time.Second)
				}

				stepRes.Output = strings.Split(string(comp), "\n")
				if err != nil {
					stepRes.Success = false
					stepRes.Error = err.Error()
				}
				stepResults = append(stepResults, stepRes)
				if !stepRes.Success {
					return &RecipeExecutionResult{
						RecipeID:    id,
						Success:     false,
						StepResults: stepResults,
						DurationMs:  float64(time.Since(start).Milliseconds()),
						Recovery:    r.RollbackInstructions,
					}, fmt.Errorf("Step '%s' failed: %v", step.Title, err)
				}
				continue
			}

			out, cmdErr := runner.CaptureContext(ctx, runner.Spec{
				Executable: executable,
				Args:       args,
				Directory:  projectPath,
			}, 60*time.Second)

			stepRes.Output = strings.Split(string(out), "\n")
			if cmdErr != nil {
				stepRes.Success = false
				stepRes.Error = cmdErr.Error()
				stepResults = append(stepResults, stepRes)
				return &RecipeExecutionResult{
					RecipeID:    id,
					Success:     false,
					StepResults: stepResults,
					DurationMs:  float64(time.Since(start).Milliseconds()),
					Recovery:    r.RollbackInstructions,
				}, fmt.Errorf("Step '%s' failed: %v", step.Title, cmdErr)
			}

		case StepWriteFile, StepAppendFile:
			target, err := recipeTarget(projectPath, step.TargetFile)
			if err != nil {
				return nil, err
			}
			content := step.Content
			if existing, err := os.ReadFile(target); err == nil {
				if step.Type == StepAppendFile {
					content = string(existing) + content
				}
				backup := fmt.Sprintf("%s.backup.%d", target, time.Now().UnixNano())
				if err := os.WriteFile(backup, existing, 0600); err != nil {
					return nil, err
				}
			} else if !os.IsNotExist(err) {
				return nil, err
			}
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			if err := os.WriteFile(target, []byte(content), 0644); err != nil {
				stepRes.Success = false
				stepRes.Error = err.Error()
				stepResults = append(stepResults, stepRes)
				return &RecipeExecutionResult{
					RecipeID:    id,
					Success:     false,
					StepResults: stepResults,
					Recovery:    r.RollbackInstructions,
				}, err
			}
			stepRes.Output = []string{fmt.Sprintf("Wrote %s", step.TargetFile)}

		case StepEnvSet:
			envPath := filepath.Join(projectPath, ".env")
			envFile, envErr := environment.ParseEnv(envPath)
			if envErr != nil {
				return nil, fmt.Errorf("read .env for recipe: %w", envErr)
			}
			{
				found := false
				for i, e := range envFile.Entries {
					if e.Key == step.EnvKey {
						envFile.Entries[i].Value = step.EnvValue
						found = true
						break
					}
				}
				if !found {
					envFile.Entries = append(envFile.Entries, environment.EnvEntry{
						Type:  "keyvalue",
						Key:   step.EnvKey,
						Value: step.EnvValue,
					})
				}
				if err := environment.SaveEnv(envFile, envFile.Fingerprint); err != nil {
					return nil, fmt.Errorf("save recipe .env: %w", err)
				}
				stepRes.Output = []string{fmt.Sprintf("Set %s=%s in .env", step.EnvKey, step.EnvValue)}
			}

		case StepVerify:
			chkPath, err := recipeTarget(projectPath, step.VerifyCheck)
			if err != nil {
				return nil, err
			}
			if _, err := os.Stat(chkPath); err != nil {
				stepRes.Success = false
				stepRes.Error = fmt.Sprintf("Verification failed: %s not found", step.VerifyCheck)
				stepResults = append(stepResults, stepRes)
				return &RecipeExecutionResult{
					RecipeID:    id,
					Success:     false,
					StepResults: stepResults,
					Recovery:    r.RollbackInstructions,
				}, errors.New(stepRes.Error)
			}
			stepRes.Output = []string{fmt.Sprintf("Verified presence of %s", step.VerifyCheck)}
		default:
			return nil, fmt.Errorf("unsupported recipe step type %q", step.Type)
		}

		stepResults = append(stepResults, stepRes)
	}

	return &RecipeExecutionResult{
		RecipeID:    id,
		Success:     true,
		StepResults: stepResults,
		DurationMs:  float64(time.Since(start).Milliseconds()),
	}, nil
}

// ExecuteProjectWizard generates a new Laravel project in destination directory.
func ExecuteProjectWizard(spec ProjectWizardSpec) (string, error) {
	return ExecuteProjectWizardContext(context.Background(), spec)
}

func ExecuteProjectWizardContext(ctx context.Context, spec ProjectWizardSpec) (string, error) {
	if spec.Directory == "" || spec.Name == "" {
		return "", errors.New("Directory and project name required")
	}

	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`).MatchString(spec.Name) {
		return "", errors.New("project name must contain only letters, digits, hyphens and underscores")
	}
	if spec.Starter != "" && spec.Starter != "none" && spec.Starter != "api" {
		return "", errors.New("supported starters are none and api")
	}
	if spec.Database == "postgres" {
		spec.Database = "pgsql"
	}
	switch spec.Database {
	case "", "sqlite", "mysql", "pgsql":
	default:
		return "", errors.New("unsupported database driver")
	}
	targetDir := filepath.Join(spec.Directory, spec.Name)
	if entries, err := os.ReadDir(targetDir); err == nil && len(entries) > 0 {
		return "", fmt.Errorf("Target directory %s is not empty. Choose an empty destination.", targetDir)
	}

	_ = os.MkdirAll(targetDir, 0755)

	command, err := ComposerSpec(spec.PHPBinary, targetDir, []string{"create-project", "laravel/laravel:^12.0", ".", "--prefer-dist", "--no-interaction"})
	if err != nil {
		return "", err
	}
	out, err := runner.CaptureContext(ctx, command, 180*time.Second)

	if err != nil {
		return string(out), fmt.Errorf("composer create-project failed: %w", err)
	}

	// Configure database in .env
	envPath := filepath.Join(targetDir, ".env")
	envFile, envErr := environment.ParseEnv(envPath)
	if envErr != nil {
		return string(out), envErr
	}
	{
		for i, e := range envFile.Entries {
			if e.Key == "DB_CONNECTION" && spec.Database != "" {
				envFile.Entries[i].Value = spec.Database
			}
		}
		if err := environment.SaveEnv(envFile, envFile.Fingerprint); err != nil {
			return string(out), err
		}
	}

	// Git init if requested
	if spec.GitInit {
		gitOutput, gitErr := runner.CaptureContext(ctx, runner.Spec{
			Executable: "git",
			Args:       []string{"init"},
			Directory:  targetDir,
		}, 10*time.Second)
		if gitErr != nil {
			return string(out) + string(gitOutput), gitErr
		}
	}
	if spec.Starter == "api" {
		php := spec.PHPBinary
		if php == "" {
			php = "php"
		}
		apiOut, err := runner.CaptureContext(ctx, runner.Spec{Executable: php, Args: []string{"artisan", "install:api", "--no-interaction"}, Directory: targetDir}, 120*time.Second)
		out = append(out, apiOut...)
		if err != nil {
			return string(out), err
		}
	}
	return string(out), nil
}

func recipeTarget(root, name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", errors.New("recipe requires a project-relative path")
	}
	path := filepath.Join(root, name)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("recipe path leaves project")
	}
	// Check existing ancestors as well as the final file to prevent symlink escapes.
	for parent := path; parent != filepath.Dir(root); parent = filepath.Dir(parent) {
		if info, err := os.Lstat(parent); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("recipe path contains a symlink")
		}
		if parent == root || parent == filepath.Dir(parent) {
			break
		}
	}
	return path, nil
}
func ComposerSpec(php, dir string, args []string) (runner.Spec, error) {
	if php == "" {
		php = "php"
	}
	candidates := []string{filepath.Join(dir, "composer.phar")}
	if managed, err := dependencies.NewManager("", nil).Resolve("composer"); err == nil {
		if strings.EqualFold(filepath.Ext(managed), ".exe") {
			return runner.Spec{Executable: managed, Args: args, Directory: dir}, nil
		}
		candidates = append(candidates, managed)
	}
	if command, err := exec.LookPath("composer"); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(command), "composer.phar"))
		if strings.EqualFold(filepath.Ext(command), ".exe") {
			return runner.Spec{Executable: command, Args: args, Directory: dir}, nil
		}
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return runner.Spec{Executable: php, Args: append([]string{path}, args...), Directory: dir}, nil
		}
	}
	return runner.Spec{}, errors.New("Composer PHAR or executable not found; install Composer/Herd or add composer.phar to the project")
}
