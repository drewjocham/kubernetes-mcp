package opencode

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
)

type Client struct {
	baseURL      string
	apiKey       string
	httpClient   *http.Client
	pollInterval time.Duration
	timeout      time.Duration
}

func New() *Client {
	baseURL := os.Getenv("OPENCODE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://opencode.ai/zen/v1/chat/completions"
	}
	apiKey := os.Getenv("OPENCODE_API_KEY")
	if apiKey == "" {
		apiKey = "sk-2oEB1XMGNjoYuDf7WzH3uTXGTK3X7jjnCMswFG4vef2uGTtHtnSIakDZ135FoBmo"
	}
	return &Client{
		baseURL:      baseURL,
		apiKey:       apiKey,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		pollInterval: 5 * time.Second,
		timeout:      5 * time.Minute,
	}
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

type StartRequest struct {
	Prompt string `json:"prompt"`
	Title  string `json:"title"`
	Config struct {
		Name       string `json:"name"`
		MCPServers []struct {
			Name string `json:"name"`
		} `json:"mcp_servers"`
	} `json:"config"`
}

type StartResponse struct {
	RunID string `json:"run_id"`
}

type StatusResponse struct {
	State  string `json:"state"`
	Output string `json:"output"`
	Error  string `json:"error"`
}

func (c *Client) Ask(ctx context.Context, prompt string, contextStr string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("OPENCODE_API_KEY is not set")
	}

	fullPrompt := prompt
	if contextStr != "" {
		fullPrompt = fmt.Sprintf("Context:\n%s\n\nQuestion: %s", contextStr, prompt)
	}

	// Detect if this is an OpenAI-compatible endpoint
	if strings.Contains(c.baseURL, "/chat/completions") {
		return c.chatCompletion(ctx, fullPrompt)
	}

	// 1. Start the run (Warp/Oz logic)
	runID, err := c.startRun(ctx, fullPrompt)
	if err != nil {
		return "", fmt.Errorf("start opencode run: %w", err)
	}

	// 2. Poll for results
	return c.pollRun(ctx, runID)
}

func (c *Client) chatCompletion(ctx context.Context, prompt string) (string, error) {
	reqBody := ChatRequest{
		Model: "big-pickle", // Default model as seen in issue description
		Messages: []ChatMessage{
			{Role: "system", Content: "You are a helpful AI assistant. Always respond with well-formatted markdown. Use code blocks for JSON, commands, and configuration. Use bullet points for lists. Do not use HTML tags like <br>, use markdown formatting instead."},
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
		return "", fmt.Errorf("chat completion error (%d): %s", resp.StatusCode, cleanupMarkdown(string(body)))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in chat completion response")
	}

	content := chatResp.Choices[0].Message.Content
	// Clean up markdown formatting issues
	return cleanupMarkdown(content), nil
}

func decodeHtmlEntities(input string) string {
	result := input
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&quot;", "\"")
	result = strings.ReplaceAll(result, "&#39;", "'")
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	return result
}

func cleanupMarkdown(input string) string {
	if input == "" {
		return input
	}

	// Decode HTML entities first
	result := decodeHtmlEntities(input)

	// Replace HTML line breaks with newlines
	result = strings.ReplaceAll(result, "<br>", "\n")
	result = strings.ReplaceAll(result, "<br/>", "\n")
	result = strings.ReplaceAll(result, "<br />", "\n")

	// Fix common malformed patterns
	// Remove duplicate consecutive code block markers
	// This is a simplified approach - for production, use proper regex
	lines := strings.Split(result, "\n")
	var cleanedLines []string
	inCodeBlock := false
	prevLine := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check if this line starts a code block
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				// Already in code block, might be duplicate opener
				// Skip if previous line was also a code block opener
				if strings.HasPrefix(strings.TrimSpace(prevLine), "```") {
					continue // Skip duplicate opener
				}
			}
			inCodeBlock = !inCodeBlock
		}

		cleanedLines = append(cleanedLines, line)
		prevLine = line
	}

	result = strings.Join(cleanedLines, "\n")

	// Remove any remaining HTML tags (simple approach)
	result = strings.ReplaceAll(result, "<strong>", "**")
	result = strings.ReplaceAll(result, "</strong>", "**")
	result = strings.ReplaceAll(result, "<b>", "**")
	result = strings.ReplaceAll(result, "</b>", "**")
	result = strings.ReplaceAll(result, "<em>", "*")
	result = strings.ReplaceAll(result, "</em>", "*")
	result = strings.ReplaceAll(result, "<i>", "*")
	result = strings.ReplaceAll(result, "</i>", "*")

	// Remove any other HTML tags (crude but works for common cases)
	for strings.Contains(result, "<") && strings.Contains(result, ">") {
		start := strings.Index(result, "<")
		end := strings.Index(result, ">")
		if start >= 0 && end > start {
			result = result[:start] + result[end+1:]
		} else {
			break
		}
	}

	// Normalize newlines (3+ newlines -> 2 newlines)
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return strings.TrimSpace(result)
}

func (c *Client) startRun(ctx context.Context, prompt string) (string, error) {
	reqBody := StartRequest{
		Prompt: prompt,
		Title:  "Arguskube Desktop Query",
	}
	reqBody.Config.Name = "arguskube-desktop-investigation"
	reqBody.Config.MCPServers = []struct {
		Name string `json:"name"`
	}{{Name: "kube-watcher"}}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/v1/agent/run", bytes.NewReader(bodyBytes))
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

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("opencode start error (%d): %s", resp.StatusCode, cleanupMarkdown(string(body)))
	}

	var startResp StartResponse
	if err := json.NewDecoder(resp.Body).Decode(&startResp); err != nil {
		return "", err
	}

	if startResp.RunID == "" {
		return "", fmt.Errorf("no run_id in response")
	}

	return startResp.RunID, nil
}

func (c *Client) pollRun(ctx context.Context, runID string) (string, error) {
	deadline := time.Now().Add(c.timeout)
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return "", fmt.Errorf("opencode run timed out after %s", c.timeout)
			}

			status, err := c.fetchStatus(ctx, runID)
			if err != nil {
				return "", err
			}

			switch strings.ToUpper(status.State) {
			case "SUCCEEDED":
				return status.Output, nil
			case "FAILED", "ERROR", "CANCELLED":
				return "", fmt.Errorf("opencode run %s: %s", status.State, cleanupMarkdown(status.Error))
			default:
				// Still running
			}
		}
	}
}

func (c *Client) fetchStatus(ctx context.Context, runID string) (StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/api/v1/agent/runs/%s", c.baseURL, runID), nil)
	if err != nil {
		return StatusResponse{}, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return StatusResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return StatusResponse{}, fmt.Errorf("opencode status error (%d): %s", resp.StatusCode, string(body))
	}

	// The status response from warp/oz is complex, mapping to our simple StatusResponse
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return StatusResponse{}, err
	}

	res := StatusResponse{
		State: getString(raw, "state"),
	}

	if res.State == "" {
		res.State = getString(raw, "run.state")
	}

	res.Output = getString(raw, "output")
	if res.Output == "" {
		res.Output = getString(raw, "response")
	}
	if res.Output == "" {
		if msg, ok := raw["status_message"].(map[string]any); ok {
			res.Output = getString(msg, "message")
		}
	}

	res.Error = getString(raw, "error")

	return res, nil
}

func getString(m map[string]any, path string) string {
	parts := strings.Split(path, ".")
	var current any = m
	for _, part := range parts {
		if mm, ok := current.(map[string]any); ok {
			current = mm[part]
		} else {
			return ""
		}
	}
	if s, ok := current.(string); ok {
		return s
	}
	return ""
}
