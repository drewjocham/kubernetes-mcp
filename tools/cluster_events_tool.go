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
		t.logger.Error("Error getting cluster events", "error", err)
		return nil, ErrFailedToGetClusterEvents
	}

	cutoffTime := time.Now().Add(-time.Duration(hoursBack) * time.Hour)
	filteredEvents := []kubernetes.EventInfo{}
	for _, event := range events {
		if event.LastTimestamp.After(cutoffTime) {
			filteredEvents = append(filteredEvents, event)
		}
	}

	if strings.ToLower(eventType) != "all" {
		typeFilteredEvents := []kubernetes.EventInfo{}
		for _, event := range filteredEvents {
			if strings.EqualFold(event.Type, eventType) {
				typeFilteredEvents = append(typeFilteredEvents, event)
			}
		}
		filteredEvents = typeFilteredEvents
	}

	sort.Slice(filteredEvents, func(i, j int) bool {
		return filteredEvents[i].LastTimestamp.After(filteredEvents[j].LastTimestamp)
	})

	if int(limit) > 0 && len(filteredEvents) > int(limit) {
		filteredEvents = filteredEvents[:int(limit)]
	}

	analysis := t.analyzeEvents(filteredEvents)

	return map[string]interface{}{
		"namespace":         namespace,
		"total_events":      len(filteredEvents),
		"event_type_filter": eventType,
		"hours_back":        hoursBack,
		"analysis":          analysis,
		"events":            t.formatEventsForDisplay(filteredEvents, 20), // First 20 for display
		"full_event_count":  len(filteredEvents),
		"time_range": map[string]interface{}{
			"from": cutoffTime.Format(time.RFC3339),
			"to":   time.Now().Format(time.RFC3339),
		},
	}, nil
}

func (t *ClusterEventsTool) analyzeEvents(events []kubernetes.EventInfo) map[string]interface{} {
	if len(events) == 0 {
		return map[string]interface{}{
			"summary":         "No events found in the specified time range",
			"warnings":        0,
			"normal":          0,
			"patterns":        []map[string]interface{}{},
			"recommendations": []string{"No events to analyze"},
		}
	}

	warningEvents := []kubernetes.EventInfo{}
	normalEvents := []kubernetes.EventInfo{}

	for _, event := range events {
		if event.Type == "Warning" {
			warningEvents = append(warningEvents, event)
		} else if event.Type == "Normal" {
			normalEvents = append(normalEvents, event)
		}
	}

	patterns := t.findEventPatterns(events)
	recommendations := t.generateEventRecommendations(warningEvents, patterns)

	reasonCounts := make(map[string]int)
	for _, event := range events {
		reasonCounts[event.Reason]++
	}

	type reasonCount struct {
		Reason string `json:"reason"`
		Count  int    `json:"count"`
	}

	var topReasons []reasonCount
	for reason, count := range reasonCounts {
		topReasons = append(topReasons, reasonCount{Reason: reason, Count: count})
	}

	sort.Slice(topReasons, func(i, j int) bool {
		return topReasons[i].Count > topReasons[j].Count
	})

	if len(topReasons) > 5 {
		topReasons = topReasons[:5]
	}

	namespaceSet := make(map[string]bool)
	for _, event := range events {
		if event.Namespace != "" {
			namespaceSet[event.Namespace] = true
		}
	}

	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}

	return map[string]interface{}{
		"summary":                  fmt.Sprintf("Found %d events (%d warnings, %d normal)", len(events), len(warningEvents), len(normalEvents)),
		"warnings":                 len(warningEvents),
		"normal":                   len(normalEvents),
		"patterns":                 patterns,
		"recommendations":          recommendations,
		"top_event_reasons":        topReasons,
		"namespaces_with_events":   namespaces,
		"event_frequency_analysis": t.analyzeEventFrequency(events),
	}
}

func (t *ClusterEventsTool) findEventPatterns(events []kubernetes.EventInfo) []map[string]interface{} {
	patterns := []map[string]interface{}{}
	reasonGroups := make(map[string][]kubernetes.EventInfo)
	for _, event := range events {
		reasonGroups[event.Reason] = append(reasonGroups[event.Reason], event)
	}

	for reason, reasonEvents := range reasonGroups {
		if len(reasonEvents) >= 3 { // Pattern if 3+ occurrences
			objects := make(map[string]bool)
			for _, event := range reasonEvents {
				objects[fmt.Sprintf("%s/%s", event.ObjectKind, event.ObjectName)] = true
			}

			objectList := make([]string, 0, len(objects))
			for obj := range objects {
				objectList = append(objectList, obj)
			}

			severity := "medium"
			hasWarnings := false
			for _, event := range reasonEvents {
				if event.Type == "Warning" {
					hasWarnings = true
					break
				}
			}
			if hasWarnings {
				severity = "high"
			}

			patterns = append(patterns, map[string]interface{}{
				"type":             "frequent_event",
				"reason":           reason,
				"count":            len(reasonEvents),
				"description":      fmt.Sprintf("Multiple '%s' events detected", reason),
				"severity":         severity,
				"objects_affected": objectList,
			})
		}
	}

	restartEvents := []kubernetes.EventInfo{}
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Reason), "restart") ||
			strings.Contains(strings.ToLower(event.Reason), "kill") ||
			strings.Contains(strings.ToLower(event.Reason), "backoff") {
			restartEvents = append(restartEvents, event)
		}
	}

	if len(restartEvents) >= 2 {
		objects := make(map[string]bool)
		for _, event := range restartEvents {
			objects[fmt.Sprintf("%s/%s", event.ObjectKind, event.ObjectName)] = true
		}

		objectList := make([]string, 0, len(objects))
		for obj := range objects {
			objectList = append(objectList, obj)
		}

		patterns = append(patterns, map[string]interface{}{
			"type":             "restart_pattern",
			"count":            len(restartEvents),
			"description":      "Multiple pod restart events detected",
			"severity":         "high",
			"objects_affected": objectList,
		})
	}

	schedulingEvents := []kubernetes.EventInfo{}
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Reason), "schedul") ||
			strings.Contains(strings.ToLower(event.Message), "pending") ||
			strings.Contains(strings.ToLower(event.Reason), "failedscheduling") {
			schedulingEvents = append(schedulingEvents, event)
		}
	}

	if len(schedulingEvents) > 0 {
		objects := make(map[string]bool)
		for _, event := range schedulingEvents {
			objects[fmt.Sprintf("%s/%s", event.ObjectKind, event.ObjectName)] = true
		}

		objectList := make([]string, 0, len(objects))
		for obj := range objects {
			objectList = append(objectList, obj)
		}

		patterns = append(patterns, map[string]interface{}{
			"type":             "scheduling_issues",
			"count":            len(schedulingEvents),
			"description":      "Pod scheduling issues detected",
			"severity":         "medium",
			"objects_affected": objectList,
		})
	}

	return patterns
}

func (t *ClusterEventsTool) generateEventRecommendations(warningEvents []kubernetes.EventInfo, patterns []map[string]interface{}) []string {
	recommendations := []string{}

	if len(warningEvents) == 0 && len(patterns) == 0 {
		recommendations = append(recommendations, "Cluster events look healthy - no warnings or concerning patterns detected.")
		return recommendations
	}

	if len(warningEvents) > 0 {
		recommendations = append(recommendations, fmt.Sprintf("Found %d warning events. Review these for potential issues.", len(warningEvents)))
	}

	for _, pattern := range patterns {
		patternType := pattern["type"].(string)
		switch patternType {
		case "frequent_event":
			reason := pattern["reason"].(string)
			count := pattern["count"].(int)
			objects := pattern["objects_affected"].([]string)
			objectsStr := strings.Join(objects[:min(3, len(objects))], ", ")
			if len(objects) > 3 {
				objectsStr += "..."
			}
			recommendations = append(recommendations,
				fmt.Sprintf("Frequent '%s' events detected (%d occurrences). Investigate affected objects: %s", reason, count, objectsStr))

		case "restart_pattern":
			objects := pattern["objects_affected"].([]string)
			objectsStr := strings.Join(objects[:min(3, len(objects))], ", ")
			if len(objects) > 3 {
				objectsStr += "..."
			}
			recommendations = append(recommendations,
				fmt.Sprintf("Multiple pod restarts detected. Check application health and resource limits. Affected: %s", objectsStr))

		case "scheduling_issues":
			recommendations = append(recommendations,
				"Scheduling issues detected. Check node resources and pod resource requests. Consider scaling nodes or adjusting resource requirements.")
		}
	}

	reasonRecommendations := map[string]string{
		"FailedScheduling": "Check node resources and pod resource requests",
		"OOMKilling":       "Increase memory limits for affected pods",
		"BackOff":          "Check pod logs and container startup configuration",
		"Unhealthy":        "Review health check configurations",
		"FailedMount":      "Check persistent volume and storage class configurations",
		"Killing":          "Review pod termination and resource constraints",
		"Failed":           "Investigate pod startup failures and resource availability",
		"Pulled":           "Image pull issues - check image availability and registry access",
	}

	warningReasons := make(map[string]bool)
	for _, event := range warningEvents {
		warningReasons[event.Reason] = true
	}

	for reason := range warningReasons {
		if rec, exists := reasonRecommendations[reason]; exists {
			recommendations = append(recommendations, fmt.Sprintf("%s events: %s", reason, rec))
		}
	}

	return recommendations
}

func (t *ClusterEventsTool) analyzeEventFrequency(events []kubernetes.EventInfo) map[string]interface{} {
	if len(events) == 0 {
		return map[string]interface{}{
			"trend": "stable",
			"note":  "No events to analyze",
		}
	}

	hourlyCount := make(map[string]int)
	for _, event := range events {
		hour := event.LastTimestamp.Truncate(time.Hour).Format("2006-01-02T15:00:00Z")
		hourlyCount[hour]++
	}

	var trend string
	if len(hourlyCount) > 1 {
		var hours []string
		for hour := range hourlyCount {
			hours = append(hours, hour)
		}
		sort.Strings(hours)

		if len(hours) >= 2 {
			firstHalf := hours[:len(hours)/2]
			secondHalf := hours[len(hours)/2:]

			firstHalfTotal := 0
			for _, hour := range firstHalf {
				firstHalfTotal += hourlyCount[hour]
			}

			secondHalfTotal := 0
			for _, hour := range secondHalf {
				secondHalfTotal += hourlyCount[hour]
			}

			if float64(secondHalfTotal) > float64(firstHalfTotal)*1.2 {
				trend = "increasing"
			} else if float64(secondHalfTotal) < float64(firstHalfTotal)*0.8 {
				trend = "decreasing"
			} else {
				trend = "stable"
			}
		} else {
			trend = "stable"
		}
	} else {
		trend = "stable"
	}

	return map[string]interface{}{
		"trend":            trend,
		"total_events":     len(events),
		"unique_hours":     len(hourlyCount),
		"average_per_hour": float64(len(events)) / float64(max(1, len(hourlyCount))),
	}
}

func (t *ClusterEventsTool) formatEventsForDisplay(events []kubernetes.EventInfo, maxEvents int) []map[string]interface{} {
	displayEvents := events
	if len(events) > maxEvents {
		displayEvents = events[:maxEvents]
	}

	formattedEvents := make([]map[string]interface{}, len(displayEvents))
	for i, event := range displayEvents {
		formattedEvents[i] = map[string]interface{}{
			"type":            event.Type,
			"reason":          event.Reason,
			"object":          fmt.Sprintf("%s/%s", event.ObjectKind, event.ObjectName),
			"message":         event.Message,
			"first_timestamp": event.FirstTimestamp.Format(time.RFC3339),
			"last_timestamp":  event.LastTimestamp.Format(time.RFC3339),
			"count":           event.Count,
			"namespace":       event.Namespace,
			"age":             time.Since(event.LastTimestamp).String(),
		}
	}

	return formattedEvents
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
