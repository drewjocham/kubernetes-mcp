package hints

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/theme"
	"kube-watcher-app/internal/ui"
)

// Screen shows recommendation cards from the MCP engine.
type Screen struct {
	recs     []data.Recommendation
	viewport viewport.Model
	selected int
	expanded map[int]bool
	width    int
	height   int
}

// New creates a Hints screen.
func New(width, height int) *Screen {
	return &Screen{
		viewport: viewport.New(width-2, height-4),
		expanded: make(map[int]bool),
		width:    width,
		height:   height,
	}
}

// SetRecommendations replaces the recommendation list.
func (s *Screen) SetRecommendations(recs []data.Recommendation) {
	s.recs = recs
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
			if s.selected < len(s.recs)-1 {
				s.selected++
				s.refresh()
			}
			return s, nil
		case "enter", " ":
			s.expanded[s.selected] = !s.expanded[s.selected]
			s.refresh()
			return s, nil
		case "ctrl+g":
			if s.selected < len(s.recs) {
				rec := s.recs[s.selected]
				return s, func() tea.Msg {
					return ui.SendToAgentMsg{
						Source:  "hint",
						Content: buildContext(rec),
					}
				}
			}
		}
	}
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s Screen) View() string {
	header := lipgloss.NewStyle().
		Foreground(theme.AccentBright).Bold(true).
		Width(s.width).Padding(0, 1).
		Render(fmt.Sprintf("Engineer Hints  (%d recommendations)", len(s.recs)))

	hint := lipgloss.NewStyle().Foreground(theme.TextMuted).
		Render("  ↑↓=navigate  enter=expand  ctrl+g=send to agent")

	return theme.PanelStyle().Width(s.width).Height(s.height - 2).
		Render(lipgloss.JoinVertical(lipgloss.Left, header, s.viewport.View(), hint))
}

func (s *Screen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.viewport = viewport.New(width-2, height-5)
	s.refresh()
}

func (s Screen) HelpBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "expand/collapse")),
		key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "send to agent")),
	}
}

func (s Screen) Title() string { return "Hints" }

func (s Screen) render() string {
	if len(s.recs) == 0 {
		return lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).Padding(2, 2).
			Render("No recommendations yet. Alerts are analyzed automatically.")
	}

	cardW := s.width - 6

	var blocks []string
	for i, rec := range s.recs {
		isSelected := i == s.selected
		isExpanded := s.expanded[i]

		var card strings.Builder

		// Title line
		sevIcon := theme.AlertBadgeStyle(rec.Severity).Render(severityIcon(rec.Severity))
		title := lipgloss.NewStyle().Foreground(theme.TextPrimary).Bold(isSelected).
			Render(fmt.Sprintf("  %s  %s", sevIcon, rec.Title))
		card.WriteString(title + "\n")

		// Summary
		summary := lipgloss.NewStyle().Foreground(theme.TextSecondary).
			Render("     " + rec.Summary)
		card.WriteString(summary + "\n")

		// Meta
		meta := lipgloss.NewStyle().Foreground(theme.TextMuted).
			Render(fmt.Sprintf("     Kind: %s  |  Freq Δ: %+.1f", rec.RelatedKind, rec.FrequencyDelta))
		card.WriteString(meta)

		// Steps (expanded only)
		if isExpanded && len(rec.Steps) > 0 {
			card.WriteString("\n\n")
			card.WriteString(lipgloss.NewStyle().Foreground(theme.AccentBright).
				Render("     Remediation Steps:") + "\n")
			for j, step := range rec.Steps {
				num := lipgloss.NewStyle().Foreground(theme.AccentPrimary).
					Render(fmt.Sprintf("     %d. ", j+1))
				card.WriteString(num + step + "\n")
			}
		} else if len(rec.Steps) > 0 {
			card.WriteString("  " + lipgloss.NewStyle().Foreground(theme.TextMuted).
				Render(fmt.Sprintf("  ↵ expand (%d steps)", len(rec.Steps))))
		}

		var cardStyle lipgloss.Style
		if isSelected {
			cardStyle = theme.HintCardStyle(cardW).BorderForeground(theme.AccentPrimary)
		} else {
			cardStyle = theme.HintCardStyle(cardW)
		}
		blocks = append(blocks, cardStyle.Render(card.String()))
	}

	return strings.Join(blocks, "\n")
}

func buildContext(rec data.Recommendation) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Recommendation: %s\n", rec.Title))
	sb.WriteString(fmt.Sprintf("Severity: %s  Kind: %s\n", rec.Severity, rec.RelatedKind))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", rec.Summary))
	if len(rec.Steps) > 0 {
		sb.WriteString("Suggested steps:\n")
		for i, s := range rec.Steps {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, s))
		}
	}
	sb.WriteString("\nPlease review and suggest additional actions.")
	return sb.String()
}

func severityIcon(s string) string {
	switch strings.ToLower(s) {
	case "critical":
		return "●"
	case "high":
		return "◆"
	case "medium":
		return "▲"
	default:
		return "○"
	}
}
