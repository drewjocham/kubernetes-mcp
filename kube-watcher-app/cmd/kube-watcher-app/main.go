package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kube-watcher-app/internal/agent"
	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/data/mcp"
	"kube-watcher-app/internal/hub"
	"kube-watcher-app/internal/layout"
	"kube-watcher-app/internal/popecli"
	"kube-watcher-app/internal/theme"
	"kube-watcher-app/internal/ui"
	alertsscreen "kube-watcher-app/internal/ui/alerts"
	anomaliesscreen "kube-watcher-app/internal/ui/anomalies"
	chartsscreen "kube-watcher-app/internal/ui/charts"
	"kube-watcher-app/internal/ui/chat"
	"kube-watcher-app/internal/ui/config"
	"kube-watcher-app/internal/ui/deployments"
	"kube-watcher-app/internal/ui/drawer"
	hintsscreen "kube-watcher-app/internal/ui/hints"
	logsscreen "kube-watcher-app/internal/ui/logs"
	"kube-watcher-app/internal/ui/modal"
	popecliscreen "kube-watcher-app/internal/ui/popecli"
	"kube-watcher-app/internal/ui/sidebar"
)

type AppModel struct {
	// Layout
	zones layout.Zones
	focus layout.FocusTarget

	// Navigation
	sidebar   sidebar.Model
	activeIdx int
	screens   []ui.Screen

	// Persistent panels
	chatPanel *chat.Model
	drawer    *drawer.Model
	helpModal *modal.HelpModal

	// Data
	hubData *hub.Hub
	program *tea.Program // set after NewProgram

	// Status
	connStatus hub.ConnectionStatusMsg
	unreadChat int

	// Configuration screen reference (for status updates)
	configIdx int
}

func newAppModel() AppModel {
	const (
		defaultW = 120
		defaultH = 35
	)

	zones := layout.Compute(defaultW, defaultH, false, false)

	mcp := mcp.New()
	agentCli := agent.New()
	h := hub.New(mcp)

	// Build screens (must match sidebar order)
	alertsSc := alertsscreen.New(zones.MainWidth, zones.MainHeight)
	anomaliesSc := anomaliesscreen.New(zones.MainWidth, zones.MainHeight)
	chartsSc := chartsscreen.New(zones.MainWidth, zones.MainHeight)
	logsSc := logsscreen.New(zones.MainWidth, zones.MainHeight)
	hintsSc := hintsscreen.New(zones.MainWidth, zones.MainHeight)
	popeciSc := popecliscreen.New(zones.MainWidth, zones.MainHeight, nil) // program set later
	deploymentsSc := deployments.New(mcp, zones.MainWidth, zones.MainHeight)
	configSc := config.New(zones.MainWidth, zones.MainHeight)

	screens := []ui.Screen{
		alertsSc,
		anomaliesSc,
		chartsSc,
		logsSc,
		hintsSc,
		popeciSc,
		deploymentsSc,
		configSc,
	}

	chatModel := chat.New(agentCli, zones.ChatWidth, zones.MainHeight)

	return AppModel{
		zones:     zones,
		focus:     layout.FocusMain,
		sidebar:   sidebar.New(zones.SidebarWidth, zones.MainHeight),
		screens:   screens,
		chatPanel: &chatModel,
		drawer:    drawer.New(zones.TotalWidth, layout.DrawerHeight+2),
		helpModal: modal.NewHelp(defaultW, defaultH),
		hubData:   h,
		configIdx: 7,
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.hubData.Init()
}

// handleGlobalKeys handles shortcuts that work regardless of focus.
// Returns (cmd, true) if the key was consumed.
func (m *AppModel) handleGlobalKeys(msg tea.KeyMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c":
		return tea.Quit, true

	case "tab":
		m.zones = layout.Compute(m.zones.TotalWidth, m.zones.TotalHeight, !m.zones.ChatOpen, m.zones.DrawerOpen)
		m.resizeAll()
		return nil, true

	case "ctrl+l":
		m.drawer.Toggle()
		m.zones = layout.Compute(m.zones.TotalWidth, m.zones.TotalHeight, m.zones.ChatOpen, m.drawer.IsOpen())
		m.resizeAll()
		return nil, true

	case "?":
		if m.helpModal.IsOpen() {
			m.helpModal.Close()
			m.focus = layout.FocusMain
		} else {
			globalBindings := globalKeyBindings()
			screenBindings := m.screens[m.activeIdx].HelpBindings()
			m.helpModal.Open(globalBindings, screenBindings)
			m.focus = layout.FocusModal
		}
		return nil, true

	case "esc":
		if m.helpModal.IsOpen() {
			m.helpModal.Close()
			m.focus = layout.FocusMain
		} else if m.focus == layout.FocusChat {
			m.chatPanel.BlurInput()
			m.focus = layout.FocusMain
		} else if m.focus == layout.FocusSidebar {
			m.focus = layout.FocusMain
		}
		return nil, true

	case "ctrl+a":
		if m.zones.ChatOpen {
			m.chatPanel.FocusInput()
			m.focus = layout.FocusChat
			m.unreadChat = 0
			m.sidebar.SetBadge(7, 0)
			return nil, true
		}

	case "ctrl+s":
		m.focus = layout.FocusSidebar
		return nil, true

	case "1", "2", "3", "4", "5", "6", "7", "8":
		idx := int(msg.String()[0] - '1')
		if idx >= 0 && idx < len(m.screens) {
			m.activateScreen(idx)
			return nil, true
		}
	}
	return nil, false
}

func (m *AppModel) activateScreen(idx int) {
	m.activeIdx = idx
	m.sidebar.SetActive(idx)
	m.focus = layout.FocusMain
}

func (m *AppModel) resizeAll() {
	m.sidebar = sidebar.New(layout.SidebarWidth, m.zones.MainHeight)
	m.sidebar.SetActive(m.activeIdx)
	for _, s := range m.screens {
		s.Resize(m.zones.MainWidth, m.zones.MainHeight)
	}
	m.chatPanel.Resize(m.zones.ChatWidth, m.zones.MainHeight)
	m.drawer.Resize(m.zones.TotalWidth, layout.DrawerHeight+2)
	m.helpModal.Resize(m.zones.TotalWidth, m.zones.TotalHeight)
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.zones = layout.Compute(msg.Width, msg.Height, m.zones.ChatOpen, m.drawer.IsOpen())
		m.resizeAll()
		return m, nil

	case tea.KeyMsg:
		if cmd, consumed := m.handleGlobalKeys(msg); consumed {
			if cmd != nil {
				return m, cmd
			}
			return m, nil
		}

		switch m.focus {
		case layout.FocusModal:
			h, cmd := m.helpModal.Update(msg)
			*m.helpModal = h
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			if !m.helpModal.IsOpen() {
				m.focus = layout.FocusMain
			}

		case layout.FocusSidebar:
			updated, cmd := m.sidebar.Update(msg)
			m.sidebar = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		case layout.FocusChat:
			updated, cmd := m.chatPanel.Update(msg)
			*m.chatPanel = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}

		default: // FocusMain
			updated, cmd := m.screens[m.activeIdx].Update(msg)
			m.screens[m.activeIdx] = updated.(ui.Screen)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case sidebar.SelectMsg:
		m.activateScreen(msg.Index)

	// Data messages
	case hub.AlertsUpdatedMsg:
		m.screens[0].(*alertsscreen.Screen).SetAlerts(msg.Records)
		m.sidebar.SetBadge(0, activeAlertCount(msg.Records))
		// Auto-inject new critical alerts to chat
		for _, r := range msg.Records {
			if r.Severity == "critical" {
				injectAlert := r
				cmds = append(cmds, func() tea.Msg {
					return chat.InjectMsg{Content: buildAlertContext(injectAlert)}
				})
				break // one at a time
			}
		}

	case hub.HistoryUpdatedMsg:
		m.screens[1].(*anomaliesscreen.Screen).SetHistory(msg.Incidents)
		m.screens[2].(*chartsscreen.Screen).SetHistory(msg.Incidents)

	case hub.RecsUpdatedMsg:
		m.screens[4].(*hintsscreen.Screen).SetRecommendations(msg.Recs)

	case hub.ConnectionStatusMsg:
		m.connStatus = msg
		m.screens[m.configIdx].(*config.Screen).SetStatus(msg)

	case hub.DataErrorMsg:
		m.drawer.Append(fmt.Sprintf("[%s] data error: %v", msg.Source, msg.Err))

	// Chat injection
	case chat.InjectMsg:
		m.chatPanel.Inject(msg.Content)
		m.unreadChat = m.chatPanel.UnreadCount()
		m.sidebar.SetBadge(7, m.unreadChat)

	case chat.UnreadMsg:
		m.unreadChat = msg.Count
		m.sidebar.SetBadge(7, m.unreadChat)

	// Screen → agent delegation
	case ui.SendToAgentMsg:
		if !m.zones.ChatOpen {
			// Auto-open chat when sending
			m.zones = layout.Compute(m.zones.TotalWidth, m.zones.TotalHeight, true, m.zones.DrawerOpen)
			m.resizeAll()
		}
		m.chatPanel.Inject(msg.Content)
		m.chatPanel.FocusInput()
		m.focus = layout.FocusChat

	// Log lines from ops
	case logsscreen.AppendLogMsg:
		ls := m.screens[3].(*logsscreen.Screen)
		ls.AppendLines(msg.Lines)
		for _, l := range msg.Lines {
			m.drawer.Append(l.Raw)
		}
	}

	// Propagate to hub polls (re-schedule tickers)
	switch msg.(type) {
	case hub.AlertsUpdatedMsg:
		cmds = append(cmds, m.hubData.PollAlerts())
	case hub.HistoryUpdatedMsg:
		cmds = append(cmds, m.hubData.PollHistory())

	case hub.RecsUpdatedMsg:
		cmds = append(cmds, m.hubData.PollRecs())
	case hub.ConnectionStatusMsg:
		cmds = append(cmds, m.hubData.PingStatus())
	}

	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	if m.zones.TotalWidth == 0 {
		return "Loading kube-watcher..."
	}

	// Header
	header := renderHeader(m)

	// Sidebar + main content + optional chat
	sidebarView := m.sidebar.View()

	var activeScreen ui.Screen = m.screens[m.activeIdx]
	screenView := activeScreen.View()

	var middle string
	if m.zones.ChatOpen {
		chatView := m.chatPanel.View()
		middle = lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, screenView, chatView)
	} else {
		middle = lipgloss.JoinHorizontal(lipgloss.Top, sidebarView, screenView)
	}

	// Drawer
	var drawerView string
	if m.drawer.IsOpen() {
		drawerView = m.drawer.View()
	}

	// Status bar
	statusBar := renderStatusBar(m)

	var parts []string
	parts = append(parts, header)
	parts = append(parts, middle)
	if drawerView != "" {
		parts = append(parts, drawerView)
	}
	parts = append(parts, statusBar)

	page := lipgloss.JoinVertical(lipgloss.Left, parts...)

	// Help modal overlay (centered)
	if m.helpModal.IsOpen() {
		overlay := m.helpModal.View()
		page = placeOverlay(m.zones.TotalWidth, m.zones.TotalHeight, page, overlay)
	}

	return page
}

func renderHeader(m AppModel) string {
	if m.zones.TotalWidth == 0 {
		return "kube-watcher"
	}

	//todo: Simple header for now
	title := "kube-watcher"
	screenTitle := " → " + m.screens[m.activeIdx].Title()
	line := title + screenTitle

	// Pad to width
	pad := m.zones.TotalWidth - len(line)
	if pad > 0 {
		line += strings.Repeat(" ", pad)
	}

	return lipgloss.NewStyle().
		Background(theme.AccentSecondary).
		Foreground(theme.TextPrimary).
		Bold(true).
		Width(m.zones.TotalWidth).
		Padding(0, 1).
		Render(line)
}

func renderStatusBar(m AppModel) string {
	var connBadge string
	if m.connStatus.MCP {
		connBadge = lipgloss.NewStyle().
			Foreground(theme.ColorSuccess).
			Bold(true).
			Render(fmt.Sprintf("🔗 MCP (%dms)", m.connStatus.MCPLatency))
	} else {
		connBadge = lipgloss.NewStyle().
			Foreground(theme.ColorCritical).
			Bold(true).
			Render("🔌 MCP offline")
	}

	alertCount := 0
	if m.activeIdx == 0 { // Alerts screen
		alertsScreen := m.screens[0].(*alertsscreen.Screen)
		alertCount = len(alertsScreen.GetRecords())
	}
	alertBadge := ""
	if alertCount > 0 {
		alertBadge = lipgloss.NewStyle().
			Background(theme.ColorHigh).
			Foreground(theme.TextPrimary).
			Padding(0, 1).
			Bold(true).
			Render(fmt.Sprintf("🚨 %d", alertCount))
	}

	// Keyboard shortcuts with modern styling
	shortcuts := []string{
		"tab→chat",
		"ctrl+l→drawer",
		"1-8→screens",
		"?→help",
		"q→quit",
	}

	shortcutsText := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Render("  " + strings.Join(shortcuts, " • "))

	// Layout
	left := connBadge
	if alertBadge != "" {
		left += " " + alertBadge
	}

	pad := m.zones.TotalWidth - lipgloss.Width(left) - lipgloss.Width(shortcutsText) - 2
	if pad < 1 {
		pad = 1
	}

	return theme.StatusBarStyle(m.zones.TotalWidth).
		Render(left + strings.Repeat(" ", pad) + shortcutsText)
}

func placeOverlay(totalW, totalH int, bg, overlay string) string {
	ow := lipgloss.Width(overlay)
	oh := lipgloss.Height(overlay)
	x := (totalW - ow) / 2
	y := (totalH - oh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	lines := strings.Split(bg, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for i, ol := range overlayLines {
		row := y + i
		if row >= len(lines) {
			break
		}
		line := lines[row]
		lineRunes := []rune(line)
		olRunes := []rune(ol)

		if x >= len(lineRunes) {
			lines[row] = line + strings.Repeat(" ", x-len(lineRunes)) + ol
		} else {
			before := string(lineRunes[:x])
			after := ""
			end := x + len(olRunes)
			if end < len(lineRunes) {
				after = string(lineRunes[end:])
			}
			lines[row] = before + ol + after
		}
		_ = ol
	}
	return strings.Join(lines, "\n")
}

func activeAlertCount(records []data.AlertRecord) int {
	count := 0
	for _, r := range records {
		if r.Status != "acknowledged" && r.Status != "dismissed" {
			count++
		}
	}
	return count
}

func buildAlertContext(r data.AlertRecord) string {
	return fmt.Sprintf("🚨 New alert: %s in %s/%s (cluster: %s)\nReason: %s\nMessage: %s",
		r.Kind, r.Namespace, r.Name, r.Cluster, r.Reason, r.Message)
}

func globalKeyBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("1", "2", "3", "4", "5", "6", "7", "8"), key.WithHelp("1-8", "jump to screen")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "toggle agent chat")),
		key.NewBinding(key.WithKeys("ctrl+l"), key.WithHelp("ctrl+l", "toggle drawer")),
		key.NewBinding(key.WithKeys("ctrl+a"), key.WithHelp("ctrl+a", "focus agent chat")),
		key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "focus sidebar")),
		key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back/close")),
		key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	}
}

func main() {
	m := newAppModel()

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	m.program = p

	popeciSc := m.screens[5].(*popecliscreen.Screen)
	popeciSc.SetProgram(p)

	if runner, err := popecli.New(); err == nil {
		popeciSc.SetRunner(runner)
	} else if envPath := os.Getenv("POPECLI_PATH"); envPath != "" {
		runner := popecli.NewWithPath(envPath)
		popeciSc.SetRunner(runner)
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
