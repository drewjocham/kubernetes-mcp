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

	method := nvl(strings.ToUpper(b.cfg.StartMethod), http.MethodPost)
	path := strings.TrimSpace(b.cfg.StartPath)
	if path == "" {
		return "", errors.New("provider start_path is required")
	}

	respBytes, err := b.doWithRetries(ctx, method, path, bodyBytes)
	if err != nil {
		return "", fmt.Errorf("start request failed: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(respBytes, &payload); err != nil {
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
				continue // Optional: log error before continuing
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
					errText = fmt.Sprintf("terminal non-success state: %s", state)
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

func (b *HTTPPollingBackend) fetchStatus(ctx context.Context, runID string) (map[string]any, error) {
	pathTmpl := strings.TrimSpace(b.cfg.StatusPathTemplate)
	if pathTmpl == "" {
		return nil, errors.New("provider status_path_template is required")
	}

	tpl, err := template.New("statusPath").Parse(pathTmpl)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, map[string]string{"run_id": runID}); err != nil {
		return nil, err
	}

	method := nvl(strings.ToUpper(b.cfg.StatusMethod), http.MethodGet)
	respBytes, err := b.doWithRetries(ctx, method, buf.String(), nil)
	if err != nil {
		return nil, err
	}

	var out map[string]any
	err = json.Unmarshal(respBytes, &out)
	return out, err
}

func (b *HTTPPollingBackend) renderStartBody(req InvestigationRequest) ([]byte, error) {
	if strings.TrimSpace(b.cfg.StartBodyTemplate) == "" {
		def := map[string]any{
			"prompt": req.Prompt,
			"title":  fmt.Sprintf("incident %s in %s", req.Kind, req.SpaceName),
			"metadata": map[string]string{
				"kind":           string(req.Kind),
				"correlation_id": req.CorrelationID,
				"space":          req.SpaceName,
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
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(b.retryBackoff):
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
	for k, v := range b.cfg.Headers {
		req.Header.Set(k, v)
	}

	if keyEnv := strings.TrimSpace(b.cfg.APIKeyEnv); keyEnv != "" {
		if token := os.Getenv(keyEnv); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("provider HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func firstString(payload map[string]any, paths []string) string {
	for _, path := range paths {
		if val, ok := findPath(payload, path); ok {
			s := strings.TrimSpace(fmt.Sprintf("%v", val))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func findPath(payload map[string]any, path string) (any, bool) {
	var cur any = payload
	for _, segment := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func nvl(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

func containsIgnoreCase(set []string, state string) bool {
	state = strings.ToUpper(strings.TrimSpace(state))
	for _, s := range set {
		if strings.ToUpper(strings.TrimSpace(s)) == state {
			return true
		}
	}
	return false
}

func compactJSON(v any) string {
	raw, _ := json.Marshal(v)
	return string(raw)
}

func statePathsOrDefault(p []string) []string {
	if len(p) > 0 {
		return p
	}
	return []string{"state", "status"}
}
func outputPathsOrDefault(p []string) []string {
	if len(p) > 0 {
		return p
	}
	return []string{"output", "result"}
}
func errorPathsOrDefault(p []string) []string {
	if len(p) > 0 {
		return p
	}
	return []string{"error", "message"}
}
func stateSetOrDefault(p []string) []string {
	if len(p) > 0 {
		return p
	}
	return []string{"SUCCEEDED", "FAILED"}
}
func successSetOrDefault(p []string) []string {
	if len(p) > 0 {
		return p
	}
	return []string{"SUCCEEDED"}
}
