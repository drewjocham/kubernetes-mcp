package logs

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/theme"
)

// AppendLogMsg carries new log lines from external sources.
type AppendLogMsg struct{ Lines []data.LogLine }

// Screen is the live ops log viewer.
type Screen struct {
	lines      []data.LogLine
	filtered   []data.LogLine
	viewport   viewport.Model
	filter     textinput.Model
	filterMode bool
	autoScroll bool
	width      int
	height     int
}

// New creates a Logs screen.
func New(width, height int) *Screen {
	ti := textinput.New()
	ti.Placeholder = "grep filter…"
	ti.Width = width - 20

	vp := viewport.New(width-2, height-5)
	return &Screen{
		viewport:   vp,
		filter:     ti,
		autoScroll: true,
		width:      width,
		height:     height,
	}
}

// AppendLines adds new log lines.
func (s *Screen) AppendLines(lines []data.LogLine) {
	s.lines = append(s.lines, lines...)
	if len(s.lines) > 5000 {
		s.lines = s.lines[len(s.lines)-5000:]
	}
	s.applyFilter()
}

func (s *Screen) applyFilter() {
	q := strings.ToLower(s.filter.Value())
	if q == "" {
		s.filtered = s.lines
	} else {
		s.filtered = nil
		for _, l := range s.lines {
			if strings.Contains(strings.ToLower(l.Raw), q) {
				s.filtered = append(s.filtered, l)
			}
		}
	}
	s.viewport.SetContent(s.renderLines())
	if s.autoScroll {
		s.viewport.GotoBottom()
	}
}

func (s Screen) Init() tea.Cmd { return nil }

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AppendLogMsg:
		s.AppendLines(msg.Lines)
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "/":
			if !s.filterMode {
				s.filterMode = true
				s.filter.Focus()
				return s, nil
			}
		case "esc":
			if s.filterMode {
				s.filterMode = false
				s.filter.Blur()
				return s, nil
			}
		case "ctrl+e":
			s.autoScroll = !s.autoScroll
			return s, nil
		}
		if s.filterMode {
			var tiCmd tea.Cmd
			s.filter, tiCmd = s.filter.Update(msg)
			s.applyFilter()
			return s, tiCmd
		}
	}

	var vpCmd tea.Cmd
	s.viewport, vpCmd = s.viewport.Update(msg)
	return s, vpCmd
}

func (s Screen) View() string {
	totalLines := len(s.lines)
	shown := len(s.filtered)
	autoMark := "auto-scroll: OFF"
	if s.autoScroll {
		autoMark = lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render("auto-scroll: ON")
	}

	header := lipgloss.NewStyle().
		Foreground(theme.AccentBright).Bold(true).Width(s.width).Padding(0, 1).
		Render(fmt.Sprintf("Ops Log  (%d/%d lines)  %s", shown, totalLines, autoMark))

	filterBar := s.renderFilterBar()
	hint := lipgloss.NewStyle().Foreground(theme.TextMuted).
		Render("  /=filter  ctrl+e=toggle auto-scroll  ↑↓=scroll")

	return theme.PanelStyle().Width(s.width).Height(s.height - 2).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			header,
			filterBar,
			s.viewport.View(),
			hint,
		))
}

func (s Screen) renderFilterBar() string {
	if s.filterMode {
		return theme.ChatInputStyle(s.width).Render(s.filter.View())
	}
	if s.filter.Value() != "" {
		return lipgloss.NewStyle().
			Foreground(theme.AccentBright).
			Background(theme.BgElevated).
			Padding(0, 1).
			Render("filter: " + s.filter.Value() + "  (esc to clear)")
	}
	return lipgloss.NewStyle().Foreground(theme.TextMuted).Render("  Press / to filter")
}

func (s *Screen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.filter.Width = width - 20
	s.viewport = viewport.New(width-2, height-6)
	s.applyFilter()
}

func (s Screen) HelpBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("ctrl+e"), key.WithHelp("ctrl+e", "toggle auto-scroll")),
	}
}

func (s Screen) Title() string { return "Ops Log" }

func (s Screen) renderLines() string {
	if len(s.filtered) == 0 {
		return lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).
			Padding(2, 2).Render("No log lines. Connect a log source or ingest events.")
	}

	var sb strings.Builder
	for _, line := range s.filtered {
		ts := lipgloss.NewStyle().Foreground(theme.TextMuted).Render(line.Timestamp.Format(time.RFC3339)[:19])
		level := colorLevel(line.Level)
		src := lipgloss.NewStyle().Foreground(theme.TextSecondary).Render(pad(line.Source, 12))
		msg := lipgloss.NewStyle().Foreground(theme.TextPrimary).Render(line.Message)
		sb.WriteString(ts + " " + level + " " + src + " " + msg + "\n")
	}
	return sb.String()
}

func colorLevel(level string) string {
	switch strings.ToUpper(level) {
	case "ERROR":
		return lipgloss.NewStyle().Foreground(theme.ColorCritical).Bold(true).Render("ERR ")
	case "WARN", "WARNING":
		return lipgloss.NewStyle().Foreground(theme.ColorMedium).Render("WARN")
	case "DEBUG":
		return lipgloss.NewStyle().Foreground(theme.TextMuted).Render("DBG ")
	default:
		return lipgloss.NewStyle().Foreground(theme.ColorInfo).Render("INFO")
	}
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}
