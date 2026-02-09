package tools

import (
	"context"
	"fmt"
	"kube-watcher/kubernetes"
	"strings"
)

type PodResourcesTool struct {
	BaseTool
}

func NewPodResourcesTool(k8sManager kubernetes.ClientInterface) *PodResourcesTool {
	return &PodResourcesTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *PodResourcesTool) Name() string {
	return "get_pod_resources"
}

func (t *PodResourcesTool) Description() string {
	return "Get resource usage and status information for pods across the cluster."
}

func (t *PodResourcesTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{
			Name:        "namespace",
			Type:        "string",
			Description: "Filter pods by namespace. Use 'all' for all namespaces or leave empty for default behavior.",
			Required:    false,
		},
		{
			Name:        "status_filter",
			Type:        "string",
			Description: "Filter pods by status (Running, Pending, Failed, Succeeded, Unknown). Leave empty for all statuses.",
			Required:    false,
		},
		{
			Name:        "high_restart_threshold",
			Type:        "number",
			Description: "Threshold for high restart count warnings (default: 5).",
			Required:    false,
			Default:     5,
		},
		{
			Name:        "include_containers",
			Type:        "boolean",
			Description: "Include detailed container information for each pod.",
			Required:    false,
			Default:     true,
		},
		{
			Name:        "problematic_only",
			Type:        "boolean",
			Description: "Only return pods with issues (not running, high restarts, etc).",
			Required:    false,
			Default:     false,
		},
	}
}

func (t *PodResourcesTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	namespace := t.GetStringArg(args, "namespace", "")
	statusFilter := t.GetStringArg(args, "status_filter", "")
	highRestartThreshold := t.GetIntArg(args, "high_restart_threshold", 5)
	includeContainers := t.GetBoolArg(args, "include_containers", true)
	problematicOnly := t.GetBoolArg(args, "problematic_only", false)

	var pods []kubernetes.PodInfo
	var err error

	if namespace == "" || strings.ToLower(namespace) == "all" {
		pods, err = t.K8sManager.GetPodsAllNamespaces(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve pods from all namespaces: %w", err)
		}
	} else {
		pods, err = t.K8sManager.GetPods(ctx, namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve pods from namespace %s: %w", namespace, err)
		}
	}

	// Initialize results
	results := map[string]interface{}{
		"total_pods":        len(pods),
		"running_pods":      0,
		"pending_pods":      0,
		"failed_pods":       0,
		"succeeded_pods":    0,
		"unknown_pods":      0,
		"high_restart_pods": []string{},
		"problematic_pods":  []string{},
		"namespaces":        make(map[string]int),
		"pods":              []map[string]interface{}{},
	}

	for _, pod := range pods {
		if statusFilter != "" && !strings.EqualFold(string(pod.Phase), statusFilter) {
			continue
		}

		switch pod.Phase {
		case "Running":
			results["running_pods"] = results["running_pods"].(int) + 1
		case "Pending":
			results["pending_pods"] = results["pending_pods"].(int) + 1
		case "Failed":
			results["failed_pods"] = results["failed_pods"].(int) + 1
		case "Succeeded":
			results["succeeded_pods"] = results["succeeded_pods"].(int) + 1
		default:
			results["unknown_pods"] = results["unknown_pods"].(int) + 1
		}

		namespaces := results["namespaces"].(map[string]int)
		namespaces[pod.Namespace]++
		results["namespaces"] = namespaces

		isHighRestart := pod.RestartCount >= highRestartThreshold
		if isHighRestart {
			results["high_restart_pods"] = append(
				results["high_restart_pods"].([]string),
				fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
			)
		}

		isProblematic := t.isPodProblematic(pod, highRestartThreshold)
		if isProblematic {
			results["problematic_pods"] = append(
				results["problematic_pods"].([]string),
				fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
			)
		}

		if problematicOnly && !isProblematic {
			continue
		}

		podDetail := map[string]interface{}{
			"name":           pod.Name,
			"namespace":      pod.Namespace,
			"status":         pod.Status,
			"phase":          string(pod.Phase),
			"node_name":      pod.NodeName,
			"age":            pod.Age.String(),
			"restart_count":  pod.RestartCount,
			"is_problematic": isProblematic,
			"issues":         t.analyzePodIssues(pod, highRestartThreshold),
		}

		if includeContainers {
			podDetail["containers"] = t.analyzeContainers(pod.Containers)
			podDetail["container_count"] = len(pod.Containers)
		}

		resourceInfo := t.extractResourceInfo(pod)
		if len(resourceInfo) > 0 {
			podDetail["resources"] = resourceInfo
		}

		podDetail["labels"] = pod.Labels

		results["pods"] = append(results["pods"].([]map[string]interface{}), podDetail)
	}

	results["total_pods"] = len(results["pods"].([]map[string]interface{}))
	results["cluster_summary"] = t.generateClusterSummary(results)

	return results, nil
}

// isPodProblematic determines if a pod has issues that need attention
func (t *PodResourcesTool) isPodProblematic(pod kubernetes.PodInfo, restartThreshold int) bool {
	// High restart count
	if pod.RestartCount >= restartThreshold {
		return true
	}

	if pod.Phase != "Running" && pod.Phase != "Succeeded" {
		return true
	}

	for _, container := range pod.Containers {
		if !container.Ready && pod.Phase == "Running" {
			return true
		}
		if container.State == "Waiting" || container.State == "Terminated" {
			return true
		}
	}

	return false
}

func (t *PodResourcesTool) analyzePodIssues(pod kubernetes.PodInfo, restartThreshold int) []string {
	var issues []string

	if pod.RestartCount >= restartThreshold {
		issues = append(issues, fmt.Sprintf("High restart count: %d", pod.RestartCount))
	}

	if pod.Phase == "Pending" {
		issues = append(issues, "Pod is stuck in Pending state")
	} else if pod.Phase == "Failed" {
		issues = append(issues, "Pod has failed")
	} else if pod.Phase == "Unknown" {
		issues = append(issues, "Pod status is unknown")
	}

	// Check container issues
	for _, container := range pod.Containers {
		if !container.Ready && pod.Phase == "Running" {
			issues = append(issues, fmt.Sprintf("Container %s is not ready", container.Name))
		}
		if container.RestartCount > 0 {
			issues = append(issues, fmt.Sprintf("Container %s has restarted %d times",
				container.Name, container.RestartCount))
		}
		if container.State == "Waiting" {
			issues = append(issues, fmt.Sprintf("Container %s is waiting", container.Name))
		} else if container.State == "Terminated" {
			issues = append(issues, fmt.Sprintf("Container %s is terminated", container.Name))
		}
	}

	if pod.NodeName == "" && pod.Phase != "Succeeded" {
		issues = append(issues, "Pod is not scheduled to any node")
	}

	return issues
}

func (t *PodResourcesTool) analyzeContainers(containers []kubernetes.ContainerInfo) []map[string]interface{} {
	var containerDetails []map[string]interface{}

	for _, container := range containers {
		detail := map[string]interface{}{
			"name":          container.Name,
			"image":         container.Image,
			"ready":         container.Ready,
			"restart_count": container.RestartCount,
			"state":         container.State,
		}

		// Add health assessment
		var health string
		if container.Ready && container.State == "Running" {
			health = "healthy"
		} else if container.State == "Waiting" {
			health = "waiting"
		} else if container.State == "Terminated" {
			health = "terminated"
		} else {
			health = "unhealthy"
		}
		detail["health"] = health

		containerDetails = append(containerDetails, detail)
	}

	return containerDetails
}

// extractResourceInfo extracts resource information from pod metadata
func (t *PodResourcesTool) extractResourceInfo(pod kubernetes.PodInfo) map[string]interface{} {
	// In a real implementation, you would parse resource requests/limits
	// from the pod spec. For now, we'll extract any resource-related annotations
	resourceInfo := make(map[string]interface{})

	// Look for resource-related annotations
	for key, value := range pod.Annotations {
		if strings.Contains(strings.ToLower(key), "resource") ||
			strings.Contains(strings.ToLower(key), "cpu") ||
			strings.Contains(strings.ToLower(key), "memory") {
			resourceInfo[key] = value
		}
	}

	// Note about limitations
	if len(resourceInfo) == 0 {
		resourceInfo["note"] = "Resource requests/limits require access to pod specifications"
	}

	return resourceInfo
}

// generateClusterSummary creates an overall cluster pod health summary
func (t *PodResourcesTool) generateClusterSummary(results map[string]interface{}) map[string]interface{} {
	totalPods := results["total_pods"].(int)
	runningPods := results["running_pods"].(int)
	problematicPods := results["problematic_pods"].([]string)
	highRestartPods := results["high_restart_pods"].([]string)

	healthPercentage := 0.0
	if totalPods > 0 {
		healthPercentage = float64(runningPods) / float64(totalPods) * 100
	}

	var status string
	var recommendations []string

	switch {
	case healthPercentage >= 95 && len(problematicPods) == 0:
		status = "healthy"
	case healthPercentage >= 80 && len(problematicPods) <= 2:
		status = "warning"
		recommendations = append(recommendations, "Monitor pods with issues closely")
	default:
		status = "critical"
		recommendations = append(recommendations, "Multiple pods require immediate attention")
	}

	if len(highRestartPods) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Investigate %d pod(s) with high restart counts", len(highRestartPods)))
	}

	if len(problematicPods) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Address issues with %d problematic pod(s)", len(problematicPods)))
	}

	// Namespace distribution
	namespaces := results["namespaces"].(map[string]int)
	namespaceSummary := make([]string, 0, len(namespaces))
	for ns, count := range namespaces {
		namespaceSummary = append(namespaceSummary, fmt.Sprintf("%s: %d", ns, count))
	}

	return map[string]interface{}{
		"status":                 status,
		"health_percentage":      healthPercentage,
		"recommendations":        recommendations,
		"namespace_distribution": namespaceSummary,
		"summary": fmt.Sprintf("%d pods total, %d running, %d problematic",
			totalPods, runningPods, len(problematicPods)),
	}
}
