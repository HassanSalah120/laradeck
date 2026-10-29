package reports

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitectureGraph(t *testing.T) {
	graph := BuildArchitectureGraph("MyLaravelApp", "mysql", "redis", "redis", true, []string{"https://api.stripe.com/v1/charges"})
	if len(graph.Nodes) < 5 {
		t.Fatalf("expected at least 5 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) < 4 {
		t.Fatalf("expected at least 4 edges, got %d", len(graph.Edges))
	}

	// Verify observed edge
	foundObserved := false
	for _, e := range graph.Edges {
		if e.Provenance == "observed" {
			foundObserved = true
		}
	}
	if !foundObserved {
		t.Errorf("expected at least one observed edge")
	}
}

func TestREADMEGenerationAndMerge(t *testing.T) {
	snap := ProjectSnapshot{
		ProjectName:    "Awesome App",
		PHPVersion:     "8.4.1",
		Framework:      "12.70.0",
		DatabaseDriver: "mysql",
		CacheDriver:    "redis",
		HostAdvice: HostingAdvice{
			RecommendedPlan: "Standard VPS",
			MinimumRAM:      "2 GB",
			OpcacheMemory:   "128M",
		},
	}

	generated := GenerateREADME(snap)
	if !strings.Contains(generated, "## Requirements") {
		t.Errorf("expected Requirements section")
	}

	existing := `# Awesome App

Custom intro written by developer that must be preserved.

## Requirements

Old requirements that should be replaced.

## Custom Section

Custom notes that must NOT be removed.
`

	merged := MergeREADME(existing, generated, []string{"requirements"})
	if !strings.Contains(merged, "Custom intro written by developer") {
		t.Errorf("developer intro was lost")
	}
	if !strings.Contains(merged, "Custom Section") {
		t.Errorf("custom section was lost")
	}
	if !strings.Contains(merged, "PHP: 8.4.1") {
		t.Errorf("expected updated PHP version in merged requirements")
	}
}

func TestExportBundleAndManifest(t *testing.T) {
	tmpDir := t.TempDir()
	snap := ProjectSnapshot{
		ProjectName: "BundleTest",
		PHPVersion:  "8.4",
		Framework:   "12.0",
		Services:    []string{"mysql", "redis"},
	}

	manifestPath := filepath.Join(tmpDir, "workbench.manifest.json")
	if err := ExportTeamManifest(snap, manifestPath); err != nil {
		t.Fatalf("ExportTeamManifest failed: %v", err)
	}
	if _, err := os.Stat(manifestPath); err != nil {
		t.Errorf("expected manifest file to exist")
	}

	bundlePath := filepath.Join(tmpDir, "support_bundle.zip")
	logs := []string{"[2026-10-16 01:00:00] local.INFO: Test log entry"}
	if err := ExportOfflineDiagnosticBundle(snap, logs, bundlePath); err != nil {
		t.Fatalf("ExportOfflineDiagnosticBundle failed: %v", err)
	}

	// Verify zip contents
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		t.Fatalf("failed to open zip bundle: %v", err)
	}
	defer zr.Close()

	if len(zr.File) < 3 {
		t.Errorf("expected 3 files in bundle zip, got %d", len(zr.File))
	}
}

func TestMergeREADMEDuplicateHeadings(t *testing.T) {
	existing := `# App
Intro text

## Notes
First notes block

## Features
Original features

## Notes
Second notes block with special instructions
`
	generated := `# App
## Features
Updated features list
`
	merged := MergeREADME(existing, generated, []string{"features"})

	// Check that both Notes blocks are preserved in order!
	if !strings.Contains(merged, "First notes block") {
		t.Errorf("expected first notes block to be preserved")
	}
	if !strings.Contains(merged, "Second notes block with special instructions") {
		t.Errorf("expected second notes block to be preserved")
	}
	if !strings.Contains(merged, "Updated features list") {
		t.Errorf("expected updated features list")
	}

	firstNotesIdx := strings.Index(merged, "First notes block")
	secondNotesIdx := strings.Index(merged, "Second notes block with special instructions")
	if firstNotesIdx >= secondNotesIdx {
		t.Errorf("expected first notes block before second notes block")
	}
}

func TestPrintableGuideEscapesProjectContent(t *testing.T) {
	html := GeneratePrintableHTMLGuide(ProjectSnapshot{ProjectName: `<script>alert(1)</script>`, Graph: ArchitectureGraph{Edges: []ArchitectureEdge{{Source: `<img onerror=evil>`, Target: "db"}}}})
	if strings.Contains(html, "<script>") || strings.Contains(html, "<img onerror") {
		t.Fatal("project content became executable HTML")
	}
}
func TestEmptySelectionDoesNotDuplicateExistingSections(t *testing.T) {
	merged := MergeREADME("# App\n## Requirements\nCustom\n", "# App\n## Requirements\nGenerated\n", nil)
	if strings.Count(merged, "## Requirements") != 1 || !strings.Contains(merged, "Custom") {
		t.Fatal(merged)
	}
}
