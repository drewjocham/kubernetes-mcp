package drawer

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/theme"
)

// AppendMsg adds lines to the drawer log.
type AppendMsg struct{ Line string }

// Model is the collapsible bottom drawer for ops log / popecli output.
type Model struct {
	viewport viewport.Model
	lines    []string
	open     bool
	width    int
	height   int
	title    string
}

// New creates a drawer.
func New(width, height int) *Model {
	return &Model{
		viewport: viewport.New(width, height-2),
		open:     false,
		width:    width,
		height:   height,
		title:    "Ops Log",
	}
}

// Toggle opens or closes the drawer.
func (m *Model) Toggle() { m.open = !m.open }

// IsOpen returns whether the drawer is currently open.
func (m Model) IsOpen() bool { return m.open }

// Append adds a line to the drawer.
func (m *Model) Append(line string) {
	m.lines = append(m.lines, line)
	if len(m.lines) > 500 {
		m.lines = m.lines[len(m.lines)-500:]
	}
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AppendMsg:
		m.Append(msg.Line)
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// Resize updates drawer dimensions.
func (m *Model) Resize(width, height int) {
	m.width = width
	m.height = height
	m.viewport = viewport.New(width, height-2)
	m.viewport.SetContent(strings.Join(m.lines, "\n"))
	m.viewport.GotoBottom()
}

func (m Model) View() string {
	if !m.open {
		return ""
	}

	titleBar := lipgloss.NewStyle().
		Background(theme.BgOverlay).
		Foreground(theme.TextSecondary).
		Width(m.width).
		Padding(0, 1).
		Render("▸ " + m.title + "  (ctrl+l to close)")

	return lipgloss.NewStyle().
		Background(theme.BgSurface).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(theme.BorderSubtle).
		Width(m.width).
		Height(m.height).
		Render(lipgloss.JoinVertical(lipgloss.Left, titleBar, m.viewport.View()))
}
