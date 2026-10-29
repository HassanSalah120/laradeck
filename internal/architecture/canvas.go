package architecture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ComponentNode struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Layer     int      `json:"layer"`     // 1: HTTP, 2: Domain, 3: Async, 4: Persistence
	LayerName string   `json:"layerName"` // "HTTP & Presentation", "Domain Workflows", "Async & Queue", "Persistence & Storage"
	Kind      string   `json:"kind"`      // "Controller", "Middleware", "Request", "Action", "Service", "Event", "Job", "Listener", "Notification", "Model", "Repository"
	FilePath  string   `json:"filePath"`
	Calls     []string `json:"calls"`     // IDs of other components it interacts with
}

type BoundaryViolation struct {
	RuleID      string `json:"ruleId"`
	Severity    string `json:"severity"` // "error", "warning", "info"
	SourceFile  string `json:"sourceFile"`
	SourceKind  string `json:"sourceKind"`
	TargetFile  string `json:"targetFile,omitempty"`
	Message     string `json:"message"`
	Explanation string `json:"explanation"`
}

type ArchitectureCanvasReport struct {
	ProjectID       string                    `json:"projectId"`
	Layers          map[string][]ComponentNode `json:"layers"`
	Violations      []BoundaryViolation       `json:"violations"`
	MermaidCode     string                    `json:"mermaidCode"`
	TotalComponents int                       `json:"totalComponents"`
}

var (
	rawDbRegex      = regexp.MustCompile(`DB::(?:table|select|statement|unprepared)\s*\(`)
	httpCallRegex   = regexp.MustCompile(`(?:Http::|Illuminate\\Support\\Facades\\Http)`)
	httpRequestRegex = regexp.MustCompile(`(?:Illuminate\\Http\\Request|\$request->)`)
)

func BuildArchitectureCanvas(ctx context.Context, projectID, projectPath string) ArchitectureCanvasReport {
	report := ArchitectureCanvasReport{
		ProjectID: projectID,
		Layers: map[string][]ComponentNode{
			"1": make([]ComponentNode, 0),
			"2": make([]ComponentNode, 0),
			"3": make([]ComponentNode, 0),
			"4": make([]ComponentNode, 0),
		},
		Violations: make([]BoundaryViolation, 0),
	}

	if projectPath == "" {
		return report
	}

	appDir := filepath.Join(projectPath, "app")
	if _, err := os.Stat(appDir); err != nil {
		return report
	}

	discovered := make([]ComponentNode, 0)

	_ = filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".php") {
			return nil
		}

		rel, _ := filepath.Rel(projectPath, path)
		rel = filepath.ToSlash(rel)
		fileName := filepath.Base(path)
		nameWithoutExt := strings.TrimSuffix(fileName, ".php")

		layer := 0
		layerName := ""
		kind := ""

		switch {
		case strings.Contains(rel, "app/Http/Controllers/"):
			layer = 1
			layerName = "HTTP & Presentation"
			kind = "Controller"
		case strings.Contains(rel, "app/Http/Middleware/"):
			layer = 1
			layerName = "HTTP & Presentation"
			kind = "Middleware"
		case strings.Contains(rel, "app/Http/Requests/"):
			layer = 1
			layerName = "HTTP & Presentation"
			kind = "Request"
		case strings.Contains(rel, "app/Actions/"):
			layer = 2
			layerName = "Domain Workflows"
			kind = "Action"
		case strings.Contains(rel, "app/Services/"):
			layer = 2
			layerName = "Domain Workflows"
			kind = "Service"
		case strings.Contains(rel, "app/Events/"):
			layer = 2
			layerName = "Domain Workflows"
			kind = "Event"
		case strings.Contains(rel, "app/Jobs/"):
			layer = 3
			layerName = "Async & Queue"
			kind = "Job"
		case strings.Contains(rel, "app/Listeners/"):
			layer = 3
			layerName = "Async & Queue"
			kind = "Listener"
		case strings.Contains(rel, "app/Notifications/"):
			layer = 3
			layerName = "Async & Queue"
			kind = "Notification"
		case strings.Contains(rel, "app/Mail/"):
			layer = 3
			layerName = "Async & Queue"
			kind = "Mail"
		case strings.Contains(rel, "app/Models/"):
			layer = 4
			layerName = "Persistence & Storage"
			kind = "Model"
		case strings.Contains(rel, "app/Repositories/"):
			layer = 4
			layerName = "Persistence & Storage"
			kind = "Repository"
		}

		if layer == 0 {
			return nil
		}

		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		content := string(contentBytes)

		// Boundary linting
		// 1. Controller raw DB query
		if kind == "Controller" && rawDbRegex.MatchString(content) {
			report.Violations = append(report.Violations, BoundaryViolation{
				RuleID:      "controller-direct-db",
				Severity:    "warning",
				SourceFile:  rel,
				SourceKind:  kind,
				Message:     fmt.Sprintf("%s directly invokes raw DB query facades.", nameWithoutExt),
				Explanation: "Controllers should orchestrate requests and responses. Move direct SQL queries into an Action, Repository, or Eloquent Model.",
			})
		}

		// 2. Model outbound HTTP
		if kind == "Model" && httpCallRegex.MatchString(content) {
			report.Violations = append(report.Violations, BoundaryViolation{
				RuleID:      "model-http-call",
				Severity:    "error",
				SourceFile:  rel,
				SourceKind:  kind,
				Message:     fmt.Sprintf("%s makes outbound HTTP calls directly.", nameWithoutExt),
				Explanation: "Persistence entities should not perform external network calls. Move HTTP communication to an Action or dedicated Service class.",
			})
		}

		// 3. Action coupled to HTTP Request
		if kind == "Action" && httpRequestRegex.MatchString(content) {
			report.Violations = append(report.Violations, BoundaryViolation{
				RuleID:      "action-coupled-request",
				Severity:    "info",
				SourceFile:  rel,
				SourceKind:  kind,
				Message:     fmt.Sprintf("%s accepts or references Illuminate Request directly.", nameWithoutExt),
				Explanation: "Actions are more reusable across CLI commands and queued jobs when they accept typed DTOs or primitive parameters rather than HTTP request objects.",
			})
		}

		node := ComponentNode{
			ID:        cleanMermaidID(nameWithoutExt),
			Name:      nameWithoutExt,
			Layer:     layer,
			LayerName: layerName,
			Kind:      kind,
			FilePath:  rel,
			Calls:     make([]string, 0),
		}

		discovered = append(discovered, node)
		return nil
	})

	// Discover authentic inter-component calls by scanning source code for referenced components
	targetRegexes := make(map[string]*regexp.Regexp, len(discovered))
	for _, n := range discovered {
		targetRegexes[n.ID] = regexp.MustCompile(`\b` + regexp.QuoteMeta(n.Name) + `\b`)
	}

	for i := range discovered {
		srcNode := &discovered[i]
		srcContent, readErr := os.ReadFile(filepath.Join(projectPath, srcNode.FilePath))
		if readErr != nil {
			continue
		}
		srcStr := string(srcContent)
		for j := range discovered {
			if i == j {
				continue
			}
			targetNode := discovered[j]
			re := targetRegexes[targetNode.ID]
			if re != nil && re.MatchString(srcStr) {
				srcNode.Calls = append(srcNode.Calls, targetNode.ID)
			}
		}
	}

	for _, node := range discovered {
		layerKey := fmt.Sprintf("%d", node.Layer)
		report.Layers[layerKey] = append(report.Layers[layerKey], node)
	}

	report.TotalComponents = len(discovered)
	report.MermaidCode = generateMermaidCode(report.Layers)

	return report
}

func cleanMermaidID(name string) string {
	res := strings.ReplaceAll(name, "-", "_")
	res = strings.ReplaceAll(res, "\\", "_")
	res = strings.ReplaceAll(res, "/", "_")
	return res
}

func generateMermaidCode(layers map[string][]ComponentNode) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")
	sb.WriteString("  %% LaraDeck Living Architecture Canvas\n")
	sb.WriteString("  classDef l1 fill:#1e1b4b,stroke:#818cf8,stroke-width:1px,color:#e0e7ff;\n")
	sb.WriteString("  classDef l2 fill:#3b0764,stroke:#c084fc,stroke-width:1px,color:#fae8ff;\n")
	sb.WriteString("  classDef l3 fill:#0c4a6e,stroke:#38bdf8,stroke-width:1px,color:#e0f2fe;\n")
	sb.WriteString("  classDef l4 fill:#064e3b,stroke:#34d399,stroke-width:1px,color:#ecfdf5;\n\n")

	layerNames := map[string]string{
		"1": "Layer 1: HTTP & Presentation",
		"2": "Layer 2: Domain Workflows",
		"3": "Layer 3: Async & Background Queue",
		"4": "Layer 4: Persistence & Storage",
	}

	for lNum := 1; lNum <= 4; lNum++ {
		key := fmt.Sprintf("%d", lNum)
		nodes := layers[key]
		if len(nodes) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("  subgraph Subgraph_L%d [\"%s\"]\n", lNum, layerNames[key]))
		for _, n := range nodes {
			sb.WriteString(fmt.Sprintf("    %s[\"%s (%s)\"]:::l%d\n", n.ID, n.Name, n.Kind, lNum))
		}
		sb.WriteString("  end\n\n")
	}

	// Connect components with genuine discovered calls
	edgesWritten := make(map[string]bool)
	hasEdges := false
	for _, nodes := range layers {
		for _, n := range nodes {
			for _, targetID := range n.Calls {
				edgeKey := n.ID + "->" + targetID
				if !edgesWritten[edgeKey] {
					edgesWritten[edgeKey] = true
					hasEdges = true
					sb.WriteString(fmt.Sprintf("  %s --> %s\n", n.ID, targetID))
				}
			}
		}
	}

	if !hasEdges {
		sb.WriteString("  %% Discovered components; no direct inter-component calls detected in scanned files.\n")
	}

	return sb.String()
}
