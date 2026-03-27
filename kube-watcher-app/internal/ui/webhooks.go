package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Webhook represents an individual webhook configuration
type Webhook struct {
	id     int
	url    string
	event  string
	active bool
}

// WebhooksModel represents the state of the webhooks UI component
type WebhooksModel struct {
	webhooks []Webhook
	cursor   int
	width    int
	height   int
	style    lipgloss.Style
	message  string
	newUrl   string
	newEvent string
}

// NewWebhooks creates a new webhooks model
func NewWebhooks(width, height int) WebhooksModel {
	return WebhooksModel{
		webhooks: []Webhook{},
		cursor:   0,
		width:    width,
		height:   height,
		style:    lipgloss.NewStyle().Background(lipgloss.Color("#222")).Foreground(lipgloss.Color("#FFF")).Padding(1, 2),
		message:  "No webhooks configured. Press 'a' to add a new webhook.",
		newUrl:   "https://example.com/webhook",
		newEvent: "alert",
	}
}

// Init initializes the webhooks model
func (m WebhooksModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the webhooks state
func (m WebhooksModel) Update(msg tea.Msg) (WebhooksModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.webhooks)-1 {
				m.cursor++
			}
		case "t":
			if len(m.webhooks) > 0 {
				m.webhooks[m.cursor].active = !m.webhooks[m.cursor].active
				status := "enabled"
				if !m.webhooks[m.cursor].active {
					status = "disabled"
				}
				m.message = fmt.Sprintf("Webhook %d %s: %s for event %s", m.webhooks[m.cursor].id, status, m.webhooks[m.cursor].url, m.webhooks[m.cursor].event)
			} else {
				m.message = "No webhooks to toggle."
			}
		case "a":
			newWebhook := Webhook{
				id:     len(m.webhooks) + 1,
				url:    m.newUrl,
				event:  m.newEvent,
				active: true,
			}
			m.webhooks = append(m.webhooks, newWebhook)
			m.message = fmt.Sprintf("New webhook added: %s for event %s", newWebhook.url, newWebhook.event)
			if len(m.webhooks) == 1 {
				m.cursor = 0
			}
		case "u":
			m.newUrl = "https://example.com/webhook" + fmt.Sprintf("%d", len(m.webhooks)+1)
			m.message = fmt.Sprintf("Updated URL to: %s", m.newUrl)
		case "e":
			if m.newEvent == "alert" {
				m.newEvent = "system"
			} else {
				m.newEvent = "alert"
			}
			m.message = fmt.Sprintf("Updated event to: %s", m.newEvent)
		}
	case WebhookMessageMsg:
		m.message = string(msg)
	}
	return m, nil
}

// View renders the webhooks component
func (m WebhooksModel) View() string {
	lines := []string{"Configured Webhooks:\n"}
	for i, webhook := range m.webhooks {
		cursor := " "
		status := "Active"
		if !webhook.active {
			status = "Inactive"
		}
		if m.cursor == i {
			cursor = ">"
			webhook.url = lipgloss.NewStyle().Bold(true).Render(webhook.url)
		}
		lines = append(lines, fmt.Sprintf("%s ID:%d [%s] %s (Event: %s)", cursor, webhook.id, status, webhook.url, webhook.event))
	}
	webhooksList := lipgloss.JoinVertical(lipgloss.Left, lines...)
	config := lipgloss.NewStyle().MarginTop(2).Render("New Webhook Configuration:\nURL: " + m.newUrl + "\nEvent: " + m.newEvent + "\nPress 'u' to change URL, 'e' to change event, 'a' to add.")
	message := lipgloss.NewStyle().MarginTop(1).Render("Message:\n" + m.message)
	content := lipgloss.JoinVertical(lipgloss.Left, webhooksList, config, message)
	return m.style.Width(m.width).Height(m.height).Render(content)
}

// GetSelectedWebhookID returns the ID of the currently selected webhook
func (m WebhooksModel) GetSelectedWebhookID() int {
	if len(m.webhooks) == 0 {
		return 0
	}
	return m.webhooks[m.cursor].id
}

// WebhookMessageMsg is a message type for updating webhook messages
type WebhookMessageMsg string
