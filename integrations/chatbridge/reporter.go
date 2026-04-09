package chatbridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

type Reporter interface {
	Post(ctx context.Context, threadName string, text string) error
}

type GoogleChatWebhookReporter struct {
	webhookURL string
	httpClient *http.Client
}

func NewGoogleChatWebhookReporter(cfg ReportingConfig) (*GoogleChatWebhookReporter, error) {
	webhookURL := strings.TrimSpace(cfg.WebhookURL)
	if webhookURL == "" && strings.TrimSpace(cfg.WebhookURLEnv) != "" {
		webhookURL = strings.TrimSpace(os.Getenv(cfg.WebhookURLEnv))
	}
	if webhookURL == "" {
		return nil, fmt.Errorf("reporting webhook URL is required (reporting.webhook_url or reporting.webhook_url_env)")
	}
	return &GoogleChatWebhookReporter{
		webhookURL: webhookURL,
		httpClient: &http.Client{},
	}, nil
}

func (r *GoogleChatWebhookReporter) Post(ctx context.Context, threadName string, text string) error {
	body := map[string]interface{}{
		"text": text,
	}
	if strings.TrimSpace(threadName) != "" {
		body["thread"] = map[string]string{"name": threadName}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.webhookURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("chat webhook HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func FormatReport(
	req InvestigationRequest,
	result InvestigationResult,
	fallbackActionPlan []string,
) string {
	planLines := fallbackActionPlan
	if len(planLines) == 0 {
		planLines = []string{
			"Confirm scope and impact in the affected namespace/component.",
			"Apply immediate mitigation to restore stability.",
			"Validate recovery with metrics/events/logs and close the incident loop.",
		}
	}

	var builder strings.Builder
	_, _ = fmt.Fprintf(&builder, "Investigation report (%s)\n", strings.ToUpper(string(req.Kind)))
	_, _ = fmt.Fprintf(&builder, "Provider: %s\n", result.Provider)
	_, _ = fmt.Fprintf(&builder, "Run ID: %s\n", result.RunID)
	_, _ = fmt.Fprintf(&builder, "State: %s\n\n", result.State)
	builder.WriteString("Findings:\n")
	builder.WriteString(result.Summary)
	builder.WriteString("\n\nPlan of action:\n")
	for i, step := range planLines {
		_, _ = fmt.Fprintf(&builder, "%d. %s\n", i+1, step)
	}
	return strings.TrimSpace(builder.String())
}
