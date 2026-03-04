package tools

import (
	"context"
	"fmt"

	"kube-watcher/pkg/kube"
)

type ClusterAnalysisTool struct {
	BaseTool
}

func NewClusterAnalysisTool(k8sManager kube.ClientInterface) *ClusterAnalysisTool {
	return &ClusterAnalysisTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *ClusterAnalysisTool) Name() string                { return "analyze_cluster_health" }
func (t *ClusterAnalysisTool) Description() string         { return "Perform a high-level health assessment" }
func (t *ClusterAnalysisTool) Parameters() []ToolParameter { return []ToolParameter{} }

func (t *ClusterAnalysisTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	info, err := t.K8sManager.GetClusterInfo(ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := t.K8sManager.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	summary := t.evaluateNodes(nodes)

	return map[string]any{
		"cluster_info": info,
		"health":       summary,
	}, nil
}

func (t *ClusterAnalysisTool) evaluateNodes(nodes []kube.NodeInfo) map[string]any {
	var issues []string
	readyCount := 0

	for _, n := range nodes {
		if n.Status == "Ready" {
			readyCount++
			continue
		}
		issues = append(issues, fmt.Sprintf("Node %s is %s", n.Name, n.Status))
	}

	status := "healthy"
	if readyCount < len(nodes) {
		status = "degraded"
	}

	return map[string]any{
		"status":      status,
		"ready_nodes": fmt.Sprintf("%d/%d", readyCount, len(nodes)),
		"issues":      issues,
	}
}
