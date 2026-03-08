package chatbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"
)

type InvestigationBackend interface {
	Run(ctx context.Context, req InvestigationRequest) (InvestigationResult, error)
}

type HTTPPollingBackend struct {
	providerName string
	cfg          ProviderConfig
	timeout      time.Duration
	pollInterval time.Duration
	retryCount   int
	retryBackoff time.Duration
	httpClient   *http.Client
}

func NewHTTPPollingBackend(
	providerName string,
	cfg ProviderConfig,
	timeout time.Duration,
	pollInterval time.Duration,
	retryCount int,
	retryBackoff time.Duration,
) *HTTPPollingBackend {
	return &HTTPPollingBackend{
		providerName: providerName,
		cfg:          cfg,
		timeout:      timeout,
		pollInterval: pollInterval,
		retryCount:   retryCount,
		retryBackoff: retryBackoff,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *HTTPPollingBackend) Run(ctx context.Context, req InvestigationRequest) (InvestigationResult, error) {
	runID, err := b.startRun(ctx, req)
	if err != nil {
		return InvestigationResult{}, err
	}
	return b.pollRun(ctx, runID)
}

func (b *HTTPPollingBackend) startRun(ctx context.Context, req InvestigationRequest) (string, error) {
	bodyBytes, err := b.renderStartBody(req)
	if err != nil {
		return "", fmt.Errorf("render start body: %w", err)
	}

	method := strings.ToUpper(strings.TrimSpace(b.cfg.StartMethod))
	if method == "" {
		method = http.MethodPost
	}
	path := strings.TrimSpace(b.cfg.StartPath)
	if path == "" {
		return "", errors.New("provider start_path is required")
	}
	respBytes, err := b.doWithRetries(ctx, method, path, bodyBytes)
	if err != nil {
		return "", fmt.Errorf("start request failed: %w", err)
	}
	payload, err := parseJSONMap(respBytes)
	if err != nil {
		return "", fmt.Errorf("decode start response: %w", err)
	}

	paths := b.cfg.RunIDPaths
	if len(paths) == 0 {
		paths = []string{"run_id", "runId", "id", "data.run_id", "data.id"}
	}
	runID := firstString(payload, paths)
	if runID == "" {
		return "", errors.New("run id not found in provider start response")
	}
	return runID, nil
}

func (b *HTTPPollingBackend) pollRun(ctx context.Context, runID string) (InvestigationResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	ticker := time.NewTicker(b.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return InvestigationResult{}, fmt.Errorf("provider run timed out after %s", b.timeout)
		case <-ticker.C:
			payload, err := b.fetchStatus(timeoutCtx, runID)
			if err != nil {
				continue
			}

			state := firstString(payload, statePathsOrDefault(b.cfg.StatePaths))
			if state == "" {
				continue
			}
			if !containsIgnoreCase(stateSetOrDefault(b.cfg.TerminalStates), state) {
				continue
			}

			if !containsIgnoreCase(successSetOrDefault(b.cfg.SuccessStates), state) {
				errText := firstString(payload, errorPathsOrDefault(b.cfg.ErrorPaths))
				if errText == "" {
					errText = fmt.Sprintf("provider reported terminal non-success state: %s", state)
				}
				return InvestigationResult{}, errors.New(errText)
			}

			summary := firstString(payload, outputPathsOrDefault(b.cfg.OutputPaths))
			if summary == "" {
				summary = compactJSON(payload)
			}

			return InvestigationResult{
				Provider: b.providerName,
				RunID:    runID,
				State:    state,
				Summary:  summary,
			}, nil
		}
	}
}

func (b *HTTPPollingBackend) fetchStatus(ctx context.Context, runID string) (map[string]interface{}, error) {
	pathTmpl := strings.TrimSpace(b.cfg.StatusPathTemplate)
	if pathTmpl == "" {
		return nil, errors.New("provider status_path_template is required")
	}

	tpl, err := template.New("statusPath").Parse(pathTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse status_path_template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, map[string]string{"run_id": runID}); err != nil {
		return nil, fmt.Errorf("render status_path_template: %w", err)
	}

	method := strings.ToUpper(strings.TrimSpace(b.cfg.StatusMethod))
	if method == "" {
		method = http.MethodGet
	}

	respBytes, err := b.doWithRetries(ctx, method, buf.String(), nil)
	if err != nil {
		return nil, err
	}
	return parseJSONMap(respBytes)
}

func (b *HTTPPollingBackend) renderStartBody(req InvestigationRequest) ([]byte, error) {
	if strings.TrimSpace(b.cfg.StartBodyTemplate) == "" {
		def := map[string]interface{}{
			"prompt": req.Prompt,
			"title":  fmt.Sprintf("incident %s in %s", req.Kind, req.SpaceName),
			"metadata": map[string]string{
				"kind":           string(req.Kind),
				"correlation_id": req.CorrelationID,
				"space":          req.SpaceName,
				"thread":         req.ThreadName,
				"event_id":       req.EventID,
			},
		}
		return json.Marshal(def)
	}

	tpl, err := template.New("startBody").Parse(b.cfg.StartBodyTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, req); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *HTTPPollingBackend) doWithRetries(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= b.retryCount; attempt++ {
		respBytes, err := b.doOnce(ctx, method, path, body)
		if err == nil {
			return respBytes, nil
		}
		lastErr = err
		if attempt < b.retryCount {
			timer := time.NewTimer(b.retryBackoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, lastErr
}

func (b *HTTPPollingBackend) doOnce(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	fullURL := strings.TrimRight(b.cfg.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range b.cfg.Headers {
		req.Header.Set(k, v)
	}

	if token := b.loadAPIKey(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("provider HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return respBody, nil
}

func (b *HTTPPollingBackend) loadAPIKey() string {
	keyEnv := strings.TrimSpace(b.cfg.APIKeyEnv)
	if keyEnv == "" {
		return ""
	}
	return strings.TrimSpace(os.Getenv(keyEnv))
}

func parseJSONMap(raw []byte) (map[string]interface{}, error) {
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func firstString(payload map[string]interface{}, paths []string) string {
	for _, path := range paths {
		if val, ok := findPath(payload, path); ok {
			switch v := val.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case fmt.Stringer:
				s := strings.TrimSpace(v.String())
				if s != "" {
					return s
				}
			case float64, bool, int:
				return strings.TrimSpace(fmt.Sprintf("%v", v))
			}
		}
	}
	return ""
}

func findPath(payload map[string]interface{}, path string) (interface{}, bool) {
	cur := interface{}(payload)
	for _, segment := range strings.Split(path, ".") {
		seg := strings.TrimSpace(segment)
		if seg == "" {
			return nil, false
		}
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		next, ok := m[seg]
		if !ok {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

func containsIgnoreCase(set []string, state string) bool {
	state = strings.TrimSpace(strings.ToUpper(state))
	for _, candidate := range set {
		if strings.TrimSpace(strings.ToUpper(candidate)) == state {
			return true
		}
	}
	return false
}

func compactJSON(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(raw)
}

func statePathsOrDefault(paths []string) []string {
	if len(paths) > 0 {
		return paths
	}
	return []string{"state", "status", "data.state", "run.state"}
}

func outputPathsOrDefault(paths []string) []string {
	if len(paths) > 0 {
		return paths
	}
	return []string{"output", "result", "final_output", "data.output", "response"}
}

func errorPathsOrDefault(paths []string) []string {
	if len(paths) > 0 {
		return paths
	}
	return []string{"error", "error.message", "message", "status_message"}
}

func stateSetOrDefault(states []string) []string {
	if len(states) > 0 {
		return states
	}
	return []string{"SUCCEEDED", "FAILED", "CANCELLED", "COMPLETED", "ERROR"}
}

func successSetOrDefault(states []string) []string {
	if len(states) > 0 {
		return states
	}
	return []string{"SUCCEEDED", "COMPLETED"}
}
