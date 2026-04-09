package alerts

import (
	"fmt"
	"strings"
	"time"

	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/theme"
	"kube-watcher-app/internal/ui"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// item wraps AlertRecord to implement list.Item.
type item struct{ record data.AlertRecord }

func (i item) FilterValue() string {
	return i.record.Kind + " " + i.record.Namespace + " " + i.record.Name
}
func (i item) Title() string       { return fmt.Sprintf("%s/%s", i.record.Namespace, i.record.Name) }
func (i item) Description() string { return i.record.Kind + " · " + i.record.Status }

// Screen is the Alerts management screen.
type Screen struct {
	list     list.Model
	detail   viewport.Model
	records  []data.AlertRecord
	width    int
	height   int
	showHelp bool
	keys     keyMap
}

type keyMap struct {
	Ack     key.Binding
	Dismiss key.Binding
	Send    key.Binding
	Detail  key.Binding
	Filter  key.Binding
}

func defaultKeys() keyMap {
	return keyMap{
		Ack:     key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "acknowledge")),
		Dismiss: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "dismiss")),
		Send:    key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "send to agent")),
		Detail:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "toggle detail")),
		Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter alerts")),
	}
}

// New creates an Alerts screen.
func New(width, height int) *Screen {
	delegate := list.NewDefaultDelegate()
	delegate.Styles = list.NewDefaultItemStyles()

	// Modern styling for selected items
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(theme.TextPrimary).
		Background(theme.AccentPrimary).
		Bold(true).
		Padding(0, 1)

	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(theme.TextSecondary).
		Background(theme.AccentPrimary).
		Padding(0, 1)

	// Normal item styling
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.
		Foreground(theme.TextPrimary)

	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.
		Foreground(theme.TextMuted)

	listW := listWidth(width)
	l := list.New(nil, delegate, listW, height-2)
	l.Title = "Alerts"
	l.Styles.Title = lipgloss.NewStyle().
		Foreground(theme.AccentBright).
		Background(theme.BgSurface).
		Bold(true).
		Padding(0, 1)
	l.Styles.TitleBar = lipgloss.NewStyle().Background(theme.BgSurface)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false) // We'll add our own header

	detail := viewport.New(detailWidth(width), height-4)

	return &Screen{
		list:   l,
		detail: detail,
		width:  width,
		height: height,
		keys:   defaultKeys(),
	}
}

func listWidth(total int) int {
	if total > 60 {
		return total / 2
	}
	return total - 2
}

func detailWidth(total int) int {
	if total > 60 {
		return total - listWidth(total) - 2
	}
	return 0
}

// SetAlerts updates the list with new records.
func (s *Screen) SetAlerts(records []data.AlertRecord) {
	s.records = records
	items := make([]list.Item, len(records))
	for i, r := range records {
		items[i] = item{record: r}
	}
	s.list.SetItems(items)
	s.refreshDetail()
}

func (s *Screen) refreshDetail() {
	sel, ok := s.list.SelectedItem().(item)
	if !ok {
		s.detail.SetContent(lipgloss.NewStyle().Foreground(theme.TextMuted).Render("  No alert selected."))
		return
	}
	s.detail.SetContent(renderDetail(sel.record, detailWidth(s.width)))
}

func (s Screen) Init() tea.Cmd { return nil }

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle specific keys first, then let list handle others
		switch {
		case key.Matches(msg, s.keys.Send):
			sel, ok := s.list.SelectedItem().(item)
			if ok {
				return s, func() tea.Msg {
					return ui.SendToAgentMsg{
						Source:   "alert",
						AlertRef: sel.record.ID,
						Content:  buildAgentContext(sel.record),
					}
				}
			}
		case key.Matches(msg, s.keys.Ack):
			// Acknowledge selected alert (local state only)
			if sel, ok := s.list.SelectedItem().(item); ok {
				for i, r := range s.records {
					if r.ID == sel.record.ID {
						s.records[i].Status = "acknowledged"
					}
				}
				s.SetAlerts(s.records)
			}
		case key.Matches(msg, s.keys.Filter):
			// Toggle filtering
			s.list.SetFilteringEnabled(!s.list.FilteringEnabled())
			if !s.list.FilteringEnabled() {
				s.list.ResetFilter()
			}
			return s, nil // Don't pass to list when handling filter toggle
		case key.Matches(msg, s.keys.Detail):
			// Toggle detail view (could be handled by list)
			return s, nil
		}

		// For all other keys, let the list handle them (including arrows)
		var listCmd tea.Cmd
		s.list, listCmd = s.list.Update(msg)
		s.refreshDetail()

		var vpCmd tea.Cmd
		s.detail, vpCmd = s.detail.Update(msg)

		return s, tea.Batch(listCmd, vpCmd)

	default:
		// Non-key messages
		var listCmd tea.Cmd
		s.list, listCmd = s.list.Update(msg)
		s.refreshDetail()

		var vpCmd tea.Cmd
		s.detail, vpCmd = s.detail.Update(msg)

		return s, tea.Batch(listCmd, vpCmd)
	}
}

func (s Screen) View() string {
	if s.width == 0 || s.height == 0 {
		return "Loading alerts..."
	}

	listView := theme.PanelStyle().
		Width(listWidth(s.width)).
		Height(s.height - 2).
		Render(s.list.View())

	var detailView string
	if s.width > 60 {
		dw := detailWidth(s.width)
		detailView = theme.PanelStyle().
			Width(dw).
			Height(s.height - 2).
			Render(s.detail.View())
	}

	if detailView != "" {
		return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
	}
	return listView
}

func (s *Screen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.list.SetWidth(listWidth(width))
	s.list.SetHeight(height - 8) // Account for header, stats, help text
	s.detail = viewport.New(detailWidth(width), height-8)
	s.refreshDetail()
}

func (s Screen) HelpBindings() []key.Binding {
	return []key.Binding{s.keys.Ack, s.keys.Dismiss, s.keys.Send, s.keys.Detail, s.keys.Filter}
}

func (s Screen) Title() string { return "Alerts" }

func (s Screen) GetRecords() []data.AlertRecord { return s.records }

// renderDetail produces a richly formatted detail panel for an alert.
func renderCard(title string, value string) string {
	return theme.CardStyle().
		Padding(0, 1).
		Render(theme.CardTitleStyle().Render(title) + "\n" + theme.CardValueStyle().Render(value))
}

func (s Screen) renderStatsCards() string {
	if len(s.records) == 0 {
		return ""
	}

	// Count alerts by severity
	critical, high, medium, low := 0, 0, 0, 0
	acknowledged := 0

	for _, r := range s.records {
		if r.Status == "acknowledged" {
			acknowledged++
			continue
		}
		switch r.Severity {
		case "critical":
			critical++
		case "high":
			high++
		case "medium":
			medium++
		default:
			low++
		}
	}

	cards := []string{
		renderCard("Total", fmt.Sprintf("%d", len(s.records))),
		renderCard("Active", fmt.Sprintf("%d", len(s.records)-acknowledged)),
		renderCard("Critical", fmt.Sprintf("%d", critical)),
		renderCard("High", fmt.Sprintf("%d", high)),
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, cards...) + "\n"
}

func renderDetail(r data.AlertRecord, width int) string {
	if width < 10 {
		return ""
	}
	var sb strings.Builder

	title := theme.TitleStyle().Render(fmt.Sprintf("%s  %s/%s", severityIcon(r.Severity), r.Namespace, r.Name))
	sb.WriteString(title + "\n\n")

	// Summary cards like ArgusKube
	summary := lipgloss.JoinHorizontal(
		lipgloss.Top,
		renderCard("Kind", r.Kind),
		" ",
		renderCard("Cluster", r.Cluster),
		" ",
		renderCard("Status", r.Status),
	)
	sb.WriteString(summary + "\n\n")

	field := func(label, value string) {
		l := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(label + ": ")
		v := lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Render(value)
		sb.WriteString(l + v + "\n")
	}
	field("ID", r.ID)
	field("Received", r.ReceivedAt.Format(time.RFC3339))
	field("Reason", r.Reason)

	if r.Message != "" {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("Message:\n"))
		sb.WriteString(theme.CodeBlockStyle().Width(width-4).Render(r.Message) + "\n")
	}

	if r.Summary != "" {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true).Render("Root Cause Analysis") + "\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Render(r.RootCause) + "\n\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(r.Summary) + "\n")
	}

	if len(r.Actions) > 0 {
		sb.WriteString("\n")
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Bold(true).Render("Recommended Actions") + "\n")
		for i, action := range r.Actions {
			num := lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Render(fmt.Sprintf("  %d. ", i+1))
			sb.WriteString(num + action + "\n")
		}
	}

	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("\n  ctrl+g → send to agent")
	sb.WriteString(hint)
	return sb.String()
}

func buildAgentContext(r data.AlertRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Alert detected in cluster %q:\n", r.Cluster))
	sb.WriteString(fmt.Sprintf("Kind: %s\nNamespace: %s\nPod: %s\nSeverity: %s\n", r.Kind, r.Namespace, r.Name, r.Severity))
	sb.WriteString(fmt.Sprintf("Reason: %s\nMessage: %s\n", r.Reason, r.Message))
	if r.RootCause != "" {
		sb.WriteString(fmt.Sprintf("Current RCA: %s\n", r.RootCause))
	}
	sb.WriteString("\nPlease analyze this alert and suggest debugging steps.")
	return sb.String()
}

func severityIcon(s string) string {
	switch strings.ToLower(s) {
	case "critical":
		return lipgloss.NewStyle().Foreground(theme.ColorCritical).Render("●")
	case "high":
		return lipgloss.NewStyle().Foreground(theme.ColorHigh).Render("●")
	case "medium":
		return lipgloss.NewStyle().Foreground(theme.ColorMedium).Render("●")
	default:
		return lipgloss.NewStyle().Foreground(theme.ColorLow).Render("●")
	}
}
