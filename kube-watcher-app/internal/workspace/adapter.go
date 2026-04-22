package workspace

import (
	"context"
	"fmt"

	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/data/mcp"
)

// MCPAdapter keeps the desktop workspace decoupled from the concrete MCP client.
type MCPAdapter struct {
	client *mcp.Client
}

// NewMCPAdapter builds an adapter around the MCP desktop client.
func NewMCPAdapter(client *mcp.Client) *MCPAdapter {
	return &MCPAdapter{client: client}
}

// Alerts loads current alerts from MCP.
func (a *MCPAdapter) Alerts(ctx context.Context) ([]data.AlertRecord, error) {
	return a.client.Alerts(ctx)
}

// History loads historical incidents from MCP.
func (a *MCPAdapter) History(ctx context.Context) ([]data.Incident, error) {
	return a.client.History(ctx)
}

// Recommendations loads remediation suggestions from MCP.
func (a *MCPAdapter) Recommendations(ctx context.Context) ([]data.Recommendation, error) {
	return a.client.Recommendations(ctx)
}

// Services loads platform service states from MCP.
func (a *MCPAdapter) Services(ctx context.Context) ([]data.ServiceStatus, error) {
	fmt.Printf("[adapter] Services called\n")
	return a.client.Services(ctx)
}
