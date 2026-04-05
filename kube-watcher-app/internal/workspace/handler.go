package workspace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"kube-watcher-app/internal/data"
)

// AlertProvider abstracts alert retrieval.
type AlertProvider interface {
	Alerts(ctx context.Context) ([]data.AlertRecord, error)
}

// HistoryProvider abstracts incident history retrieval.
type HistoryProvider interface {
	History(ctx context.Context) ([]data.Incident, error)
}

// RecommendationProvider abstracts recommendation retrieval.
type RecommendationProvider interface {
	Recommendations(ctx context.Context) ([]data.Recommendation, error)
}

// ServiceProvider abstracts runtime service retrieval.
type ServiceProvider interface {
	Services(ctx context.Context) ([]data.ServiceStatus, error)
}

// Handler assembles AI-first desktop workspace state.
type Handler struct {
	alerts          AlertProvider
	history         HistoryProvider
	recommendations RecommendationProvider
	services        ServiceProvider
	strategies      []DeploymentStrategy
}

// NewHandler wires the workspace handler from its providers and strategies.
func NewHandler(
	alerts AlertProvider,
	history HistoryProvider,
	recommendations RecommendationProvider,
	services ServiceProvider,
	strategies []DeploymentStrategy,
) *Handler {
	return &Handler{
		alerts:          alerts,
		history:         history,
		recommendations: recommendations,
		services:        services,
		strategies:      strategies,
	}
}

// Build returns the desktop AI workspace model.
func (h *Handler) Build(ctx context.Context) (data.AIWorkspace, error) {
	alerts, err := h.alerts.Alerts(ctx)
	if err != nil {
		return data.AIWorkspace{}, fmt.Errorf("load alerts: %w", err)
	}

	incidents, err := h.history.History(ctx)
	if err != nil {
		return data.AIWorkspace{}, fmt.Errorf("load history: %w", err)
	}

	recommendations, err := h.recommendations.Recommendations(ctx)
	if err != nil {
		return data.AIWorkspace{}, fmt.Errorf("load recommendations: %w", err)
	}

	services, err := h.services.Services(ctx)
	if err != nil {
		return data.AIWorkspace{}, fmt.Errorf("load services: %w", err)
	}

	criticalAlerts := 0
	anomstackAlerts := 0
	for _, alert := range alerts {
		if severityRank(alert.Severity) >= severityRank("critical") {
			criticalAlerts++
		}
		// Count anomstack alerts (they have specific ID patterns)
		if strings.HasPrefix(alert.ID, "anomaly-") {
			anomstackAlerts++
		}
	}

	headline := "Arguskube Sentinels"
	if criticalAlerts > 0 {
		headline = fmt.Sprintf("%d critical signals need attention before automation fan-out.", criticalAlerts)
	}
	if anomstackAlerts > 0 {
		headline = fmt.Sprintf("Anomstack detected %d anomalies", anomstackAlerts)
		if criticalAlerts > 0 {
			headline = fmt.Sprintf("Anomstack detected %d anomalies, %d critical signals need attention", anomstackAlerts, criticalAlerts)
		}
	}

	subheadline := ""
	if len(incidents) > 0 {
		subheadline = fmt.Sprintf("%d recent incidents are available as context.", len(incidents))
	}
	if anomstackAlerts > 0 {
		if subheadline != "" {
			subheadline += " "
		}
		subheadline += fmt.Sprintf("Anomstack AI monitoring active with %d detected anomalies.", anomstackAlerts)
	}

	workspace := data.AIWorkspace{
		Summary: data.AIWorkspaceSummary{
			Headline:        headline,
			Subheadline:     subheadline,
			OpenAlerts:      len(alerts),
			CriticalAlerts:  criticalAlerts,
			AutomationReady: len(recommendations),
			DeployTargets:   len(h.strategies),
		},
		Priorities:      buildPriorities(alerts, recommendations, services),
		Playbooks:       buildPlaybooks(alerts, recommendations, incidents),
		DeploymentPlans: buildPlans(h.strategies),
	}

	return workspace, nil
}

func buildPlans(strategies []DeploymentStrategy) []data.AnomalyDeploymentPlan {
	plans := make([]data.AnomalyDeploymentPlan, 0, len(strategies))
	for _, strategy := range strategies {
		plans = append(plans, strategy.Plan())
	}
	return plans
}

func buildPriorities(alerts []data.AlertRecord, recommendations []data.Recommendation, services []data.ServiceStatus) []data.AIPriority {
	priorities := make([]data.AIPriority, 0, 3)

	// Check for anomstack anomalies first
	var anomstackAlerts []data.AlertRecord
	for _, alert := range alerts {
		if strings.HasPrefix(alert.ID, "anomaly-") {
			anomstackAlerts = append(anomstackAlerts, alert)
		}
	}

	if len(anomstackAlerts) > 0 {
		// Prioritize anomstack anomalies with enhanced AI analysis
		topAnomaly := anomstackAlerts[0]
		detail := fmt.Sprintf("AI anomaly detection: %s", topAnomaly.Message)

		// Add specific recommendations based on anomaly type
		if strings.Contains(strings.ToLower(topAnomaly.Name), "memory") {
			detail += " - Consider resource limits and autoscaling"
		} else if strings.Contains(strings.ToLower(topAnomaly.Name), "latency") {
			detail += " - Check network policies and service mesh configuration"
		} else if strings.Contains(strings.ToLower(topAnomaly.Name), "restart") {
			detail += " - Investigate pod stability and liveness probes"
		} else if strings.Contains(strings.ToLower(topAnomaly.Name), "log") {
			detail += " - Review application logging and SigNoz log aggregation"
		}

		priorities = append(priorities, data.AIPriority{
			Title:       fmt.Sprintf("Anomstack: %s", topAnomaly.Name),
			Severity:    topAnomaly.Severity,
			Detail:      detail,
			ActionLabel: "Review anomaly details",
		})
	} else if top := topAlert(alerts); top != nil {
		priorities = append(priorities, data.AIPriority{
			Title:       fmt.Sprintf("%s/%s", fallback(top.Namespace, "cluster"), top.Name),
			Severity:    top.Severity,
			Detail:      top.Message,
			ActionLabel: "Open incident context",
		})
	}

	if len(recommendations) > 0 {
		rec := recommendations[0]
		priorities = append(priorities, data.AIPriority{
			Title:       rec.Title,
			Severity:    rec.Severity,
			Detail:      rec.Summary,
			ActionLabel: "Send to Arguskube",
		})
	}

	unhealthy := make([]string, 0)
	for _, service := range services {
		if strings.EqualFold(service.Status, "running") {
			continue
		}
		unhealthy = append(unhealthy, service.Name)
	}
	if len(unhealthy) > 0 {
		priorities = append(priorities, data.AIPriority{
			Title:       "Recover platform dependencies",
			Severity:    "medium",
			Detail:      fmt.Sprintf("Services not running: %s", strings.Join(unhealthy, ", ")),
			ActionLabel: "Restart stack",
		})
	}

	if len(priorities) == 0 {
		priorities = append(priorities, data.AIPriority{
			Title:       "No urgent drift detected",
			Severity:    "low",
			Detail:      "Use the Arguskube to run a proactive cluster scan or validate the anomaly pipeline.",
			ActionLabel: "Run proactive scan",
		})
	}

	return priorities
}

func buildPlaybooks(alerts []data.AlertRecord, recommendations []data.Recommendation, incidents []data.Incident) []data.AIPlaybook {
	recentKinds := "pods, nodes, services"
	anomstackAlertCount := 0
	for _, alert := range alerts {
		if strings.HasPrefix(alert.ID, "anomaly-") {
			anomstackAlertCount++
		}
	}

	if len(incidents) > 0 {
		kinds := make([]string, 0, len(incidents))
		seen := map[string]struct{}{}
		for _, incident := range incidents {
			if _, ok := seen[incident.Kind]; ok || strings.TrimSpace(incident.Kind) == "" {
				continue
			}
			seen[incident.Kind] = struct{}{}
			kinds = append(kinds, incident.Kind)
			if len(kinds) == 3 {
				break
			}
		}
		if len(kinds) > 0 {
			recentKinds = strings.Join(kinds, ", ")
		}
	}

	playbooks := []data.AIPlaybook{
		{
			Title:       "Incident commander",
			Prompt:      "Summarize the live alerts, likely blast radius, and the safest first remediation step.",
			Target:      "Arguskube",
			Description: "Pulls current MCP alerts and recent incident history into a response draft.",
			Commands: []string{
				"kw view insights --window 6h",
				"kw view cluster-analysis",
			},
		},
		{
			Title:       "Anomaly launch assistant",
			Prompt:      "Start the Anomstack stack and list the first validation checks.",
			Target:      "Deployment",
			Description: "Uses the existing `kw anomstack start` path with a user-provided repo path.",
			Commands: []string{
				"kw anomstack start --path /path/to/anomstack",
				"kw anomstack status --path /path/to/anomstack",
			},
		},
		{
			Title:       "Trend explainer",
			Prompt:      fmt.Sprintf("Explain recurring anomaly patterns across %s and rank the next three checks.", recentKinds),
			Target:      "History",
			Description: fmt.Sprintf("Blends %d live alerts with %d remediation hints.", len(alerts), len(recommendations)),
			Commands: []string{
				"kw view health",
				"kw view tools",
			},
		},
	}

	// Add anomstack-specific playbook if anomalies are detected
	if anomstackAlertCount > 0 {
		playbooks = append(playbooks, data.AIPlaybook{
			Title:       "AI Anomaly Response",
			Prompt:      fmt.Sprintf("Analyze %d AI-detected anomalies from SigNoz monitoring, prioritize remediation steps, suggest preventive measures, and recommend SigNoz dashboard views for deeper investigation.", anomstackAlertCount),
			Target:      "Arguskube",
			Description: fmt.Sprintf("AI-powered anomaly analysis for %d detected issues with automated recommendations and SigNoz integration.", anomstackAlertCount),
			Commands: []string{
				"kw view anomalies",
				"kw ai analyze-anomalies",
				"kw alert create --from-anomstack",
				"kw signoz dashboard --anomalies",
				"kw signoz logs --filter=anomaly",
			},
		})
	}

	for i := range playbooks {
		if playbooks[i].Target == "ArgusKube" {
			playbooks[i].Target = "Arguskube"
		}
	}

	return playbooks
}

// generateSigNozRecommendations creates SigNoz-specific recommendations for anomalies
func generateSigNozRecommendations(anomstackAlerts []data.AlertRecord) []string {
	recommendations := []string{}

	for _, alert := range anomstackAlerts {
		if strings.Contains(strings.ToLower(alert.Name), "memory") {
			recommendations = append(recommendations,
				"View SigNoz memory metrics dashboard for detailed resource usage patterns",
				"Check container memory limits and consider horizontal pod autoscaling",
			)
		} else if strings.Contains(strings.ToLower(alert.Name), "latency") {
			recommendations = append(recommendations,
				"Analyze network latency in SigNoz distributed tracing dashboard",
				"Review service mesh configurations and network policies",
			)
		} else if strings.Contains(strings.ToLower(alert.Name), "restart") {
			recommendations = append(recommendations,
				"Examine pod restart patterns in SigNoz logs dashboard",
				"Check liveness and readiness probe configurations",
			)
		} else if strings.Contains(strings.ToLower(alert.Name), "log") {
			recommendations = append(recommendations,
				"Review application logs in SigNoz log explorer with anomaly filters",
				"Set up log-based alerting rules in SigNoz for proactive monitoring",
			)
		}
	}

	return recommendations
}

func topAlert(alerts []data.AlertRecord) *data.AlertRecord {
	if len(alerts) == 0 {
		return nil
	}

	ordered := append([]data.AlertRecord(nil), alerts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return severityRank(ordered[i].Severity) > severityRank(ordered[j].Severity)
	})
	return &ordered[0]
}

func severityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 4
	case "high", "error":
		return 3
	case "medium", "warn", "warning":
		return 2
	case "low", "info":
		return 1
	default:
		return 0
	}
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
