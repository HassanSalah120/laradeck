package telemetry

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type RawTimelineEvent struct {
	Description string  `json:"description"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Duration    float64 `json:"duration"`
}

type WaterfallSegment struct {
	ID         string  `json:"id"`
	Category   string  `json:"category"` // "boot", "middleware", "controller", "view", "database", "cache", "event", "other"
	Label      string  `json:"label"`
	StartMs    float64 `json:"startMs"`
	DurationMs float64 `json:"durationMs"`
	Percentage float64 `json:"percentage"`
	Details    string  `json:"details,omitempty"`
	Status     string  `json:"status,omitempty"` // "ok", "slow", "warning"
}

type WaterfallFlamegraph struct {
	RequestID     string             `json:"requestId"`
	TotalDuration float64            `json:"totalDuration"`
	Segments      []WaterfallSegment `json:"segments"`
	Breakdown     map[string]float64 `json:"breakdown"`
}

func categorizeEvent(desc string) string {
	d := strings.ToLower(desc)
	switch {
	case strings.Contains(d, "boot") || strings.Contains(d, "framework"):
		return "boot"
	case strings.Contains(d, "middleware") || strings.Contains(d, "pipeline"):
		return "middleware"
	case strings.Contains(d, "controller") || strings.Contains(d, "action") || strings.Contains(d, "@"):
		return "controller"
	case strings.Contains(d, "view") || strings.Contains(d, "blade") || strings.Contains(d, "render"):
		return "view"
	case strings.Contains(d, "select") || strings.Contains(d, "insert") || strings.Contains(d, "update") || strings.Contains(d, "delete") || strings.Contains(d, "query") || strings.Contains(d, "db"):
		return "database"
	case strings.Contains(d, "cache") || strings.Contains(d, "redis") || strings.Contains(d, "memcached"):
		return "cache"
	case strings.Contains(d, "event") || strings.Contains(d, "listener") || strings.Contains(d, "dispatch"):
		return "event"
	default:
		return "other"
	}
}

// BuildWaterfall transforms timeline events and database queries into a waterfall flamegraph
func BuildWaterfall(requestID string, totalDuration float64, events []RawTimelineEvent, queries []RawQueryItem) WaterfallFlamegraph {
	if totalDuration <= 0 {
		totalDuration = 0.1
	}

	segments := make([]WaterfallSegment, 0, len(events)+len(queries))
	breakdown := make(map[string]float64)

	// Add timeline events
	for i, ev := range events {
		dur := ev.Duration
		if dur <= 0 && ev.End > ev.Start {
			dur = (ev.End - ev.Start) * 1000 // if stored in seconds
		}
		if dur < 0 {
			dur = 0
		}

		cat := categorizeEvent(ev.Description)
		pct := 0.0
		if totalDuration > 0 {
			pct = math.Round((dur/totalDuration)*1000) / 10
		}
		if pct > 100 {
			pct = 100
		}

		status := "ok"
		if dur > 50.0 || pct > 40.0 {
			status = "slow"
		} else if dur > 20.0 || pct > 20.0 {
			status = "warning"
		}

		segments = append(segments, WaterfallSegment{
			ID:         fmt.Sprintf("event-%d", i),
			Category:   cat,
			Label:      ev.Description,
			StartMs:    math.Round(ev.Start*100) / 100,
			DurationMs: math.Round(dur*100) / 100,
			Percentage: pct,
			Status:     status,
		})
		breakdown[cat] += dur
	}

	// Add database queries into waterfall
	for i, q := range queries {
		dur := q.Duration
		pct := 0.0
		if totalDuration > 0 {
			pct = math.Round((dur/totalDuration)*1000) / 10
		}
		if pct > 100 {
			pct = 100
		}

		status := "ok"
		if dur > 50.0 {
			status = "slow"
		} else if dur > 15.0 {
			status = "warning"
		}

		label := q.Query
		if len(label) > 80 {
			label = label[:77] + "..."
		}

		startMs := q.StartMs
		if startMs < 0 {
			startMs = 0
		}

		segments = append(segments, WaterfallSegment{
			ID:         fmt.Sprintf("query-%d", i),
			Category:   "database",
			Label:      label,
			StartMs:    math.Round(startMs*100) / 100,
			DurationMs: math.Round(dur*100) / 100,
			Percentage: pct,
			Details:    q.Query,
			Status:     status,
		})
		breakdown["database"] += dur
	}

	// If no events were captured, synthesize a high-level breakdown
	if len(segments) == 0 {
		segments = append(segments, WaterfallSegment{
			ID:         "total-request",
			Category:   "controller",
			Label:      "Request Execution",
			StartMs:    0,
			DurationMs: math.Round(totalDuration*100) / 100,
			Percentage: 100,
			Status:     "ok",
		})
		breakdown["controller"] = totalDuration
	}

	// Round breakdown numbers
	for k, v := range breakdown {
		breakdown[k] = math.Round(v*100) / 100
	}

	// Sort segments by start time
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartMs < segments[j].StartMs
	})

	return WaterfallFlamegraph{
		RequestID:     requestID,
		TotalDuration: math.Round(totalDuration*100) / 100,
		Segments:      segments,
		Breakdown:     breakdown,
	}
}
