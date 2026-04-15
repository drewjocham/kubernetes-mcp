package agent

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"kube-watcher-app/internal/httpclient"

	tea "github.com/charmbracelet/bubbletea"
)

type ResponseMsg struct {
	Content string
	Err     error
}

type Message struct {
	Role    string `json:"role"` // "user" | "assistant" | "system"
	Content string `json:"content"`
}

type Client struct {
	*httpclient.BaseClient
}

// KW_AGENT_ENDPOINT — HTTP endpoint, default http://localhost:3000/api/agent
// KW_AGENT_API_KEY  — bearer token
func New() *Client {
	ep := strings.TrimSpace(os.Getenv("KW_AGENT_ENDPOINT"))
	if ep == "" {
		ep = "http://localhost:3000/api/agent"
	}
	apiKey := strings.TrimSpace(os.Getenv("KW_AGENT_API_KEY"))
	return &Client{
		BaseClient: httpclient.NewBaseClient(ep, apiKey, 60*time.Second),
	}
}

func NewWithConfig(endpoint, apiKey string) *Client {
	return &Client{
		BaseClient: httpclient.NewBaseClient(endpoint, apiKey, 60*time.Second),
	}
}

func (c *Client) Send(history []Message) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		payload := map[string]any{
			"messages": history,
			"task":     "kubernetes_sre_assistance",
		}
		var out struct {
			Response string `json:"response"`
			Message  string `json:"message"`
			Content  string `json:"content"`

			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := c.DoRequest(ctx, http.MethodPost, "", payload, &out); err != nil {
			return ResponseMsg{Err: err}
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
