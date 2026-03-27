package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SidebarModel represents the state of the sidebar component
type SidebarModel struct {
	items  []string
	cursor int
	width  int
	style  lipgloss.Style
}

// NewSidebar creates a new sidebar model with predefined items
func NewSidebar(width int) SidebarModel {
	return SidebarModel{
		items:  []string{"Server Management", "Tools", "Alerts", "Webhooks", "Deployments", "Agent"},
		cursor: 0,
		width:  width,
		style: lipgloss.NewStyle().
			Background(lipgloss.Color("#333")).
			Foreground(lipgloss.Color("#FFF")).
			Padding(1, 2),
	}
}

// Init initializes the sidebar model
func (m SidebarModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the sidebar state
func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
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
		}
	}
	return m, nil
}

// View renders the sidebar component
func (m SidebarModel) View() string {
	lines := []string{}
	for i, item := range m.items {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			item = lipgloss.NewStyle().Bold(true).Render(item)
		}
		lines = append(lines, fmt.Sprintf("%s %s", cursor, item))
	}
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return m.style.Width(m.width).Render(content)
}

// GetSelectedItem returns the currently selected sidebar item
func (m SidebarModel) GetSelectedItem() string {
	return m.items[m.cursor]
}
