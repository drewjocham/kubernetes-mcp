package tools

import (
	"context"
	"time"

	"kube-watcher/internal/recommendation"
	kwatch "kube-watcher/kubernetes/watch"
)

type RecommendationTool struct {
	engine *recommendation.Engine
}

func NewRecommendationTool(engine *recommendation.Engine) *RecommendationTool {
	return &RecommendationTool{engine: engine}
}

func (t *RecommendationTool) Name() string {
	return "recommend_cluster_fix"
}

func (t *RecommendationTool) Description() string {
	return "Generate remediation guidance for a cluster alert."
}

func (t *RecommendationTool) Parameters() []ToolParameter {
	return []ToolParameter{
		{Name: "kind", Type: "string", Description: "Alert kind (node, pod, event)."},
		{Name: "name", Type: "string", Description: "Object name associated with the alert."},
		{Name: "namespace", Type: "string", Description: "Namespace for the object (if applicable)."},
		{Name: "severity", Type: "string", Description: "Severity level for context."},
		{Name: "reason", Type: "string", Description: "Short reason or condition."},
		{Name: "message", Type: "string", Description: "Detailed description of the issue."},
	}
}

func (t *RecommendationTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	alert := kwatch.Alert{
		Kind:       kwatch.AlertKind(getStringArg(args, "kind")),
		Name:       getStringArg(args, "name"),
		Namespace:  getStringArg(args, "namespace"),
		Severity:   getStringArg(args, "severity"),
		Reason:     getStringArg(args, "reason"),
		Message:    getStringArg(args, "message"),
		OccurredAt: time.Now(),
	}
	rec, err := t.engine.ForAlert(ctx, alert)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"alert":          alert,
		"recommendation": rec,
	}, nil
}

func getStringArg(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}
	return ""
}
