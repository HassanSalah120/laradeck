package flags

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type FeatureFlag struct {
	Name         string   `json:"name"`
	DefinedIn    string   `json:"definedIn"`
	Locations    []string `json:"locations"`
	DefaultState string   `json:"defaultState"` // "active", "inactive", "dynamic"
	Provider     string   `json:"provider"`     // "pennant", "custom"
}

type FlagsReport struct {
	HasPennant bool          `json:"hasPennant"`
	Flags      []FeatureFlag `json:"flags"`
	TotalFlags int           `json:"totalFlags"`
}

var (
	featureDefineRegex = regexp.MustCompile(`Feature::define\s*\(\s*['"]([a-zA-Z0-9_\-:]+)['"]`)
	featureUsageRegex  = regexp.MustCompile(`(?:Feature::(?:active|inactive|allAreActive|someAreActive|value|when|unless)\s*\(\s*['"]([a-zA-Z0-9_\-:]+)['"]|@(?:feature|unlessfeature)\s*\(\s*['"]([a-zA-Z0-9_\-:]+)['"])`)
)

func ScanFlags(ctx context.Context, projectPath string) FlagsReport {
	report := FlagsReport{
		Flags: make([]FeatureFlag, 0),
	}

	if projectPath == "" {
		return report
	}

	// 1. Check composer.json for laravel/pennant
	composerPath := filepath.Join(projectPath, "composer.json")
	if data, err := os.ReadFile(composerPath); err == nil {
		var comp struct {
			Require    map[string]string `json:"require"`
			RequireDev map[string]string `json:"require-dev"`
		}
		if json.Unmarshal(data, &comp) == nil {
			if _, ok := comp.Require["laravel/pennant"]; ok {
				report.HasPennant = true
			}
			if _, ok := comp.RequireDev["laravel/pennant"]; ok {
				report.HasPennant = true
			}
		}
	}

	flagMap := make(map[string]*FeatureFlag)

	// Scan target directories
	scanDirs := []string{"app", "config", "routes", "resources"}
	for _, dirName := range scanDirs {
		dir := filepath.Join(projectPath, dirName)
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".php" && ext != ".blade" && !strings.HasSuffix(path, ".blade.php") {
				return nil
			}

			rel, _ := filepath.Rel(projectPath, path)
			rel = filepath.ToSlash(rel)

			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return nil
			}
			str := string(content)

			// Find defines
			defines := featureDefineRegex.FindAllStringSubmatch(str, -1)
			for _, d := range defines {
				if len(d) > 1 && d[1] != "" {
					name := d[1]
					fl, exists := flagMap[name]
					if !exists {
						fl = &FeatureFlag{
							Name:         name,
							DefinedIn:    rel,
							Locations:    make([]string, 0),
							DefaultState: "dynamic",
							Provider:     "pennant",
						}
						flagMap[name] = fl
					} else if fl.DefinedIn == "" {
						fl.DefinedIn = rel
					}
				}
			}

			// Find usages
			usages := featureUsageRegex.FindAllStringSubmatch(str, -1)
			for _, u := range usages {
				name := ""
				if len(u) > 1 && u[1] != "" {
					name = u[1]
				} else if len(u) > 2 && u[2] != "" {
					name = u[2]
				}
				if name == "" {
					continue
				}

				fl, exists := flagMap[name]
				if !exists {
					fl = &FeatureFlag{
						Name:         name,
						Locations:    make([]string, 0),
						DefaultState: "dynamic",
						Provider:     "pennant",
					}
					flagMap[name] = fl
				}
				// Add location if not already added
				already := false
				for _, loc := range fl.Locations {
					if loc == rel {
						already = true
						break
					}
				}
				if !already {
					fl.Locations = append(fl.Locations, rel)
				}
			}

			return nil
		})
	}

	for _, fl := range flagMap {
		if !report.HasPennant && fl.Provider == "pennant" {
			fl.Provider = "custom"
		}
		report.Flags = append(report.Flags, *fl)
	}
	report.TotalFlags = len(report.Flags)

	return report
}
