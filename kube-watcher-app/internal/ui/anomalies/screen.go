package anomalies

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/charts"
	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/theme"
)

// Screen shows incident history with inline sparklines.
type Screen struct {
	incidents []data.Incident
	viewport  viewport.Model
	width     int
	height    int
	selected  int
}

// New creates an Anomalies screen.
func New(width, height int) *Screen {
	vp := viewport.New(width-2, height-4)
	return &Screen{viewport: vp, width: width, height: height}
}

// SetHistory replaces the incident list and refreshes the view.
func (s *Screen) SetHistory(incidents []data.Incident) {
	s.incidents = incidents
	s.selected = 0
	s.refresh()
}

func (s *Screen) refresh() {
	s.viewport.SetContent(s.render())
}

func (s Screen) Init() tea.Cmd { return nil }

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.selected > 0 {
				s.selected--
				s.refresh()
			}
			return s, nil
		case "down", "j":
			if s.selected < len(s.incidents)-1 {
				s.selected++
				s.refresh()
			}
			return s, nil
		}
	}
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s Screen) View() string {
	header := lipgloss.NewStyle().
		Foreground(theme.AccentBright).
		Bold(true).
		Width(s.width).
		Padding(0, 1).
		Render(fmt.Sprintf("Anomaly History  (%d incidents)", len(s.incidents)))

	colHeaders := lipgloss.NewStyle().
		Foreground(theme.TextSecondary).
		Width(s.width).
		Padding(0, 1).
		Render(fmt.Sprintf("%-8s %-10s %-16s %-20s %-12s %s",
			"SEVERITY", "KIND", "NAMESPACE/NAME", "REASON", "OCCURRENCES", "TREND"))

	divider := lipgloss.NewStyle().Foreground(theme.BorderSubtle).Render(strings.Repeat("─", s.width))

	content := theme.PanelStyle().Width(s.width).Height(s.height - 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, colHeaders, divider, s.viewport.View()))
	return content
}

func (s *Screen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.viewport = viewport.New(width-2, height-6)
	s.refresh()
}

func (s Screen) HelpBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "previous")),
		key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "next")),
	}
}

func (s Screen) Title() string { return "Anomalies" }

func (s Screen) render() string {
	if len(s.incidents) == 0 {
		return lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).
			Padding(2, 2).Render("No anomaly incidents recorded.")
	}

	var rows []string
	for i, inc := range s.incidents {
		namespaceName := inc.Namespace + "/" + inc.Name
		if len(namespaceName) > 20 {
			namespaceName = namespaceName[:19] + "…"
		}
		reason := inc.Reason
		if len(reason) > 12 {
			reason = reason[:11] + "…"
		}

		// Sparkline from history
		sparkColor := theme.ColorLow
		if inc.Severity == "critical" {
			sparkColor = theme.ColorCritical
		} else if inc.Severity == "high" {
			sparkColor = theme.ColorHigh
		}
		spark := charts.SparklineColored(inc.History, 12, theme.ColorInfo, sparkColor)

		// Frequency delta arrow
		trend := "  —"
		if len(inc.History) >= 2 {
			last := inc.History[len(inc.History)-1]
			prev := inc.History[len(inc.History)-2]
			if last > prev {
				trend = lipgloss.NewStyle().Foreground(theme.ColorCritical).Render("  ▲")
			} else if last < prev {
				trend = lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render("  ▼")
			}
		}

		var rowStyle lipgloss.Style
		if i == s.selected {
			rowStyle = theme.TableSelectedStyle().Width(s.width - 4)
		} else {
			rowStyle = theme.TableRowStyle().Width(s.width - 4)
		}

		sevStyle := theme.AlertBadgeStyle(inc.Severity)
		row := rowStyle.Render(fmt.Sprintf("%-8s %-10s %-20s %-12s %-12d %s%s",
			sevStyle.Render(severityShort(inc.Severity)),
			inc.Kind,
			namespaceName,
			reason,
			inc.Occurrences,
			spark,
			trend,
		))
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func severityShort(s string) string {
	switch strings.ToLower(s) {
	case "critical":
		return "CRIT"
	case "high":
		return "HIGH"
	case "medium":
		return "MED"
	default:
		return "LOW"
	}
}
