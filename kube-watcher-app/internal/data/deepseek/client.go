package deepseek

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
	"kube-watcher-app/internal/data/mcp"
)

// Client is an AI agent that uses DeepSeek API and can automatically
// use MCP tools to investigate anomalies.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	mcpClient  *mcp.Client
}

// New creates a new DeepSeek client with MCP integration
func New(mcpClient *mcp.Client) *Client {
	//apiKey := os.Getenv("DEEPSEEK_API_KEY")
	apiKey := "sk-9573feef9ebf4544be9e8474717372b1"
	if apiKey == "" {
		apiKey = "sk-2oEB1XMGNjoYuDf7WzH3uTXGTK3X7jjnCMswFG4vef2uGTtHtnSIakDZ135FoBmo" // Default to OpenCode key for now
	}

	baseURL := os.Getenv("DEEPSEEK_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com/v1/chat/completions"
	}

	return &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		mcpClient:  mcpClient,
	}
}

// ChatRequest represents a request to DeepSeek API
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a response from DeepSeek API
type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

// ToolCall represents a function call from the AI
type ToolCall struct {
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolDefinition defines an MCP tool that the AI can use
type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function"`
}

// InvestigateAnomalies analyzes anomalies and automatically uses MCP tools
// to investigate and provide recommendations
func (c *Client) InvestigateAnomalies(ctx context.Context, anomalies []data.AlertRecord) (string, error) {
	if len(anomalies) == 0 {
		return "No anomalies detected. The cluster appears to be healthy.", nil
	}

	// Build context about the anomalies
	anomalyContext := buildAnomalyContext(anomalies)

	// Get available MCP tools
	tools, err := c.mcpClient.ListTools(ctx)
	if err != nil {
		tools = []data.ToolSummary{} // Continue without tools if we can't get them
	}

	// Build the prompt for DeepSeek
	prompt := buildInvestigationPrompt(anomalyContext, tools)

	// Get cluster status for additional context
	status, _ := c.mcpClient.Status(ctx)
	history, _ := c.mcpClient.History(ctx)

	// Add cluster context to prompt
	fullPrompt := fmt.Sprintf("%s\n\nCluster Status: %s\nRecent Incidents: %d\n\n%s",
		prompt, status.Cluster, len(history),
		"Analyze these anomalies and use available MCP tools to investigate. Provide actionable recommendations.")

	// Call DeepSeek with tool definitions
	return c.chatWithTools(ctx, fullPrompt, tools, anomalies)
}

// Ask sends a prompt to DeepSeek and returns the response
func (c *Client) Ask(ctx context.Context, prompt string, contextStr string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("DEEPSEEK_API_KEY is not set")
	}

	fullPrompt := prompt
	if contextStr != "" {
		fullPrompt = fmt.Sprintf("Context:\n%s\n\nQuestion: %s", contextStr, prompt)
	}

	return c.chatCompletion(ctx, fullPrompt)
}

// chatCompletion sends a simple chat completion request
func (c *Client) chatCompletion(ctx context.Context, prompt string) (string, error) {
	reqBody := ChatRequest{
		Model: "deepseek-chat", // DeepSeek model
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("DeepSeek API error (%d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in DeepSeek response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// chatWithTools sends a chat request with tool definitions and executes tool calls
func (c *Client) chatWithTools(ctx context.Context, prompt string, tools []data.ToolSummary, anomalies []data.AlertRecord) (string, error) {
	// Build tool definitions for the AI
	//toolDefinitions := buildToolDefinitions(tools)

	// Create messages for the conversation
	messages := []ChatMessage{
		{
			Role: "system",
			Content: `You are Arguskube, an AI Kubernetes expert. You have access to MCP tools to investigate cluster issues.
				When you detect anomalies, you should:
				1. Analyze the anomaly patterns
				2. Use appropriate MCP tools to gather more information
				3. Provide actionable recommendations
				4. Suggest specific commands or fixes

				Format your response in clear markdown with:
				- Headers for sections
				- Bullet points for lists
				- Code blocks for commands
				- Bold text for important information
				- Tables for comparisons`,
		},
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// For now, we'll do a simple chat completion
	// In a more advanced implementation, we would handle tool calls
	reqBody := ChatRequest{
		Model:    "deepseek-chat",
		Messages: messages,
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("DeepSeek API error (%d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in DeepSeek response")
	}

	response := chatResp.Choices[0].Message.Content

	// Enhance the response with actual MCP tool data if needed
	enhancedResponse, err := c.enhanceWithMCPData(ctx, response, anomalies)
	if err != nil {
		// If enhancement fails, return the original response
		return response, nil
	}

	return enhancedResponse, nil
}

// enhanceWithMCPData enhances the AI response with actual data from MCP tools
func (c *Client) enhanceWithMCPData(ctx context.Context, aiResponse string, anomalies []data.AlertRecord) (string, error) {
	// Check if we should gather more data based on the response
	if strings.Contains(strings.ToLower(aiResponse), "node") ||
		strings.Contains(strings.ToLower(aiResponse), "cluster status") {

		// Use node_status tool
		result, err := c.mcpClient.ExecuteTool(ctx, "node_status", map[string]any{})
		if err == nil && result.Output != nil {
			// Parse and add node status information
			if nodes, ok := result.Output["nodes"].([]interface{}); ok {
				healthyNodes := 0
				totalNodes := len(nodes)

				for _, node := range nodes {
					if nodeMap, ok := node.(map[string]interface{}); ok {
						if status, ok := nodeMap["status"].(string); ok && status == "Ready" {
							healthyNodes++
						}
					}
				}

				enhancement := fmt.Sprintf("\n\n**Actual Cluster Status from MCP:**\n- Total Nodes: %d\n- Healthy Nodes: %d\n- Unhealthy Nodes: %d\n",
					totalNodes, healthyNodes, totalNodes-healthyNodes)

				return aiResponse + enhancement, nil
			}
		}
	}

	// Check for pod-related issues
	if strings.Contains(strings.ToLower(aiResponse), "pod") ||
		strings.Contains(strings.ToLower(aiResponse), "container") ||
		strings.Contains(strings.ToLower(aiResponse), "deployment") {

		// Use pod_resources tool
		result, err := c.mcpClient.ExecuteTool(ctx, "pod_resources", map[string]any{})
		if err == nil && result.Output != nil {
			// Add pod resource information
			enhancement := "\n\n**Pod Resource Analysis from MCP:**\n"

			if summary, ok := result.Output["summary"].(map[string]interface{}); ok {
				if total, ok := summary["total_pods"].(float64); ok {
					enhancement += fmt.Sprintf("- Total Pods: %.0f\n", total)
				}
				if running, ok := summary["running_pods"].(float64); ok {
					enhancement += fmt.Sprintf("- Running Pods: %.0f\n", running)
				}
				if pending, ok := summary["pending_pods"].(float64); ok {
					enhancement += fmt.Sprintf("- Pending Pods: %.0f\n", pending)
				}
				if failed, ok := summary["failed_pods"].(float64); ok {
					enhancement += fmt.Sprintf("- Failed Pods: %.0f\n", failed)
				}
			}

			return aiResponse + enhancement, nil
		}
	}

	return aiResponse, nil
}

// buildAnomalyContext creates a descriptive context from anomalies
func buildAnomalyContext(anomalies []data.AlertRecord) string {
	if len(anomalies) == 0 {
		return "No anomalies detected."
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Detected %d anomalies:\n\n", len(anomalies)))

	// Group by severity
	severityCount := make(map[string]int)
	for _, anomaly := range anomalies {
		severityCount[anomaly.Severity]++
	}

	builder.WriteString("**Severity Breakdown:**\n")
	for severity, count := range severityCount {
		builder.WriteString(fmt.Sprintf("- %s: %d\n", severity, count))
	}
	builder.WriteString("\n")

	// List critical anomalies first
	builder.WriteString("**Critical Anomalies:**\n")
	for _, anomaly := range anomalies {
		if anomaly.Severity == "critical" || anomaly.Severity == "error" {
			builder.WriteString(fmt.Sprintf("- %s: %s (Namespace: %s)\n",
				anomaly.Name, anomaly.Message, anomaly.Namespace))
		}
	}
	builder.WriteString("\n")

	// List other anomalies
	builder.WriteString("**Other Anomalies:**\n")
	for _, anomaly := range anomalies {
		if anomaly.Severity != "critical" && anomaly.Severity != "error" {
			builder.WriteString(fmt.Sprintf("- %s: %s (Namespace: %s)\n",
				anomaly.Name, anomaly.Message, anomaly.Namespace))
		}
	}

	return builder.String()
}

// buildInvestigationPrompt creates a prompt for anomaly investigation
func buildInvestigationPrompt(anomalyContext string, tools []data.ToolSummary) string {
	var builder strings.Builder

	builder.WriteString("# Anomaly Investigation Request\n\n")
	builder.WriteString(anomalyContext)
	builder.WriteString("\n\n")

	if len(tools) > 0 {
		builder.WriteString("**Available MCP Tools:**\n")
		for _, tool := range tools {
			builder.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
		}
		builder.WriteString("\n")
	}

	builder.WriteString(`Please investigate these anomalies and provide:
1. Root cause analysis
2. Immediate actions needed
3. Long-term recommendations
4. Specific commands or configurations to fix issues
5. Prevention strategies

Use MCP tools to gather more information if needed.`)

	return builder.String()
}

// buildToolDefinitions creates tool definitions for the AI
func buildToolDefinitions(tools []data.ToolSummary) []ToolDefinition {
	var definitions []ToolDefinition

	// Define common tools based on available MCP tools
	toolMap := map[string]string{
		"node_status":      "Get status of all nodes in the cluster",
		"pod_resources":    "Get resource usage of all pods",
		"cluster_analysis": "Analyze cluster health and issues",
		"namespace_list":   "List all namespaces and their status",
		"cluster_events":   "Get recent cluster events",
		"pod_logs":         "Get logs from specific pods",
		"history":          "Get historical incident data",
		"recommendation":   "Get AI-generated recommendations",
	}

	for _, tool := range tools {
		if description, exists := toolMap[tool.Name]; exists {
			definitions = append(definitions, ToolDefinition{
				Type: "function",
				Function: map[string]interface{}{
					"name":        tool.Name,
					"description": description,
					"parameters": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{},
					},
				},
			})
		}
	}

	return definitions
}
