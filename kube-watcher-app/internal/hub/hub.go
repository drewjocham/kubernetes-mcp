package hub

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"kube-watcher-app/internal/data"
	mcpclient "kube-watcher-app/internal/data/mcp"
)

// Tick messages emitted to AppModel
type (
	AlertsUpdatedMsg    struct{ Records []data.AlertRecord }
	HistoryUpdatedMsg   struct{ Incidents []data.Incident }
	RecsUpdatedMsg      struct{ Recs []data.Recommendation }
	ConnectionStatusMsg struct {
		MCP        bool
		MCPLatency int64
	}
	DataErrorMsg struct {
		Source string
		Err    error
	}
)

// Hub owns all periodic data-fetch operations.
type Hub struct {
	mcp *mcpclient.Client
}

// New creates a Hub.
func New(mcp *mcpclient.Client) *Hub {
	return &Hub{mcp: mcp}
}

// FetchAlerts immediately fetches alerts (one-shot).
func (h *Hub) FetchAlerts() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		records, err := h.mcp.Alerts(ctx)
		if err != nil {
			return DataErrorMsg{Source: "alerts", Err: err}
		}
		return AlertsUpdatedMsg{Records: records}
	}
}

// PollAlerts returns a tea.Cmd that refetches alerts every 15s.
func (h *Hub) PollAlerts() tea.Cmd {
	return tea.Tick(15*time.Second, func(_ time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		records, err := h.mcp.Alerts(ctx)
		if err != nil {
			return DataErrorMsg{Source: "alerts", Err: err}
		}
		return AlertsUpdatedMsg{Records: records}
	})
}

// FetchHistory immediately fetches incident history (one-shot).
func (h *Hub) FetchHistory() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		incidents, err := h.mcp.History(ctx)
		if err != nil {
			return DataErrorMsg{Source: "history", Err: err}
		}
		return HistoryUpdatedMsg{Incidents: incidents}
	}
}

// PollHistory returns a tea.Cmd that refetches history every 60s.
func (h *Hub) PollHistory() tea.Cmd {
	return tea.Tick(60*time.Second, func(_ time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		incidents, err := h.mcp.History(ctx)
		if err != nil {
			return DataErrorMsg{Source: "history", Err: err}
		}
		return HistoryUpdatedMsg{Incidents: incidents}
	})
}

// FetchStatus pings the MCP server once.
func (h *Hub) FetchStatus() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		status, err := h.mcp.Status(ctx)
		if err != nil {
			return ConnectionStatusMsg{MCP: false}
		}
		return ConnectionStatusMsg{MCP: true, MCPLatency: status.Latency}
	}
}

// PingStatus returns a tea.Cmd that pings the server every 30s.
func (h *Hub) PingStatus() tea.Cmd {
	return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		status, err := h.mcp.Status(ctx)
		if err != nil {
			return ConnectionStatusMsg{MCP: false}
		}
		return ConnectionStatusMsg{MCP: true, MCPLatency: status.Latency}
	})
}

// FetchRecs immediately fetches recommendations (one-shot).
func (h *Hub) FetchRecs() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		recs, err := h.mcp.Recommendations(ctx)
		if err != nil {
			return DataErrorMsg{Source: "recommendations", Err: err}
		}
		return RecsUpdatedMsg{Recs: recs}
	}
}

// PollRecs returns a tea.Cmd that refetches recommendations every 30s.
func (h *Hub) PollRecs() tea.Cmd {
	return tea.Tick(30*time.Second, func(_ time.Time) tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		recs, err := h.mcp.Recommendations(ctx)
		if err != nil {
			return DataErrorMsg{Source: "recommendations", Err: err}
		}
		return RecsUpdatedMsg{Recs: recs}
	})
}

// Init starts all background polling.
func (h *Hub) Init() tea.Cmd {
	return tea.Batch(
		h.FetchStatus(),
		h.FetchAlerts(),
		h.FetchHistory(),
		h.FetchRecs(),
		h.PingStatus(),
		h.PollAlerts(),
		h.PollHistory(),
		h.PollRecs(),
	)
}
