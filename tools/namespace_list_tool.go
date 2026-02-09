package tools

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"kube-watcher/kubernetes"
)

var (
	ErrFailedToGetNamespaces = errors.New("failed to get namespaces")
)

type NamespaceListTool struct {
	k8sManager kubernetes.ClientInterface
	logger     *slog.Logger
}

func NewNamespaceListTool(k8sManager kubernetes.ClientInterface, logger *slog.Logger) *NamespaceListTool {
	return &NamespaceListTool{
		k8sManager: k8sManager,
		logger:     logger,
	}
}

func (t *NamespaceListTool) Name() string {
	return "list_namespaces"
}

func (t *NamespaceListTool) Description() string {
	return "Get information about all namespaces including resource usage and pod counts"
}

func (t *NamespaceListTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "include_system", Type: "boolean", Description: "Include system namespaces (kube-system, etc.) in the results"},
		{Name: "include_quotas", Type: "boolean", Description: "Include resource quota information for each namespace"},
	}
}

func (t *NamespaceListTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	includeSystem, _ := args["include_system"].(bool)
	includeQuotas, _ := args["include_quotas"].(bool)

	namespaceNames, err := t.k8sManager.GetNamespaces(ctx)
	if err != nil {
		t.logger.Error("Error getting namespaces", "error", err)
		return nil, ErrFailedToGetNamespaces
	}

	results := map[string]interface{}{
		"total_namespaces":  len(namespaceNames),
		"system_namespaces": 0,
		"user_namespaces":   0,
		"total_pods":        0,
		"total_quotas":      0,
		"namespaces":        []map[string]interface{}{},
		"system_ns_list":    []string{},
		"user_ns_list":      []string{},
	}

	for _, nsName := range namespaceNames {
		isSystem := t.isSystemNamespace(nsName)

		if isSystem && !includeSystem {
			continue
		}

		pods, err := t.k8sManager.GetPods(ctx, nsName)
		if err != nil {
			t.logger.Warn("Error getting pods for namespace", "namespace", nsName, "error", err)
			continue
		}

		var quotas []kubernetes.ResourceQuotaInfo
		if includeQuotas {
			quotas, err = t.k8sManager.GetResourceQuotas(ctx, nsName)
			if err != nil {
				t.logger.Warn("Error getting resource quotas for namespace", "namespace", nsName, "error", err)
				quotas = []kubernetes.ResourceQuotaInfo{}
			}
		}

		nsInfo := map[string]interface{}{
			"name":            nsName,
			"is_system":       isSystem,
			"pod_count":       len(pods),
			"resource_quotas": len(quotas),
			"status":          "Active",
			"age":             "N/A",
		}

		if includeQuotas && len(quotas) > 0 {
			quotaDetails := make([]map[string]interface{}, len(quotas))
			for i, quota := range quotas {
				quotaDetails[i] = map[string]interface{}{
					"name": quota.Name,
					"hard": quota.Hard,
					"used": quota.Used,
				}
			}
			nsInfo["quota_details"] = quotaDetails
		}

		podStatus := t.analyzePodStatus(pods)
		nsInfo["pod_status_breakdown"] = podStatus

		results["total_pods"] = results["total_pods"].(int) + len(pods)
		results["total_quotas"] = results["total_quotas"].(int) + len(quotas)

		if isSystem {
			results["system_namespaces"] = results["system_namespaces"].(int) + 1
			results["system_ns_list"] = append(results["system_ns_list"].([]string), nsName)
		} else {
			results["user_namespaces"] = results["user_namespaces"].(int) + 1
			results["user_ns_list"] = append(results["user_ns_list"].([]string), nsName)
		}

		results["namespaces"] = append(results["namespaces"].([]map[string]interface{}), nsInfo)
	}

	results["analysis"] = t.generateNamespaceAnalysis(results)

	return results, nil
}

func (t *NamespaceListTool) isSystemNamespace(name string) bool {
	systemPrefixes := []string{
		"kube-",
		"kubernetes-",
		"openshift-",
		"istio-",
		"cert-manager",
		"ingress-",
	}

	systemNames := []string{
		"default",
		"monitoring",
		"logging",
		"prometheus",
		"grafana",
		"calico-system",
		"tigera-operator",
		"metallb-system",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	for _, sysName := range systemNames {
		if name == sysName {
			return true
		}
	}

	return false
}

func (t *NamespaceListTool) analyzePodStatus(pods []kubernetes.PodInfo) map[string]interface{} {
	status := map[string]interface{}{
		"running":   0,
		"pending":   0,
		"succeeded": 0,
		"failed":    0,
		"unknown":   0,
		"total":     len(pods),
	}

	for _, pod := range pods {
		switch pod.Phase {
		case "Running":
			status["running"] = status["running"].(int) + 1
		case "Pending":
			status["pending"] = status["pending"].(int) + 1
		case "Succeeded":
			status["succeeded"] = status["succeeded"].(int) + 1
		case "Failed":
			status["failed"] = status["failed"].(int) + 1
		default:
			status["unknown"] = status["unknown"].(int) + 1
		}
	}

	return status
}

func (t *NamespaceListTool) generateNamespaceAnalysis(results map[string]interface{}) map[string]interface{} {
	totalNS := results["total_namespaces"].(int)
	systemNS := results["system_namespaces"].(int)
	userNS := results["user_namespaces"].(int)
	totalPods := results["total_pods"].(int)
	totalQuotas := results["total_quotas"].(int)

	analysis := map[string]interface{}{
		"namespace_distribution": map[string]interface{}{
			"system_percentage": float64(systemNS) / float64(totalNS) * 100,
			"user_percentage":   float64(userNS) / float64(totalNS) * 100,
		},
		"resource_usage": map[string]interface{}{
			"average_pods_per_namespace": float64(totalPods) / float64(totalNS),
			"quota_coverage_percentage":  float64(totalQuotas) / float64(totalNS) * 100,
		},
		"recommendations": []string{},
	}

	recommendations := []string{}

	if totalNS > 50 {
		recommendations = append(recommendations,
			"Large number of namespaces detected. Consider namespace consolidation or implementing namespace lifecycle management.")
	}

	if float64(totalQuotas)/float64(totalNS)*100 < 50 {
		recommendations = append(recommendations,
			"Less than 50% of namespaces have resource quotas. Consider implementing resource quotas for better resource governance.")
	}

	avgPodsPerNS := float64(totalPods) / float64(totalNS)
	if avgPodsPerNS > 100 {
		recommendations = append(recommendations,
			fmt.Sprintf("High pod density (%.1f pods per namespace). Monitor resource usage and consider namespace splitting if needed.", avgPodsPerNS))
	} else if avgPodsPerNS < 5 && totalNS > 10 {
		recommendations = append(recommendations,
			"Low pod density suggests possible namespace sprawl. Consider consolidating underutilized namespaces.")
	}

	if userNS == 0 {
		recommendations = append(recommendations,
			"No user namespaces detected. Consider creating dedicated namespaces for different applications or environments.")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"Namespace organization looks healthy. Continue monitoring resource usage and consider implementing namespace-based RBAC if not already in place.")
	}

	analysis["recommendations"] = recommendations

	var healthStatus string
	if len(recommendations) <= 1 {
		healthStatus = "healthy"
	} else if len(recommendations) <= 3 {
		healthStatus = "warning"
	} else {
		healthStatus = "needs_attention"
	}

	analysis["health_status"] = healthStatus

	return analysis
}
