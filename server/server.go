package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"kube-watcher/internal/history"
	"kube-watcher/internal/recommendation"
	"kube-watcher/kubernetes"
	kwatch "kube-watcher/kubernetes/watch"
	"kube-watcher/tools"
)

type serverError string

func (e serverError) Error() string {
	return string(e)
}

const (
	defaultHistoryPath  = "~/.kube-watcher/history.jsonl"
	alertsResourceURI   = "kube://alerts/current"
	historyResourceURI  = "kube://history/incidents"
	jsonMIMEType        = "application/json"
	maxAlertRecords     = 100
	historyEntryLimit   = 100
	defaultHistoryRange = 72 * time.Hour
	defaultPollInterval = 30 * time.Second

	errLoggerRequired serverError = "server: logger is required"
	errToolNotFound   serverError = "server: tool not found"
)

var incidentKinds = []history.IssueKind{
	history.IncidentTypeNode,
	history.IncidentTypePod,
	history.IncidentTypeEvent,
}

// Tool describes the legacy tool abstraction used throughout the project.
type Tool interface {
	Name() string
	Description() string
	Parameters() []tools.ToolParameter
	Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}

// Config drives server bootstrap.
type Config struct {
	Version      string
	GitCommit    string
	BuildDate    string
	HistoryPath  string
	PollInterval time.Duration
}

func (cfg *Config) applyDefaults() {
	if cfg.HistoryPath == "" {
		cfg.HistoryPath = defaultHistoryPath
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultPollInterval
	}
}

type toolExecutor func(context.Context, map[string]interface{}) (map[string]interface{}, error)

type alertRecord struct {
	Alert          kwatch.Alert                  `json:"alert"`
	Recommendation recommendation.Recommendation `json:"recommendation"`
}

// MCPServer wraps the go-sdk server with kube-specific capabilities.
type MCPServer struct {
	logger  *slog.Logger
	mcp     *mcp.Server
	client  kubernetes.ClientInterface
	history *history.Store
	engine  *recommendation.Engine
	watcher *kwatch.Manager

	tools     map[string]Tool
	executors map[string]toolExecutor

	alertsMu sync.RWMutex
	alerts   []alertRecord
}

// NewMCPServer creates a new kube-watcher MCP server instance.
func NewMCPServer(logger *slog.Logger, cfg Config) (*MCPServer, error) {
	if logger == nil {
		return nil, errLoggerRequired
	}
	cfg.applyDefaults()

	client, err := kubernetes.NewClient(logger)
	if err != nil {
		return nil, err
	}
	store, err := history.NewStore(cfg.HistoryPath)
	if err != nil {
		return nil, err
	}

	engine := recommendation.NewEngine(store, logger)
	watcher := kwatch.NewManager(client, logger, cfg.PollInterval)

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "kube-watcher",
		Title:   "Kube Watcher",
		Version: cfg.Version,
	}, &mcp.ServerOptions{
		Logger:             logger,
		KeepAlive:          45 * time.Second,
		SubscribeHandler:   func(ctx context.Context, req *mcp.SubscribeRequest) error { return nil },
		UnsubscribeHandler: func(ctx context.Context, req *mcp.UnsubscribeRequest) error { return nil },
	})

	server := &MCPServer{
		logger:    logger,
		mcp:       mcpServer,
		client:    client,
		history:   store,
		engine:    engine,
		watcher:   watcher,
		tools:     make(map[string]Tool),
		executors: make(map[string]toolExecutor),
	}

	server.registerResources()
	server.registerTools(cfg)

	return server, nil
}

// ListTools exposes tool metadata for CLI mode.
func (s *MCPServer) ListTools() map[string]interface{} {
	toolList := make([]map[string]interface{}, 0, len(s.tools))
	for name, tool := range s.tools {
		params := tool.Parameters()
		paramList := make([]map[string]interface{}, len(params))
		for i, p := range params {
			paramList[i] = map[string]interface{}{
				"name":        p.Name,
				"type":        p.Type,
				"description": p.Description,
			}
		}
		toolList = append(toolList, map[string]interface{}{
			"name":        name,
			"description": tool.Description(),
			"parameters":  paramList,
		})
	}
	return map[string]interface{}{
		"tools":       toolList,
		"tool_count":  len(toolList),
		"server_info": s.getServerInfo(),
	}
}

// ExecuteTool allows CLI invocation of registered tools.
func (s *MCPServer) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error) {
	exec, ok := s.executors[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errToolNotFound, name)
	}
	return exec(ctx, args)
}

// HealthCheck reports server status.
func (s *MCPServer) HealthCheck(ctx context.Context) map[string]interface{} {
	health := map[string]interface{}{
		"server_status":    "healthy",
		"tools_count":      len(s.tools),
		"k8s_connectivity": "healthy",
	}
	if err := s.client.HealthCheck(ctx); err != nil {
		health["k8s_connectivity"] = "failed"
		health["k8s_error"] = err.Error()
		health["server_status"] = "degraded"
	} else if info, err := s.client.GetClusterInfo(ctx); err == nil {
		health["cluster_info"] = info
	}
	return health
}

// Start begins watching the cluster and serving MCP traffic over stdio.
func (s *MCPServer) Start(ctx context.Context) error {
	alertCh := s.watcher.Start(ctx)
	go s.consumeAlerts(ctx, alertCh)

	s.logger.Info("starting MCP server")
	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

func (s *MCPServer) registerResources() {
	s.addJSONResource(alertsResourceURI, "Current Alerts", s.readAlertsResource)
	s.addJSONResource(historyResourceURI, "Incident History", s.readHistoryResource)
}

func (s *MCPServer) addJSONResource(uri, name string, handler mcp.ResourceHandler) {
	s.mcp.AddResource(&mcp.Resource{
		URI:      uri,
		Name:     name,
		MIMEType: jsonMIMEType,
	}, handler)
}

func (s *MCPServer) registerTools(cfg Config) {
	s.installTool(tools.NewNodeStatusTool(s.client))
	s.installTool(tools.NewPodResourcesTool(s.client))
	s.installTool(tools.NewClusterAnalysisTool(s.client))
	s.installTool(tools.NewNamespaceListTool(s.client, s.logger))
	s.installTool(tools.NewClusterEventsTool(s.client, s.logger))
	s.installTool(tools.NewHistoryInsightsTool(s.history, s.history))
	s.installTool(tools.NewRecommendationTool(s.client, s.engine))
	s.installTool(tools.NewVersionTool(cfg.Version, cfg.GitCommit, cfg.BuildDate))
}

func (s *MCPServer) installTool(tool Tool) {
	name := tool.Name()
	s.tools[name] = tool
	s.executors[name] = tool.Execute

	mcpTool := &mcp.Tool{
		Name:        name,
		Description: tool.Description(),
		InputSchema: buildSchema(tool.Parameters()),
	}

	s.mcp.AddTool(mcpTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := make(map[string]interface{})
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return nil, err
			}
		}
		result, err := tool.Execute(ctx, args)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
				IsError: true,
			}, nil
		}
		encoded, _ := json.Marshal(result)
		return &mcp.CallToolResult{
			Content:           []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
			StructuredContent: result,
		}, nil
	})
}

func (s *MCPServer) consumeAlerts(ctx context.Context, alerts <-chan kwatch.Alert) {
	for {
		select {
		case <-ctx.Done():
			return
		case alert, ok := <-alerts:
			if !ok {
				return
			}
			s.handleAlert(ctx, alert)
		}
	}
}

func (s *MCPServer) handleAlert(ctx context.Context, alert kwatch.Alert) {
	rec, err := s.engine.ForAlert(ctx, alert)
	if err != nil {
		s.logger.Warn("failed to create recommendation", "error", err)
		return
	}
	record := alertRecord{
		Alert:          alert,
		Recommendation: rec,
	}

	s.storeAlert(record)

	incident := history.Incident{
		ID:          uuid.NewString(),
		Timestamp:   alert.OccurredAt,
		Kind:        history.IssueKind(alert.Kind),
		Severity:    alert.Severity,
		Namespace:   alert.Namespace,
		Name:        alert.Name,
		Reason:      alert.Reason,
		Message:     alert.Message,
		Occurrences: 1,
	}
	if err := s.history.Record(ctx, incident); err != nil {
		s.logger.Warn("failed to record incident", "error", err)
	}

	_ = s.mcp.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{
		URI: alertsResourceURI,
	})
}

func (s *MCPServer) storeAlert(record alertRecord) {
	s.alertsMu.Lock()
	defer s.alertsMu.Unlock()
	s.alerts = append([]alertRecord{record}, s.alerts...)
	if len(s.alerts) > maxAlertRecords {
		s.alerts = s.alerts[:maxAlertRecords]
	}
}

func (s *MCPServer) snapshotAlerts() []alertRecord {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	snapshot := make([]alertRecord, len(s.alerts))
	copy(snapshot, s.alerts)
	return snapshot
}

func (s *MCPServer) recentIncidents(ctx context.Context, window time.Duration, limit int) ([]history.Incident, error) {
	var incidents []history.Incident
	for _, kind := range incidentKinds {
		kindIncidents, err := s.history.List(ctx, kind, window)
		if err != nil {
			return nil, err
		}
		incidents = append(incidents, kindIncidents...)
	}
	sort.Slice(incidents, func(i, j int) bool {
		return incidents[i].Timestamp.After(incidents[j].Timestamp)
	})
	if limit > 0 && len(incidents) > limit {
		incidents = incidents[:limit]
	}
	return incidents, nil
}

func jsonResourceResult(uri string, payload interface{}) (*mcp.ReadResourceResult, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{URI: uri, Text: string(data)},
		},
	}, nil
}

func (s *MCPServer) readAlertsResource(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return jsonResourceResult(alertsResourceURI, s.snapshotAlerts())
}

func (s *MCPServer) readHistoryResource(ctx context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	incidents, err := s.recentIncidents(ctx, defaultHistoryRange, historyEntryLimit)
	if err != nil {
		return nil, err
	}
	return jsonResourceResult(historyResourceURI, incidents)
}

func (s *MCPServer) getServerInfo() map[string]interface{} {
	return map[string]interface{}{
		"name":        "kube-watcher",
		"version":     "1.0.0",
		"description": "Kubernetes monitoring and analysis MCP server",
		"capabilities": []string{
			"node_monitoring",
			"pod_analysis",
			"cluster_health",
			"event_analysis",
			"history_tracking",
		},
	}
}

func buildSchema(params []tools.ToolParameter) map[string]interface{} {
	properties := make(map[string]interface{})
	for _, p := range params {
		properties[p.Name] = map[string]interface{}{
			"type":        parameterTypeToJSON(p.Type),
			"description": p.Description,
		}
	}
	return map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
}

func parameterTypeToJSON(paramType string) string {
	switch strings.ToLower(paramType) {
	case "boolean":
		return "boolean"
	case "number", "integer":
		return "number"
	default:
		return "string"
	}
}
