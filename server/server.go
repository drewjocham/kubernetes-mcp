package server

import (
	"context"
	"encoding/json"
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

func (e serverError) Error() string { return string(e) }

const (
	alertsResourceURI   = "kube://alerts/current"
	historyResourceURI  = "kube://history/incidents"
	jsonMIMEType        = "application/json"
	maxAlertRecords     = 100
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

type Tool interface {
	Name() string
	Description() string
	Parameters() []tools.ToolParameter
	Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}

type Config struct {
	Version      string
	GitCommit    string
	BuildDate    string
	HistoryPath  string
	K8sClient    *kubernetes.Client
	HistoryStore *history.Store
	Watcher      *kwatch.Manager
	PollInterval time.Duration
}

type alertRecord struct {
	Alert          kwatch.Alert                  `json:"alert"`
	Recommendation recommendation.Recommendation `json:"recommendation"`
}

type MCPServer struct {
	logger  *slog.Logger
	mcp     *mcp.Server
	client  *kubernetes.Client
	history *history.Store
	engine  *recommendation.Engine
	watcher *kwatch.Manager

	tools     map[string]Tool
	executors map[string]func(context.Context, map[string]interface{}) (map[string]interface{}, error)

	alertsMu sync.RWMutex
	alerts   []alertRecord
}

func NewMCPServer(logger *slog.Logger, cfg Config) (*MCPServer, error) {
	if logger == nil {
		return nil, errLoggerRequired
	}

	mcpServer := mcp.NewServer(&mcp.Implementation{
		Name:    "kube-watcher",
		Version: cfg.Version,
	}, &mcp.ServerOptions{
		Logger: logger,
	})

	server := &MCPServer{
		logger:    logger,
		mcp:       mcpServer,
		client:    cfg.K8sClient,
		history:   cfg.HistoryStore,
		engine:    recommendation.NewEngine(cfg.HistoryStore, logger),
		watcher:   cfg.Watcher,
		tools:     make(map[string]Tool),
		executors: make(map[string]func(context.Context, map[string]interface{}) (map[string]interface{}, error)),
	}

	server.setupResources()
	server.setupTools(cfg)

	return server, nil
}

func (s *MCPServer) Start(ctx context.Context) error {
	alertCh := s.watcher.Start(ctx)
	go s.listenForAlerts(ctx, alertCh)

	return s.mcp.Run(ctx, &mcp.StdioTransport{})
}

func (s *MCPServer) setupResources() {
	s.mcp.AddResource(&mcp.Resource{
		URI:      alertsResourceURI,
		Name:     "Current Alerts",
		MIMEType: jsonMIMEType,
	}, s.handleReadAlerts)

	s.mcp.AddResource(&mcp.Resource{
		URI:      historyResourceURI,
		Name:     "Incident History",
		MIMEType: jsonMIMEType,
	}, s.handleReadHistory)
}

func (s *MCPServer) setupTools(cfg Config) {
	allTools := []Tool{
		tools.NewNodeStatusTool(s.client),
		tools.NewPodResourcesTool(s.client),
		tools.NewNamespaceListTool(s.client, s.logger),
		tools.NewHistoryInsightsTool(s.client, s.history),
		tools.NewVersionTool(cfg.Version, cfg.GitCommit, cfg.BuildDate),
	}

	for _, t := range allTools {
		s.registerTool(t)
	}
}

func (s *MCPServer) registerTool(t Tool) {
	name := t.Name()
	s.tools[name] = t
	s.executors[name] = t.Execute

	s.mcp.AddTool(&mcp.Tool{
		Name:        name,
		Description: t.Description(),
		InputSchema: s.generateSchema(t.Parameters()),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := make(map[string]interface{})
		if req.Params.Arguments != nil {
			_ = json.Unmarshal(req.Params.Arguments, &args)
		}

		res, err := t.Execute(ctx, args)
		if err != nil {
			return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil
		}

		out, _ := json.Marshal(res)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(out)}},
		}, nil
	})
}

func (s *MCPServer) listenForAlerts(ctx context.Context, ch <-chan kwatch.Alert) {
	for {
		select {
		case <-ctx.Done():
			return
		case a, ok := <-ch:
			if !ok {
				return
			}
			s.processAlert(ctx, a)
		}
	}
}

func (s *MCPServer) processAlert(ctx context.Context, a kwatch.Alert) {
	rec, _ := s.engine.ForAlert(ctx, a)
	s.alertsMu.Lock()
	s.alerts = append([]alertRecord{{Alert: a, Recommendation: rec}}, s.alerts...)
	if len(s.alerts) > maxAlertRecords {
		s.alerts = s.alerts[:maxAlertRecords]
	}
	s.alertsMu.Unlock()

	_ = s.history.Record(ctx, history.Incident{
		ID:        uuid.NewString(),
		Timestamp: a.OccurredAt,
		Kind:      history.IssueKind(a.Kind),
		Severity:  a.Severity,
		Name:      a.Name,
		Message:   a.Message,
	})

	_ = s.mcp.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: alertsResourceURI})
}

func (s *MCPServer) handleReadAlerts(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	data, _ := json.Marshal(s.alerts)
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: alertsResourceURI, Text: string(data)}}}, nil
}

func (s *MCPServer) handleReadHistory(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	var results []history.Incident
	for _, k := range incidentKinds {
		items, _ := s.history.List(ctx, k, defaultHistoryRange)
		results = append(results, items...)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Timestamp.After(results[j].Timestamp) })
	data, _ := json.Marshal(results)
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: historyResourceURI, Text: string(data)}}}, nil
}

func (s *MCPServer) generateSchema(params []tools.ToolParameter) map[string]interface{} {
	props := make(map[string]interface{})
	for _, p := range params {
		t := "string"
		switch strings.ToLower(p.Type) {
		case "boolean":
			t = "boolean"
		case "number", "integer":
			t = "number"
		}
		props[p.Name] = map[string]interface{}{"type": t, "description": p.Description}
	}
	return map[string]interface{}{"type": "object", "properties": props}
}

func (s *MCPServer) ListTools() map[string]interface{} {
	return map[string]interface{}{"tools": s.tools}
}

func (s *MCPServer) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (map[string]interface{}, error) {
	if f, ok := s.executors[name]; ok {
		return f(ctx, args)
	}
	return nil, errToolNotFound
}

func (s *MCPServer) HealthCheck(ctx context.Context) map[string]interface{} {
	err := s.client.HealthCheck(ctx)
	status := "healthy"
	if err != nil {
		status = "degraded"
	}
	return map[string]interface{}{"status": status}
}
