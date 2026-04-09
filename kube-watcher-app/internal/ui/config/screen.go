package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/hub"
	"kube-watcher-app/internal/theme"
)

// Screen shows active connection configuration and live health.
type Screen struct {
	viewport  viewport.Model
	connOK    bool
	latencyMs int64
	width     int
	height    int
}

// New creates a Config screen.
func New(width, height int) *Screen {
	return &Screen{
		viewport: viewport.New(width-2, height-4),
		width:    width,
		height:   height,
	}
}

// SetStatus updates the connection health display.
func (s *Screen) SetStatus(msg hub.ConnectionStatusMsg) {
	s.connOK = msg.MCP
	s.latencyMs = msg.MCPLatency
	s.refresh()
}

func (s *Screen) refresh() {
	s.viewport.SetContent(s.render())
}

func (s Screen) Init() tea.Cmd { return nil }

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s Screen) View() string {
	header := lipgloss.NewStyle().
		Foreground(theme.AccentBright).Bold(true).
		Width(s.width).Padding(0, 1).
		Render("◎ Configuration")

	return theme.PanelStyle().Width(s.width).Height(s.height - 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, s.viewport.View()))
}

func (s *Screen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.viewport = viewport.New(width-2, height-4)
	s.refresh()
}

func (s Screen) HelpBindings() []key.Binding { return nil }

func (s Screen) Title() string { return "Config" }

func (s Screen) render() string {
	var sb strings.Builder

	section := func(title string) {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.AccentBright).Bold(true).
			Render("  "+title) + "\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(theme.BorderSubtle).
			Render("  "+strings.Repeat("─", 40)) + "\n")
	}

	field := func(label, value, source string) {
		l := lipgloss.NewStyle().Foreground(theme.TextSecondary).Render(fmt.Sprintf("  %-24s", label))
		v := lipgloss.NewStyle().Foreground(theme.TextPrimary).Render(value)
		s2 := lipgloss.NewStyle().Foreground(theme.TextMuted).Render("  " + source)
		sb.WriteString(l + v + s2 + "\n")
	}

	masked := func(v string) string {
		if v == "" {
			return "(not set)"
		}
		if len(v) <= 8 {
			return strings.Repeat("*", len(v))
		}
		return v[:4] + strings.Repeat("*", len(v)-4)
	}

	// MCP connection
	section("MCP Server")
	endpoint := getEnv("KW_TOOLS_ENDPOINT", "http://localhost:8080/v1")
	token := getEnv("KW_TOOLS_API_TOKEN", "")
	field("Endpoint", endpoint, "env: KW_TOOLS_ENDPOINT")
	field("Token", masked(token), "env: KW_TOOLS_API_TOKEN")

	// Health
	connStr := lipgloss.NewStyle().Foreground(theme.ColorCritical).Render("✗ disconnected")
	if s.connOK {
		connStr = lipgloss.NewStyle().Foreground(theme.ColorSuccess).
			Render(fmt.Sprintf("✓ connected (%dms)", s.latencyMs))
	}
	field("Status", connStr, "live ping")

	// Agent
	section("Agent Endpoint")
	agentEp := getEnv("KW_AGENT_ENDPOINT", "http://localhost:3000/api/agent")
	agentKey := getEnv("KW_AGENT_API_KEY", "")
	field("Endpoint", agentEp, "env: KW_AGENT_ENDPOINT")
	field("API Key", masked(agentKey), "env: KW_AGENT_API_KEY")

	// Prometheus
	section("Prometheus")
	promURL := getEnv("KW_PROMETHEUS_URL", "http://localhost:9090")
	field("URL", promURL, "env: KW_PROMETHEUS_URL")

	// Popecli
	section("Popecli")
	popeciPath := getEnv("POPECLI_PATH", "popecli (auto-discovered from PATH)")
	field("Binary", popeciPath, "env: POPECLI_PATH or PATH lookup")

	// Environment reference
	section("Environment Variables")
	envVars := []struct{ k, desc string }{
		{"KW_TOOLS_ENDPOINT", "MCP server base URL"},
		{"KW_TOOLS_API_TOKEN", "MCP bearer token"},
		{"KW_AGENT_ENDPOINT", "Agent chat endpoint"},
		{"KW_AGENT_API_KEY", "Agent API key"},
		{"KW_PROMETHEUS_URL", "Prometheus base URL"},
	}
	for _, ev := range envVars {
		v := getEnv(ev.k, "(not set)")
		if strings.Contains(ev.k, "TOKEN") || strings.Contains(ev.k, "KEY") {
			v = masked(v)
		}
		field(ev.k, v, ev.desc)
	}

	return sb.String()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
