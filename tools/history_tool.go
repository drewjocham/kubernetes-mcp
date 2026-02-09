package tools

import (
	"context"
	"time"

	"kube-watcher/internal/history"
)

type HistoryInsightsTool struct {
	store *history.Store
}

func NewHistoryInsightsTool(store *history.Store) *HistoryInsightsTool {
	return &HistoryInsightsTool{store: store}
}

func (t *HistoryInsightsTool) Name() string {
	return "list_repeating_issues"
}

func (t *HistoryInsightsTool) Description() string {
	return "Review incident history and compare occurrence frequency."
}

func (t *HistoryInsightsTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "kind", Type: "string", Description: "Incident kind filter (node_anomaly, pod_anomaly, event_spike)."},
		{Name: "since_hours", Type: "number", Description: "Time window (hours) to consider (default 24)."},
		{Name: "severity", Type: "string", Description: "Optional severity filter."},
		{Name: "limit", Type: "number", Description: "Maximum incidents to return (default 20)."},
	}
}

func (t *HistoryInsightsTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	sinceHours := 24.0
	if val, ok := args["since_hours"].(float64); ok && val > 0 {
		sinceHours = val
	}
	limit := 20
	if val, ok := args["limit"].(float64); ok && val > 0 {
		limit = int(val)
	}
	kind := history.IssueKind("")
	if val, ok := args["kind"].(string); ok {
		kind = history.IssueKind(val)
	}
	severity, _ := args["severity"].(string)

	incidents := t.store.List(ctx, history.Query{
		Since:    time.Duration(sinceHours) * time.Hour,
		Kind:     kind,
		Severity: severity,
		Limit:    limit,
	})

	comparison := t.store.CompareFrequency(ctx, kind, time.Duration(sinceHours)*time.Hour, time.Duration(sinceHours)*time.Hour)

	return map[string]interface{}{
		"kind":            kind,
		"since_hours":     sinceHours,
		"severity":        severity,
		"incident_count":  len(incidents),
		"incidents":       incidents,
		"frequency_delta": comparison,
	}, nil
}
