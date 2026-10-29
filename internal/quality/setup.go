package quality

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"herdworkbench/internal/recipes"
	"herdworkbench/internal/runner"
	"herdworkbench/internal/storage"
)

const BrowserPlaywrightVersion = "1.64.1"
const BrowserAxeVersion = "4.14.0"

func BrowserRuntimeDir() string { return filepath.Join(storage.AppDataDir(), "tools", "audit-browser") }

type SetupFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
type AuditSetupPlan struct {
	ID           string            `json:"id"`
	ProjectID    string            `json:"projectId"`
	Kind         string            `json:"kind"`
	Manager      string            `json:"manager"`
	Message      string            `json:"message"`
	Commands     []runner.Spec     `json:"commands"`
	Files        []SetupFile       `json:"files"`
	Fingerprints map[string]string `json:"fingerprints"`
}

func fileFingerprint(path string) string {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return "missing"
	}
	if err != nil {
		return "unreadable"
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
func PackageManagers(root string) []string {
	managers := []string{}
	for _, m := range []struct {
		id    string
		files []string
	}{{"npm", []string{"package-lock.json", "npm-shrinkwrap.json"}}, {"pnpm", []string{"pnpm-lock.yaml"}}, {"yarn", []string{"yarn.lock"}}, {"bun", []string{"bun.lock", "bun.lockb"}}} {
		if existing(root, m.files...) != "" {
			managers = append(managers, m.id)
		}
	}
	return managers
}
func PackageManagerCommand(manager, root string, args ...string) (runner.Spec, error) {
	if manager == "bun" || manager == "pnpm" || manager == "yarn" {
		if binary, err := exec.LookPath(manager + ".exe"); err == nil {
			return runner.Spec{Executable: binary, Args: args, Directory: root, Env: []string{"YARN_ENABLE_SCRIPTS=false", "npm_config_ignore_scripts=true"}}, nil
		}
	}
	node, err := NodeBinary()
	if err != nil {
		return runner.Spec{}, err
	}
	directories := []string{filepath.Dir(node), filepath.Join(os.Getenv("APPDATA"), "npm")}
	if shim, err := exec.LookPath(manager + ".cmd"); err == nil {
		directories = append(directories, filepath.Dir(shim))
	}
	names := map[string][]string{"npm": {"node_modules/npm/bin/npm-cli.js"}, "pnpm": {"node_modules/pnpm/bin/pnpm.cjs", "node_modules/corepack/dist/pnpm.js"}, "yarn": {"node_modules/yarn/bin/yarn.js", "node_modules/corepack/dist/yarn.js"}}
	for _, directory := range directories {
		if script := existing(directory, names[manager]...); script != "" {
			return runner.Spec{Executable: node, Args: append([]string{script}, args...), Directory: root, Env: []string{"YARN_ENABLE_SCRIPTS=false", "npm_config_ignore_scripts=true"}}, nil
		}
	}
	return runner.Spec{}, fmt.Errorf("%s executable or JavaScript entry point is missing; install that package manager or select another explicitly", manager)
}

const starterESLint = `const js = require('@eslint/js');
const ts = require('typescript-eslint');
const hooks = require('eslint-plugin-react-hooks');
const a11y = require('eslint-plugin-jsx-a11y');
const react = require('eslint-plugin-react');
const globals = require('globals');
const typed = ['resources/js/**/*.{ts,tsx}', 'src/**/*.{ts,tsx}'];
const frontend = ['resources/js/**/*.{js,jsx,ts,tsx}', 'src/**/*.{js,jsx,ts,tsx}'];
module.exports = [
  { ignores: ['node_modules/**', 'vendor/**', 'public/build/**', 'dist/**'] },
  { ...js.configs.recommended, files: frontend },
  ...ts.configs.recommended.map(config => ({ ...config, files: typed })),
  { files: frontend,
    languageOptions: { globals: { ...globals.browser, ...globals.es2022 }, parserOptions: { ecmaFeatures: { jsx: true } } },
    plugins: { 'react-hooks': hooks, 'jsx-a11y': a11y, react },
    rules: { ...hooks.configs.recommended.rules, ...a11y.configs.recommended.rules, 'react/jsx-uses-vars': 'error' },
  },
  { files: typed,
    languageOptions: { parserOptions: { projectService: true, tsconfigRootDir: __dirname } },
    rules: {
      '@typescript-eslint/no-floating-promises': ['warn', { ignoreVoid: true }],
      '@typescript-eslint/no-misused-promises': 'warn',
      '@typescript-eslint/no-explicit-any': 'off',
    },
  },
];
`
const starterTSConfig = `{
  "compilerOptions": {
    "target": "ES2022", "module": "ESNext", "moduleResolution": "Bundler",
    "jsx": "react-jsx", "lib": ["ES2022", "DOM"], "strict": true,
    "noEmit": true, "skipLibCheck": true, "allowSyntheticDefaultImports": true
  },
  "include": ["resources/js/**/*.ts", "resources/js/**/*.tsx", "src/**/*.ts", "src/**/*.tsx"]
}
`

func PreviewAuditSetup(root, php, kind, manager string) (AuditSetupPlan, error) {
	plan := AuditSetupPlan{Kind: kind, Manager: manager, Files: []SetupFile{}, Commands: []runner.Spec{}, Fingerprints: map[string]string{}}
	watch := func(path string) { plan.Fingerprints[path] = fileFingerprint(path) }
	for _, name := range []string{"composer.json", "composer.lock", "package.json", "package-lock.json", "npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb"} {
		watch(filepath.Join(root, name))
	}
	switch kind {
	case "larastan":
		if existing(root, "composer.json") == "" {
			return plan, fmt.Errorf("Project has no composer.json")
		}
		command, err := recipes.ComposerSpec(php, root, []string{"require", "--dev", "larastan/larastan", "--no-interaction", "--prefer-dist", "--no-scripts"})
		if err != nil {
			return plan, err
		}
		plan.Commands = append(plan.Commands, command)
		if config := PHPStanConfiguration(root); config != "" {
			watch(config)
		} else {
			path := filepath.Join(root, "phpstan.neon")
			watch(path)
			plan.Files = append(plan.Files, SetupFile{path, "includes:\n    - vendor/larastan/larastan/extension.neon\nparameters:\n    level: 5\n    paths:\n        - app\n"})
		}
		plan.Message = "Installs a compatible Larastan development dependency using Composer. Existing PHPStan configuration is preserved. A new starter, if shown, explicitly selects level 5. Composer plugins may execute; project scripts are disabled."
	case "eslint":
		if existing(root, "package.json") == "" {
			return plan, fmt.Errorf("Project has no package.json")
		}
		detected := PackageManagers(root)
		if manager == "" {
			if len(detected) > 1 {
				return plan, fmt.Errorf("Multiple package-manager lockfiles found: %s. Select a manager explicitly.", strings.Join(detected, ", "))
			}
			manager = "npm"
			if len(detected) == 1 {
				manager = detected[0]
			}
			plan.Manager = manager
		}
		packages := []string{"eslint@^9", "@eslint/js@^9", "typescript-eslint@^8", "typescript@^5", "eslint-plugin-react-hooks@^7", "eslint-plugin-react@^7", "eslint-plugin-jsx-a11y@^6", "globals@^16"}
		// Preserve declared analyzer/plugin versions rather than forcing a major downgrade.
		var manifest struct {
			Dependencies    map[string]string `json:"dependencies"`
			PackageManager  string            `json:"packageManager"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		data, readErr := os.ReadFile(filepath.Join(root, "package.json"))
		if readErr != nil || json.Unmarshal(data, &manifest) != nil {
			return plan, fmt.Errorf("Cannot read project package.json")
		}
		for i, requirement := range packages {
			name := requirement[:strings.LastIndex(requirement, "@")]
			declared := manifest.DevDependencies[name]
			if declared == "" {
				declared = manifest.Dependencies[name]
			}
			if declared != "" {
				packages[i] = name + "@" + declared
			}
		}
		var args []string
		switch manager {
		case "npm":
			args = []string{"install", "--save-dev", "--ignore-scripts", "--no-audit", "--no-fund"}
		case "pnpm":
			args = []string{"add", "--save-dev", "--ignore-scripts"}
		case "yarn":
			if !strings.HasPrefix(manifest.PackageManager, "yarn@") {
				return plan, fmt.Errorf("Yarn setup requires a packageManager field such as yarn@4.9.0 in package.json so script controls can be previewed without executing project code")
			}
			args = []string{"add", "--dev"}
			if strings.HasPrefix(manifest.PackageManager, "yarn@1.") {
				args = append(args, "--ignore-scripts")
			}

		case "bun":
			args = []string{"add", "--dev", "--ignore-scripts"}
		default:
			return plan, fmt.Errorf("Select npm, pnpm, yarn or bun")
		}
		command, err := PackageManagerCommand(manager, root, append(args, packages...)...)
		if err != nil {
			return plan, err
		}
		plan.Commands = append(plan.Commands, command)
		if config := ESLintConfiguration(root); config != "" {
			watch(config)
		} else {
			path := filepath.Join(root, "eslint.config.cjs")
			watch(path)
			plan.Files = append(plan.Files, SetupFile{path, starterESLint})
			if existing(root, "tsconfig.json") == "" {
				path = filepath.Join(root, "tsconfig.json")
				watch(path)
				plan.Files = append(plan.Files, SetupFile{path, starterTSConfig})
			}
		}
		plan.Message = "Adds project development tools and updates the selected manager's lockfile. Existing analyzer configuration is preserved. Review the exact starter files below; generated rules can be customized for your UI components. Existing declared tool versions are retained. New starters use ESLint 9 because the selected React and jsx-a11y plugins do not yet declare ESLint 10 support; npm may warn that ESLint 9 is unsupported. Dependency scripts are disabled."
	case "browser":
		plan.Fingerprints = map[string]string{}
		runtime := BrowserRuntimeDir()
		node, err := NodeBinary()
		if err != nil {
			return plan, err
		}
		// npm is used only for this private runtime, independent of project lockfiles.
		command, err := PackageManagerCommand("npm", root, "install", "--prefix", runtime, "--ignore-scripts", "--no-audit", "--no-fund", "--registry=https://registry.npmjs.org", "--save-exact", "playwright@"+BrowserPlaywrightVersion, "axe-core@"+BrowserAxeVersion)
		if err != nil {
			return plan, err
		}
		command.Directory = runtime
		plan.Commands = append(plan.Commands, command, runner.Spec{Executable: node, Directory: root, Args: []string{filepath.Join(runtime, "node_modules", "playwright", "cli.js"), "install", "chromium"}, Env: []string{"PLAYWRIGHT_BROWSERS_PATH=" + filepath.Join(runtime, "browsers")}})
		for _, name := range []string{"package.json", "package-lock.json"} {
			watch(filepath.Join(runtime, name))
		}
		plan.Message = "Downloads pinned Playwright, axe-core and Chromium into Workbench's private runtime. Project manifests/configuration are not modified. Browser sessions are temporary and never saved for reuse."
	default:
		return plan, fmt.Errorf("Unknown audit setup kind")
	}
	return plan, nil
}
func ApplyAuditSetup(ctx context.Context, plan AuditSetupPlan, progress func(string)) error {
	for path, expected := range plan.Fingerprints {
		if expected == "unreadable" || fileFingerprint(path) != expected {
			return fmt.Errorf("%s changed or is unreadable; generate a fresh preview", filepath.Base(path))
		}
	}
	for path, expected := range plan.Fingerprints {
		if expected == "missing" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path+fmt.Sprintf(".workbench-backup.%d", time.Now().UnixNano()), data, 0600); err != nil {
			return err
		}
	}
	if plan.Kind == "browser" {
		if err := os.MkdirAll(BrowserRuntimeDir(), 0700); err != nil {
			return err
		}
	}
	lastProgress := ""
	for _, command := range plan.Commands {
		if progress != nil {
			progress("Installing " + plan.Kind + " tools; waiting for the package manager")
		}
		result, err := runner.CaptureExecContextWithProgress(ctx, command, 5*time.Minute, func(events []runner.Event) {
			if progress != nil && len(events) > 0 {
				for i := len(events) - 1; i >= 0; i-- {
					if events[i].Text != "" {
						text := SanitizeAuditText(events[i].Text)
						if text != lastProgress {
							progress(text)
							lastProgress = text
						}
						break
					}
				}
			}
		})
		if err != nil {
			return err
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("Setup command failed: %s", SanitizeAuditText(result.Combined))
		}
	}
	// Configs are created only after dependencies install, never overwritten.
	for _, file := range plan.Files {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		f, err := os.OpenFile(file.Path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("Configuration appeared during setup; review manually: %w", err)
		}
		_, err = f.WriteString(file.Content)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// JSON is used only to fingerprint previews, never as shell quoting.
func SetupFingerprint(plan AuditSetupPlan) string {
	data, _ := json.Marshal(plan)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
