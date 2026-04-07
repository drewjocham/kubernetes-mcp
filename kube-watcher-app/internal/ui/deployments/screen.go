package deployments

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/data"
	mcpclient "kube-watcher-app/internal/data/mcp"
	"kube-watcher-app/internal/theme"
)

// serviceActionDoneMsg is sent when a start/stop/restart completes.
type serviceActionDoneMsg struct {
	name string
	err  error
}

// serviceLogsMsg carries fetched log lines for a service.
type serviceLogsMsg struct {
	name  string
	lines []string
}

// mode controls what the bottom panel shows.
type mode int

const (
	modeTable mode = iota
	modeLogs
)

// Screen shows Docker service status with live control.
type Screen struct {
	mcp      *mcpclient.Client
	services []data.ServiceStatus

	viewport    viewport.Model
	logsView    viewport.Model
	spinner     spinner.Model
	selected    int
	width       int
	height      int
	mode        mode
	actionName  string // service being actioned
	logService  string // service whose logs are shown
	logLines    []string
	busy        bool
	busyService string
}

// New creates a Deployments screen.
func New(mcp *mcpclient.Client, width, height int) *Screen {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.AccentBright)

	s := &Screen{
		mcp:      mcp,
		viewport: viewport.New(width-2, tableHeight(height)),
		logsView: viewport.New(width-2, logsHeight(height)),
		spinner:  sp,
		width:    width,
		height:   height,
	}
	s.refresh()
	return s
}

// SetServices updates the service list from hub polling.
func (s *Screen) SetServices(svcs []data.ServiceStatus) {
	s.services = svcs
	s.refresh()
}

func tableHeight(h int) int {
	if h < 12 {
		return h - 4
	}
	return (h - 4) / 2
}

func logsHeight(h int) int {
	return h - tableHeight(h) - 7
}

func (s *Screen) refresh() {
	s.viewport.SetContent(s.renderTable())
}

func (s *Screen) refreshLogs() {
	if len(s.logLines) == 0 {
		s.logsView.SetContent(lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).
			Render("  No log lines."))
		return
	}
	var sb strings.Builder
	for _, l := range s.logLines {
		sb.WriteString("  " + l + "\n")
	}
	s.logsView.SetContent(sb.String())
	s.logsView.GotoBottom()
}

func (s Screen) Init() tea.Cmd { return s.spinner.Tick }

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selected > 0 {
				s.selected--
				s.refresh()
			}
			return s, nil
		case "down", "j":
			if s.selected < len(s.services)-1 {
				s.selected++
				s.refresh()
			}
			return s, nil
		case "s":
			if len(s.services) == 0 {
				return s, nil
			}
			if !s.busy && s.selected < len(s.services) {
				svc := s.services[s.selected]
				s.busy = true
				s.busyService = svc.Name
				return s, s.doServiceAction(svc.Name, "start")
			}
		case "x":
			if len(s.services) == 0 {
				return s, nil
			}
			if !s.busy && s.selected < len(s.services) {
				svc := s.services[s.selected]
				s.busy = true
				s.busyService = svc.Name
				return s, s.doServiceAction(svc.Name, "stop")
			}
		case "r":
			if len(s.services) == 0 {
				return s, nil
			}
			if !s.busy && s.selected < len(s.services) {
				svc := s.services[s.selected]
				s.busy = true
				s.busyService = svc.Name
				return s, s.doServiceAction(svc.Name, "restart")
			}
		case "l":
			if len(s.services) == 0 {
				return s, nil
			}
			if s.selected < len(s.services) {
				svc := s.services[s.selected]
				s.mode = modeLogs
				s.logService = svc.Name
				s.logLines = nil
				s.refreshLogs()
				return s, s.fetchLogs(svc.Name)
			}
		case "esc", "q":
			if s.mode == modeLogs {
				s.mode = modeTable
			}
			return s, nil
		}

	case serviceActionDoneMsg:
		s.busy = false
		s.busyService = ""
		s.refresh()
		// Trigger a fresh service fetch via the hub — emit a no-op that main.go
		// catches to re-poll. For now just mark clean.
		return s, nil

	case serviceLogsMsg:
		if msg.name == s.logService {
			s.logLines = msg.lines
			s.refreshLogs()
		}
		return s, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spinner, cmd = s.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	var vpCmd tea.Cmd
	if s.mode == modeLogs {
		s.logsView, vpCmd = s.logsView.Update(msg)
	} else {
		s.viewport, vpCmd = s.viewport.Update(msg)
	}
	cmds = append(cmds, vpCmd)

	return s, tea.Batch(cmds...)
}

func (s *Screen) doServiceAction(name, action string) tea.Cmd {
	cli := s.mcp
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		var err error
		switch action {
		case "start":
			err = cli.StartService(ctx, name)
		case "stop":
			err = cli.StopService(ctx, name)
		case "restart":
			err = cli.RestartService(ctx, name)
		}
		return serviceActionDoneMsg{name: name, err: err}
	}
}

func (s *Screen) fetchLogs(name string) tea.Cmd {
	cli := s.mcp
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		lines, err := cli.FetchServiceLogs(ctx, name, "150")
		if err != nil {
			return serviceLogsMsg{name: name, lines: []string{"error: " + err.Error()}}
		}
		return serviceLogsMsg{name: name, lines: lines}
	}
}

func (s Screen) View() string {
	header := lipgloss.NewStyle().
		Foreground(theme.AccentBright).Bold(true).
		Width(s.width).Padding(0, 1).
		Render("⬡ Services")

	var hint string
	if s.mode == modeLogs {
		hint = lipgloss.NewStyle().Foreground(theme.TextMuted).
			Render("  ↑↓=scroll  esc=back to services")
	} else if s.busy {
		hint = s.spinner.View() + lipgloss.NewStyle().Foreground(theme.TextMuted).
			Render(fmt.Sprintf("  %s…", s.busyService))
	} else if len(s.services) == 0 {
		hint = lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).
			Render("  Docker Compose support removed. See Kubernetes services via cluster analysis tool.")
	} else {
		hint = lipgloss.NewStyle().Foreground(theme.TextMuted).
			Render("  ↑↓/jk=navigate  s=start  x=stop  r=restart  l=logs")
	}

	var body string
	if s.mode == modeLogs {
		logsHeader := lipgloss.NewStyle().Foreground(theme.AccentPrimary).Bold(true).
			Render(fmt.Sprintf("  Logs: %s", s.logService))
		body = lipgloss.JoinVertical(lipgloss.Left,
			header,
			s.viewport.View(),
			logsHeader,
			s.logsView.View(),
			hint,
		)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left,
			header,
			s.viewport.View(),
			hint,
		)
	}

	return theme.PanelStyle().Width(s.width).Height(s.height - 2).Render(body)
}

func (s *Screen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.viewport = viewport.New(width-2, tableHeight(height))
	s.logsView = viewport.New(width-2, logsHeight(height))
	s.refresh()
	s.refreshLogs()
}

func (s Screen) HelpBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "previous")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "next")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "start service")),
		key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop service")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart service")),
		key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "view logs")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

func (s Screen) Title() string { return "Services" }

func (s Screen) renderTable() string {
	colHeader := lipgloss.NewStyle().Foreground(theme.TextSecondary).
		Render(fmt.Sprintf("  %-22s %-20s %-12s %s", "SERVICE", "IMAGE", "STATUS", "STARTED"))
	divider := lipgloss.NewStyle().Foreground(theme.BorderSubtle).
		Render(strings.Repeat("─", s.width-4))

	var rows []string
	rows = append(rows, colHeader, divider)

	if len(s.services) == 0 {
		rows = append(rows, lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).Padding(1, 2).
			Render("No services found. Docker Compose support has been removed."))
		return strings.Join(rows, "\n")
	}

	for i, svc := range s.services {
		statusStyle := lipgloss.NewStyle().Foreground(theme.TextMuted)
		statusLabel := "○  " + svc.Status
		switch svc.Status {
		case "running":
			statusStyle = theme.ConnectedStyle()
			statusLabel = "●  running"
		case "exited", "stopped":
			statusStyle = theme.DisconnectedStyle()
			statusLabel = "●  stopped"
		}

		busy := s.busy && s.busyService == svc.Name
		if busy {
			statusStyle = lipgloss.NewStyle().Foreground(theme.AccentBright)
			statusLabel = "⟳  working"
		}

		var rowStyle lipgloss.Style
		if i == s.selected {
			rowStyle = theme.TableSelectedStyle().Width(s.width - 4)
		} else {
			rowStyle = theme.TableRowStyle().Width(s.width - 4)
		}

		name := truncate(svc.Name, 21)
		image := truncate(svc.Image, 19)
		started := "-"
		if svc.StartedAt != "" {
			started = svc.StartedAt
			if len(started) > 19 {
				started = started[:19]
			}
		}

		row := rowStyle.Render(
			"  " + fmt.Sprintf("%-22s", name) +
				fmt.Sprintf("%-20s", image) +
				statusStyle.Render(fmt.Sprintf("%-12s", statusLabel)) +
				" " + started,
		)
		rows = append(rows, row)
	}

	return strings.Join(rows, "\n")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
