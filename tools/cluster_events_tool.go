package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"kube-watcher/kubernetes"
)

var (
	ErrFailedToGetClusterEvents = errors.New("failed to get cluster events")
)

type ClusterEventsTool struct {
	k8sManager kubernetes.ClientInterface
	logger     *slog.Logger
}

func NewClusterEventsTool(k8sManager kubernetes.ClientInterface, logger *slog.Logger) *ClusterEventsTool {
	return &ClusterEventsTool{
		k8sManager: k8sManager,
		logger:     logger,
	}
}

func (t *ClusterEventsTool) Name() string {
	return "get_cluster_events"
}

func (t *ClusterEventsTool) Description() string {
	return "Get and analyze recent cluster events"
}

func (t *ClusterEventsTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "namespace", Type: "string", Description: "Specific namespace to get events from (optional, gets all namespaces if not provided)"},
		{Name: "limit", Type: "number", Description: "Maximum number of events to return"},
		{Name: "event_type", Type: "string", Description: "Filter by event type (Warning, Normal, or all)"},
		{Name: "hours_back", Type: "number", Description: "How many hours back to look for events"},
	}
}

func (t *ClusterEventsTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	namespace, _ := args["namespace"].(string)
	limit, _ := args["limit"].(float64)
	eventType, _ := args["event_type"].(string)
	hoursBack, _ := args["hours_back"].(float64)

	if limit == 0 {
		limit = 50
	}
	if eventType == "" {
		eventType = "all"
	}
	if hoursBack == 0 {
		hoursBack = 24
	}

	var events []kubernetes.EventInfo
	var err error

	if namespace == "" {
		events, err = t.k8sManager.GetEventsAllNamespaces(ctx)
	} else {
		events, err = t.k8sManager.GetEvents(ctx, namespace)
	}

	if err != nil {
		t.logger.Error("failed to get events", "error", err)
		return nil, ErrFailedToGetClusterEvents
	}

	cutoff := time.Now().Add(-time.Duration(hoursBack) * time.Hour)
	var filtered []kubernetes.EventInfo
	for _, e := range events {
		if e.LastTimestamp.After(cutoff) {
			if strings.ToLower(eventType) == "all" || strings.EqualFold(e.Type, eventType) {
				filtered = append(filtered, e)
			}
		}
	}

	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].LastTimestamp.After(filtered[j].LastTimestamp)
	})

	if int(limit) > 0 && len(filtered) > int(limit) {
		filtered = filtered[:int(limit)]
	}

	return map[string]interface{}{
		"summary":      t.analyzeEvents(filtered),
		"events":       t.formatEvents(filtered, 20),
		"total_found":  len(filtered),
		"filter_range": hoursBack,
	}, nil
}

func (t *ClusterEventsTool) analyzeEvents(events []kubernetes.EventInfo) map[string]interface{} {
	if len(events) == 0 {
		return map[string]interface{}{"status": "no events found"}
	}

	reasons := make(map[string]int)
	warnings := 0
	for _, e := range events {
		reasons[e.Reason]++
		if e.Type == "Warning" {
			warnings++
		}
	}

	patterns := t.findPatterns(events)

	return map[string]interface{}{
		"total":           len(events),
		"warnings":        warnings,
		"top_reasons":     reasons,
		"patterns":        patterns,
		"recommendations": t.generateRecommendations(patterns, warnings),
		"frequency":       t.analyzeFrequency(events),
	}
}

func (t *ClusterEventsTool) findPatterns(events []kubernetes.EventInfo) []map[string]interface{} {
	patterns := []map[string]interface{}{}
	groups := make(map[string][]string)

	for _, e := range events {
		groups[e.Reason] = append(groups[e.Reason], fmt.Sprintf("%s/%s", e.ObjectKind, e.ObjectName))
	}

	for reason, objects := range groups {
		if len(objects) >= 3 {
			patterns = append(patterns, map[string]interface{}{
				"reason": reason,
				"count":  len(objects),
				"type":   "frequent_event",
			})
		}
	}
	return patterns
}

func (t *ClusterEventsTool) generateRecommendations(patterns []map[string]interface{}, warnings int) []string {
	recs := []string{}
	if warnings > 0 {
		recs = append(recs, "Review individual Warning events for critical failures.")
	}
	for _, p := range patterns {
		recs = append(recs, fmt.Sprintf("Investigate repeated '%s' events.", p["reason"]))
	}
	return recs
}

func (t *ClusterEventsTool) analyzeFrequency(events []kubernetes.EventInfo) string {
	if len(events) < 10 {
		return "stable"
	}
	mid := len(events) / 2
	if events[0].LastTimestamp.Sub(events[mid].LastTimestamp) < events[mid].LastTimestamp.Sub(events[len(events)-1].LastTimestamp) {
		return "increasing"
	}
	return "stable"
}

func (t *ClusterEventsTool) formatEvents(events []kubernetes.EventInfo, max int) []map[string]interface{} {
	count := len(events)
	if count > max {
		count = max
	}

	out := make([]map[string]interface{}, count)
	for i := 0; i < count; i++ {
		e := events[i]
		out[i] = map[string]interface{}{
			"type":      e.Type,
			"reason":    e.Reason,
			"object":    fmt.Sprintf("%s/%s", e.ObjectKind, e.ObjectName),
			"message":   e.Message,
			"timestamp": e.LastTimestamp.Format(time.RFC3339),
			"count":     e.Count,
		}
	}
	return out
}
