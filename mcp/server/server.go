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
	clusterAnalysisTool := tools.NewClusterAnalysisTool(s.client)
	allTools := []Tool{
		tools.NewNodeStatusTool(s.client),
		tools.NewPodResourcesTool(s.client),
		tools.NewPodLogsTool(s.client),
		tools.NewNamespaceListTool(s.client, s.logger),
		tools.NewHistoryInsightsTool(s.history),
		tools.NewVersionTool(cfg.Version, cfg.GitCommit, cfg.BuildDate),
		clusterAnalysisTool,
		tools.NewClusterAnalysisLegacyTool(clusterAnalysisTool),
		tools.NewRecommendationTool(s.client, s.engine),
		tools.NewClusterEventsTool(s.client, s.logger),
		tools.NewHistoryInsightsToolWithClient(s.client, s.history),
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
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				s.logger.Warn("failed to unmarshal tool arguments", "tool", name, "error", err)
			}
		}

		res, err := t.Execute(ctx, args)
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}},
			}, nil
		}

		out, err := json.Marshal(res)
		if err != nil {
			s.logger.Warn("failed to marshal tool result", "tool", name, "error", err)
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: "internal error: failed to marshal result"}},
			}, nil
		}
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
	rec, err := s.engine.ForAlert(ctx, a)
	if err != nil {
		s.logger.Warn("failed to generate recommendation", "alert", a.Name, "error", err)
	}
	s.alertsMu.Lock()
	s.alerts = append([]AlertRecord{{Alert: a, Recommendation: rec}}, s.alerts...)
	if len(s.alerts) > maxAlertRecords {
		s.alerts = s.alerts[:maxAlertRecords]
	}
	s.alertsMu.Unlock()

	if err := s.history.Record(ctx, history.Incident{
		ID:        uuid.NewString(),
		Timestamp: a.OccurredAt,
		Kind:      history.IssueKind(a.Kind),
		Severity:  a.Severity,
		Name:      a.Name,
		Message:   a.Message,
	}); err != nil {
		s.logger.Warn("failed to record incident", "alert", a.Name, "error", err)
	}

	if err := s.mcp.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{URI: alertsResourceURI}); err != nil {
		s.logger.Warn("failed to send resource update notification", "error", err)
	}
}

func (s *MCPServer) handleReadAlerts(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	data, err := json.Marshal(s.AlertsSnapshot())
	if err != nil {
		s.logger.Warn("failed to marshal alerts", "error", err)
		return nil, err
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: alertsResourceURI, Text: string(data)}},
	}, nil
}

func (s *MCPServer) handleReadHistory(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	results, err := s.IncidentHistory(ctx, defaultHistoryRange)
	if err != nil {
		s.logger.Warn("failed to fetch incident history", "error", err)
		return nil, err
	}
	data, err := json.Marshal(results)
	if err != nil {
		s.logger.Warn("failed to marshal history", "error", err)
		return nil, err
	}
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
	var all []history.Incident
	for _, k := range history.SupportedKinds {
		incidents, err := s.history.List(ctx, k, window)
		if err != nil {
			return nil, err
		}
		all = append(all, incidents...)
	}
	return all, nil
}

// Recommendations returns a deduplicated slice of recommendations derived
// from the current alert snapshot, sorted by severity.
func (s *MCPServer) Recommendations() []recommendation.Recommendation {
	s.alertsMu.RLock()
	defer s.alertsMu.RUnlock()
	seen := make(map[string]bool, len(s.alerts))
	var out []recommendation.Recommendation
	for _, ar := range s.alerts {
		key := ar.Recommendation.Title
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ar.Recommendation)
	}
	return out
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
