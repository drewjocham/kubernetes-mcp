package chat

import (
	"fmt"
	"strings"
	"time"

	"kube-watcher-app/internal/agent"
	"kube-watcher-app/internal/theme"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Role int

const (
	RoleUser Role = iota
	RoleAgent
	RoleSystem
)

// Message is a single chat turn.
type Message struct {
	Role      Role
	Content   string
	Timestamp time.Time
}

// UnreadMsg is sent to the AppModel to update the badge count.
type UnreadMsg struct{ Count int }

// Model is the persistent right-side agent chat panel.
type Model struct {
	messages   []Message
	viewport   viewport.Model
	input      textarea.Model
	agentCli   *agent.Client
	thinking   bool
	width      int
	height     int
	unread     int
	inputFocus bool
}

// New creates a chat panel with the given agent client.
func New(agentCli *agent.Client, width, height int) Model {
	ta := textarea.New()
	ta.Placeholder = "Ask the agent..."
	ta.SetWidth(width - 4)
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.CharLimit = 4096

	vp := viewport.New(width-2, height-8)
	vp.SetContent(welcomeMsg())

	return Model{
		agentCli: agentCli,
		viewport: vp,
		input:    ta,
		width:    width,
		height:   height,
	}
}

func welcomeMsg() string {
	style := lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true)
	return style.Render("  Agent ready. Alerts and findings are automatically sent here.\n  Press ctrl+a to focus this panel.")
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+g":
			// Send focused message to agent
			if !m.thinking && strings.TrimSpace(m.input.Value()) != "" {
				return m.sendMessage()
			}
		case "enter":
			if !msg.Alt && !m.thinking && strings.TrimSpace(m.input.Value()) != "" {
				return m.sendMessage()
			}
			var taCmd tea.Cmd
			m.input, taCmd = m.input.Update(msg)
			cmds = append(cmds, taCmd)
		default:
			var taCmd tea.Cmd
			m.input, taCmd = m.input.Update(msg)
			cmds = append(cmds, taCmd)
		}

	case agent.ResponseMsg:
		m.thinking = false
		if msg.Err != nil {
			m.addMessage(RoleSystem, "⚠ Agent error: "+msg.Err.Error())
		} else {
			m.addMessage(RoleAgent, msg.Content)
		}
		m.viewport.GotoBottom()
		m.refreshViewport()

	case InjectMsg:
		m.addMessage(RoleSystem, msg.Content)
		m.unread++
		m.viewport.GotoBottom()
		m.refreshViewport()
		cmds = append(cmds, func() tea.Msg { return UnreadMsg{Count: m.unread} })
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return m, tea.Batch(cmds...)
}

// FocusInput activates the text input.
func (m *Model) FocusInput() {
	m.inputFocus = true
	m.unread = 0
	m.input.Focus()
}

// BlurInput deactivates the text input.
func (m *Model) BlurInput() {
	m.inputFocus = false
	m.input.Blur()
}

// Inject pushes a system-sourced message (alert, finding) into the chat.
func (m *Model) Inject(content string) {
	m.addMessage(RoleSystem, content)
	m.unread++
	m.viewport.GotoBottom()
	m.refreshViewport()
}

// UnreadCount returns the current unread message count.
func (m Model) UnreadCount() int { return m.unread }

func (m *Model) sendMessage() (Model, tea.Cmd) {
	text := strings.TrimSpace(m.input.Value())
	m.input.Reset()
	m.addMessage(RoleUser, text)
	m.thinking = true
	m.viewport.GotoBottom()
	m.refreshViewport()
	return *m, m.agentCli.Send(m.toAgentHistory())
}

func (m *Model) addMessage(role Role, content string) {
	m.messages = append(m.messages, Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
}

func (m *Model) toAgentHistory() []agent.Message {
	var history []agent.Message
	// System prompt
	history = append(history, agent.Message{
		Role:    "system",
		Content: "You are a Kubernetes SRE assistant. The user is debugging cluster issues. Be concise and actionable.",
	})
	for _, msg := range m.messages {
		switch msg.Role {
		case RoleUser:
			history = append(history, agent.Message{Role: "user", Content: msg.Content})
		case RoleAgent:
			history = append(history, agent.Message{Role: "assistant", Content: msg.Content})
		case RoleSystem:
			history = append(history, agent.Message{Role: "user", Content: "[Context] " + msg.Content})
		}
	}
	return history
}

func (m *Model) refreshViewport() {
	m.viewport.SetContent(m.renderMessages())
}

func (m Model) renderMessages() string {
	if len(m.messages) == 0 {
		return welcomeMsg()
	}
	var sb strings.Builder
	w := m.width - 4
	if w < 10 {
		w = 10
	}
	for _, msg := range m.messages {
		ts := lipgloss.NewStyle().Foreground(theme.TextMuted).Render(msg.Timestamp.Format("15:04"))
		var block string
		switch msg.Role {
		case RoleUser:
			block = theme.ChatMessageUserStyle(w).Render(msg.Content)
			sb.WriteString(lipgloss.JoinVertical(lipgloss.Right, ts, block))
		case RoleAgent:
			block = theme.ChatMessageAgentStyle(w).Render(msg.Content)
			sb.WriteString(lipgloss.JoinVertical(lipgloss.Left, ts, block))
		case RoleSystem:
			block = theme.ChatMessageSystemStyle(w).Render(msg.Content)
			sb.WriteString(lipgloss.JoinVertical(lipgloss.Left, ts, block))
		}
		sb.WriteString("\n\n")
	}
	if m.thinking {
		dots := lipgloss.NewStyle().Foreground(theme.AccentBright).Render("  ● thinking…")
		sb.WriteString(dots + "\n")
	}
	return sb.String()
}

// Resize updates panel dimensions.
func (m *Model) Resize(width, height int) {
	m.width = width
	m.height = height
	vpH := height - 9
	if vpH < 3 {
		vpH = 3
	}
	m.viewport = viewport.New(width-2, vpH)
	m.input.SetWidth(width - 4)
	m.refreshViewport()
}

func (m Model) View() string {
	if m.width < 10 {
		return ""
	}

	titleBar := lipgloss.NewStyle().
		Background(theme.BgOverlay).
		Foreground(theme.AccentBright).
		Bold(true).
		Width(m.width).
		Padding(0, 1).
		Render("⊹ Agent Chat")

	vpView := m.viewport.View()

	var inputView string
	if m.inputFocus {
		inputView = theme.ChatInputStyle(m.width).Render(m.input.View())
	} else {
		dimInput := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.BorderSubtle).
			Padding(0, 1).
			Width(m.width - 4).
			Foreground(theme.TextMuted).
			Render("Press ctrl+a to type…")
		inputView = dimInput
	}

	hint := lipgloss.NewStyle().
		Foreground(theme.TextMuted).
		Width(m.width).
		Render(fmt.Sprintf("  enter=send  ctrl+a=focus  tab=close"))

	panel := lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		vpView,
		inputView,
		hint,
	)

	borderStyle := theme.PanelStyle().Width(m.width).Height(m.height)
	if m.inputFocus {
		borderStyle = theme.PanelFocusedStyle().Width(m.width).Height(m.height)
	}
	return borderStyle.Render(panel)
}

// InjectMsg carries context from screens to the chat panel.
type InjectMsg struct {
	Content string
}
