package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"kube-watcher-app/internal/data"
)

// Client talks to the kube-watcher MCP REST API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a Client configured from environment variables or defaults.
//
//	KW_TOOLS_ENDPOINT — base URL, default http://localhost:8080/v1
//	KW_TOOLS_API_TOKEN — bearer token
func New() *Client {
	base := strings.TrimSpace(os.Getenv("KW_TOOLS_ENDPOINT"))
	if base == "" {
		base = "http://localhost:8080/v1"
	}
	return &Client{
		baseURL: strings.TrimRight(base, "/"),
		token:   strings.TrimSpace(os.Getenv("KW_TOOLS_API_TOKEN")),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// NewWithConfig returns a Client using explicit values.
func NewWithConfig(endpoint, token string) *Client {
	if endpoint == "" {
		endpoint = "http://localhost:8080/v1"
	}
	return &Client{
		baseURL: strings.TrimRight(endpoint, "/"),
		token:   token,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) put(ctx context.Context, path string, body any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

// Endpoint returns the base URL of the MCP server.
func (c *Client) Endpoint() string {
	return c.baseURL
}

// Status pings the server and returns its status.
func (c *Client) Status(ctx context.Context) (data.StatusResponse, error) {
	start := time.Now()
	var resp data.StatusResponse
	err := c.get(ctx, "/status", &resp)
	resp.Latency = time.Since(start).Milliseconds()
	return resp, err
}

// Alerts returns all current alerts.
func (c *Client) Alerts(ctx context.Context) ([]data.AlertRecord, error) {
	var payload struct {
		Alerts []struct {
			ID    string `json:"id"`
			Alert struct {
				Kind       string    `json:"kind"`
				Severity   string    `json:"severity"`
				Namespace  string    `json:"namespace"`
				Name       string    `json:"name"`
				Reason     string    `json:"reason"`
				Message    string    `json:"message"`
				OccurredAt time.Time `json:"occurred_at"`
			} `json:"alert"`
			Recommendation map[string]interface{} `json:"recommendation"`
			PodExists      bool                   `json:"pod_exists,omitempty"`
			PodCache       map[string]interface{} `json:"pod_cache,omitempty"`
			State          string                 `json:"state,omitempty"`
			Comments       []data.Comment         `json:"comments,omitempty"`
		} `json:"alerts"`
	}
	if err := c.get(ctx, "/alerts", &payload); err != nil {
		// Try alternate shape: direct array
		var direct []data.AlertRecord
		if err2 := c.get(ctx, "/alerts", &direct); err2 == nil {
			return direct, nil
		}
		return nil, err
	}
	var alerts []data.AlertRecord
	for _, item := range payload.Alerts {
		alert := item.Alert
		alerts = append(alerts, data.AlertRecord{
			ID:         item.ID,
			Kind:       alert.Kind,
			Namespace:  alert.Namespace,
			Name:       alert.Name,
			Cluster:    "", // Not provided by API
			Severity:   alert.Severity,
			Reason:     alert.Reason,
			Message:    alert.Message,
			Status:     "detected",
			State:      item.State,
			PodExists:  item.PodExists,
			Comments:   item.Comments,
			ReceivedAt: alert.OccurredAt,
		})
	}
	return alerts, nil
}

// UpdateAlertState updates the state of an alert.
func (c *Client) UpdateAlertState(ctx context.Context, id string, state string) error {
	req := struct {
		State string `json:"state"`
	}{
		State: state,
	}
	return c.put(ctx, fmt.Sprintf("/alerts/%s/state", id), req)
}

// AddAlertComment adds a comment to an alert.
func (c *Client) AddAlertComment(ctx context.Context, id string, author string, content string) error {
	req := struct {
		Author  string `json:"author"`
		Content string `json:"content"`
	}{
		Author:  author,
		Content: content,
	}
	return c.post(ctx, fmt.Sprintf("/alerts/%s/comments", id), req, nil)
}

// History returns recent incidents from BadgerDB via the history endpoint.
func (c *Client) History(ctx context.Context) ([]data.Incident, error) {
	var response struct {
		Records []data.Incident `json:"records"`
		Window  string          `json:"window"`
	}
	if err := c.get(ctx, "/history", &response); err != nil {
		// Try alternate shape: direct array
		var direct []data.Incident
		if err2 := c.get(ctx, "/history", &direct); err2 == nil {
			return direct, nil
		}
		return nil, err
	}
	return response.Records, nil
}

// ListTools returns available MCP tools.
func (c *Client) ListTools(ctx context.Context) ([]data.ToolSummary, error) {
	var payload struct {
		Tools []data.ToolSummary `json:"tools"`
	}
	if err := c.get(ctx, "/tools", &payload); err != nil {
		return nil, err
	}
	return payload.Tools, nil
}

// ExecuteTool runs a tool with the given arguments.
func (c *Client) ExecuteTool(ctx context.Context, name string, args map[string]any) (data.ToolResult, error) {
	if args == nil {
		args = map[string]any{}
	}
	var raw map[string]any
	if err := c.post(ctx, "/tools/"+name, args, &raw); err != nil {
		return data.ToolResult{Tool: name, Err: err.Error()}, err
	}
	b, _ := json.MarshalIndent(raw, "", "  ")
	return data.ToolResult{Tool: name, Output: raw, RawJSON: string(b)}, nil
}

// Recommendations returns recommendations from the MCP engine.
func (c *Client) Recommendations(ctx context.Context) ([]data.Recommendation, error) {
	var payload struct {
		Recommendations []data.Recommendation `json:"recommendations"`
	}
	if err := c.get(ctx, "/recommendations", &payload); err != nil {
		return nil, err
	}
	return payload.Recommendations, nil
}

// Services returns empty slice (Docker Compose support removed).
func (c *Client) Services(ctx context.Context) ([]data.ServiceStatus, error) {
	return []data.ServiceStatus{}, nil
}

// GetAnomstackAnomalies fetches anomalies from the anomstack service
func (c *Client) GetAnomstackAnomalies(ctx context.Context) ([]data.AlertRecord, error) {
	// Connect to anomstack service via kubectl proxy
	// kubectl proxy provides access to cluster services from outside
	anomstackURL := "http://localhost:8001/api/v1/namespaces/kw-anomaly/services/anomstack:8080/proxy/anomalies"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, anomstackURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request to anomstack: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// If kubectl proxy is not running, provide helpful error message
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host") {
			return nil, fmt.Errorf("cannot connect to anomstack service. Please ensure kubectl proxy is running: kubectl proxy --port=8001")
		}
		return nil, fmt.Errorf("failed to connect to anomstack service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anomstack service returned HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Anomalies []struct {
			ID        string  `json:"id"`
			Title     string  `json:"title"`
			Severity  string  `json:"severity"`
			Message   string  `json:"message"`
			Timestamp float64 `json:"timestamp"`
		} `json:"anomalies"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode anomstack response: %w", err)
	}

	var alerts []data.AlertRecord
	for _, anomaly := range payload.Anomalies {
		alerts = append(alerts, data.AlertRecord{
			ID:         anomaly.ID,
			Kind:       "Anomaly",
			Namespace:  "kw-anomaly",
			Name:       anomaly.Title,
			Cluster:    "minikube",
			Severity:   anomaly.Severity,
			Reason:     "AnomalyDetected",
			Message:    anomaly.Message,
			Status:     "detected",
			ReceivedAt: time.Unix(int64(anomaly.Timestamp/1000), 0),
		})
	}

	return alerts, nil
}
