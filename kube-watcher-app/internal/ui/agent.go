package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AgentModel represents the state of the agent interaction UI component
type AgentModel struct {
	questions       []string
	responses       []string
	currentQuestion string
	width           int
	height          int
	style           lipgloss.Style
	message         string
}

// NewAgent creates a new agent model
func NewAgent(width, height int) AgentModel {
	return AgentModel{
		questions:       []string{},
		responses:       []string{},
		currentQuestion: "",
		width:           width,
		height:          height,
		style:           lipgloss.NewStyle().Background(lipgloss.Color("#222")).Foreground(lipgloss.Color("#FFF")).Padding(1, 2),
		message:         "Type your question and press 'Enter' to ask the agent.",
	}
}

// Init initializes the agent model
func (m AgentModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the agent state
func (m AgentModel) Update(msg tea.Msg) (AgentModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.currentQuestion != "" {
				m.questions = append(m.questions, m.currentQuestion)
				m.responses = append(m.responses, fmt.Sprintf("Agent: Here's a response to '%s'", m.currentQuestion))
				m.message = "Question sent. Here's the agent's response."
				m.currentQuestion = ""
			} else {
				m.message = "No question entered. Type a question first."
			}
		case "backspace":
			if len(m.currentQuestion) > 0 {
				m.currentQuestion = m.currentQuestion[:len(m.currentQuestion)-1]
				m.message = "Type your question and press 'Enter' to ask the agent."
			}
		default:
			if len(msg.String()) == 1 {
				m.currentQuestion += msg.String()
				m.message = "Type your question and press 'Enter' to ask the agent."
			}
		}
	case AgentMessageMsg:
		m.message = string(msg)
	}
	return m, nil
}

// View renders the agent component
func (m AgentModel) View() string {
	lines := []string{"Agent Interaction:\n"}
	for i, q := range m.questions {
		lines = append(lines, fmt.Sprintf("Q: %s", q))
		lines = append(lines, fmt.Sprintf("A: %s", m.responses[i]))
	}
	history := lipgloss.JoinVertical(lipgloss.Left, lines...)
	current := lipgloss.NewStyle().MarginTop(2).Render("Current Question:\n" + m.currentQuestion)
	message := lipgloss.NewStyle().MarginTop(1).Render("Message:\n" + m.message)
	content := lipgloss.JoinVertical(lipgloss.Left, history, current, message)
	return m.style.Width(m.width).Height(m.height).Render(content)
}

// AgentMessageMsg is a message type for updating agent messages
type AgentMessageMsg string
