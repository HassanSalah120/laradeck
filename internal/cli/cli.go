package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"herdworkbench/internal/environment"
	"herdworkbench/internal/quality"
	"herdworkbench/internal/reports"
)

type CLIHandler struct{}

func NewCLIHandler() *CLIHandler {
	return &CLIHandler{}
}

// Run executes command-line subcommands when herd-workbench is invoked with --cli or related flags.
func (c *CLIHandler) Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "help", "-h", "--help":
		printUsage()
		return 0

	case "version", "-v", "--version":
		fmt.Println("Herd Workbench v1.0.0 (Windows x64)")
		return 0

	case "audit":
		return c.handleAudit(subArgs)

	case "report":
		return c.handleReport(subArgs)

	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Println("Herd Workbench CLI")
	fmt.Println("Usage: herd-workbench --cli <subcommand> [options]")
	fmt.Println("\nSubcommands:")
	fmt.Println("  audit   --project <dir>      Run configuration and security audit on project")
	fmt.Println("  report  --project <dir>      Generate markdown README and architecture report")
	fmt.Println("  version                      Print application version")
	fmt.Println("  help                         Show this help message")
}

func (c *CLIHandler) handleAudit(args []string) int {
	fs := flag.NewFlagSet("audit", flag.ContinueOnError)
	projectDir := fs.String("project", ".", "Path to project root")
	jsonOutput := fs.Bool("json", false, "Output results in JSON format")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	cleanDir := filepath.Clean(*projectDir)
	fi, err := os.Stat(cleanDir)
	if err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: project directory %q does not exist or is not a directory\n", cleanDir)
		return 1
	}

	envPath := filepath.Join(cleanDir, ".env")
	envFile, err := environment.ParseEnv(envPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not read or parse %s: %v\n", envPath, err)
		return 1
	}

	envMap := make(map[string]string)
	for _, e := range envFile.Entries {
		if e.Type == "keyvalue" {
			envMap[e.Key] = e.Value
		}
	}

	auditItems := quality.AuditConfiguration(envMap, nil)

	if *jsonOutput {
		data, _ := json.MarshalIndent(auditItems, "", "  ")
		fmt.Println(string(data))
		return 0
	}

	fmt.Printf("Configuration Audit for %s\n", cleanDir)
	fmt.Println(strings.Repeat("-", 60))
	for _, item := range auditItems {
		statusSymbol := "[✓]"
		if item.Status == "warning" {
			statusSymbol = "[!]"
		} else if item.Status == "info" {
			statusSymbol = "[i]"
		}
		fmt.Printf("%s %s: %s\n", statusSymbol, item.Key, item.Message)
		if item.FixGuidance != "" {
			fmt.Printf("    Fix: %s\n", item.FixGuidance)
		}
	}
	return 0
}

func (c *CLIHandler) handleReport(args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	projectDir := fs.String("project", ".", "Path to project root")
	outFile := fs.String("out", "", "Output file path (default stdout)")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	cleanDir := filepath.Clean(*projectDir)
	fi, err := os.Stat(cleanDir)
	if err != nil || !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: project directory %q does not exist or is not a directory\n", cleanDir)
		return 1
	}

	// Validate this is an actual Laravel / PHP project (Finding 11)
	artisanPath := filepath.Join(cleanDir, "artisan")
	composerPath := filepath.Join(cleanDir, "composer.json")
	hasArtisan := false
	if _, err := os.Stat(artisanPath); err == nil {
		hasArtisan = true
	}
	hasComposer := false
	if _, err := os.Stat(composerPath); err == nil {
		hasComposer = true
	}

	if !hasArtisan && !hasComposer {
		fmt.Fprintf(os.Stderr, "Error: directory %q is not a valid Laravel project (neither artisan nor composer.json was found)\n", cleanDir)
		return 1
	}

	projectName := filepath.Base(cleanDir)
	phpVersion := "Unknown (not specified in composer.json)"
	frameworkVersion := "Laravel (version unverified)"

	if hasComposer {
		if cBytes, err := os.ReadFile(composerPath); err == nil {
			var comp struct {
				Name    string            `json:"name"`
				Require map[string]string `json:"require"`
			}
			if json.Unmarshal(cBytes, &comp) == nil {
				if comp.Name != "" {
					projectName = comp.Name
				}
				if v, ok := comp.Require["php"]; ok {
					phpVersion = v
				}
				if v, ok := comp.Require["laravel/framework"]; ok {
					frameworkVersion = "Laravel " + v
				}
			}
		}
	}

	dbDriver := "unconfigured"
	cacheDriver := "unconfigured"
	queueDriver := "unconfigured"

	envPath := filepath.Join(cleanDir, ".env")
	if envFile, err := environment.ParseEnv(envPath); err == nil {
		for _, e := range envFile.Entries {
			if e.Type == "keyvalue" {
				switch e.Key {
				case "DB_CONNECTION":
					dbDriver = e.Value
				case "CACHE_STORE", "CACHE_DRIVER":
					cacheDriver = e.Value
				case "QUEUE_CONNECTION", "QUEUE_DRIVER":
					queueDriver = e.Value
				case "APP_NAME":
					if e.Value != "" {
						projectName = e.Value
					}
				}
			}
		}
	}

	graph := reports.BuildArchitectureGraph(projectName, dbDriver, cacheDriver, queueDriver, false, nil)

	snap := reports.ProjectSnapshot{
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		ProjectName:    projectName,
		ProjectPath:    cleanDir,
		PHPVersion:     phpVersion,
		Framework:      frameworkVersion,
		DatabaseDriver: dbDriver,
		Graph:          graph,
		CacheDriver:    cacheDriver,
		QueueDriver:    queueDriver,
		HostAdvice:     reports.GenerateHostingAdvice(dbDriver, cacheDriver, queueDriver, 0, 0),
	}

	reportContent := reports.GenerateREADME(snap)

	if *outFile != "" {
		if err := os.WriteFile(*outFile, []byte(reportContent), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write report to %s: %v\n", *outFile, err)
			return 1
		}
		fmt.Printf("Report exported to %s\n", *outFile)
		return 0
	}

	fmt.Println(reportContent)
	return 0
}
