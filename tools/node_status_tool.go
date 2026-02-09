package tools

import (
	"context"
	"fmt"
	"kube-watcher/kubernetes"
)

// NodeStatusTool embeds BaseTool and implements the Tool interface.
type NodeStatusTool struct {
	BaseTool
}

// NewNodeStatusTool is the factory function for the NodeStatusTool.
func NewNodeStatusTool(k8sManager kubernetes.ClientInterface) *NodeStatusTool {
	return &NodeStatusTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

// Name implements the Tool interface.
func (t *NodeStatusTool) Name() string {
	return "get_node_status"
}

// Description implements the Tool interface.
func (t *NodeStatusTool) Description() string {
	return "Get detailed status and health information about cluster nodes."
}

// Parameters implements the Tool interface.
func (t *NodeStatusTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{
			Name:        "include_metrics",
			Type:        "boolean",
			Description: "Include current CPU and memory utilization metrics.",
			Required:    false,
			Default:     true,
		},
		{
			Name:        "taints_only",
			Type:        "boolean",
			Description: "Only return nodes with active taints or conditions.",
			Required:    false,
			Default:     false,
		},
		{
			Name:        "node_name",
			Type:        "string",
			Description: "Get status for a specific node by name. If not specified, returns all nodes.",
			Required:    false,
		},
	}
}

// Execute implements the Tool interface and performs the analysis.
func (t *NodeStatusTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	includeMetrics := t.GetBoolArg(args, "include_metrics", true)
	taintsOnly := t.GetBoolArg(args, "taints_only", false)
	nodeName := t.GetStringArg(args, "node_name", "")

	var nodes []kubernetes.NodeInfo
	var err error

	if nodeName != "" {
		node, err := t.K8sManager.GetNode(ctx, nodeName)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve node %s: %w", nodeName, err)
		}
		nodes = []kubernetes.NodeInfo{*node}
	} else {
		nodes, err = t.K8sManager.GetNodes(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve nodes: %w", err)
		}
	}

	results := map[string]interface{}{
		"total_nodes":     len(nodes),
		"ready_count":     0,
		"not_ready_count": 0,
		"unhealthy_nodes": []string{},
		"tainted_nodes":   []string{},
		"nodes":           []map[string]interface{}{},
	}

	// Process each node
	for _, node := range nodes {
		if taintsOnly && len(node.Taints) == 0 {
			continue
		}

		// Count node statuses
		if node.Status == "Ready" {
			results["ready_count"] = results["ready_count"].(int) + 1
		} else {
			results["not_ready_count"] = results["not_ready_count"].(int) + 1
			results["unhealthy_nodes"] = append(results["unhealthy_nodes"].([]string), node.Name)
		}

		// Track tainted nodes
		if len(node.Taints) > 0 {
			results["tainted_nodes"] = append(results["tainted_nodes"].([]string), node.Name)
		}

		nodeDetail := map[string]interface{}{
			"name":        node.Name,
			"status":      node.Status,
			"age":         node.Age.String(),
			"taint_count": len(node.Taints),
			"conditions":  t.analyzeNodeConditions(node.Conditions),
		}

		if includeMetrics {
			nodeDetail["capacity"] = node.Capacity
			nodeDetail["allocatable"] = node.Allocatable
			nodeDetail["resource_pressure"] = t.calculateResourcePressure(node)
		}

		if len(node.Taints) > 0 {
			var taintDetails []map[string]interface{}
			for _, taint := range node.Taints {
				taintDetails = append(taintDetails, map[string]interface{}{
					"key":    taint.Key,
					"value":  taint.Value,
					"effect": string(taint.Effect),
				})
			}
			nodeDetail["taints"] = taintDetails
		}

		// Add labels for debugging/filtering
		nodeDetail["labels"] = node.Labels

		results["nodes"] = append(results["nodes"].([]map[string]interface{}), nodeDetail)
	}

	results["health_summary"] = t.generateHealthSummary(results)

	return results, nil
}

// analyzeNodeConditions processes node conditions to extract meaningful insights
func (t *NodeStatusTool) analyzeNodeConditions(conditions []kubernetes.NodeCondition) map[string]interface{} {
	conditionSummary := map[string]interface{}{
		"ready":               false,
		"disk_pressure":       false,
		"memory_pressure":     false,
		"pid_pressure":        false,
		"network_unavailable": false,
		"issues":              []string{},
	}

	for _, condition := range conditions {
		switch condition.Type {
		case "Ready":
			conditionSummary["ready"] = condition.Status == "True"
			if condition.Status != "True" {
				conditionSummary["issues"] = append(
					conditionSummary["issues"].([]string),
					fmt.Sprintf("Node not ready: %s", condition.Message),
				)
			}
		case "DiskPressure":
			conditionSummary["disk_pressure"] = condition.Status == "True"
			if condition.Status == "True" {
				conditionSummary["issues"] = append(
					conditionSummary["issues"].([]string),
					fmt.Sprintf("Disk pressure: %s", condition.Message),
				)
			}
		case "MemoryPressure":
			conditionSummary["memory_pressure"] = condition.Status == "True"
			if condition.Status == "True" {
				conditionSummary["issues"] = append(
					conditionSummary["issues"].([]string),
					fmt.Sprintf("Memory pressure: %s", condition.Message),
				)
			}
		case "PIDPressure":
			conditionSummary["pid_pressure"] = condition.Status == "True"
			if condition.Status == "True" {
				conditionSummary["issues"] = append(
					conditionSummary["issues"].([]string),
					fmt.Sprintf("PID pressure: %s", condition.Message),
				)
			}
		case "NetworkUnavailable":
			conditionSummary["network_unavailable"] = condition.Status == "True"
			if condition.Status == "True" {
				conditionSummary["issues"] = append(
					conditionSummary["issues"].([]string),
					fmt.Sprintf("Network unavailable: %s", condition.Message),
				)
			}
		}
	}

	return conditionSummary
}

func (t *NodeStatusTool) calculateResourcePressure(node kubernetes.NodeInfo) map[string]interface{} {
	// init
	pressure := map[string]interface{}{
		"cpu_allocatable":     node.Allocatable["cpu"],
		"memory_allocatable":  node.Allocatable["memory"],
		"storage_allocatable": node.Allocatable["ephemeral-storage"],
		"pods_allocatable":    node.Allocatable["pods"],
	}

	// calculate Pressure Flags based on Conditions
	// This maps the condition booleans into the pressure report
	conditions := t.analyzeNodeConditions(node.Conditions)
	pressure["has_memory_pressure"] = conditions["memory_pressure"]
	pressure["has_disk_pressure"] = conditions["disk_pressure"]
	pressure["has_pid_pressure"] = conditions["pid_pressure"]

	// 3. Logic for "High Load" indicators
	// Note: node.Capacity vs node.Allocatable helps identify overhead
	// In a production scenario, you would calculate: (CurrentUsage / Allocatable) * 100

	pressure["usage_summary"] = "Pending real-time metrics"

	// Example of providing a 'pressure score' if conditions are active
	pressure_score := 0
	if conditions["memory_pressure"].(bool) {
		pressure_score += 50
	}
	if conditions["disk_pressure"].(bool) {
		pressure_score += 30
	}
	if conditions["pid_pressure"].(bool) {
		pressure_score += 20
	}

	pressure["pressure_score_pct"] = pressure_score
	pressure["note"] = "Metrics integration (Prometheus/Metrics-Server) recommended for real-time utilization."

	return pressure
}
func (t *NodeStatusTool) generateHealthSummary(results map[string]interface{}) map[string]interface{} {
	totalNodes := results["total_nodes"].(int)
	readyCount := results["ready_count"].(int)
	unhealthyNodes := results["unhealthy_nodes"].([]string)
	taintedNodes := results["tainted_nodes"].([]string)

	healthPercentage := 0.0
	if totalNodes > 0 {
		healthPercentage = float64(readyCount) / float64(totalNodes) * 100
	}

	var status string
	var recommendations []string

	switch {
	case healthPercentage >= 95:
		status = "healthy"
	case healthPercentage >= 80:
		status = "warning"
		recommendations = append(recommendations, "Monitor unhealthy nodes closely")
	default:
		status = "critical"
		recommendations = append(recommendations, "Immediate attention required for cluster stability")
	}

	if len(taintedNodes) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Review taints on %d node(s) - may affect scheduling", len(taintedNodes)))
	}

	if len(unhealthyNodes) > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Investigate and resolve issues with: %v", unhealthyNodes))
	}

	return map[string]interface{}{
		"status":            status,
		"health_percentage": healthPercentage,
		"recommendations":   recommendations,
		"summary": fmt.Sprintf("%d/%d nodes ready (%d tainted)",
			readyCount, totalNodes, len(taintedNodes)),
	}
}
