package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
)

const (
	defaultModelUserPrompt = "Analyze this Kubernetes resource event and detect if it indicates a cluster issue."
)

type ModelEnricher struct {
	logger     *slog.Logger
	cfg        config.ModelSettings
	httpClient *http.Client
	now        func() time.Time
}

type modelAnalysisRequest struct {
	SystemPrompt string                 `json:"system_prompt"`
	UserPrompt   string                 `json:"user_prompt"`
	Event        map[string]interface{} `json:"event"`
}

type modelAnalysisResponse struct {
	IssueDetected bool                   `json:"issue_detected"`
	Severity      string                 `json:"severity"`
	Summary       string                 `json:"summary"`
	Confidence    float64                `json:"confidence"`
	Signals       []string               `json:"signals"`
	Raw           map[string]interface{} `json:"raw,omitempty"`
}

func NewModelEnricher(logger *slog.Logger, cfg config.ModelSettings) *ModelEnricher {
	if logger == nil {
		logger = slog.Default()
	}
	return &ModelEnricher{
		logger: logger,
		cfg:    cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		now: time.Now,
	}
}

func (m *ModelEnricher) Enrich(ctx context.Context, evt events.ResourceEvent) (events.ResourceEvent, error) {
	if !m.cfg.Enabled || evt.Object == nil || strings.TrimSpace(m.cfg.Endpoint) == "" {
		return evt, nil
	}

	resp, err := m.analyze(ctx, evt)
	if err != nil {
		m.logger.Warn("model analysis failed", "key", evt.Key(), "error", err)
		evt.Object["model_analysis_error"] = err.Error()
		evt.Object["model_issue_detected"] = false
		return evt, nil
	}

	detected := resp.IssueDetected && resp.Confidence >= m.cfg.MinConfidence
	evt.Object["model_issue_detected"] = detected
	evt.Object["model_issue_confidence"] = resp.Confidence
	evt.Object["model_checked_at"] = m.now().UTC().Format(time.RFC3339)
	if resp.Severity != "" {
		evt.Object["model_issue_severity"] = strings.ToLower(strings.TrimSpace(resp.Severity))
	}
	if resp.Summary != "" {
		evt.Object["model_issue_summary"] = strings.TrimSpace(resp.Summary)
	}
	if len(resp.Signals) > 0 {
		evt.Object["model_issue_signals"] = resp.Signals
	}
	return evt, nil
}

func (m *ModelEnricher) analyze(ctx context.Context, evt events.ResourceEvent) (modelAnalysisResponse, error) {
	reqBody := modelAnalysisRequest{
		SystemPrompt: m.cfg.SystemPrompt,
		UserPrompt:   defaultModelUserPrompt,
		Event: map[string]interface{}{
			"kind":             evt.Kind,
			"namespace":        evt.Namespace,
			"name":             evt.Name,
			"resource_version": evt.ResourceVersion,
			"object":           evt.Object,
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return modelAnalysisResponse{}, fmt.Errorf("marshal model request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return modelAnalysisResponse{}, fmt.Errorf("create model request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(os.Getenv(m.cfg.APIKeyEnv)); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return modelAnalysisResponse{}, fmt.Errorf("call model endpoint: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return modelAnalysisResponse{}, fmt.Errorf("read model response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusNoContent {
		return modelAnalysisResponse{}, fmt.Errorf("model endpoint returned %s", resp.Status)
	}

	var out modelAnalysisResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return modelAnalysisResponse{}, fmt.Errorf("decode model response: %w", err)
	}
	return out, nil
}
