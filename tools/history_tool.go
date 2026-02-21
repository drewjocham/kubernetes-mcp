package tools

import (
	"context"
	"time"

	"kube-watcher/internal/history"
	"kube-watcher/internal/kubernetes"
)

type HistoryInsightsTool struct {
	BaseTool
	store *history.Store
}

func NewHistoryInsightsTool(k8sManager kubernetes.ClientInterface, store *history.Store) *HistoryInsightsTool {
	return &HistoryInsightsTool{
		BaseTool: NewBaseTool(k8sManager),
		store:    store,
	}
}

func (t *HistoryInsightsTool) Name() string {
	return "list_repeating_issues"
}

func (t *HistoryInsightsTool) Description() string {
	return "Analyze incident history to identify trends, recurring anomalies, and frequency deltas."
}

func (t *HistoryInsightsTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "kind", Type: "string", Description: "Filter: node_anomaly, pod_anomaly, or event_spike."},
		{Name: "since_hours", Type: "number", Default: 24},
		{Name: "severity", Type: "string"},
		{Name: "limit", Type: "number", Default: 20},
	}
}

func (t *HistoryInsightsTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	sinceHours := t.GetFloatArg(args, "since_hours", 24.0)
	limit := t.GetIntArg(args, "limit", 20)
	kind := history.IssueKind(t.GetStringArg(args, "kind", ""))
	severity := t.GetStringArg(args, "severity", "")

	// 1. Load raw data from BadgerDB
	allIncidents, err := t.store.Load(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Functional Filtering (Pure)
	window := time.Duration(sinceHours) * time.Hour
	cutoff := time.Now().Add(-window)

	filtered := history.Filter(allIncidents, func(inc history.Incident) bool {
		if !kind.IsZero() && inc.Kind != kind {
			return false
		}
		if severity != "" && inc.Severity != severity {
			return false
		}
		return inc.Timestamp.After(cutoff)
	})

	if len(filtered) > limit {
		filtered = filtered[:limit]
	}

	comparison := history.ComputeFrequency(allIncidents, kind, window, window)

	return map[string]interface{}{
		"query_context": map[string]interface{}{
			"kind":        kind,
			"since_hours": sinceHours,
			"severity":    severity,
		},
		"results": map[string]interface{}{
			"count":           len(filtered),
			"incidents":       filtered,
			"frequency_trend": comparison,
		},
		"insight": t.generateInsight(comparison),
	}, nil
}

func (t *HistoryInsightsTool) generateInsight(f history.FrequencyComparison) string {
	if f.PercentChange > 50 {
		return "Critical: Significant spike in issue frequency detected compared to previous window."
	}
	if f.PercentChange > 0 {
		return "Warning: Issues are trending upward."
	}
	if f.RecentCount == 0 {
		return "No issues detected in the current time window."
	}
	return "Issue frequency is stable or declining."
}
