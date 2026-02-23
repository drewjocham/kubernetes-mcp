package server

import (
	"context"
	"encoding/json"
	"errors"
	"kube-watcher/mcp/monitoring/history"
	"kube-watcher/mcp/monitoring/recommendation"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"kube-watcher/mcp/tools"
	"kube-watcher/pkg/kube"
	kwatch "kube-watcher/pkg/kube/watch"
)

var (
	ErrLoggerRequired = errors.New("server: logger is required")
	ErrToolNotFound   = errors.New("server: tool not found")
)

const (
	alertsResourceURI   = "kube://alerts/current"
	historyResourceURI  = "kube://history/incidents"
	jsonMIMEType        = "application/json"
	maxAlertRecords     = 100
	defaultHistoryRange = 72 * time.Hour
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
	K8sClient    kube.ClientInterface
	HistoryStore history.Recorder
	Watcher      *kwatch.Manager
	PollInterval time.Duration
}

type AlertRecord struct {
	Alert          kwatch.Alert                  `json:"alert"`
	Recommendation recommendation.Recommendation `json:"recommendation"`
}

type ToolSummary struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Parameters  []tools.ToolParameter `json:"parameters,omitempty"`
}

type MCPServer struct {
	logger  *slog.Logger
	mcp     *mcp.Server
	client  kube.ClientInterface
	history history.Recorder
	engine  *recommendation.Engine
	watcher *kwatch.Manager

	tools     map[string]Tool
	executors map[string]func(context.Context, map[string]interface{}) (map[string]interface{}, error)

	alertsMu sync.RWMutex
	alerts   []AlertRecord
}

func NewMCPServer(logger *slog.Logger, cfg Config) (*MCPServer, error) {
	if logger == nil {
		return nil, ErrLoggerRequired
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

	if err := s.mcp.Run(ctx, &mcp.StdioTransport{}); err != nil {
		s.logger.Warn("mcp server disconnected", "error", err)
	}

	<-ctx.Done()
	return nil
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
		tools.NewHistoryInsightsTool(s.history),
		tools.NewVersionTool(cfg.Version, cfg.GitCommit, cfg.BuildDate),
		tools.NewClusterAnalysisTool(s.client),
		tools.NewRecommendationTool(s.client, s.engine),
		tools.NewClusterEventsTool(s.client, s.logger),
	}

	for _, t := range allTools {
		s.registerTool(t)
	}
}

func (s *MCPServer) registerTool(t Tool) {
	name := t.Name()
	s.tools[name] = t
	s.executors[name] = t.Execute

	mcpTool := &mcp.Tool{
		Name:        name,
		Description: t.Description(),
		InputSchema: s.generateSchema(t.Parameters()),
	}

	s.mcp.AddTool(mcpTool, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := make(map[string]interface{})
		if req.Params.Arguments != nil {
			_ = json.Unmarshal(req.Params.Arguments, &args)
		}

		res, err := t.Execute(ctx, args)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil
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
	s.alerts = append([]AlertRecord{{Alert: a, Recommendation: rec}}, s.alerts...)
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
	data, _ := json.Marshal(s.AlertsSnapshot())
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: alertsResourceURI, Text: string(data)}},
	}, nil
}

func (s *MCPServer) handleReadHistory(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	results, _ := s.IncidentHistory(ctx, defaultHistoryRange)
	data, _ := json.Marshal(results)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: historyResourceURI, Text: string(data)}},
	}, nil
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
	return nil, ErrToolNotFound
}

func (s *MCPServer) HealthCheck(ctx context.Context) map[string]interface{} {
	err := s.client.HealthCheck(ctx)
	status := "healthy"
	if err != nil {
		status = "degraded"
	}
	return map[string]interface{}{"status": status}
}

func (s *MCPServer) AlertsSnapshot() []AlertRecord {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	out := make([]AlertRecord, len(s.alerts))
	copy(out, s.alerts)
	return out
}

func (s *MCPServer) IncidentHistory(ctx context.Context, window time.Duration) ([]history.Incident, error) {
	if window <= 0 {
		window = defaultHistoryRange
	}
	var results []history.Incident
	for _, k := range incidentKinds {
		items, err := s.history.List(ctx, k, window)
		if err != nil {
			return nil, err
		}
		results = append(results, items...)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	return results, nil
}

func (s *MCPServer) ToolSummaries() []ToolSummary {
	summaries := make([]ToolSummary, 0, len(s.tools))
	for _, t := range s.tools {
		summary := ToolSummary{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		}
		summaries = append(summaries, summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})
	return summaries
}
