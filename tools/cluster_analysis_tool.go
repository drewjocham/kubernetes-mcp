package tools

import (
	"context"
	"fmt"
	"kube-watcher/kubernetes"
	"time"
)

type ClusterAnalysisTool struct {
	BaseTool
}

// NewClusterAnalysisTool is the factory function for the ClusterAnalysisTool.
func NewClusterAnalysisTool(k8sManager kubernetes.ClientInterface) *ClusterAnalysisTool {
	return &ClusterAnalysisTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

// Name implements the Tool interface.
func (t *ClusterAnalysisTool) Name() string {
	return "analyze_cluster"
}

// Description implements the Tool interface.
func (t *ClusterAnalysisTool) Description() string {
	return "Perform comprehensive cluster analysis including nodes, pods, services, events, and overall health assessment."
}

// Parameters implements the Tool interface.
func (t *ClusterAnalysisTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{
			Name:        "include_pods",
			Type:        "boolean",
			Description: "Include detailed pod analysis in the cluster report.",
			Required:    false,
			Default:     true,
		},
		{
			Name:        "include_events",
			Type:        "boolean",
			Description: "Include recent cluster events in the analysis.",
			Required:    false,
			Default:     true,
		},
		{
			Name:        "event_hours_back",
			Type:        "number",
			Description: "How many hours back to look for events (default: 24).",
			Required:    false,
			Default:     24,
		},
		{
			Name:        "include_services",
			Type:        "boolean",
			Description: "Include service analysis in the cluster report.",
			Required:    false,
			Default:     false,
		},
		{
			Name:        "detailed_analysis",
			Type:        "boolean",
			Description: "Perform deep analysis with recommendations and insights.",
			Required:    false,
			Default:     true,
		},
	}
}

// Execute implements the Tool interface and performs comprehensive cluster analysis.
func (t *ClusterAnalysisTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	// Extract arguments
	includePods := t.GetBoolArg(args, "include_pods", true)
	includeEvents := t.GetBoolArg(args, "include_events", true)
	eventHoursBack := t.GetIntArg(args, "event_hours_back", 24)
	includeServices := t.GetBoolArg(args, "include_services", false)
	detailedAnalysis := t.GetBoolArg(args, "detailed_analysis", true)

	// Initialize results structure
	results := map[string]interface{}{
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"analysis_scope": t.buildAnalysisScope(includePods, includeEvents, includeServices),
	}

	// Get cluster basic information
	clusterInfo, err := t.K8sManager.GetClusterInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster info: %w", err)
	}
	results["cluster_info"] = clusterInfo

	// Analyze nodes
	nodeAnalysis, err := t.analyzeNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze nodes: %w", err)
	}
	results["node_analysis"] = nodeAnalysis

	// Analyze pods if requested
	if includePods {
		podAnalysis, err := t.analyzePods(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze pods: %w", err)
		}
		results["pod_analysis"] = podAnalysis
	}

	// Analyze events if requested
	if includeEvents {
		eventAnalysis, err := t.analyzeEvents(ctx, eventHoursBack)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze events: %w", err)
		}
		results["event_analysis"] = eventAnalysis
	}

	// Analyze services if requested
	if includeServices {
		serviceAnalysis, err := t.analyzeServices(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze services: %w", err)
		}
		results["service_analysis"] = serviceAnalysis
	}

	// Perform detailed analysis and generate recommendations
	if detailedAnalysis {
		results["health_assessment"] = t.performHealthAssessment(results)
		results["recommendations"] = t.generateRecommendations(results)
		results["alerts"] = t.generateAlerts(results)
	}

	return results, nil
}

// buildAnalysisScope creates a summary of what was analyzed
func (t *ClusterAnalysisTool) buildAnalysisScope(includePods, includeEvents, includeServices bool) map[string]interface{} {
	scope := map[string]interface{}{
		"nodes":    true, // Always included
		"pods":     includePods,
		"events":   includeEvents,
		"services": includeServices,
	}
	return scope
}

// analyzeNodes performs comprehensive node analysis
func (t *ClusterAnalysisTool) analyzeNodes(ctx context.Context) (map[string]interface{}, error) {
	nodes, err := t.K8sManager.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	analysis := map[string]interface{}{
		"total_nodes":      len(nodes),
		"ready_nodes":      0,
		"not_ready_nodes":  0,
		"tainted_nodes":    0,
		"resource_summary": make(map[string]interface{}),
		"node_roles":       make(map[string]int),
		"node_conditions":  make(map[string]int),
		"issues":           []string{},
	}

	totalCPU := 0.0
	totalMemory := 0.0

	for _, node := range nodes {
		// Count ready/not ready
		if node.Status == "Ready" {
			analysis["ready_nodes"] = analysis["ready_nodes"].(int) + 1
		} else {
			analysis["not_ready_nodes"] = analysis["not_ready_nodes"].(int) + 1
			analysis["issues"] = append(analysis["issues"].([]string),
				fmt.Sprintf("Node %s is not ready", node.Name))
		}

		// Count tainted nodes
		if len(node.Taints) > 0 {
			analysis["tainted_nodes"] = analysis["tainted_nodes"].(int) + 1
		}

		// Analyze node roles from labels
		nodeRoles := analysis["node_roles"].(map[string]int)
		if role, exists := node.Labels["kubernetes.io/role"]; exists {
			nodeRoles[role]++
		} else if _, isMaster := node.Labels["node-role.kubernetes.io/master"]; isMaster {
			nodeRoles["master"]++
		} else if _, isControl := node.Labels["node-role.kubernetes.io/control-plane"]; isControl {
			nodeRoles["control-plane"]++
		} else {
			nodeRoles["worker"]++
		}
		analysis["node_roles"] = nodeRoles

		// Count conditions
		conditions := analysis["node_conditions"].(map[string]int)
		for _, condition := range node.Conditions {
			if condition.Status == "True" {
				conditions[condition.Type]++

				// Flag problematic conditions
				if condition.Type != "Ready" {
					analysis["issues"] = append(analysis["issues"].([]string),
						fmt.Sprintf("Node %s has condition %s: %s", node.Name, condition.Type, condition.Message))
				}
			}
		}
		analysis["node_conditions"] = conditions

		// Sum up resources (simplified - assumes CPU is in cores and memory in Ki)
		if cpuStr, exists := node.Allocatable["cpu"]; exists {
			if cpuStr != "" {
				totalCPU += 1.0 // Placeholder
			}
		}
		if memStr, exists := node.Allocatable["memory"]; exists {
			// Simple parsing - in reality you'd use proper quantity parsing
			if memStr != "" {
				totalMemory += 1.0 // Placeholder
			}
		}
	}

	analysis["resource_summary"] = map[string]interface{}{
		"total_allocatable_cpu":    fmt.Sprintf("%.1f cores (estimated)", totalCPU),
		"total_allocatable_memory": fmt.Sprintf("%.1f GB (estimated)", totalMemory),
		"note":                     "Accurate resource calculation requires quantity parsing",
	}

	return analysis, nil
}

// analyzePods performs comprehensive pod analysis
func (t *ClusterAnalysisTool) analyzePods(ctx context.Context) (map[string]interface{}, error) {
	pods, err := t.K8sManager.GetPodsAllNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	analysis := map[string]interface{}{
		"total_pods":             len(pods),
		"phase_distribution":     make(map[string]int),
		"namespace_distribution": make(map[string]int),
		"problematic_pods":       []string{},
		"high_restart_pods":      []string{},
		"resource_issues":        []string{},
		"scheduling_issues":      []string{},
	}

	for _, pod := range pods {
		// Count by phase
		phases := analysis["phase_distribution"].(map[string]int)
		phases[string(pod.Phase)]++
		analysis["phase_distribution"] = phases

		// Count by namespace
		namespaces := analysis["namespace_distribution"].(map[string]int)
		namespaces[pod.Namespace]++
		analysis["namespace_distribution"] = namespaces

		// Identify problematic pods
		if pod.Phase == "Failed" || pod.Phase == "Unknown" {
			analysis["problematic_pods"] = append(analysis["problematic_pods"].([]string),
				fmt.Sprintf("%s/%s (Phase: %s)", pod.Namespace, pod.Name, pod.Phase))
		}

		// Check for high restart counts
		if pod.RestartCount >= 5 {
			analysis["high_restart_pods"] = append(analysis["high_restart_pods"].([]string),
				fmt.Sprintf("%s/%s (Restarts: %d)", pod.Namespace, pod.Name, pod.RestartCount))
		}

		// Check for scheduling issues
		if pod.NodeName == "" && pod.Phase != "Succeeded" {
			analysis["scheduling_issues"] = append(analysis["scheduling_issues"].([]string),
				fmt.Sprintf("%s/%s is not scheduled to any node", pod.Namespace, pod.Name))
		}

		// Check container issues
		for _, container := range pod.Containers {
			if !container.Ready && pod.Phase == "Running" {
				analysis["resource_issues"] = append(analysis["resource_issues"].([]string),
					fmt.Sprintf("%s/%s container %s is not ready", pod.Namespace, pod.Name, container.Name))
			}
		}
	}

	return analysis, nil
}

// analyzeEvents analyzes cluster events for issues and patterns
func (t *ClusterAnalysisTool) analyzeEvents(ctx context.Context, hoursBack int) (map[string]interface{}, error) {
	events, err := t.K8sManager.GetEventsAllNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	cutoffTime := time.Now().Add(-time.Duration(hoursBack) * time.Hour)

	analysis := map[string]interface{}{
		"total_events":     0,
		"warning_events":   0,
		"error_events":     0,
		"event_types":      make(map[string]int),
		"event_reasons":    make(map[string]int),
		"critical_events":  []map[string]interface{}{},
		"namespace_events": make(map[string]int),
	}

	for _, event := range events {
		// Filter by time
		if event.LastTimestamp.Before(cutoffTime) {
			continue
		}

		analysis["total_events"] = analysis["total_events"].(int) + 1

		// Count by type
		eventTypes := analysis["event_types"].(map[string]int)
		eventTypes[event.Type]++
		analysis["event_types"] = eventTypes

		// Count by reason
		reasons := analysis["event_reasons"].(map[string]int)
		reasons[event.Reason]++
		analysis["event_reasons"] = reasons

		// Count by namespace
		namespaceEvents := analysis["namespace_events"].(map[string]int)
		namespaceEvents[event.Namespace]++
		analysis["namespace_events"] = namespaceEvents

		// Count warnings and errors
		if event.Type == "Warning" {
			analysis["warning_events"] = analysis["warning_events"].(int) + 1
		} else if event.Type == "Error" {
			analysis["error_events"] = analysis["error_events"].(int) + 1
		}

		// Collect critical events
		if t.isCriticalEvent(event) {
			criticalEvents := analysis["critical_events"].([]map[string]interface{})
			criticalEvents = append(criticalEvents, map[string]interface{}{
				"type":      event.Type,
				"reason":    event.Reason,
				"object":    fmt.Sprintf("%s/%s", event.ObjectKind, event.ObjectName),
				"message":   event.Message,
				"timestamp": event.LastTimestamp.Format(time.RFC3339),
				"namespace": event.Namespace,
				"count":     event.Count,
			})
			analysis["critical_events"] = criticalEvents
		}
	}

	return analysis, nil
}

// analyzeServices performs service analysis
func (t *ClusterAnalysisTool) analyzeServices(ctx context.Context) (map[string]interface{}, error) {
	services, err := t.K8sManager.GetServicesAllNamespaces(ctx)
	if err != nil {
		return nil, err
	}

	analysis := map[string]interface{}{
		"total_services":     len(services),
		"service_types":      make(map[string]int),
		"namespace_services": make(map[string]int),
		"external_services":  []string{},
		"potential_issues":   []string{},
	}

	for _, service := range services {
		// Count by type
		serviceTypes := analysis["service_types"].(map[string]int)
		serviceTypes[string(service.Type)]++
		analysis["service_types"] = serviceTypes

		// Count by namespace
		namespaceServices := analysis["namespace_services"].(map[string]int)
		namespaceServices[service.Namespace]++
		analysis["namespace_services"] = namespaceServices

		// Track external services
		if service.Type == "LoadBalancer" || service.Type == "NodePort" {
			analysis["external_services"] = append(analysis["external_services"].([]string),
				fmt.Sprintf("%s/%s (%s)", service.Namespace, service.Name, service.Type))
		}

		// Check for potential issues
		if len(service.Selector) == 0 && service.Type != "ExternalName" {
			analysis["potential_issues"] = append(analysis["potential_issues"].([]string),
				fmt.Sprintf("%s/%s has no selector", service.Namespace, service.Name))
		}
	}

	return analysis, nil
}

// isCriticalEvent determines if an event is critical and should be highlighted
func (t *ClusterAnalysisTool) isCriticalEvent(event kubernetes.EventInfo) bool {
	criticalReasons := []string{
		"Failed", "FailedScheduling", "FailedMount", "FailedAttachVolume",
		"FailedSync", "Unhealthy", "BackOff", "FailedCreatePodSandBox",
		"NetworkNotReady", "CNINotReady",
	}

	for _, reason := range criticalReasons {
		if event.Reason == reason || event.Type == "Error" {
			return true
		}
	}

	return false
}

// performHealthAssessment creates overall cluster health assessment
func (t *ClusterAnalysisTool) performHealthAssessment(results map[string]interface{}) map[string]interface{} {
	score := 100
	issues := []string{}

	// Check node health
	if nodeAnalysis, exists := results["node_analysis"]; exists {
		nodeData := nodeAnalysis.(map[string]interface{})
		notReadyNodes := nodeData["not_ready_nodes"].(int)
		totalNodes := nodeData["total_nodes"].(int)

		if notReadyNodes > 0 {
			score -= (notReadyNodes * 20)
			issues = append(issues, fmt.Sprintf("%d nodes not ready", notReadyNodes))
		}

		if taintedNodes := nodeData["tainted_nodes"].(int); taintedNodes > totalNodes/2 {
			score -= 10
			issues = append(issues, "High number of tainted nodes")
		}
	}

	// Check pod health
	if podAnalysis, exists := results["pod_analysis"]; exists {
		podData := podAnalysis.(map[string]interface{})
		problematicPods := len(podData["problematic_pods"].([]string))
		highRestartPods := len(podData["high_restart_pods"].([]string))

		score -= (problematicPods * 5)
		score -= (highRestartPods * 3)

		if problematicPods > 0 {
			issues = append(issues, fmt.Sprintf("%d pods in failed/unknown state", problematicPods))
		}
		if highRestartPods > 0 {
			issues = append(issues, fmt.Sprintf("%d pods with high restart counts", highRestartPods))
		}
	}

	// Check events
	if eventAnalysis, exists := results["event_analysis"]; exists {
		eventData := eventAnalysis.(map[string]interface{})
		criticalEvents := len(eventData["critical_events"].([]map[string]interface{}))

		score -= (criticalEvents * 2)

		if criticalEvents > 10 {
			issues = append(issues, fmt.Sprintf("%d critical events recently", criticalEvents))
		}
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	var status string
	switch {
	case score >= 90:
		status = "healthy"
	case score >= 70:
		status = "warning"
	case score >= 50:
		status = "degraded"
	default:
		status = "critical"
	}

	return map[string]interface{}{
		"score":   score,
		"status":  status,
		"issues":  issues,
		"summary": fmt.Sprintf("Cluster health score: %d/100 (%s)", score, status),
	}
}

// generateRecommendations provides actionable recommendations
func (t *ClusterAnalysisTool) generateRecommendations(results map[string]interface{}) []string {
	recommendations := []string{}

	// Node recommendations
	if nodeAnalysis, exists := results["node_analysis"]; exists {
		nodeData := nodeAnalysis.(map[string]interface{})
		if nodeData["not_ready_nodes"].(int) > 0 {
			recommendations = append(recommendations,
				"Investigate and resolve issues with not-ready nodes")
		}
		if len(nodeData["issues"].([]string)) > 0 {
			recommendations = append(recommendations,
				"Review node conditions and address reported issues")
		}
	}

	// Pod recommendations
	if podAnalysis, exists := results["pod_analysis"]; exists {
		podData := podAnalysis.(map[string]interface{})
		if len(podData["problematic_pods"].([]string)) > 0 {
			recommendations = append(recommendations,
				"Investigate failed and unknown pods, check logs and resource requests")
		}
		if len(podData["high_restart_pods"].([]string)) > 0 {
			recommendations = append(recommendations,
				"Analyze pod logs for high-restart pods to identify root causes")
		}
		if len(podData["scheduling_issues"].([]string)) > 0 {
			recommendations = append(recommendations,
				"Review resource requests and node capacity for unscheduled pods")
		}
	}

	// Event recommendations
	if eventAnalysis, exists := results["event_analysis"]; exists {
		eventData := eventAnalysis.(map[string]interface{})
		if len(eventData["critical_events"].([]map[string]interface{})) > 5 {
			recommendations = append(recommendations,
				"Review critical events and address recurring issues")
		}
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"Cluster appears healthy - continue regular monitoring")
	}

	return recommendations
}

// generateAlerts creates alert-worthy items
func (t *ClusterAnalysisTool) generateAlerts(results map[string]interface{}) []map[string]interface{} {
	alerts := []map[string]interface{}{}

	// Check for critical issues that need immediate attention
	if healthAssessment, exists := results["health_assessment"]; exists {
		healthData := healthAssessment.(map[string]interface{})
		if healthData["status"].(string) == "critical" {
			alerts = append(alerts, map[string]interface{}{
				"severity": "critical",
				"title":    "Cluster Health Critical",
				"message":  "Cluster health score is below 50 - immediate attention required",
			})
		}
	}

	if nodeAnalysis, exists := results["node_analysis"]; exists {
		nodeData := nodeAnalysis.(map[string]interface{})
		if nodeData["not_ready_nodes"].(int) > 0 {
			alerts = append(alerts, map[string]interface{}{
				"severity": "high",
				"title":    "Nodes Not Ready",
				"message":  fmt.Sprintf("%d nodes are not ready", nodeData["not_ready_nodes"].(int)),
			})
		}
	}

	return alerts
}
