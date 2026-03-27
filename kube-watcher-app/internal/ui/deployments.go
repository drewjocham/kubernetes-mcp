package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Deployment represents an individual deployment configuration
type Deployment struct {
	id     int
	name   string
	target string
	status string
}

// DeploymentsModel represents the state of the deployments UI component
type DeploymentsModel struct {
	deployments []Deployment
	cursor      int
	width       int
	height      int
	style       lipgloss.Style
	message     string
}

// NewDeployments creates a new deployments model
func NewDeployments(width, height int) DeploymentsModel {
	return DeploymentsModel{
		deployments: []Deployment{},
		cursor:      0,
		width:       width,
		height:      height,
		style:       lipgloss.NewStyle().Background(lipgloss.Color("#222")).Foreground(lipgloss.Color("#FFF")).Padding(1, 2),
		message:     "No deployments configured. Press 'd' to deploy MCP, 'w' to deploy Watcher.",
	}
}

// Init initializes the deployments model
func (m DeploymentsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the deployments state
func (m DeploymentsModel) Update(msg tea.Msg) (DeploymentsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.deployments)-1 {
				m.cursor++
			}
		case "d":
			newDeployment := Deployment{
				id:     len(m.deployments) + 1,
				name:   "MCP",
				target: "Docker",
				status: "Deploying",
			}
			m.deployments = append(m.deployments, newDeployment)
			m.message = fmt.Sprintf("Deploying MCP to Docker...")
			if len(m.deployments) == 1 {
				m.cursor = 0
			}
		case "w":
			newDeployment := Deployment{
				id:     len(m.deployments) + 1,
				name:   "Watcher",
				target: "Kubernetes",
				status: "Deploying",
			}
			m.deployments = append(m.deployments, newDeployment)
			m.message = fmt.Sprintf("Deploying Watcher to Kubernetes...")
			if len(m.deployments) == 1 {
				m.cursor = 0
			}
		case "c":
			if len(m.deployments) > 0 {
				m.deployments[m.cursor].status = "Cleaning"
				m.message = fmt.Sprintf("Cleaning up deployment %s...", m.deployments[m.cursor].name)
			} else {
				m.message = "No deployments to clean up."
			}
		case "s":
			if len(m.deployments) > 0 {
				m.deployments[m.cursor].status = "Running"
				m.message = fmt.Sprintf("Deployment %s status updated to Running.", m.deployments[m.cursor].name)
			} else {
				m.message = "No deployments to update status."
			}
		}
	case DeploymentMessageMsg:
		m.message = string(msg)
	}
	return m, nil
}

// View renders the deployments component
func (m DeploymentsModel) View() string {
	lines := []string{"Configured Deployments:\n"}
	for i, deployment := range m.deployments {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			deployment.name = lipgloss.NewStyle().Bold(true).Render(deployment.name)
		}
		lines = append(lines, fmt.Sprintf("%s ID:%d Name: %s Target: %s Status: %s", cursor, deployment.id, deployment.name, deployment.target, deployment.status))
	}
	deploymentsList := lipgloss.JoinVertical(lipgloss.Left, lines...)
	instructions := lipgloss.NewStyle().MarginTop(2).Render("Instructions:\nPress 'd' to deploy MCP, 'w' to deploy Watcher, 'c' to cleanup selected, 's' to set status to Running.")
	message := lipgloss.NewStyle().MarginTop(1).Render("Message:\n" + m.message)
	content := lipgloss.JoinVertical(lipgloss.Left, deploymentsList, instructions, message)
	return m.style.Width(m.width).Height(m.height).Render(content)
}

// GetSelectedDeploymentID returns the ID of the currently selected deployment
func (m DeploymentsModel) GetSelectedDeploymentID() int {
	if len(m.deployments) == 0 {
		return 0
	}
	return m.deployments[m.cursor].id
}

// DeploymentMessageMsg is a message type for updating deployment messages
type DeploymentMessageMsg string
