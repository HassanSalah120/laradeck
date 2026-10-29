package quality

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	bladeQueryRegex = regexp.MustCompile(`(?i)\b(App\\Models\\[A-Za-z0-9_]+|[A-Z][a-zA-Z0-9_]+)::(all|where|find|findOrFail|get|first|pluck|latest|oldest)\s*\(|\bDB::table\s*\(`)
	knownNonModels  = map[string]bool{
		"Route": true, "Auth": true, "Session": true, "Gate": true, "Request": true,
		"Str": true, "Arr": true, "URL": true, "Vite": true, "Carbon": true, "Date": true,
		"Cookie": true, "View": true, "Blade": true, "Lang": true, "App": true, "Storage": true,
		"File": true, "Schema": true, "Config": true, "Log": true, "Event": true, "Http": true,
	}

	pluralModels = map[string]bool{
		"Users.php": true, "Orders.php": true, "Products.php": true, "Articles.php": true,
		"Posts.php": true, "Customers.php": true, "Categories.php": true, "Comments.php": true,
		"Invoices.php": true, "Payments.php": true, "Roles.php": true, "Permissions.php": true,
	}

	pluralControllers = map[string]bool{
		"UsersController.php": true, "OrdersController.php": true, "ProductsController.php": true,
		"ArticlesController.php": true, "PostsController.php": true, "CustomersController.php": true,
	}
)

// ScanBladeAndStructure scans non-PHP assets, Blade templates, and project structure conventions.
func ScanBladeAndStructure(projectPath string) ([]CodeBestPracticeFinding, error) {
	var scanErr error
	var findings []CodeBestPracticeFinding

	// 1. Blade views scanner
	viewsDir := filepath.Join(projectPath, "resources", "views")
	if _, err := os.Stat(viewsDir); err == nil {
		scanErr = filepath.Walk(viewsDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(info.Name(), ".blade.php") {
				return nil
			}
			rel, _ := filepath.Rel(projectPath, path)
			rel = filepath.ToSlash(rel)

			f, openErr := os.Open(path)
			if openErr != nil {
				return openErr
			}
			defer f.Close()

			scanner := bufio.NewScanner(f)
			lineNum := 0
			for scanner.Scan() {
				lineNum++
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "{{--") || strings.HasPrefix(line, "<!--") {
					continue
				}

				if matches := bladeQueryRegex.FindStringSubmatch(line); len(matches) > 1 {
					class := matches[1]
					if strings.Contains(class, `\`) {
						parts := strings.Split(class, `\`)
						class = parts[len(parts)-1]
					}
					if !knownNonModels[class] {
						findings = append(findings, CodeBestPracticeFinding{
							Category:    "Performance",
							Rule:        "NoQueriesInBlade",
							Severity:    "error",
							File:        rel,
							Line:        lineNum,
							Message:     "Database or Eloquent query executed directly inside a Blade view template.",
							Snippet:     line,
							FixGuidance: "Preload and query data in the Controller or Action, passing the collection into the view to eliminate N+1 query loops.",
							GithubRef:   "https://github.com/alexeymezenin/laravel-best-practices#do-not-execute-queries-in-blade-templates-and-use-eager-loading-n--1-problem",
						})
					}
				}
			}
			return scanner.Err()
		})
	} else if !os.IsNotExist(err) {
		scanErr = errors.Join(scanErr, err)
	}

	// 2. Model naming conventions
	modelsDir := filepath.Join(projectPath, "app", "Models")
	if entries, err := os.ReadDir(modelsDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if pluralModels[name] {
				singular := strings.TrimSuffix(name, "s.php") + ".php"
				rel := "app/Models/" + name
				findings = append(findings, CodeBestPracticeFinding{
					Category:    "Conventions",
					Rule:        "ModelNamingSingular",
					Severity:    "info",
					File:        rel,
					Line:        1,
					Message:     "Model filename appears to be plural. Laravel conventions recommend singular model names.",
					Snippet:     "class " + strings.TrimSuffix(name, ".php"),
					FixGuidance: "Rename model to singular form (e.g. " + singular + "). Eloquent automatically infers the plural table name.",
					GithubRef:   "https://github.com/alexeymezenin/laravel-best-practices#follow-laravel-naming-conventions",
				})
			}
		}
	}

	// 3. Controller naming conventions
	ctrlDir := filepath.Join(projectPath, "app", "Http", "Controllers")
	if entries, err := os.ReadDir(ctrlDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if pluralControllers[name] {
				rel := "app/Http/Controllers/" + name
				singular := strings.TrimSuffix(name, "sController.php") + "Controller.php"
				findings = append(findings, CodeBestPracticeFinding{
					Category:    "Conventions",
					Rule:        "ControllerNamingSingular",
					Severity:    "info",
					File:        rel,
					Line:        1,
					Message:     "Controller filename appears to use a plural noun. Laravel conventions recommend singular controller names.",
					Snippet:     "class " + strings.TrimSuffix(name, ".php"),
					FixGuidance: "Rename controller to singular form (e.g. " + singular + ").",
					GithubRef:   "https://github.com/alexeymezenin/laravel-best-practices#follow-laravel-naming-conventions",
				})
			}
		}
	}

	// 4. Lazy loading prevention guard
	providerPath := filepath.Join(projectPath, "app", "Providers", "AppServiceProvider.php")
	if content, err := os.ReadFile(providerPath); err == nil {
		if !strings.Contains(string(content), "preventLazyLoading") {
			lineNum := 1
			snippet := "public function boot(): void"
			lines := strings.Split(string(content), "\n")
			for i, l := range lines {
				if strings.Contains(l, "function boot(") {
					lineNum = i + 1
					snippet = strings.TrimSpace(l)
					break
				}
			}
			findings = append(findings, CodeBestPracticeFinding{
				Category:    "Performance",
				Rule:        "PreventLazyLoadingGuard",
				Severity:    "info",
				File:        "app/Providers/AppServiceProvider.php",
				Line:        lineNum,
				Message:     "Model::preventLazyLoading() is not configured in AppServiceProvider::boot().",
				Snippet:     snippet,
				FixGuidance: "Add Model::preventLazyLoading(! app()->isProduction()) to catch un-eager loaded relation regressions during development.",
				GithubRef:   "https://laravel.com/docs/eloquent-relationships#preventing-lazy-loading",
			})
		}
	}

	return findings, scanErr
}

// ScanTSXAndReact scans React, TypeScript, and Inertia components using a scope-aware state machine.
func ScanTSXAndReact(projectPath string) ([]CodeBestPracticeFinding, error) {
	var findings []CodeBestPracticeFinding
	jsDir := filepath.Join(projectPath, "resources", "js")
	if _, err := os.Stat(jsDir); err != nil {
		if os.IsNotExist(err) {
			return findings, nil
		}
		return findings, err
	}

	scanErr := filepath.Walk(jsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext != ".tsx" && ext != ".jsx" && ext != ".ts" && ext != ".js" {
			return nil
		}

		rel, _ := filepath.Rel(projectPath, path)
		rel = filepath.ToSlash(rel)

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		lines := strings.Split(string(data), "\n")
		totalLines := len(lines)

		// Check Rule 29: ExtractDomainLogic (Components > 300 LOC with heavy state/logic)
		if totalLines > 320 && (ext == ".tsx" || ext == ".jsx") && strings.Contains(rel, "Pages/") {
			if strings.Count(string(data), "useState") > 6 || strings.Count(string(data), "useEffect") > 3 {
				findings = append(findings, CodeBestPracticeFinding{
					Category:    "React & UI",
					Rule:        "ExtractDomainLogic",
					Severity:    "info",
					File:        rel,
					Line:        1,
					Message:     "Large component exceeding 300 lines with dense state management.",
					Snippet:     lines[0],
					FixGuidance: "Extract complex workflows, form states, and API operations into custom hooks or smaller sub-components.",
					GithubRef:   "https://react.dev/learn/reusing-logic-with-custom-hooks",
				})
			}
		}

		inEffect := false
		effectStartLine := 0
		effectBraceDepth := 0
		effectHasListen := false
		effectHasCleanup := false
		effectHasFetchWithoutAbort := false
		effectHasListenerOrTimer := false
		braceDepth := 0
		ifBlockDepth := -1

		inListenCallback := false
		listenCallbackDepth := 0
		listenStateVars := []string{}

		for idx, rawLine := range lines {
			lineNum := idx + 1
			line := strings.TrimSpace(rawLine)

			// Track if-block depth for RulesOfHooks
			if strings.HasPrefix(line, "if ") || strings.HasPrefix(line, "if(") {
				ifBlockDepth = braceDepth
			}

			// Brace depth tracking
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if ifBlockDepth >= 0 && braceDepth <= ifBlockDepth {
				ifBlockDepth = -1
			}

			// Rule 24: RulesOfHooks (hooks inside if blocks)
			if ifBlockDepth >= 0 && (strings.Contains(line, "useState(") || strings.Contains(line, "useEffect(") || strings.Contains(line, "useMemo(") || strings.Contains(line, "useCallback(")) {
				findings = append(findings, CodeBestPracticeFinding{
					Category:    "React & UI",
					Rule:        "RulesOfHooks",
					Severity:    "error",
					File:        rel,
					Line:        lineNum,
					Message:     "React hook called conditionally inside an if block.",
					Snippet:     line,
					FixGuidance: "Move all hook calls to the top level of the component before any early returns or conditional blocks.",
					GithubRef:   "https://react.dev/warnings/invalid-hook-call-warning",
				})
			}

			// Skip pure comment lines for rule checks
			if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
				continue
			}

			// Clean line without inline comments for accurate matching
			cleanLine := line
			if commentIdx := strings.Index(cleanLine, "//"); commentIdx >= 0 {
				cleanLine = strings.TrimSpace(cleanLine[:commentIdx])
			}

			// Rule 23: TypedInertiaPageProps
			if strings.Contains(cleanLine, "usePage()") && !strings.Contains(cleanLine, "usePage<") {
				accessesProps := false
				if strings.Contains(cleanLine, ".props") || regexp.MustCompile(`\{\s*[^}]*\bprops\b[^}]*\}\s*=\s*usePage\(\)`).MatchString(cleanLine) {
					accessesProps = true
				} else {
					if match := regexp.MustCompile(`const\s+([a-zA-Z0-9_]+)\s*=\s*usePage\(\)`).FindStringSubmatch(cleanLine); len(match) > 1 {
						varName := match[1]
						if regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\.props\b`).MatchString(string(data)) {
							accessesProps = true
						}
					}
				}

				if accessesProps {
					findings = append(findings, CodeBestPracticeFinding{
						Category:    "Inertia.js",
						Rule:        "TypedInertiaPageProps",
						Severity:    "info",
						File:        rel,
						Line:        lineNum,
						Message:     "usePage() props accessed without TypeScript generic parameter.",
						Snippet:     line,
						FixGuidance: "Use usePage<PageProps>().props to ensure strict compile-time type safety for Inertia shared and page props.",
						GithubRef:   "https://inertiajs.com/typescript",
					})
				}
			}

			// Rule 19: InertiaFilePutSpoofing
			if (strings.Contains(line, "form.put(") || strings.Contains(line, "form.patch(")) &&
				(strings.Contains(string(data), "new File") || strings.Contains(string(data), "File | null") || strings.Contains(string(data), "type=\"file\"") || strings.Contains(string(data), "type='file'")) {
				findings = append(findings, CodeBestPracticeFinding{
					Category:    "Inertia.js",
					Rule:        "InertiaFilePutSpoofing",
					Severity:    "error",
					File:        rel,
					Line:        lineNum,
					Message:     "form.put() or form.patch() used on form with file input. PHP natively drops multipart/form-data on PUT/PATCH requests.",
					Snippet:     line,
					FixGuidance: "Use router.post(url, { ...data, _method: 'put' }) or Inertia v2 forceFormData to properly upload files via method spoofing.",
				})
			}

			// Rule: SafeTargetBlankRel (Tier 2 Optional Advice)
			if (strings.Contains(line, "<a ") || strings.Contains(line, "<a\t")) && (strings.Contains(line, `target="_blank"`) || strings.Contains(line, `target='_blank'`)) {
				if !strings.Contains(line, "noopener") && !strings.Contains(line, "noreferrer") {
					findings = append(findings, CodeBestPracticeFinding{
						Category:    "Security",
						Rule:        "SafeTargetBlankRel",
						Severity:    "info",
						File:        rel,
						Line:        lineNum,
						Message:     "External anchor link with target=\"_blank\" does not specify rel=\"noopener noreferrer\".",
						Snippet:     line,
						FixGuidance: "Add rel=\"noopener noreferrer\" to prevent reverse tabnabbing and unauthorized window.opener access.",
						GithubRef:   "https://owasp.org/www-community/attacks/Reverse_Tabnabbing",
					})
				}
			}

			// Rule 25: StableListKeys (.map index as key)
			if strings.Contains(line, "key={index}") || strings.Contains(line, "key={idx}") || strings.Contains(line, "key={i}") || strings.Contains(line, "key={k}") {
				// Suppress for loading skeletons and decorative dummy arrays
				isSkeletonOrDecorative := false
				lowerLine := strings.ToLower(line)
				if strings.Contains(lowerLine, "animate-pulse") || strings.Contains(lowerLine, "skeleton") {
					isSkeletonOrDecorative = true
				}
				for prevIdx := max(0, idx-3); prevIdx < idx; prevIdx++ {
					pLine := strings.ToLower(lines[prevIdx])
					if strings.Contains(pLine, "animate-pulse") || strings.Contains(pLine, "skeleton") ||
						strings.Contains(pLine, "array(") || strings.Contains(pLine, "array.from(") || strings.Contains(pLine, ".fill(") {
						isSkeletonOrDecorative = true
						break
					}
				}

				// Suppress for static marketing pages, landing pages, and documentation where array order is immutable
				isMarketingOrStatic := strings.Contains(rel, "/Marketing/") ||
					strings.Contains(rel, "/marketing/") ||
					strings.Contains(rel, "/Landing/") ||
					strings.Contains(rel, "/landing/") ||
					strings.Contains(rel, "/Docs/") ||
					strings.Contains(rel, "/docs/") ||
					strings.Contains(rel, "AboutPage") ||
					strings.Contains(rel, "SecurityPage") ||
					strings.Contains(rel, "FeatureDetail") ||
					strings.Contains(rel, "FeaturesIndex") ||
					strings.Contains(rel, "MenaLanding") ||
					strings.Contains(rel, "LandingPage") ||
					strings.Contains(rel, "FaqPage") ||
					strings.Contains(rel, "PricingPage")

				if !isSkeletonOrDecorative && !isMarketingOrStatic {
					findings = append(findings, CodeBestPracticeFinding{
						Category:    "React & UI",
						Rule:        "StableListKeys",
						Severity:    "info",
						File:        rel,
						Line:        lineNum,
						Message:     "Array index used as key in list item rendering.",
						Snippet:     line,
						FixGuidance: "Use persistent unique identifiers (e.g. key={item.id}) to prevent rendering artifacts and state leakage across rows.",
						GithubRef:   "https://react.dev/learn/rendering-lists#why-does-react-need-keys",
					})
				}
			}

			// Rule 32: UnthrottledClientWhisper
			if strings.Contains(line, ".whisper(") {
				if !strings.Contains(line, "debounce") && !strings.Contains(line, "throttle") {
					findings = append(findings, CodeBestPracticeFinding{
						Category:    "Real-Time & Sockets",
						Rule:        "UnthrottledClientWhisper",
						Severity:    "warning",
						File:        rel,
						Line:        lineNum,
						Message:     "Client whisper event emitted inside high-frequency DOM event without throttling.",
						Snippet:     line,
						FixGuidance: "Wrap whisper emissions in debounce() or throttle() to prevent flooding WebSocket peers and triggering rate limits.",
						GithubRef:   "https://laravel.com/docs/broadcasting#client-events",
					})
				}
			}

			// Rule 30 & 31: useEffect Echo and Fetch tracker
			isEffectStart := strings.Contains(line, "useEffect(")
			if isEffectStart {
				inEffect = true
				effectStartLine = lineNum
				effectBraceDepth = braceDepth
				effectHasListen = false
				effectHasCleanup = false
				effectHasFetchWithoutAbort = false
				effectHasListenerOrTimer = false
			}

			if inEffect {
				// Constraint 4: Only manual Echo.channel / Echo.private / .listen(), exclude useEcho() hook!
				if (strings.Contains(line, "Echo.channel(") || strings.Contains(line, "Echo.private(") || strings.Contains(line, "Echo.join(")) && strings.Contains(line, ".listen(") {
					effectHasListen = true
				}
				if strings.Contains(line, ".listen(") && !strings.Contains(line, "useEcho(") {
					effectHasListen = true
					inListenCallback = true
					listenCallbackDepth = braceDepth
				}

				if (strings.Contains(line, "fetch(") || strings.Contains(line, "axios.")) && !strings.Contains(line, "signal") && !strings.Contains(line, "AbortController") {
					effectHasFetchWithoutAbort = true
				}

				if strings.Contains(line, "addEventListener(") || strings.Contains(line, "setInterval(") {
					effectHasListenerOrTimer = true
				}

				// Check cleanup return
				if strings.Contains(line, "return () =>") || strings.Contains(line, "return function") {
					effectHasCleanup = true
				}

				// Stale closure tracking inside .listen()
				if inListenCallback {
					if strings.Contains(line, "set") && strings.Contains(line, "[...") && !strings.Contains(line, "prev =>") && !strings.Contains(line, "current =>") {
						findings = append(findings, CodeBestPracticeFinding{
							Category:    "Real-Time & Sockets",
							Rule:        "StaleClosureInSocketListener",
							Severity:    "warning",
							File:        rel,
							Line:        lineNum,
							Message:     "State variable referenced directly inside Echo listener callback without functional updater.",
							Snippet:     line,
							FixGuidance: "Use functional updater syntax (e.g. setItems(prev => [...prev, newItem])) to prevent stale closure wiping out updates.",
							GithubRef:   "https://react.dev/reference/react/useState#updating-state-based-on-the-previous-state",
						})
					}
					if braceDepth < listenCallbackDepth {
						inListenCallback = false
					}
				}

				// End of useEffect block
				if !isEffectStart && braceDepth < effectBraceDepth && strings.Contains(line, ")") {
					// Rule 31: EchoCleanupUnmount
					if effectHasListen && !effectHasCleanup {
						findings = append(findings, CodeBestPracticeFinding{
							Category:    "Real-Time & Sockets",
							Rule:        "EchoCleanupUnmount",
							Severity:    "error",
							File:        rel,
							Line:        effectStartLine,
							Message:     "Component subscribes to Echo channel in useEffect without a return cleanup function.",
							Snippet:     lines[effectStartLine-1],
							FixGuidance: "Return a cleanup function calling channel.stopListening('EventName') on unmount to prevent zombie listeners and memory leaks.",
							GithubRef:   "https://laravel.com/docs/broadcasting#leaving-a-channel",
						})
					}

					// Rule 26: AbortEffectFetches
					if effectHasFetchWithoutAbort && !effectHasCleanup {
						findings = append(findings, CodeBestPracticeFinding{
							Category:    "React & UI",
							Rule:        "AbortEffectFetches",
							Severity:    "warning",
							File:        rel,
							Line:        effectStartLine,
							Message:     "Async fetch or axios call inside useEffect without an AbortController cancellation return.",
							Snippet:     lines[effectStartLine-1],
							FixGuidance: "Use const controller = new AbortController(); and return () => controller.abort(); to prevent memory leaks on unmount.",
							GithubRef:   "https://react.dev/reference/react/useEffect#fetching-data-with-effects",
						})
					}

					// Rule: ReactEffectCleanupRequired
					if effectHasListenerOrTimer && !effectHasCleanup {
						findings = append(findings, CodeBestPracticeFinding{
							Category:    "React & UI",
							Rule:        "ReactEffectCleanupRequired",
							Severity:    "warning",
							File:        rel,
							Line:        effectStartLine,
							Message:     "useEffect registers an event listener or interval without returning a cleanup function.",
							Snippet:     lines[effectStartLine-1],
							FixGuidance: "Return a cleanup function from useEffect: return () => { window.removeEventListener(...); clearInterval(...); }; to prevent memory leaks and zombie listeners.",
							GithubRef:   "https://react.dev/reference/react/useEffect#connecting-to-an-external-system",
						})
					}

					inEffect = false
				}
			}
		}

		_ = listenStateVars
		return nil
	})

	return findings, scanErr
}
