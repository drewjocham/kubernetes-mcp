package modal

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/theme"
)

// CloseMsg is emitted when the user dismisses the modal.
type CloseMsg struct{}

// HelpModal shows all key bindings.
type HelpModal struct {
	viewport viewport.Model
	open     bool
	width    int
	height   int
	bindings [][]key.Binding
}

// NewHelp creates a HelpModal.
func NewHelp(width, height int) *HelpModal {
	w := min(width-8, 70)
	h := min(height-6, 30)
	return &HelpModal{
		viewport: viewport.New(w-4, h-4),
		width:    w,
		height:   h,
	}
}

// Open shows the modal with the given bindings.
func (m *HelpModal) Open(global []key.Binding, screen []key.Binding) {
	m.open = true
	m.bindings = [][]key.Binding{global, screen}
	m.viewport.SetContent(m.renderBindings())
}

// Close hides the modal.
func (m *HelpModal) Close() { m.open = false }

// IsOpen returns whether the modal is visible.
func (m HelpModal) IsOpen() bool { return m.open }

func (m HelpModal) Init() tea.Cmd { return nil }

func (m HelpModal) Update(msg tea.Msg) (HelpModal, tea.Cmd) {
	if !m.open {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "?":
			m.open = false
			return m, func() tea.Msg { return CloseMsg{} }
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m HelpModal) View() string {
	if !m.open {
		return ""
	}

	titleBar := lipgloss.NewStyle().
		Foreground(theme.AccentBright).Bold(true).
		Width(m.width-4).Padding(0, 1).
		Render("Keyboard Shortcuts")

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		strings.Repeat("─", m.width-4),
		m.viewport.View(),
		lipgloss.NewStyle().Foreground(theme.TextMuted).Render("  esc / ? to close"),
	)

	return lipgloss.NewStyle().
		Background(theme.BgOverlay).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.BorderFocus).
		Width(m.width).
		Height(m.height).
		Padding(1, 2).
		Render(content)
}

// Resize updates modal dimensions.
func (m *HelpModal) Resize(width, height int) {
	m.width = min(width-8, 70)
	m.height = min(height-6, 30)
	m.viewport = viewport.New(m.width-4, m.height-4)
	m.viewport.SetContent(m.renderBindings())
}

func (m HelpModal) renderBindings() string {
	labels := []string{"Global Shortcuts", "Screen Shortcuts"}
	var sb strings.Builder
	for i, group := range m.bindings {
		if i < len(labels) {
			sb.WriteString(lipgloss.NewStyle().Foreground(theme.AccentBright).Bold(true).
				Render(labels[i]) + "\n\n")
		}
		for _, b := range group {
			keys := strings.Join(b.Keys(), ", ")
			help := b.Help().Desc
			line := lipgloss.NewStyle().Foreground(theme.AccentPrimary).Render(padR(keys, 18)) +
				lipgloss.NewStyle().Foreground(theme.TextPrimary).Render(help)
			sb.WriteString("  " + line + "\n")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func padR(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
