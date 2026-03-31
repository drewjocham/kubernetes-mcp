package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// Screen is implemented by every full-page view in the TUI.
// It extends tea.Model with dimension management and metadata.
type Screen interface {
	tea.Model
	// Resize updates the screen's allocated dimensions without destroying state.
	Resize(width, height int)
	// HelpBindings returns key bindings shown in the help overlay for this screen.
	HelpBindings() []key.Binding
	// Title is shown in the header bar while this screen is active.
	Title() string
}

// SendToAgentMsg is sent when a screen wants to push context into the chat panel.
type SendToAgentMsg struct {
	Content  string
	Source   string // "alert" | "popecli" | "hint"
	AlertRef string
}
