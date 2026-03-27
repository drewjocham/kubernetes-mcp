package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/kube-watcher-app/internal/mcp"
	"github.com/user/kube-watcher-app/internal/ui"
)

type AppModel struct {
	sidebar      ui.SidebarModel
	mainContent  ui.MainContentModel
	tools        ui.ToolsModel
	alerts       ui.AlertsModel
	webhooks     ui.WebhooksModel
	deployments  ui.DeploymentsModel
	agent        ui.AgentModel
	server       *mcp.Server
	activeTab    string
	width        int
	height       int
	logs         []string
	logCollapsed bool
}

func NewAppModel() AppModel {
	return AppModel{
		sidebar:      ui.NewSidebar(30),
		mainContent:  ui.NewMainContent(80, 20),
		tools:        ui.NewTools(80, 20),
		alerts:       ui.NewAlerts(80, 20),
		webhooks:     ui.NewWebhooks(80, 20),
		deployments:  ui.NewDeployments(80, 20),
		agent:        ui.NewAgent(80, 20),
		server:       mcp.NewServer(),
		activeTab:    "Server Management",
		width:        110,
		height:       25,
		logs:         []string{"Application started. Press l to toggle logs panel."},
		logCollapsed: false,
	}
}

func (m AppModel) Init() tea.Cmd {
	return nil
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "l":
			m.logCollapsed = !m.logCollapsed
			m.appendLog(fmt.Sprintf("Log panel toggled. collapsed=%v", m.logCollapsed))
			return m, nil
		case "enter":
			selected := m.sidebar.GetSelectedItem()
			m.activeTab = selected
			m.appendLog(fmt.Sprintf("Switched to tab: %s", selected))
			if selected == "Server Management" {
				content := fmt.Sprintf("Server Status: %s\nPress 's' to start, 'x' to stop the server.", m.server.GetStatus())
				m.mainContent, cmd = m.mainContent.Update(ui.ContentUpdateMsg(content))
				return m, cmd
			} else if selected == "Tools" {
				m.mainContent, cmd = m.mainContent.Update(ui.ContentUpdateMsg("Tools content managed separately."))
				if m.server.IsRunning() {
					toolNames := m.server.ListTools()
					if len(toolNames) > 0 {
						m.tools = m.tools.SetTools(toolNames)
						m.appendLog(fmt.Sprintf("Loaded %d tools from MCP endpoint.", len(toolNames)))
					} else {
						m.appendLog("Could not load tools from MCP endpoint, using local list.")
					}
					tool := m.tools.GetSelectedTool()
					m.tools, cmd = m.tools.Update(ui.ToolOutputMsg(fmt.Sprintf("Running tool: %s\nPlease wait for output...", tool)))
					m.appendLog(fmt.Sprintf("Executing tool: %s", tool))
					return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						output, err := m.server.RunTool(ctx, tool)
						if err != nil {
							return ui.ToolOutputMsg(fmt.Sprintf("Error running tool %s: %v", tool, err))
						}
						return ui.ToolOutputMsg(output)
					})
				} else {
					m.tools, cmd = m.tools.Update(ui.ToolOutputMsg("Server is not running. Please start the server to run tools."))
					m.appendLog("Tool execution blocked: server is stopped.")
					return m, cmd
				}
			} else if selected == "Alerts" {
				m.mainContent, cmd = m.mainContent.Update(ui.ContentUpdateMsg("Alerts content managed separately."))
				return m, cmd
			} else if selected == "Webhooks" {
				m.mainContent, cmd = m.mainContent.Update(ui.ContentUpdateMsg("Webhooks content managed separately."))
				return m, cmd
			} else if selected == "Deployments" {
				m.mainContent, cmd = m.mainContent.Update(ui.ContentUpdateMsg("Deployments content managed separately."))
				return m, cmd
			} else if selected == "Agent" {
				m.mainContent, cmd = m.mainContent.Update(ui.ContentUpdateMsg("Agent content managed separately."))
				return m, cmd
			} else {
				contentMsg := ui.ContentUpdateMsg(fmt.Sprintf("Selected: %s\nHere you will see content related to %s.", selected, selected))
				m.mainContent, cmd = m.mainContent.Update(contentMsg)
				return m, cmd
			}
		case "s":
			if m.activeTab == "Server Management" {
				err := m.server.Start()
				content := fmt.Sprintf("Server Status: %s", m.server.GetStatus())
				if err != nil {
					content += fmt.Sprintf("\nError starting server: %v", err)
					m.appendLog(fmt.Sprintf("Server start failed: %v", err))
				} else {
					m.appendLog("Server connected successfully.")
				}
				m.mainContent, cmd = m.mainContent.Update(ui.ContentUpdateMsg(content))
				return m, cmd
			} else if m.activeTab == "Alerts" {
				m.alerts, cmd = m.alerts.Update(msg)
				return m, cmd
			} else if m.activeTab == "Deployments" {
				m.deployments, cmd = m.deployments.Update(msg)
				return m, cmd
			}
		case "x":
			if m.activeTab == "Server Management" {
				err := m.server.Stop()
				content := fmt.Sprintf("Server Status: %s", m.server.GetStatus())
				if err != nil {
					content += fmt.Sprintf("\nError stopping server: %v", err)
					m.appendLog(fmt.Sprintf("Server stop failed: %v", err))
				} else {
					m.appendLog("Server stopped.")
				}
				m.mainContent, cmd = m.mainContent.Update(ui.ContentUpdateMsg(content))
				return m, cmd
			}
		case "a":
			if m.activeTab == "Alerts" || m.activeTab == "Webhooks" {
				if m.activeTab == "Alerts" {
					m.alerts, cmd = m.alerts.Update(msg)
				} else {
					m.webhooks, cmd = m.webhooks.Update(msg)
				}
				return m, cmd
			}
		case "t":
			if m.activeTab == "Webhooks" {
				m.webhooks, cmd = m.webhooks.Update(msg)
				return m, cmd
			}
		case "u", "e":
			if m.activeTab == "Webhooks" {
				m.webhooks, cmd = m.webhooks.Update(msg)
				return m, cmd
			}
		case "d", "w", "c":
			if m.activeTab == "Deployments" {
				m.deployments, cmd = m.deployments.Update(msg)
				return m, cmd
			}
		case "backspace":
			if m.activeTab == "Agent" {
				m.agent, cmd = m.agent.Update(msg)
				return m, cmd
			}
		default:
			if m.activeTab == "Agent" && len(msg.String()) == 1 {
				m.agent, cmd = m.agent.Update(msg)
				return m, cmd
			}
		}
		if m.activeTab == "Tools" {
			m.tools, cmd = m.tools.Update(msg)
			return m, cmd
		} else if m.activeTab == "Alerts" {
			m.alerts, cmd = m.alerts.Update(msg)
			return m, cmd
		} else if m.activeTab == "Webhooks" {
			m.webhooks, cmd = m.webhooks.Update(msg)
			return m, cmd
		} else if m.activeTab == "Deployments" {
			m.deployments, cmd = m.deployments.Update(msg)
			return m, cmd
		} else if m.activeTab == "Agent" {
			m.agent, cmd = m.agent.Update(msg)
			return m, cmd
		} else {
			m.sidebar, cmd = m.sidebar.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sidebar = ui.NewSidebar(msg.Width / 4)
		m.mainContent = ui.NewMainContent(msg.Width-(msg.Width/4), msg.Height-3)
		m.tools = ui.NewTools(msg.Width-(msg.Width/4), msg.Height-3)
		m.alerts = ui.NewAlerts(msg.Width-(msg.Width/4), msg.Height-3)
		m.webhooks = ui.NewWebhooks(msg.Width-(msg.Width/4), msg.Height-3)
		m.deployments = ui.NewDeployments(msg.Width-(msg.Width/4), msg.Height-3)
		m.agent = ui.NewAgent(msg.Width-(msg.Width/4), msg.Height-3)
		return m, nil
	case ui.ToolOutputMsg:
		if m.activeTab == "Tools" {
			m.tools, cmd = m.tools.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m AppModel) View() string {
	appStyle := lipgloss.NewStyle().Width(m.width).Height(m.height)
	sidebar := m.sidebar.View()
	var content string
	if m.activeTab == "Tools" {
		content = m.tools.View()
	} else if m.activeTab == "Alerts" {
		content = m.alerts.View()
	} else if m.activeTab == "Webhooks" {
		content = m.webhooks.View()
	} else if m.activeTab == "Deployments" {
		content = m.deployments.View()
	} else if m.activeTab == "Agent" {
		content = m.agent.View()
	} else {
		content = m.mainContent.View()
	}
	mainArea := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, content)
	if !m.logCollapsed {
		mainArea = lipgloss.JoinVertical(lipgloss.Left, mainArea, m.renderLogPane())
	}
	statusBar := lipgloss.NewStyle().
		Background(lipgloss.Color("#444")).
		Foreground(lipgloss.Color("#FFF")).
		Width(m.width).
		Height(1).
		Render(fmt.Sprintf("Status: %s | Health: Unknown | Logs: %s (toggle: l)", m.server.GetStatus(), m.logPaneState()))
	return appStyle.Render(lipgloss.JoinVertical(lipgloss.Left, mainArea, statusBar))
}

func (m *AppModel) appendLog(message string) {
	timestamp := time.Now().Format("15:04:05")
	m.logs = append(m.logs, fmt.Sprintf("[%s] %s", timestamp, message))
	if len(m.logs) > 400 {
		m.logs = m.logs[len(m.logs)-400:]
	}
}

func (m AppModel) renderLogPane() string {
	const maxLines = 6
	start := 0
	if len(m.logs) > maxLines {
		start = len(m.logs) - maxLines
	}
	lines := m.logs[start:]
	content := "Logs\n" + strings.Join(lines, "\n")
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#111")).
		Foreground(lipgloss.Color("#E5E7EB")).
		BorderTop(true).
		Padding(0, 1).
		Width(m.width).
		Height(maxLines + 2).
		Render(content)
}

func (m AppModel) logPaneState() string {
	if m.logCollapsed {
		return "collapsed"
	}
	return "expanded"
}

func main() {
	p := tea.NewProgram(NewAppModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
