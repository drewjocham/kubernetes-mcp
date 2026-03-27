package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// MainContentModel represents the state of the main content area
type MainContentModel struct {
	content string
	width   int
	height  int
	style   lipgloss.Style
}

// NewMainContent creates a new main content model
func NewMainContent(width, height int) MainContentModel {
	return MainContentModel{
		content: "Select an item from the sidebar to view content.",
		width:   width,
		height:  height,
		style: lipgloss.NewStyle().
			Background(lipgloss.Color("#222")).
			Foreground(lipgloss.Color("#FFF")).
			Padding(1, 2),
	}
}

// Init initializes the main content model
func (m MainContentModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the main content state
func (m MainContentModel) Update(msg tea.Msg) (MainContentModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ContentUpdateMsg:
		m.content = string(msg)
	}
	return m, nil
}

// View renders the main content component
func (m MainContentModel) View() string {
	return m.style.Width(m.width).Height(m.height).Render(m.content)
}

// ContentUpdateMsg is a message type for updating main content
type ContentUpdateMsg string
