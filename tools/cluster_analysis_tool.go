package tools

import (
	"context"
	"fmt"
	"kube-watcher/kubernetes"
)

type ClusterAnalysisTool struct {
	BaseTool
}

func NewClusterAnalysisTool(k8sManager kubernetes.ClientInterface) *ClusterAnalysisTool {
	return &ClusterAnalysisTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *ClusterAnalysisTool) Name() string {
	return "analyze_cluster_health"
}

func (t *ClusterAnalysisTool) Description() string {
	return "Perform a high-level health assessment of the cluster, including nodes and system pods."
}

func (t *ClusterAnalysisTool) Parameters() []ToolParameter {
	return []ToolParameter{}
}

func (t *ClusterAnalysisTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	info, err := t.K8sManager.GetClusterInfo(ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := t.K8sManager.GetNodes(ctx)
	if err != nil {
		return nil, err
	}

	var nodeIssues []string
	readyNodes := 0
	for _, node := range nodes {
		if node.Status == "Ready" {
			readyNodes++
		} else {
			nodeIssues = append(nodeIssues, fmt.Sprintf("Node %s is %s", node.Name, node.Status))
		}
	}

	status := "healthy"
	if readyNodes < len(nodes) || len(nodeIssues) > 0 {
		status = "degraded"
	}

	return map[string]interface{}{
		"cluster_info": info,
		"health": map[string]interface{}{
			"status":      status,
			"ready_nodes": fmt.Sprintf("%d/%d", readyNodes, len(nodes)),
			"issues":      nodeIssues,
		},
	}, nil
}
