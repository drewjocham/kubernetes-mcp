package tools

import (
	"context"
	"fmt"
	"kube-watcher/kubernetes"
	"strings"
	"time"
)

type ClusterAnalysisTool struct {
	BaseTool
}

type AnalysisReport struct {
	Nodes    map[string]any `json:"node_analysis"`
	Pods     map[string]any `json:"pod_analysis,omitempty"`
	Events   map[string]any `json:"event_analysis,omitempty"`
	Services map[string]any `json:"service_analysis,omitempty"`
}

func NewClusterAnalysisTool(k8sManager kubernetes.ClientInterface) *ClusterAnalysisTool {
	return &ClusterAnalysisTool{
		BaseTool: NewBaseTool(k8sManager),
	}
}

func (t *ClusterAnalysisTool) Name() string { return "analyze_cluster" }

func (t *ClusterAnalysisTool) Description() string {
	return "Perform a comprehensive cluster health audit across nodes, workloads, and events."
}

func (t *ClusterAnalysisTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "include_pods", Type: "boolean", Default: true},
		{Name: "include_events", Type: "boolean", Default: true},
		{Name: "event_hours_back", Type: "number", Default: 24},
		{Name: "include_services", Type: "boolean", Default: false},
	}
}

func (t *ClusterAnalysisTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	report := AnalysisReport{}

	info, err := t.K8sManager.GetClusterInfo(ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := t.K8sManager.GetNodes(ctx)
	if err == nil {
		report.Nodes = t.processNodes(nodes)
	}

	if t.GetBoolArg(args, "include_pods", true) {
		if pods, err := t.K8sManager.GetPodsAllNamespaces(ctx); err == nil {
			report.Pods = t.processPods(pods)
		}
	}

	if t.GetBoolArg(args, "include_events", true) {
		hours := t.GetIntArg(args, "event_hours_back", 24)
		if events, err := t.K8sManager.GetEventsAllNamespaces(ctx); err == nil {
			report.Events = t.processEvents(events, hours)
		}
	}

	if t.GetBoolArg(args, "include_services", false) {
		if svcs, err := t.K8sManager.GetServicesAllNamespaces(ctx); err == nil {
			report.Services = t.processServices(svcs)
		}
	}

	assessment := t.runHealthCheck(report)

	return map[string]any{
		"timestamp":       time.Now().UTC().Format(time.RFC3339),
		"cluster_info":    info,
		"report":          report,
		"assessment":      assessment,
		"recommendations": t.getRecs(assessment),
	}, nil
}

func (t *ClusterAnalysisTool) processNodes(nodes []kubernetes.NodeInfo) map[string]any {
	var ready, tainted int
	conditions := make(map[string]int)
	roles := make(map[string]int)

	for _, n := range nodes {
		if n.Status == "Ready" {
			ready++
		}
		if len(n.Taints) > 0 {
			tainted++
		}
		for _, c := range n.Conditions {
			if c.Status == "True" {
				conditions[c.Type]++
			}
		}
		role := "worker"
		for k := range n.Labels {
			if strings.Contains(k, "master") || strings.Contains(k, "control-plane") {
				role = "control-plane"
				break
			}
		}
		roles[role]++
	}

	return map[string]any{
		"total":      len(nodes),
		"ready":      ready,
		"tainted":    tainted,
		"roles":      roles,
		"conditions": conditions,
	}
}

func (t *ClusterAnalysisTool) processPods(pods []kubernetes.PodInfo) map[string]any {
	phases := make(map[string]int)
	var problematic, highRestarts int

	for _, p := range pods {
		phases[strings.ToLower(string(p.Phase))]++
		if p.Phase != "Running" && p.Phase != "Succeeded" {
			problematic++
		}
		if p.RestartCount > 5 {
			highRestarts++
		}
	}

	return map[string]any{
		"total":         len(pods),
		"phases":        phases,
		"problematic":   problematic,
		"high_restarts": highRestarts,
	}
}

func (t *ClusterAnalysisTool) processEvents(events []kubernetes.EventInfo, hours int) map[string]any {
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	var warnings, critical int

	for _, e := range events {
		if e.LastTimestamp.Before(cutoff) {
			continue
		}
		if e.Type == "Warning" {
			warnings++
		}
		if t.isCritical(e.Reason) {
			critical++
		}
	}

	return map[string]any{
		"recent_warnings": warnings,
		"critical_count":  critical,
	}
}

func (t *ClusterAnalysisTool) processServices(svcs []kubernetes.ServiceInfo) map[string]any {
	types := make(map[string]int)
	for _, s := range svcs {
		types[string(s.Type)]++
	}
	return map[string]any{
		"total": len(svcs),
		"types": types,
	}
}

func (t *ClusterAnalysisTool) runHealthCheck(r AnalysisReport) map[string]any {
	score := 100
	var issues []string

	if n, ok := r.Nodes["total"].(int); ok {
		ready := r.Nodes["ready"].(int)
		if ready < n {
			score -= (n - ready) * 20
			issues = append(issues, fmt.Sprintf("%d nodes not ready", n-ready))
		}
	}

	if r.Pods != nil {
		prob := r.Pods["problematic"].(int)
		score -= (prob * 2)
		if prob > 0 {
			issues = append(issues, fmt.Sprintf("%d non-running pods", prob))
		}
	}

	if r.Events != nil {
		crit := r.Events["critical_count"].(int)
		if crit > 10 {
			score -= 10
			issues = append(issues, "High volume of critical events")
		}
	}

	status := "healthy"
	if score < 90 {
		status = "warning"
	}
	if score < 75 {
		status = "degraded"
	}
	if score < 50 {
		status = "critical"
	}

	return map[string]any{
		"score":  max(0, score),
		"status": status,
		"issues": issues,
	}
}

func (t *ClusterAnalysisTool) isCritical(reason string) bool {
	criticalReasons := map[string]bool{
		"Failed": true, "BackOff": true, "Unhealthy": true,
		"FailedMount": true, "FailedScheduling": true,
	}
	return criticalReasons[reason]
}

func (t *ClusterAnalysisTool) getRecs(assessment map[string]any) []string {
	issues := assessment["issues"].([]string)
	if len(issues) == 0 {
		return []string{"Cluster is healthy. Monitor for event spikes."}
	}
	return issues
}
