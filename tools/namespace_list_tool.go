package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"kube-watcher/kubernetes"
)

type NamespaceListTool struct {
	k8sManager kubernetes.ClientInterface
	logger     *slog.Logger
}

type nsDetail struct {
	Name     string
	IsSystem bool
	Pods     []kubernetes.PodInfo
	Quotas   []kubernetes.ResourceQuotaInfo
}

func NewNamespaceListTool(k8sManager kubernetes.ClientInterface, logger *slog.Logger) *NamespaceListTool {
	return &NamespaceListTool{
		k8sManager: k8sManager,
		logger:     logger,
	}
}

func (t *NamespaceListTool) Name() string { return "list_namespaces" }

func (t *NamespaceListTool) Description() string {
	return "Get comprehensive information about namespaces, including resource density and governance analysis."
}

func (t *NamespaceListTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "include_system", Type: "boolean", Description: "Include system namespaces (kube-system, etc.)"},
		{Name: "include_quotas", Type: "boolean", Description: "Fetch and analyze ResourceQuotas"},
	}
}

func (t *NamespaceListTool) Execute(ctx context.Context, args map[string]any) (map[string]any, error) {
	includeSystem, _ := args["include_system"].(bool)
	includeQuotas, _ := args["include_quotas"].(bool)

	allNames, err := t.k8sManager.GetNamespaces(ctx)
	if err != nil {
		t.logger.Error("Failed to fetch namespaces", "error", err)
		return nil, err
	}

	details := t.fetchParallel(ctx, allNames, includeSystem, includeQuotas)

	var (
		totalPods   int
		totalQuotas int
		userNSCount int
		nsResults   = make([]map[string]any, 0)
	)

	for _, d := range details {
		totalPods += len(d.Pods)
		totalQuotas += len(d.Quotas)
		if !d.IsSystem {
			userNSCount++
		}

		nsResults = append(nsResults, map[string]any{
			"name":          d.Name,
			"is_system":     d.IsSystem,
			"pod_count":     len(d.Pods),
			"quota_count":   len(d.Quotas),
			"pod_breakdown": t.analyzePodStatus(d.Pods),
			"quota_details": t.mapQuotas(d.Quotas),
		})
	}

	return map[string]any{
		"summary": map[string]any{
			"total_namespaces": len(allNames),
			"total_pods":       totalPods,
			"user_namespaces":  userNSCount,
		},
		"namespaces": nsResults,
		"analysis":   t.generateAnalysis(len(allNames), userNSCount, totalPods, totalQuotas),
	}, nil
}

func (t *NamespaceListTool) fetchParallel(ctx context.Context, names []string, incSys, incQuo bool) []nsDetail {
	var wg sync.WaitGroup
	resChan := make(chan nsDetail, len(names))

	for _, name := range names {
		isSys := t.isSystemNamespace(name)
		if isSys && !incSys {
			continue
		}

		wg.Add(1)
		go func(n string, sys bool) {
			defer wg.Done()
			pods, _ := t.k8sManager.GetPods(ctx, n)
			var quotas []kubernetes.ResourceQuotaInfo
			if incQuo {
				quotas, _ = t.k8sManager.GetResourceQuotas(ctx, n)
			}
			resChan <- nsDetail{Name: n, IsSystem: sys, Pods: pods, Quotas: quotas}
		}(name, isSys)
	}

	go func() {
		wg.Wait()
		close(resChan)
	}()

	var results []nsDetail
	for d := range resChan {
		results = append(results, d)
	}
	return results
}

func (t *NamespaceListTool) isSystemNamespace(name string) bool {
	prefixes := []string{"kube-", "kubernetes-", "openshift-", "istio-", "cert-manager", "ingress-"}
	names := []string{"default", "monitoring", "logging", "prometheus", "grafana"}

	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	for _, s := range names {
		if name == s {
			return true
		}
	}
	return false
}

func (t *NamespaceListTool) analyzePodStatus(pods []kubernetes.PodInfo) map[string]int {
	counts := map[string]int{"running": 0, "pending": 0, "failed": 0, "succeeded": 0}
	for _, p := range pods {
		counts[strings.ToLower(string(p.Phase))]++
	}
	return counts
}

func (t *NamespaceListTool) mapQuotas(quotas []kubernetes.ResourceQuotaInfo) []map[string]any {
	out := make([]map[string]any, len(quotas))
	for i, q := range quotas {
		out[i] = map[string]any{"name": q.Name, "hard": q.Hard, "used": q.Used}
	}
	return out
}

func (t *NamespaceListTool) generateAnalysis(total, user, pods, quotas int) map[string]any {
	if total == 0 {
		return nil
	}

	avgPods := float64(pods) / float64(total)
	quotaCoverage := (float64(quotas) / float64(total)) * 100

	recs := []string{}
	if quotaCoverage < 50 {
		recs = append(recs, "Low quota coverage: implement ResourceQuotas for better stability.")
	}
	if avgPods > 100 {
		recs = append(recs, fmt.Sprintf("High pod density (%.1f avg). Consider namespace splitting.", avgPods))
	}
	if user == 0 {
		recs = append(recs, "No user namespaces: check if applications are wrongly placed in default/system namespaces.")
	}

	return map[string]any{
		"metrics": map[string]any{
			"avg_pods_per_ns":    avgPods,
			"quota_coverage_pct": quotaCoverage,
		},
		"recommendations": recs,
		"health_status":   t.deriveHealth(len(recs)),
	}
}

func (t *NamespaceListTool) deriveHealth(issueCount int) string {
	if issueCount == 0 {
		return "healthy"
	}
	if issueCount <= 2 {
		return "warning"
	}
	return "critical"
}
