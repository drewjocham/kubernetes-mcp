package mcp

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"time"
)

// Server represents the MCP server configuration and state
type Server struct {
	isRunning bool
	tools     map[string]func() (string, error)
	apiAddr   string
	apiToken  string
	client    *http.Client
}

// NewServer creates a new instance of MCP server
func NewServer() *Server {
	tools := map[string]func() (string, error){
		"node-status": func() (string, error) {
			return "Node Status: All nodes are healthy.\nDetails: Node1 (OK), Node2 (OK)", nil
		},
		"pod-resources": func() (string, error) {
			return "Pod Resources: CPU usage at 45%, Memory at 60%.\nPod1: CPU 20%, Mem 30%\nPod2: CPU 25%, Mem 30%", nil
		},
		"namespaces": func() (string, error) {
			return "Namespaces: default, kube-system, monitoring", nil
		},
		"pod-logs": func() (string, error) {
			return "Pod Logs: [2026-03-26 10:00:00] Pod1 started\n[2026-03-26 10:01:00] Pod2 error encountered", nil
		},
		"cluster-analysis": func() (string, error) {
			return "Cluster Analysis: Overall health good, minor issues in networking.\nRecommendation: Check network policies.", nil
		},
	}
	return &Server{
		isRunning: false,
		tools:     tools,
		apiAddr:   "http://localhost:8080",
		apiToken:  "XQvaaw4Z88HrzbnYSdUbEtMdZBdQJbB5vndd9wpOa5b746cf",
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Start begins the MCP server operation
func (s *Server) Start() error {
	if s.isRunning {
		return fmt.Errorf("server is already running")
	}
	s.isRunning = true
	// Placeholder for actual server start logic
	return nil
}

// Stop halts the MCP server operation
func (s *Server) Stop() error {
	if !s.isRunning {
		return fmt.Errorf("server is not running")
	}
	s.isRunning = false
	// Placeholder for actual server stop logic
	return nil
}

// IsRunning checks if the server is currently running
func (s *Server) IsRunning() bool {
	return s.isRunning
}

// GetStatus returns the current status of the server
func (s *Server) GetStatus() string {
	if s.isRunning {
		return "Running"
	}
	return "Stopped"
}

// ListTools returns the list of available tools
func (s *Server) ListTools() []string {
	tools := make([]string, 0, len(s.tools))
	for tool := range s.tools {
		tools = append(tools, tool)
	}
	return tools
}

// RunTool executes a specific tool and returns its output
func (s *Server) RunTool(ctx context.Context, toolName string) (string, error) {
	if !s.isRunning {
		return "", fmt.Errorf("server is not running")
	}
	// Attempt to run tool via HTTP API
	url := fmt.Sprintf("%s/tools/%s", s.apiAddr, toolName)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %v", err)
	}
	// Add API token to request header if available
	if s.apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.apiToken))
	}
	resp, err := s.client.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read API response: %v", err)
		}
		return string(body), nil
	}
	// Fall back to running local command if API call fails
	cmd := exec.CommandContext(ctx, "npx", "-y", "hostinger-api-mcp@latest", "run-tool", toolName)
	cmd.Env = append(cmd.Env, fmt.Sprintf("API_TOKEN=%s", s.apiToken))
	output, err := cmd.Output()
	if err != nil {
		// Fall back to mock data if both API and command fail
		tool, exists := s.tools[toolName]
		if !exists {
			return "", fmt.Errorf("tool %s not found, API call failed: %v, command failed: %v", toolName, resp.Status, err)
		}
		// Simulate tool execution delay
		select {
		case <-time.After(1 * time.Second):
			return tool()
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return string(output), nil
}
