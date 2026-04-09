package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ResponseMsg carries a completed agent response.
type ResponseMsg struct {
	Content string
	Err     error
}

// Message is a single turn in the chat conversation.
type Message struct {
	Role    string `json:"role"` // "user" | "assistant" | "system"
	Content string `json:"content"`
}

// Client sends chat messages to a configured HTTP endpoint.
type Client struct {
	endpoint string
	apiKey   string
	http     *http.Client
}

// New returns a Client from environment variables.
//
//	KW_AGENT_ENDPOINT — HTTP endpoint, default http://localhost:3000/api/agent
//	KW_AGENT_API_KEY  — bearer token
func New() *Client {
	ep := strings.TrimSpace(os.Getenv("KW_AGENT_ENDPOINT"))
	if ep == "" {
		ep = "http://localhost:3000/api/agent"
	}
	return &Client{
		endpoint: ep,
		apiKey:   strings.TrimSpace(os.Getenv("KW_AGENT_API_KEY")),
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

// NewWithConfig returns a Client using explicit values.
func NewWithConfig(endpoint, apiKey string) *Client {
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		http:     &http.Client{Timeout: 60 * time.Second},
	}
}

// Send posts the conversation history and returns a tea.Cmd that yields ResponseMsg.
func (c *Client) Send(history []Message) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		payload := map[string]any{
			"messages": history,
			"task":     "kubernetes_sre_assistance",
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("marshal: %w", err)}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(b))
		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("build request: %w", err)}
		}
		req.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return ResponseMsg{Err: fmt.Errorf("request: %w", err)}
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return ResponseMsg{Err: fmt.Errorf("agent endpoint returned HTTP %s", resp.Status)}
		}

		var out struct {
			Response string `json:"response"`
			Message  string `json:"message"`
			Content  string `json:"content"`
			// OpenAI-compatible shape
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return ResponseMsg{Err: fmt.Errorf("decode response: %w", err)}
		}

		content := out.Response
		if content == "" {
			content = out.Message
		}
		if content == "" {
			content = out.Content
		}
		if content == "" && len(out.Choices) > 0 {
			content = out.Choices[0].Message.Content
		}
		if content == "" {
			content = "(empty response from agent)"
		}
		return ResponseMsg{Content: content}
	}
}
