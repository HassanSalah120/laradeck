package telemetry

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

var (
	singleQuoteRegex = regexp.MustCompile(`'(?:''|[^'])*'`)
	doubleQuoteRegex = regexp.MustCompile(`"(?:""|[^"])*"`)
	numberRegex      = regexp.MustCompile(`\b\d+\b`)
	inListRegex      = regexp.MustCompile(`(?i)\bin\s*\([^\)]*\)`)
	whitespaceRegex  = regexp.MustCompile(`\s+`)
	tableRegex       = regexp.MustCompile("(?i)\\b(?:FROM|INTO|UPDATE)\\s+[`\"']?([a-zA-Z0-9_]+)[`\"']?")
	whereColRegex    = regexp.MustCompile("(?i)\\bWHERE\\s+[`\"']?([a-zA-Z0-9_.]+)[`\"']?\\s*(?:=|<|>|<=|>=|LIKE|IN)")
)

type RawQueryItem struct {
	Query      string  `json:"query"`
	Duration   float64 `json:"duration"`
	Connection string  `json:"connection,omitempty"`
	StartMs    float64 `json:"startMs,omitempty"`
}

type NPlusOneIssue struct {
	Fingerprint     string  `json:"fingerprint"`
	NormalizedQuery string  `json:"normalizedQuery"`
	Count           int     `json:"count"`
	TotalDuration   float64 `json:"totalDuration"`
	Table           string  `json:"table"`
	Column          string  `json:"column"`
	EagerLoadFix    string  `json:"eagerLoadFix"`
	Severity        string  `json:"severity"`
	Explanation     string  `json:"explanation"`
}

type NPlusOneReport struct {
	RequestID  string          `json:"requestId"`
	URI        string          `json:"uri"`
	TotalLoops int             `json:"totalLoops"`
	WastedTime float64         `json:"wastedTime"`
	Issues     []NPlusOneIssue `json:"issues"`
}

// NormalizeQuery strips dynamic values to create a stable query signature
func NormalizeQuery(sql string) string {
	s := strings.TrimSpace(sql)
	s = singleQuoteRegex.ReplaceAllString(s, "?")
	s = doubleQuoteRegex.ReplaceAllString(s, "?")
	s = inListRegex.ReplaceAllString(s, "IN (?)")
	s = numberRegex.ReplaceAllString(s, "?")
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = whitespaceRegex.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// DetectNPlusOne analyzes queries in a request and detects repetitive query loops
func DetectNPlusOne(requestID, uri string, queries []RawQueryItem) NPlusOneReport {
	type queryGroup struct {
		normalized string
		count      int
		duration   float64
		sample     string
	}

	groups := make(map[string]*queryGroup)

	for _, q := range queries {
		if strings.TrimSpace(q.Query) == "" {
			continue
		}
		norm := NormalizeQuery(q.Query)
		g, exists := groups[norm]
		if !exists {
			g = &queryGroup{
				normalized: norm,
				sample:     q.Query,
			}
			groups[norm] = g
		}
		g.count++
		g.duration += q.Duration
	}

	report := NPlusOneReport{
		RequestID: requestID,
		URI:       uri,
		Issues:    make([]NPlusOneIssue, 0),
	}

	for _, g := range groups {
		// Threshold: 3 or more identical query fingerprints per request
		if g.count < 3 {
			continue
		}

		table := ""
		if matches := tableRegex.FindStringSubmatch(g.sample); len(matches) > 1 {
			table = matches[1]
		}

		col := ""
		if matches := whereColRegex.FindStringSubmatch(g.sample); len(matches) > 1 {
			col = matches[1]
			if idx := strings.LastIndex(col, "."); idx != -1 {
				col = col[idx+1:]
			}
		}

		severity := "warning"
		if g.count >= 10 || g.duration > 50.0 {
			severity = "critical"
		}

		// Suggest eager-load fix
		relationCandidate := table
		if strings.HasSuffix(relationCandidate, "s") {
			relationCandidate = strings.TrimSuffix(relationCandidate, "s")
		}
		if col != "" && strings.HasSuffix(col, "_id") {
			relationCandidate = strings.TrimSuffix(col, "_id")
		}

		eagerFix := fmt.Sprintf("->with('%s')", table)
		if relationCandidate != "" {
			eagerFix = fmt.Sprintf("->with('%s') or ->with('%s')", relationCandidate, table)
		}

		avgDuration := g.duration / float64(g.count)
		wasted := float64(g.count-1) * avgDuration
		report.WastedTime += wasted
		report.TotalLoops++

		explanation := fmt.Sprintf(
			"Detected %d repetitive executions of query on '%s' taking %.2fms total. This typically occurs when accessing relationships inside a @foreach or loop without eager loading.",
			g.count, table, g.duration,
		)

		report.Issues = append(report.Issues, NPlusOneIssue{
			Fingerprint:     g.normalized,
			NormalizedQuery: g.normalized,
			Count:           g.count,
			TotalDuration:   math.Round(g.duration*100) / 100,
			Table:           table,
			Column:          col,
			EagerLoadFix:    eagerFix,
			Severity:        severity,
			Explanation:     explanation,
		})
	}

	// Sort issues by count descending
	sort.Slice(report.Issues, func(i, j int) bool {
		return report.Issues[i].Count > report.Issues[j].Count
	})

	report.WastedTime = math.Round(report.WastedTime*100) / 100
	return report
}
