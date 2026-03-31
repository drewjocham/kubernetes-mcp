package mcp

import (
	"bufio"
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
		Alerts []data.AlertRecord `json:"alerts"`
	}
	if err := c.get(ctx, "/alerts", &payload); err != nil {
		// Try alternate shape: direct array
		var direct []data.AlertRecord
		if err2 := c.get(ctx, "/alerts", &direct); err2 == nil {
			return direct, nil
		}
		return nil, err
	}
	return payload.Alerts, nil
}

// History returns recent incidents from BadgerDB via the history endpoint.
func (c *Client) History(ctx context.Context) ([]data.Incident, error) {
	var payload struct {
		Incidents []data.Incident `json:"incidents"`
	}
	if err := c.get(ctx, "/history", &payload); err != nil {
		var direct []data.Incident
		if err2 := c.get(ctx, "/history", &direct); err2 == nil {
			return direct, nil
		}
		return nil, err
	}
	return payload.Incidents, nil
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

// Services returns the status of Docker-managed compose services.
func (c *Client) Services(ctx context.Context) ([]data.ServiceStatus, error) {
	var payload struct {
		Services []data.ServiceStatus `json:"services"`
	}
	if err := c.get(ctx, "/services", &payload); err != nil {
		return nil, err
	}
	return payload.Services, nil
}

// StartService starts a named service container.
func (c *Client) StartService(ctx context.Context, name string) error {
	return c.post(ctx, "/services/"+name+"/start", nil, nil)
}

// StopService stops a named service container.
func (c *Client) StopService(ctx context.Context, name string) error {
	return c.post(ctx, "/services/"+name+"/stop", nil, nil)
}

// RestartService restarts a named service container.
func (c *Client) RestartService(ctx context.Context, name string) error {
	return c.post(ctx, "/services/"+name+"/restart", nil, nil)
}

// ServiceLogsURL returns the URL to stream logs for a named service.
// The TUI fetches this via an HTTP GET with chunked streaming.
func (c *Client) ServiceLogsURL(name string, tail string) string {
	if tail == "" {
		tail = "200"
	}
	return c.baseURL + "/services/" + name + "/logs?tail=" + tail
}

// FetchServiceLogs fetches recent logs for a service as a slice of lines.
func (c *Client) FetchServiceLogs(ctx context.Context, name, tail string) ([]string, error) {
	if tail == "" {
		tail = "100"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.baseURL+"/services/"+name+"/logs?tail="+tail, nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	var lines []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
