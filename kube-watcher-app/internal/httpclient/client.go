package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BaseClient provides common HTTP client functionality for API clients.
type BaseClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewBaseClient creates a new BaseClient with the given endpoint, optional token, and timeout.
func NewBaseClient(endpoint, token string, timeout time.Duration) *BaseClient {
	if endpoint == "" {
		endpoint = "http://localhost:8080"
	}
	return &BaseClient{
		baseURL: strings.TrimRight(endpoint, "/"),
		token:   strings.TrimSpace(token),
		http:    &http.Client{Timeout: timeout},
	}
}

// DoRequest performs an HTTP request with the given method, path, and optional body.
// If out is not nil, the response body will be JSON-decoded into out.
// Returns an error if the response status is not 2xx.
func (c *BaseClient) DoRequest(ctx context.Context, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %s: %s", resp.Status, strings.TrimSpace(string(raw)))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// Get performs a GET request and decodes the response into out.
func (c *BaseClient) Get(ctx context.Context, path string, out any) error {
	return c.DoRequest(ctx, http.MethodGet, path, nil, out)
}

// Post performs a POST request with body and decodes the response into out.
func (c *BaseClient) Post(ctx context.Context, path string, body, out any) error {
	return c.DoRequest(ctx, http.MethodPost, path, body, out)
}

// Put performs a PUT request with body (no response decoding).
func (c *BaseClient) Put(ctx context.Context, path string, body any) error {
	return c.DoRequest(ctx, http.MethodPut, path, body, nil)
}

// BaseURL returns the configured base URL.
func (c *BaseClient) BaseURL() string {
	return c.baseURL
}

// HTTPClient returns the underlying http.Client (for custom requests).
func (c *BaseClient) HTTPClient() *http.Client {
	return c.http
}

// HasToken returns true if a bearer token is configured.
func (c *BaseClient) HasToken() bool {
	return c.token != ""
}
