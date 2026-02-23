package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"kube-watcher/mcp/monitoring/history"
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
	return &HistoryInsightsTool{
		BaseTool: NewBaseTool(nil),
		store:    store,
	}
}

func NewHistoryInsightsToolWithClient(k8sManager kube.ClientInterface, store history.Recorder) *HistoryInsightsTool {
	return &HistoryInsightsTool{
		BaseTool: NewBaseTool(k8sManager),
		store:    store,
	}
}

func (t *HistoryInsightsTool) Name() string { return "list_repeating_issues" }
func (t *HistoryInsightsTool) Description() string {
	return "Analyze incident history to identify trends and recurring anomalies."
}

func (t *HistoryInsightsTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "kind", Type: "string", Description: "Filter: node_anomaly, pod_anomaly, or event_spike."},
		{Name: "since_hours", Type: "number", Default: 24},
		{Name: "severity", Type: "string", Description: "Filter: critical, high, low."},
		{Name: "limit", Type: "number", Default: 20},
	}
}

func (t *HistoryInsightsTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	sinceHours := t.getFloat64(args, "since_hours", 24.0)
	limit := t.GetIntArg(args, "limit", 20)
	kind := history.IssueKind(t.GetStringArg(args, "kind", ""))
	severity := t.GetStringArg(args, "severity", "")

	window := time.Duration(sinceHours) * time.Hour

	allIncidents, err := t.loadIncidents(ctx, kind, window)
	if err != nil {
		return nil, fmt.Errorf("loading incidents: %w", err)
	}

	filtered := t.filterIncidents(allIncidents, severity, limit)

	comparison, err := t.computeFrequency(ctx, kind, window)
	if err != nil {
		return nil, fmt.Errorf("computing frequency: %w", err)
	}

	return map[string]any{
		"query_context": map[string]any{
			"kind":        kind,
			"since_hours": sinceHours,
			"severity":    severity,
		},
		"results": map[string]any{
			"count":           len(filtered),
			"incidents":       filtered,
			"frequency_trend": comparison,
		},
		"insight": t.generateInsight(comparison),
	}, nil
}

func (t *HistoryInsightsTool) loadIncidents(ctx context.Context, kind history.IssueKind, window time.Duration) ([]history.Incident, error) {
	targetKinds := supportedHistoryKinds
	if kind != "" {
		targetKinds = []history.IssueKind{kind}
	}

	var results []history.Incident
	for _, k := range targetKinds {
		data, err := t.store.List(ctx, k, window)
		if err != nil {
			return nil, err
		}
		results = append(results, data...)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	return results, nil
}

func (t *HistoryInsightsTool) filterIncidents(incidents []history.Incident, severity string, limit int) []history.Incident {
	out := make([]history.Incident, 0)
	for _, inc := range incidents {
		if severity != "" && !strings.EqualFold(inc.Severity, severity) {
			continue
		}
		out = append(out, inc)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func (t *HistoryInsightsTool) computeFrequency(ctx context.Context, kind history.IssueKind, window time.Duration) (history.FrequencyComparison, error) {
	if kind != "" {
		return t.store.CompareFrequency(ctx, kind, window, window)
	}

	combined := history.FrequencyComparison{
		Kind:             "all",
		WindowHours:      window.Hours(),
		PreviousWindowHr: window.Hours(),
	}

	for _, k := range supportedHistoryKinds {
		f, err := t.store.CompareFrequency(ctx, k, window, window)
		if err != nil {
			return combined, err
		}
		combined.RecentCount += f.RecentCount
		combined.PreviousCount += f.PreviousCount
	}

	combined.PercentChange = calcPercentChange(combined.PreviousCount, combined.RecentCount)
	return combined, nil
}

func (t *HistoryInsightsTool) generateInsight(f history.FrequencyComparison) string {
	switch {
	case f.RecentCount == 0:
		return "No issues detected in the current window."
	case f.PercentChange > 50:
		return "Critical: Significant spike in issue frequency detected."
	case f.PercentChange > 0:
		return "Warning: Issues are trending upward."
	default:
		return "Issue frequency is stable or declining."
	}
}

func (t *HistoryInsightsTool) getFloat64(args map[string]any, key string, fallback float64) float64 {
	v, ok := args[key]
	if !ok {
		return fallback
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return fallback
	}
}

func calcPercentChange(prev, curr int) float64 {
	if prev == 0 {
		if curr > 0 {
			return 100.0
		}
		return 0.0
	}
	return (float64(curr-prev) / float64(prev)) * 100.0
}
