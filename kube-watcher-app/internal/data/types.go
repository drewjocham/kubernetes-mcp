package data

import "time"

// AlertRecord mirrors the alert store shape from the dashboard server.
type AlertRecord struct {
	ID         string    `json:"id"`
	Kind       string    `json:"kind"`
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	Cluster    string    `json:"cluster"`
	Severity   string    `json:"severity"`
	Reason     string    `json:"reason"`
	Message    string    `json:"message"`
	Status     string    `json:"status"` // detected | thinking | report_ready | failed
	State      string    `json:"state,omitempty"`
	ReceivedAt time.Time `json:"receivedAt"`
	PodExists  bool      `json:"podExists,omitempty"`
	Comments   []Comment `json:"comments,omitempty"`
	// RCA fields (populated when status=report_ready)
	RootCause  string   `json:"rootCause,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Actions    []string `json:"actions,omitempty"`
	Confidence float64  `json:"confidence,omitempty"`
}

// Comment represents a user comment on an alert
type Comment struct {
	ID        string    `json:"id"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Incident is a historical event record from the MCP /history endpoint.
type Incident struct {
	ID          string         `json:"id"`
	Timestamp   time.Time      `json:"timestamp"`
	Kind        string         `json:"kind"`
	Severity    string         `json:"severity"`
	Namespace   string         `json:"namespace"`
	Name        string         `json:"name"`
	Reason      string         `json:"reason"`
	Message     string         `json:"message"`
	Occurrences int            `json:"occurrences"`
	History     []float64      `json:"history,omitempty"` // per-period occurrence counts
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// ToolSummary describes a single MCP tool.
type ToolSummary struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToolResult holds the output of executing an MCP tool.
type ToolResult struct {
	Tool    string         `json:"tool"`
	Output  map[string]any `json:"output,omitempty"`
	RawJSON string         `json:"raw,omitempty"`
	Err     string         `json:"error,omitempty"`
}

// StatusResponse is returned by GET /v1/status.
type StatusResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Cluster string `json:"cluster"`
	Latency int64  // milliseconds, set by client
}

// Recommendation is a generated remediation suggestion.
type Recommendation struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Severity       string   `json:"severity"`
	Summary        string   `json:"summary"`
	Steps          []string `json:"steps"`
	RelatedKind    string   `json:"relatedKind"`
	FrequencyDelta float64  `json:"frequencyDelta"`
	AlertRef       string   `json:"alertRef,omitempty"`
}

// MetricSeries is a labelled time series returned by Prometheus or the K8s metrics API.
type MetricSeries struct {
	Labels map[string]string
	Points []DataPoint
}

// DataPoint is a single (timestamp, value) sample.
type DataPoint struct {
	Timestamp time.Time
	Value     float64
}

// LogLine is a single structured log entry from ops log streaming.
type LogLine struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // INFO | WARN | ERROR | DEBUG
	Source    string    `json:"source"`
	Message   string    `json:"message"`
	Raw       string    `json:"raw"`
}

// ServiceStatus is a placeholder (Docker Compose support removed).
type ServiceStatus struct {
	Name      string `json:"name"`
	Image     string `json:"image"`
	Status    string `json:"status"` // running | exited | not_found | unknown
	StartedAt string `json:"startedAt,omitempty"`
	ID        string `json:"id,omitempty"`
}

// AIWorkspace is the desktop-ready AI control plane summary.
type AIWorkspace struct {
	Summary         AIWorkspaceSummary      `json:"summary"`
	Priorities      []AIPriority            `json:"priorities"`
	Playbooks       []AIPlaybook            `json:"playbooks"`
	DeploymentPlans []AnomalyDeploymentPlan `json:"deploymentPlans"`
}

// AIWorkspaceSummary captures the high-level operating posture.
type AIWorkspaceSummary struct {
	Headline        string `json:"headline"`
	Subheadline     string `json:"subheadline"`
	OpenAlerts      int    `json:"openAlerts"`
	CriticalAlerts  int    `json:"criticalAlerts"`
	AutomationReady int    `json:"automationReady"`
	DeployTargets   int    `json:"deployTargets"`
}

// AIPriority is a single high-value operator priority surfaced to the AI panel.
type AIPriority struct {
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Detail      string `json:"detail"`
	ActionLabel string `json:"actionLabel"`
}

// AIPlaybook captures a repeatable AI workflow.
type AIPlaybook struct {
	Title       string   `json:"title"`
	Prompt      string   `json:"prompt"`
	Target      string   `json:"target"`
	Description string   `json:"description"`
	Commands    []string `json:"commands"`
}

// AnomalyDeploymentPlan describes a supported deployment path for anomaly detection.
type AnomalyDeploymentPlan struct {
	Profile    string   `json:"profile"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Mode       string   `json:"mode"`
	Namespace  string   `json:"namespace"`
	Services   []string `json:"services"`
	Commands   []string `json:"commands"`
	Validation []string `json:"validation"`
	Artifacts  []string `json:"artifacts"`
}

// PodInfo represents a Kubernetes pod with essential metadata and status.
type PodInfo struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Status       string            `json:"status"`
	Phase        string            `json:"phase"`
	NodeName     string            `json:"nodeName"`
	Age          time.Duration     `json:"age"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	Containers   []ContainerInfo   `json:"containers"`
	RestartCount int               `json:"restartCount"`
}

// ContainerInfo represents a single container within a pod.
type ContainerInfo struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	Ready        bool   `json:"ready"`
	RestartCount int32  `json:"restartCount"`
	State        string `json:"state"`
}
