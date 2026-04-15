package k8sgpt

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

	"kube-watcher-app/internal/util"
)

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
	timeout    time.Duration
}

func New() *Client {
	baseURL := os.Getenv("K8SGPT_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8089"
	}
	apiKey := os.Getenv("K8SGPT_API_KEY")
	model := os.Getenv("K8SGPT_MODEL")
	if model == "" {
		model = "gpt-3.5-turbo"
	}
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		timeout:    5 * time.Minute,
	}
}

// AnalyzeRequest is for direct K8sGPT analysis
type AnalyzeRequest struct {
	Filters   []string `json:"filters,omitempty"`
	Namespace string   `json:"namespace,omitempty"`
	Explain   bool     `json:"explain,omitempty"`
	Backend   string   `json:"backend,omitempty"`
	Model     string   `json:"model,omitempty"`
}

// AnalyzeResponse represents K8sGPT analysis result
type AnalyzeResponse struct {
	Status   string `json:"status"`
	Problems []struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Details     string `json:"details"`
		Severity    string `json:"severity"`
		Solution    string `json:"solution"`
	} `json:"problems"`
	Summary string `json:"summary"`
}

// MCPRequest for MCP server interaction
type MCPRequest struct {
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      string      `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
}

// MCPResponse from MCP server
type MCPResponse struct {
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	JSONRPC string          `json:"jsonrpc"`
}

type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolCallRequest for calling MCP tools
type ToolCallRequest struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
}

func (c *Client) Ask(ctx context.Context, prompt string, contextStr string) (string, error) {
	if !IsKubernetesQuestion(prompt) {
		return "", fmt.Errorf("question is not Kubernetes-related, use opencode client instead")
	}

	fullPrompt := prompt
	if contextStr != "" {
		fullPrompt = fmt.Sprintf("Context:\n%s\n\nQuestion: %s", contextStr, prompt)
	}

	// Try MCP server first, fall back to direct analysis
	response, err := c.queryMCP(ctx, fullPrompt)
	if err != nil {
		// Fall back to direct analysis
		return c.analyzeCluster(ctx, fullPrompt)
	}
	return response, nil
}

func (c *Client) queryMCP(ctx context.Context, prompt string) (string, error) {
	// Try to use K8sGPT's MCP tools for analysis
	toolCalls := []ToolCallRequest{
		{
			ToolName: "analyze_cluster",
			Args: map[string]interface{}{
				"explain": true,
			},
		},
	}

	// Adding context to the prompt
	enhancedPrompt := fmt.Sprintf("%s\n\nBased on the cluster analysis above, please answer: %s", prompt, prompt)

	mcpReq := MCPRequest{
		Method: "tools/call",
		Params: map[string]interface{}{
			"tools": toolCalls,
		},
		ID:      "1",
		JSONRPC: "2.0",
	}

	bodyBytes, _ := json.Marshal(mcpReq)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/mcp", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("MCP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("MCP error (%d): %s", resp.StatusCode, util.CleanupMarkdown(string(body)))
	}

	var mcpResp MCPResponse
	if err := json.NewDecoder(resp.Body).Decode(&mcpResp); err != nil {
		return "", fmt.Errorf("decode MCP response: %w", err)
	}

	if mcpResp.Error != nil {
		return "", fmt.Errorf("MCP error: %s", util.CleanupMarkdown(mcpResp.Error.Message))
	}

	// Extract analysis from response
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(mcpResp.Result, &result); err != nil {
		return "", fmt.Errorf("parse MCP result: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("no content in MCP response")
	}

	analysis := result.Content[0].Text
	response := fmt.Sprintf("K8sGPT Analysis:\n%s\n\nAnswer to your question: %s", analysis, enhancedPrompt)
	return util.CleanupMarkdown(response), nil
}

func (c *Client) analyzeCluster(ctx context.Context, prompt string) (string, error) {
	// Direct analysis using K8sGPT API
	reqBody := AnalyzeRequest{
		Explain: true,
		Backend: "openai", // Default backend
		Model:   c.model,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/analyze", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("analysis request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("analysis error (%d): %s", resp.StatusCode, util.CleanupMarkdown(string(body)))
	}

	var analysisResp AnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&analysisResp); err != nil {
		return "", fmt.Errorf("decode analysis response: %w", err)
	}

	// Format the analysis response
	var builder strings.Builder
	builder.WriteString("## K8sGPT Cluster Analysis\n\n")

	if analysisResp.Summary != "" {
		builder.WriteString(fmt.Sprintf("**Summary**: %s\n\n", analysisResp.Summary))
	}

	if len(analysisResp.Problems) > 0 {
		builder.WriteString("### Issues Found:\n")
		for _, problem := range analysisResp.Problems {
			builder.WriteString(fmt.Sprintf("- **%s** (%s): %s\n", problem.Name, problem.Severity, problem.Description))
			if problem.Solution != "" {
				builder.WriteString(fmt.Sprintf("  Solution: %s\n", problem.Solution))
			}
			if problem.Details != "" {
				builder.WriteString(fmt.Sprintf("  Details: %s\n", problem.Details))
			}
			builder.WriteString("\n")
		}
	} else {
		builder.WriteString("No issues found in the cluster.\n\n")
	}

	builder.WriteString(fmt.Sprintf("**Question**: %s\n\n", prompt))
	builder.WriteString("Based on the cluster analysis above, if your question relates to any of these issues, the solution is provided. If not, please provide more specific details about your Kubernetes question.")

	return util.CleanupMarkdown(builder.String()), nil
}

func IsKubernetesQuestion(prompt string) bool {
	lower := strings.ToLower(prompt)
	kubernetesKeywords := []string{
		"kube", "kubernetes", "cluster", "pod", "deployment", "service", "node",
		"namespace", "configmap", "secret", "ingress", "crd", "custom resource",
		"helm", "kubectl", "container", "docker", "image", "registry",
		"resource", "limit", "request", "hpa", "autoscaling", "network policy",
		"pv", "pvc", "storage", "volume", "statefulset", "daemonset", "job",
		"cronjob", "rbac", "serviceaccount", "role", "rolebinding",
		"taint", "toleration", "affinity", "node selector", "label",
		"annotation", "probe", "liveness", "readiness", "startup",
		"rollout", "update", "strategy", "rolling update", "blue-green",
		"canary", "istio", "linkerd", "service mesh", "cncf",
		"operator", "controller", "reconcile", "finalizer",
		"event", "log", "metric", "monitoring", "prometheus",
		"alert", "alertmanager", "grafana", "dashboard",
		"backup", "restore", "disaster recovery", "dr",
		"security", "policy", "opa", "gatekeeper", "kyverno",
		"compliance", "audit", "cis", "benchmark",
	}

	for _, keyword := range kubernetesKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	return false
}

func (c *Client) SetConfig(baseURL, apiKey, model string) {
	if baseURL != "" {
		c.baseURL = baseURL
	}
	if apiKey != "" {
		c.apiKey = apiKey
	}
	if model != "" {
		c.model = model
	}
}
