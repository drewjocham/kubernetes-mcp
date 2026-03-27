package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ToolsModel represents the state of the tools UI component
type ToolsModel struct {
	tools     []string
	cursor    int
	width     int
	height    int
	style     lipgloss.Style
	output    string
	table     table.Model
	tableData [][]string
}

// NewTools creates a new tools model
func NewTools(width, height int) ToolsModel {
	return ToolsModel{
		tools:     []string{"node-status", "pod-resources", "namespaces", "pod-logs", "cluster-analysis"},
		cursor:    0,
		width:     width,
		height:    height,
		style:     lipgloss.NewStyle().Background(lipgloss.Color("#222")).Foreground(lipgloss.Color("#FFF")).Padding(1, 2),
		output:    "Select a tool to run and see the output here.",
		table:     table.New(table.WithColumns([]table.Column{{Title: "Key", Width: 20}, {Title: "Value", Width: 40}})),
		tableData: [][]string{},
	}
}

// Init initializes the tools model
func (m ToolsModel) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the tools state
func (m ToolsModel) Update(msg tea.Msg) (ToolsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tools)-1 {
				m.cursor++
			}
		case "enter":
			m.output = fmt.Sprintf("Running tool: %s\nOutput will appear here...", m.tools[m.cursor])
		}
	case ToolOutputMsg:
		m.output = string(msg)
		m.tableData = parseOutputToTable(string(msg))
		rows := []table.Row{}
		for _, row := range m.tableData {
			rows = append(rows, table.Row{row[0], row[1]})
		}
		m.table = table.New(
			table.WithColumns([]table.Column{{Title: "Key", Width: 20}, {Title: "Value", Width: 40}}),
			table.WithRows(rows),
			table.WithFocused(true),
			table.WithHeight(7),
		)
	}
	return m, nil
}

// View renders the tools component
func (m ToolsModel) View() string {
	lines := []string{"Available Tools:\n"}
	for i, tool := range m.tools {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
			tool = lipgloss.NewStyle().Bold(true).Render(tool)
		}
		lines = append(lines, fmt.Sprintf("%s %s", cursor, tool))
	}
	toolsList := lipgloss.JoinVertical(lipgloss.Left, lines...)
	output := lipgloss.NewStyle().MarginTop(2).Render("Output:\n" + m.output)
	tableView := lipgloss.NewStyle().MarginTop(1).Render(m.table.View())
	content := lipgloss.JoinVertical(lipgloss.Left, toolsList, output, tableView)
	return m.style.Width(m.width).Height(m.height).Render(content)
}

// GetSelectedTool returns the currently selected tool
func (m ToolsModel) GetSelectedTool() string {
	return m.tools[m.cursor]
}

// ToolOutputMsg is a message type for updating tool output
type ToolOutputMsg string

// parseOutputToTable parses the tool output string into a 2D slice for table display
func parseOutputToTable(output string) [][]string {
	lines := strings.Split(output, "\n")
	result := [][]string{}
	for _, line := range lines {
		if strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result = append(result, []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])})
			}
		} else if strings.Contains(line, "-") || strings.Contains(line, "|") {
			// Skip separator lines
			continue
		} else {
			result = append(result, []string{"Info", strings.TrimSpace(line)})
		}
	}
	return result
}
