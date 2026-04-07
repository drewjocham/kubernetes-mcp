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

	"kube-watcher-app/internal/util"
)

type Client struct {
	baseURL      string
	apiKey       string
	provider     string
	model        string
	backend      string
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
	// If API key is not set, client will fail when attempting to use it
	// User must set OPENCODE_API_KEY environment variable
	provider := os.Getenv("OPENCODE_PROVIDER")
	if provider == "" {
		provider = "opencode"
	}
	model := os.Getenv("OPENCODE_MODEL")
	if model == "" {
		model = "big-pickle"
	}
	backend := os.Getenv("OPENCODE_BACKEND")
	if backend == "" {
		backend = "openai"
	}
	return &Client{
		baseURL:      baseURL,
		apiKey:       apiKey,
		provider:     provider,
		model:        model,
		backend:      backend,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		pollInterval: 5 * time.Second,
		timeout:      5 * time.Minute,
	}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
}

type AnthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []AnthropicMessage `json:"messages"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
}

type AnthropicMessage struct {
	Role    string                    `json:"role"`
	Content []AnthropicMessageContent `json:"content"`
}

type AnthropicMessageContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type AnthropicResponse struct {
	Content []AnthropicResponseContent `json:"content"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type AnthropicResponseContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
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

	// Route based on provider
	fmt.Printf("[opencode] Ask: provider=%q, baseURL=%q, model=%q, backend=%q\n",
		c.provider, c.baseURL, c.model, c.backend)
	switch c.provider {
	case "anthropic":
		return c.anthropicCompletion(ctx, fullPrompt)
	case "opencode":
		// If baseURL contains /chat/completions, treat as OpenAI-compatible
		if strings.Contains(c.baseURL, "/chat/completions") {
			return c.chatCompletion(ctx, fullPrompt)
		}
		// Otherwise use Warp/Oz logic
		runID, err := c.startRun(ctx, fullPrompt)
		if err != nil {
			return "", fmt.Errorf("start opencode run: %w", err)
		}
		return c.pollRun(ctx, runID)
	default:
		// openai, azure, ollama, custom - treat as OpenAI-compatible
		return c.chatCompletion(ctx, fullPrompt)
	}
}

func (c *Client) chatCompletion(ctx context.Context, prompt string) (string, error) {
	reqBody := ChatRequest{
		Model: c.model,
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
	if c.provider == "azure" || strings.Contains(c.baseURL, "azure.com") {
		req.Header.Set("api-key", c.apiKey)
	} else if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("chat completion error (%d): %s", resp.StatusCode, util.CleanupMarkdown(string(body)))
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
	return util.CleanupMarkdown(content), nil
}

func (c *Client) anthropicCompletion(ctx context.Context, prompt string) (string, error) {
	// Anthropic API endpoint
	endpoint := c.baseURL
	if endpoint == "" || !strings.Contains(endpoint, "anthropic.com") {
		endpoint = "https://api.anthropic.com/v1/messages"
	}

	// Prepare messages: Anthropic expects array of content blocks
	content := []AnthropicMessageContent{
		{Type: "text", Text: prompt},
	}
	messages := []AnthropicMessage{
		{Role: "user", Content: content},
	}

	reqBody := AnthropicRequest{
		Model:     c.model,
		Messages:  messages,
		MaxTokens: 4096,
		System:    "You are a helpful AI assistant. Always respond with well-formatted markdown. Use code blocks for JSON, commands, and configuration. Use bullet points for lists. Do not use HTML tags like <br>, use markdown formatting instead.",
	}

	bodyBytes, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic completion error (%d): %s", resp.StatusCode, util.CleanupMarkdown(string(body)))
	}

	var anthropicResp AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return "", err
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("no content in anthropic response")
	}

	// Extract text from content blocks
	var builder strings.Builder
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			builder.WriteString(block.Text)
		}
	}

	return util.CleanupMarkdown(builder.String()), nil
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
		return "", fmt.Errorf("opencode start error (%d): %s", resp.StatusCode, util.CleanupMarkdown(string(body)))
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
				return "", fmt.Errorf("opencode run %s: %s", status.State, util.CleanupMarkdown(status.Error))
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

func (c *Client) SetConfig(baseURL, apiKey, provider, model, backend string) {
	fmt.Printf("[opencode] SetConfig called: provider=%q, apiKey=%q (len=%d), baseURL=%q, model=%q, backend=%q\n",
		provider, util.MaskAPIKey(apiKey), len(apiKey), baseURL, model, backend)
	fmt.Printf("[opencode] Before update: provider=%q, apiKey=%q, baseURL=%q\n",
		c.provider, util.MaskAPIKey(c.apiKey), c.baseURL)
	if provider != "" {
		c.provider = provider
	}
	if apiKey != "" {
		c.apiKey = apiKey
	}
	if model != "" {
		c.model = model
	}
	if backend != "" {
		c.backend = backend
	}
	// Set appropriate default base URL based on provider if not provided
	if baseURL != "" {
		c.baseURL = baseURL
	} else {
		// Set default URLs based on provider
		switch c.provider {
		case "openai":
			c.baseURL = "https://api.openai.com/v1/chat/completions"
		case "azure":
			// Azure OpenAI requires resource and deployment in URL
			// User should provide full URL: https://{resource}.openai.azure.com/openai/deployments/{deployment}/chat/completions?api-version=2023-05-15
			// Keep existing baseURL if already set
			if c.baseURL == "" {
				c.baseURL = "https://opencode.ai/zen/v1/chat/completions" // Fallback
			}
		case "anthropic":
			c.baseURL = "https://api.anthropic.com/v1/messages"
		case "ollama":
			c.baseURL = "http://localhost:11434/v1/chat/completions"
		case "custom":
			// For custom, keep existing or use opencode default
			if c.baseURL == "" {
				c.baseURL = "https://opencode.ai/zen/v1/chat/completions"
			}
		default:
			// opencode or unknown provider
			if c.baseURL == "" {
				c.baseURL = "https://opencode.ai/zen/v1/chat/completions"
			}
		}
	}
}
