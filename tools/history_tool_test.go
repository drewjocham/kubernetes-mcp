package tools

import (
	"context"
	"testing"
	"time"

	"kube-watcher/internal/history"
)

func TestHistoryInsightsToolExecute(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := history.NewStore(dir)
	if err != nil {
		t.Fatalf("failed to init store: %v", err)
	}
	defer store.Close()

	now := time.Now()
	incidents := []history.Incident{
		{
			ID:        "1",
			Kind:      history.IncidentTypePod,
			Severity:  "high",
			Timestamp: now.Add(-1 * time.Hour),
			Namespace: "default",
			Name:      "api",
		},
		{
			ID:        "2",
			Kind:      history.IncidentTypePod,
			Severity:  "low",
			Timestamp: now.Add(-3 * time.Hour),
			Namespace: "default",
			Name:      "api",
		},
	}
	for _, inc := range incidents {
		if err := store.Record(ctx, inc); err != nil {
			t.Fatalf("failed to record incident: %v", err)
		}
	}

	tool := NewHistoryInsightsTool(store)

	tests := []struct {
		name        string
		args        map[string]interface{}
		wantCount   int
		wantInsight string
	}{
		{
			name: "filters_by_severity_and_limit",
			args: map[string]interface{}{
				"kind":        string(history.IncidentTypePod),
				"since_hours": 4.0,
				"severity":    "high",
				"limit":       1,
			},
			wantCount:   1,
			wantInsight: "Critical: Significant spike in issue frequency detected compared to previous window.",
		},
		{
			name:        "all_incidents_without_filters",
			args:        map[string]interface{}{},
			wantCount:   2,
			wantInsight: "Critical: Significant spike in issue frequency detected compared to previous window.",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res, err := tool.Execute(ctx, tc.args)
			if err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			results := res["results"].(map[string]interface{})
			count := results["count"].(int)
			if count != tc.wantCount {
				t.Fatalf("expected %d incidents, got %d", tc.wantCount, count)
			}
			if res["insight"] != tc.wantInsight {
				t.Fatalf("unexpected insight: %v", res["insight"])
			}
		})
	}
}
