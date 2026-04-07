package watcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"kube-watcher-app/internal/httpclient"
)

// Client talks to the watcher-engine REST API.
type Client struct {
	*httpclient.BaseClient
}

// New returns a Client configured from environment variables or defaults.
//
//	KW_WATCHER_ENDPOINT — base URL, default http://localhost:8085
func New() *Client {
	base := strings.TrimSpace(os.Getenv("KW_WATCHER_ENDPOINT"))
	if base == "" {
		base = "http://localhost:8085"
	}
	return &Client{
		BaseClient: httpclient.NewBaseClient(base, "", 10*time.Second),
	}
}

// NewWithConfig returns a Client using explicit values.
func NewWithConfig(endpoint string) *Client {
	if endpoint == "" {
		endpoint = "http://localhost:8085"
	}
	return &Client{
		BaseClient: httpclient.NewBaseClient(endpoint, "", 10*time.Second),
	}
}

// GetConfig returns the watcher configuration.
func (c *Client) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	var config map[string]interface{}
	err := c.Get(ctx, "/api/config", &config)
	return config, err
}

// GetRules returns the watcher rules.
func (c *Client) GetRules(ctx context.Context) ([]interface{}, error) {
	var rules []interface{}
	err := c.Get(ctx, "/api/rules", &rules)
	return rules, err
}

// GetResources returns the list of resources currently being tracked.
func (c *Client) GetResources(ctx context.Context) ([]string, error) {
	var resources []string
	err := c.Get(ctx, "/api/resources", &resources)
	return resources, err
}

// GetStatus returns the watcher engine status.
func (c *Client) GetStatus(ctx context.Context) (map[string]interface{}, error) {
	var status map[string]interface{}
	err := c.Get(ctx, "/api/status", &status)
	return status, err
}

// StreamLogs streams watcher logs via Server-Sent Events.
// The caller should handle the response body and close it.
func (c *Client) StreamLogs(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL()+"/api/logs/stream", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := c.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return resp, nil
}
