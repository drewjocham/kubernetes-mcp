package tools

import (
	"context"
	"kube-watcher/mcp/monitoring/history"
	"sort"
	"time"

	"kube-watcher/pkg/kube"
)

var supportedHistoryKinds = []history.IssueKind{
	history.IncidentTypeNode,
	history.IncidentTypePod,
	history.IncidentTypeEvent,
}

type HistoryInsightsTool struct {
	BaseTool
	store history.Recorder
}

func NewHistoryInsightsTool(store history.Recorder) *HistoryInsightsTool {
	return NewHistoryInsightsToolWithClient(nil, store)
}

func NewHistoryInsightsToolWithClient(k8sManager kube.ClientInterface, store history.Recorder) *HistoryInsightsTool {
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
	sinceHours := t.getFloatArg(args, "since_hours", 24.0)
	limit := t.GetIntArg(args, "limit", 20)
	kind := history.IssueKind(t.GetStringArg(args, "kind", ""))
	severity := t.GetStringArg(args, "severity", "")

	window := time.Duration(sinceHours) * time.Hour
	allIncidents, err := t.loadIncidents(ctx, kind, window)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-window)
	filtered := t.filterIncidents(allIncidents, kind, severity, cutoff, limit)

	comparison, err := t.computeFrequency(ctx, kind, window)
	if err != nil {
		return nil, err
	}

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

func (t *HistoryInsightsTool) getFloatArg(args map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := args[key].(float64); ok {
		return val
	}
	if val, ok := args[key].(int); ok {
		return float64(val)
	}
	return defaultVal
}

func (t *HistoryInsightsTool) loadIncidents(ctx context.Context, kind history.IssueKind, window time.Duration) ([]history.Incident, error) {
	kinds := supportedHistoryKinds
	if kind != "" {
		kinds = []history.IssueKind{kind}
	}

	var incidents []history.Incident
	for _, k := range kinds {
		data, err := t.store.List(ctx, k, window)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, data...)
	}

	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].Timestamp.After(incidents[j].Timestamp)
	})

	return incidents, nil
}

func (t *HistoryInsightsTool) filterIncidents(incidents []history.Incident, kind history.IssueKind, severity string, cutoff time.Time, limit int) []history.Incident {
	filtered := make([]history.Incident, 0, len(incidents))
	for _, inc := range incidents {
		if kind != "" && inc.Kind != kind {
			continue
		}
		if severity != "" && inc.Severity != severity {
			continue
		}
		if inc.Timestamp.Before(cutoff) {
			continue
		}
		filtered = append(filtered, inc)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	return filtered
}

func (t *HistoryInsightsTool) computeFrequency(ctx context.Context, kind history.IssueKind, window time.Duration) (history.FrequencyComparison, error) {
	if kind != "" {
		return t.store.CompareFrequency(ctx, kind, window, window)
	}

	var combined history.FrequencyComparison
	combined.Kind = history.IssueKind("all")
	combined.WindowHours = window.Hours()
	combined.PreviousWindowHr = window.Hours()

	for _, k := range supportedHistoryKinds {
		freq, err := t.store.CompareFrequency(ctx, k, window, window)
		if err != nil {
			return history.FrequencyComparison{}, err
		}
		combined.RecentCount += freq.RecentCount
		combined.PreviousCount += freq.PreviousCount
	}

	combined.PercentChange = percentChange(combined.PreviousCount, combined.RecentCount)
	return combined, nil
}

func percentChange(previous, current int) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}
	return (float64(current-previous) / float64(previous)) * 100.0
}
