package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Alert represents an individual alert
type Alert struct {
	id      int
	message string
	active  bool
}

// AlertsModel represents the state of the alerts UI component
type AlertsModel struct {
	alerts  []Alert
	cursor  int
	width   int
	height  int
	style   lipgloss.Style
	message string
}

// NewAlerts creates a new alerts model
func NewAlerts(width, height int) AlertsModel {
	return AlertsModel{
		alerts:  []Alert{},
		cursor:  0,
		width:   width,
		height:  height,
		style:   lipgloss.NewStyle().Background(lipgloss.Color("#222")).Foreground(lipgloss.Color("#FFF")).Padding(1, 2),
		message: "No alerts currently active. Press 'a' to add a new alert.",
	}
}

// Init initializes the alerts model
func (m AlertsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the alerts state
func (m AlertsModel) Update(msg tea.Msg) (AlertsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.alerts)-1 {
				m.cursor++
			}
		case "s":
			if len(m.alerts) > 0 {
				m.alerts[m.cursor].active = false
				m.message = fmt.Sprintf("Alert %d silenced: %s", m.alerts[m.cursor].id, m.alerts[m.cursor].message)
			} else {
				m.message = "No alerts to silence."
			}
		case "a":
			newAlert := Alert{
				id:      len(m.alerts) + 1,
				message: fmt.Sprintf("Critical issue detected at %s", time.Now().Format(time.RFC3339)),
				active:  true,
			}
			m.alerts = append(m.alerts, newAlert)
			m.message = fmt.Sprintf("New alert added: %s", newAlert.message)
			if len(m.alerts) == 1 {
				m.cursor = 0
			}
		}
	case AlertMessageMsg:
		m.message = string(msg)
	}
	return m, nil
}

// View renders the alerts component
func (m AlertsModel) View() string {
	lines := []string{"Active Alerts:\n"}
	for i, alert := range m.alerts {
		cursor := " "
		status := "Active"
		if !alert.active {
			status = "Silenced"
		}
		if m.cursor == i {
			cursor = ">"
			alert.message = lipgloss.NewStyle().Bold(true).Render(alert.message)
		}
		lines = append(lines, fmt.Sprintf("%s ID:%d [%s] %s", cursor, alert.id, status, alert.message))
	}
	alertsList := lipgloss.JoinVertical(lipgloss.Left, lines...)
	message := lipgloss.NewStyle().MarginTop(2).Render("Message:\n" + m.message)
	content := lipgloss.JoinVertical(lipgloss.Left, alertsList, message)
	return m.style.Width(m.width).Height(m.height).Render(content)
}

// GetSelectedAlertID returns the ID of the currently selected alert
func (m AlertsModel) GetSelectedAlertID() int {
	if len(m.alerts) == 0 {
		return 0
	}
	return m.alerts[m.cursor].id
}

// AlertMessageMsg is a message type for updating alert messages
type AlertMessageMsg string
