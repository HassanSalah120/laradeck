package reports

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"herdworkbench/internal/storage"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ArchitectureNode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Type  string `json:"type"` // "app", "database", "cache", "queue", "mail", "external"
}

type ArchitectureEdge struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Label      string `json:"label"`
	Provenance string `json:"provenance"` // "declared", "inferred", "observed"
}

type ArchitectureGraph struct {
	Nodes []ArchitectureNode `json:"nodes"`
	Edges []ArchitectureEdge `json:"edges"`
}

type ProjectSnapshot struct {
	GeneratedAt    string            `json:"generatedAt"`
	ProjectName    string            `json:"projectName"`
	ProjectPath    string            `json:"projectPath"`
	ProjectURL     string            `json:"projectUrl"`
	PHPVersion     string            `json:"phpVersion"`
	Framework      string            `json:"framework"`
	Clockwork      string            `json:"clockwork"`
	RoutesCount    int               `json:"routesCount"`
	Routes         []string          `json:"routesSample"`
	Services       []string          `json:"services"`
	EventsCount    int               `json:"eventsCount"`
	DatabaseDriver string            `json:"databaseDriver"`
	CacheDriver    string            `json:"cacheDriver"`
	QueueDriver    string            `json:"queueDriver"`
	EnvKeys        []string          `json:"envKeys"` // non-sensitive key names only
	AuditGood      int               `json:"auditGood"`
	AuditWarn      int               `json:"auditWarn"`
	AuditInfo      int               `json:"auditInfo"`
	HostAdvice     HostingAdvice     `json:"hostingAdvice"`
	Graph          ArchitectureGraph `json:"graph"`
}

type HostingAdvice struct {
	RecommendedPlan string   `json:"recommendedPlan"`
	MinimumRAM      string   `json:"minimumRam"`
	CPURequirement  string   `json:"cpuRequirement"`
	PhpFpmWorkers   int      `json:"phpFpmWorkers"`
	OpcacheMemory   string   `json:"opcacheMemory"`
	Recommendations []string `json:"recommendations"`
}

type TeamManifest struct {
	Version     string            `json:"version"`
	ProjectName string            `json:"projectName"`
	PHPVersion  string            `json:"phpVersion"`
	Framework   string            `json:"framework"`
	Services    []ManifestService `json:"services"`
	Commands    []string          `json:"commands"`
	CreatedAt   string            `json:"createdAt"`
}

type ManifestService struct {
	Name string `json:"name"`
	Port int    `json:"port"`
	Type string `json:"type"`
}

// BuildArchitectureGraph constructs a declared/inferred/observed graph from configuration.
func BuildArchitectureGraph(appName, dbDriver, cacheDriver, queueDriver string, hasMailpit bool, observedCalls []string) ArchitectureGraph {
	nodes := []ArchitectureNode{
		{ID: "app", Label: appName, Type: "app"},
	}
	var edges []ArchitectureEdge

	// Database
	if dbDriver != "" {
		dbID := "db_" + dbDriver
		nodes = append(nodes, ArchitectureNode{ID: dbID, Label: strings.ToUpper(dbDriver) + " Database", Type: "database"})
		edges = append(edges, ArchitectureEdge{
			Source:     "app",
			Target:     dbID,
			Label:      "Queries (" + dbDriver + ")",
			Provenance: "declared",
		})
	}

	// Cache
	if cacheDriver != "" && cacheDriver != "file" && cacheDriver != "array" {
		cacheID := "cache_" + cacheDriver
		nodes = append(nodes, ArchitectureNode{ID: cacheID, Label: strings.ToUpper(cacheDriver) + " Cache", Type: "cache"})
		edges = append(edges, ArchitectureEdge{
			Source:     "app",
			Target:     cacheID,
			Label:      "Store (" + cacheDriver + ")",
			Provenance: "declared",
		})
	}

	// Queue
	if queueDriver != "" && queueDriver != "sync" {
		queueID := "queue_" + queueDriver
		nodes = append(nodes, ArchitectureNode{ID: queueID, Label: "Queue Worker (" + queueDriver + ")", Type: "queue"})
		edges = append(edges, ArchitectureEdge{
			Source:     "app",
			Target:     queueID,
			Label:      "Jobs dispatch",
			Provenance: "declared",
		})
	}

	// Mail
	if hasMailpit {
		nodes = append(nodes, ArchitectureNode{ID: "mail_mailpit", Label: "Mailpit Sandbox", Type: "mail"})
		edges = append(edges, ArchitectureEdge{
			Source:     "app",
			Target:     "mail_mailpit",
			Label:      "SMTP 1025",
			Provenance: "declared",
		})
	}

	// Observed outgoing HTTP calls from telemetry
	for idx, call := range observedCalls {
		extID := fmt.Sprintf("ext_http_%d", idx+1)
		nodes = append(nodes, ArchitectureNode{ID: extID, Label: call, Type: "external"})
		edges = append(edges, ArchitectureEdge{
			Source:     "app",
			Target:     extID,
			Label:      "Observed Outgoing HTTP",
			Provenance: "observed",
		})
	}

	return ArchitectureGraph{
		Nodes: nodes,
		Edges: edges,
	}
}

// GenerateHostingAdvice provides explainable deployment and infrastructure recommendations.
func GenerateHostingAdvice(dbDriver, cacheDriver, queueDriver string, routeCount, avgQueries int) HostingAdvice {
	plan := "Sizing requires production workload measurements"
	minRAM := "Unknown: measure PHP worker RSS and peak concurrency"
	cpu := "Unknown: benchmark representative requests"
	workers := 0 // No production worker budget can be derived from .env or route counts.
	opcache := "Measure compiled script usage before selecting a memory budget"
	recs := []string{"Enable PHP OPcache in production and size its memory from measured script usage.", "Estimate pm.max_children from available RAM divided by measured peak PHP worker RSS; validate under representative concurrent traffic."}
	if dbDriver != "" && dbDriver != "sqlite" {
		recs = append(recs, "Budget database memory separately from PHP workers.")
	}
	if cacheDriver == "redis" {
		recs = append(recs, "Measure Redis dataset memory and configure a suitable maxmemory policy.")
	}

	if queueDriver != "" && queueDriver != "sync" {
		recs = append(recs, "Use Supervisor or systemd daemon to maintain long-running queue workers.")
	}

	if routeCount > 50 || avgQueries > 20 {
		recs = append(recs, "Cache Laravel configuration, routes, and views: run 'php artisan optimize'.")
	}

	return HostingAdvice{
		RecommendedPlan: plan,
		MinimumRAM:      minRAM,
		CPURequirement:  cpu,
		PhpFpmWorkers:   workers,
		OpcacheMemory:   opcache,
		Recommendations: recs,
	}
}

// GenerateREADME produces structured documentation sections.
func GenerateREADME(s ProjectSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", s.ProjectName)
	b.WriteString("Generated by **Herd Workbench**.\n\n")

	b.WriteString("## Requirements\n\n")
	fmt.Fprintf(&b, "- PHP: %s\n", s.PHPVersion)
	fmt.Fprintf(&b, "- Framework: %s\n", s.Framework)
	if s.DatabaseDriver != "" {
		fmt.Fprintf(&b, "- Database: %s\n", strings.ToUpper(s.DatabaseDriver))
	}
	if s.CacheDriver != "" {
		fmt.Fprintf(&b, "- Cache Store: %s\n", s.CacheDriver)
	}
	b.WriteString("\n")

	b.WriteString("## Local Development\n\n")
	b.WriteString("1. Clone repository\n")
	b.WriteString("2. Copy environment file: `cp .env.example .env`\n")
	b.WriteString("3. Install dependencies: `composer install && npm install`\n")
	b.WriteString("4. Generate app key: `php artisan key:generate`\n")
	b.WriteString("5. Run migrations: `php artisan migrate`\n")
	b.WriteString("6. Start Vite development server: `npm run dev`\n\n")

	if len(s.Routes) > 0 {
		b.WriteString("## Sample Endpoints\n\n")
		b.WriteString("| Method / URI |\n| --- |\n")
		for _, r := range s.Routes {
			fmt.Fprintf(&b, "| `%s` |\n", r)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Infrastructure & Deployment Profile\n\n")
	fmt.Fprintf(&b, "- **Target Sizing**: %s\n", s.HostAdvice.RecommendedPlan)
	fmt.Fprintf(&b, "- **Minimum RAM**: %s\n", s.HostAdvice.MinimumRAM)
	fmt.Fprintf(&b, "- **Recommended OPcache**: %s\n", s.HostAdvice.OpcacheMemory)
	for _, rec := range s.HostAdvice.Recommendations {
		fmt.Fprintf(&b, "- %s\n", rec)
	}
	b.WriteString("\n")

	return b.String()
}

type ReadmeSection struct {
	Index   int    `json:"index"`
	Heading string `json:"heading"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// MergeREADME selectively updates only generated sections while preserving user prose and duplicate headings.
func MergeREADME(existing, generated string, selectedSections []string) string {
	if existing == "" {
		return generated
	}

	selectedMap := make(map[string]bool)
	for _, s := range selectedSections {
		clean := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(s, "## ")))
		selectedMap[clean] = true
	}

	existSections := parseReadmeSections(existing)
	genSections := parseReadmeSections(generated)

	genMap := make(map[string]string)
	for _, gs := range genSections {
		norm := strings.ToLower(gs.Title)
		if _, exists := genMap[norm]; !exists {
			genMap[norm] = gs.Content
		}
	}

	var result strings.Builder
	genUsed := make(map[string]bool)
	existingTitles := make(map[string]bool)
	for _, es := range existSections {
		existingTitles[strings.ToLower(es.Title)] = true
	}

	// Process existing sections in their original order, preserving duplicate headings
	for _, es := range existSections {
		norm := strings.ToLower(es.Title)
		if es.Heading != "intro" && selectedMap[norm] && genMap[norm] != "" {
			if !genUsed[norm] {
				result.WriteString(genMap[norm])
				genUsed[norm] = true
			} else {
				// If duplicate heading in existing, keep the original content
				result.WriteString(es.Content)
			}
		} else {
			result.WriteString(es.Content)
		}
	}

	// Append any new generated sections that were not present in existing
	for _, gs := range genSections {
		if gs.Heading == "intro" {
			continue
		}
		norm := strings.ToLower(gs.Title)
		if !genUsed[norm] && !existingTitles[norm] {
			if len(selectedSections) == 0 || selectedMap[norm] {
				result.WriteString(gs.Content)
				genUsed[norm] = true
			}
		}
	}

	return result.String()
}

func parseReadmeSections(doc string) []ReadmeSection {
	var sections []ReadmeSection
	lines := strings.Split(doc, "\n")
	currentHeading := "intro"
	currentTitle := "intro"
	var currentLines []string
	idx := 0

	for _, line := range lines {
		if strings.HasPrefix(line, "## ") {
			if len(currentLines) > 0 {
				sections = append(sections, ReadmeSection{
					Index:   idx,
					Heading: currentHeading,
					Title:   currentTitle,
					Content: strings.Join(currentLines, "\n") + "\n",
				})
				idx++
			}
			currentHeading = strings.TrimSpace(line)
			currentTitle = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			currentLines = []string{line}
		} else {
			currentLines = append(currentLines, line)
		}
	}
	if len(currentLines) > 0 {
		sections = append(sections, ReadmeSection{
			Index:   idx,
			Heading: currentHeading,
			Title:   currentTitle,
			Content: strings.Join(currentLines, "\n") + "\n",
		})
	}
	return sections
}

// GeneratePrintableHTMLGuide generates a self-contained printable guide with CSS print rules.
func GeneratePrintableHTMLGuide(snapshot ProjectSnapshot) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Architecture & Operations Guide - ` + html.EscapeString(snapshot.ProjectName) + `</title>
<style>
@page { size: A4; margin: 20mm; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; color: #1e293b; line-height: 1.5; padding: 20px; }
h1 { color: #0f172a; border-bottom: 2px solid #e2e8f0; padding-bottom: 8px; }
h2 { color: #334155; margin-top: 24px; border-bottom: 1px solid #e2e8f0; padding-bottom: 4px; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 12px; font-weight: bold; background: #e0f2fe; color: #0369a1; margin-right: 6px; }
table { width: 100%; border-collapse: collapse; margin-top: 12px; }
th, td { border: 1px solid #cbd5e1; padding: 8px 12px; text-align: left; }
th { background: #f8fafc; font-weight: 600; }
.card { background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; padding: 12px; margin-top: 10px; }
@media print {
  body { padding: 0; }
  .no-print { display: none; }
}
</style>
</head>
<body>
<h1>` + html.EscapeString(snapshot.ProjectName) + ` · System Guide</h1>
<p>Generated: ` + html.EscapeString(snapshot.GeneratedAt) + `</p>
<div>
  <span class="badge">PHP ` + html.EscapeString(snapshot.PHPVersion) + `</span>
  <span class="badge">` + html.EscapeString(snapshot.Framework) + `</span>
  <span class="badge">Clockwork ` + html.EscapeString(snapshot.Clockwork) + `</span>
</div>

<h2>Architecture Components</h2>
<table>
  <tr><th>Component</th><th>Type</th><th>Provenance</th></tr>
`)
	for _, edge := range snapshot.Graph.Edges {
		fmt.Fprintf(&b, "  <tr><td>%s &rarr; %s</td><td>%s</td><td><strong>%s</strong></td></tr>\n",
			html.EscapeString(edge.Source), html.EscapeString(edge.Target), html.EscapeString(edge.Label), html.EscapeString(edge.Provenance))
	}
	b.WriteString(`</table>

<h2>Hosting & Sizing</h2>
<div class="card">
  <p><strong>Recommended Configuration:</strong> ` + html.EscapeString(snapshot.HostAdvice.RecommendedPlan) + `</p>
  <p><strong>Minimum RAM:</strong> ` + html.EscapeString(snapshot.HostAdvice.MinimumRAM) + ` | <strong>OPcache:</strong> ` + html.EscapeString(snapshot.HostAdvice.OpcacheMemory) + `</p>
  <ul>
`)
	for _, r := range snapshot.HostAdvice.Recommendations {
		fmt.Fprintf(&b, "    <li>%s</li>\n", html.EscapeString(r))
	}
	b.WriteString(`  </ul>
</div>
</body>
</html>`)
	return b.String()
}

// ExportTeamManifest creates non-secret team setup manifest file.
func ExportTeamManifest(snapshot ProjectSnapshot, outPath string) error {
	manifest := TeamManifest{
		Version:     "1.0",
		ProjectName: snapshot.ProjectName,
		PHPVersion:  snapshot.PHPVersion,
		Framework:   snapshot.Framework,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		Commands: []string{
			"composer install",
			"npm install",
			"php artisan migrate",
			"npm run dev",
		},
	}
	for _, s := range snapshot.Services {
		manifest.Services = append(manifest.Services, ManifestService{
			Name: s,
			Type: "local-service",
		})
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0644)
}

// ExportOfflineDiagnosticBundle packages sanitized snapshot data and logs into a zip.
func ExportOfflineDiagnosticBundle(snapshot ProjectSnapshot, logs []string, outZipPath string) error {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Add snapshot.json
	snapshotData, err := json.MarshalIndent(snapshot, "", "  ")
	if err == nil {
		if f, err := zw.Create("diagnostic_snapshot.json"); err == nil {
			_, _ = f.Write(snapshotData)
		}
	}

	// Add logs.txt
	if f, err := zw.Create("sanitized_logs.txt"); err == nil {
		_, _ = f.Write([]byte(storage.RedactSensitive(strings.Join(logs, "\n"))))
	}

	// Add printable guide.html
	if f, err := zw.Create("system_guide.html"); err == nil {
		_, _ = f.Write([]byte(GeneratePrintableHTMLGuide(snapshot)))
	}

	if err := zw.Close(); err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(outZipPath), 0755)
	return os.WriteFile(outZipPath, buf.Bytes(), 0644)
}
