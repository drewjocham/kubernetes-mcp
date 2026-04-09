package sidebar

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/theme"
)

// SelectMsg is emitted when the user confirms a sidebar selection.
type SelectMsg struct{ Index int }

// Item represents a single sidebar navigation entry.
type Item struct {
	Icon  string
	Label string
	Badge int // unread/alert count badge; 0 = hidden
}

var defaultItems = []Item{
	{Icon: "⚠", Label: "Alerts"},
	{Icon: "◈", Label: "Anomalies"},
	{Icon: "▲", Label: "Charts"},
	{Icon: "≡", Label: "Logs"},
	{Icon: "✦", Label: "Hints"},
	{Icon: "⌘", Label: "Popecli"},
	{Icon: "⬡", Label: "Deployments"},
	{Icon: "◎", Label: "Config"},
}

// Model is the sidebar navigation component.
type Model struct {
	items     []Item
	cursor    int
	active    int
	width     int
	height    int
	confirmed bool
}

// New creates a sidebar with default items.
func New(width, height int) Model {
	return Model{
		items:  defaultItems,
		width:  width,
		height: height,
	}
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	m.confirmed = false
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.active = m.cursor
			m.confirmed = true
			return m, func() tea.Msg { return SelectMsg{Index: m.active} }
		case "1", "2", "3", "4", "5", "6", "7", "8":
			idx := int(msg.String()[0] - '1')
			if idx >= 0 && idx < len(m.items) {
				m.cursor = idx
				m.active = idx
				m.confirmed = true
				return m, func() tea.Msg { return SelectMsg{Index: m.active} }
			}
		}
	}
	return m, nil
}

// SetActive sets the active item programmatically (e.g. from global number keys).
func (m *Model) SetActive(idx int) {
	if idx >= 0 && idx < len(m.items) {
		m.cursor = idx
		m.active = idx
	}
}

// SetBadge updates the badge count for a given item index.
func (m *Model) SetBadge(idx, count int) {
	if idx >= 0 && idx < len(m.items) {
		m.items[idx].Badge = count
	}
}

// Confirmed returns true if the user just confirmed a selection this frame.
func (m Model) Confirmed() bool { return m.confirmed }

// SelectedIdx returns the currently active (confirmed) item index.
func (m Model) SelectedIdx() int { return m.active }

func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "nav"
	}

	// Simple header
	header := lipgloss.NewStyle().
		Foreground(theme.TextPrimary).
		Bold(true).
		Width(m.width).
		Padding(0, 1).
		Render("kube-watcher")

	// Subtitle
	subtitle := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Width(m.width).
		Padding(0, 1).
		Render("k8s monitor")

	var rows []string
	rows = append(rows, header)
	rows = append(rows, subtitle)
	rows = append(rows, "")

	// Simple navigation items
	for i, item := range m.items {
		label := fmt.Sprintf("%d %s %s", i+1, item.Icon, item.Label)
		if item.Badge > 0 {
			label += fmt.Sprintf(" (%d)", item.Badge)
		}

		style := lipgloss.NewStyle().Width(m.width).Padding(0, 1)
		if i == m.active {
			style = style.
				Foreground(theme.TextPrimary).
				Background(theme.AccentPrimary).
				Bold(true)
		} else {
			style = style.Foreground(theme.TextMuted)
		}

		rows = append(rows, style.Render(label))
	}

	// Fill remaining height
	rendered := lipgloss.JoinVertical(lipgloss.Left, rows...)
	lines := strings.Count(rendered, "\n") + 1
	remaining := m.height - lines
	if remaining > 0 {
		filler := lipgloss.NewStyle().
			Background(theme.BgAccent).
			Width(m.width).
			Height(remaining).
			Render("")
		rendered = lipgloss.JoinVertical(lipgloss.Left, rendered, filler)
	}

	return lipgloss.NewStyle().
		Background(theme.BgAccent).
		Width(m.width).
		Height(m.height).
		Render(rendered)
}
