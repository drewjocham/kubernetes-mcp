package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Server struct {
	isRunning bool
	apiAddr   string
	apiToken  string
	client    *http.Client
}

func NewServer() *Server {
	baseURL := strings.TrimSpace(os.Getenv("KW_TOOLS_ENDPOINT"))
	if baseURL == "" {
		baseURL = "http://localhost:8080/v1"
	}
	return &Server{
		isRunning: false,
		apiAddr:   strings.TrimRight(baseURL, "/"),
		apiToken:  strings.TrimSpace(os.Getenv("KW_TOOLS_API_TOKEN")),
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Server) Start() error {
	if s.isRunning {
		return fmt.Errorf("server is already running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/status", s.apiAddr), nil)
	if err != nil {
		return fmt.Errorf("build status request: %w", err)
	}
	if s.apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiToken))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("connect mcp api: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("mcp api status check failed: %s", resp.Status)
	}
	s.isRunning = true
	return nil
}

func (s *Server) Stop() error {
	if !s.isRunning {
		return fmt.Errorf("server is not running")
	}
	s.isRunning = false
	return nil
}

func (s *Server) IsRunning() bool {
	return s.isRunning
}

func (s *Server) GetStatus() string {
	if s.isRunning {
		return "Running"
	}
	return "Stopped"
}

func (s *Server) ListTools() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/tools", s.apiAddr), nil)
	if err != nil {
		return nil
	}
	if s.apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiToken))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var payload struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}

	tools := make([]string, 0, len(payload.Tools))
	for _, tool := range payload.Tools {
		tools = append(tools, tool.Name)
	}
	return tools
}

func (s *Server) RunTool(ctx context.Context, toolName string) (string, error) {
	if !s.isRunning {
		return "", fmt.Errorf("server is not running")
	}
	url := fmt.Sprintf("%s/tools/%s", s.apiAddr, toolName)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiToken))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("tool request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read tool response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tool %s failed: %s: %s", toolName, resp.Status, strings.TrimSpace(string(body)))
	}
	return string(body), nil
}
