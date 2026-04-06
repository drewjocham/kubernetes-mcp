package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	markdownadapter "kube-watcher/terminal/internal/adapters/markdown"
	"kube-watcher/terminal/internal/core/blocks"
	terminalcore "kube-watcher/terminal/internal/core/terminal"
	"kube-watcher/terminal/internal/domain"
)

const maxBlockOutputBytes = 512 * 1024

var ansiRegexp = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

type Option func(*Model)

func WithBlocksChangedCallback(cb func([]domain.Block)) Option {
	return func(m *Model) {
		m.onBlocksChanged = cb
	}
}

func (m *Model) executeCommand(commandText string) {
	command := strings.TrimSpace(commandText)
	if command != "" {
		m.blockManager.SetActiveCommand(command)
	}
	_ = m.ptyHandler.Write([]byte(commandText + "\n"))
	m.blockManager.SealAndNew()
	m.inputField.SetValue("")
	m.ghostText = ""
	m.highlightedBlockID = ""
	m.refreshViewport()
	m.viewport.GotoBottom()
}

func (m *Model) createSplit(splitID string) {
	for _, id := range m.splits {
		if id == splitID {
			m.activeSplitID = splitID
			return
		}
	}
	m.splits = append(m.splits, splitID)
	m.activeSplitID = splitID
}

func (m *Model) focusSplit(splitID string) {
	for _, id := range m.splits {
		if id == splitID {
			m.activeSplitID = splitID
			return
		}
	}
}

func WithDiagnosticRequestCallback(cb func(domain.AgentDiagnosticRequestMsg)) Option {
	return func(m *Model) {
		m.onDiagnosticRequest = cb
	}
}

type Model struct {
	ptyHandler *terminalcore.Handler
	program    *tea.Program
	shellPath  string

	blockManager *blocks.Manager
	markdown     domain.RenderStrategy
	plain        domain.RenderStrategy

	inputField textinput.Model
	viewport   viewport.Model

	ready  bool
	width  int
	height int

	ghostText          string
	highlightedBlockID string
	artifacts          []domain.Artifact
	activeArtifactIdx  int
	splits             []string
	activeSplitID      string

	onBlocksChanged     func([]domain.Block)
	onDiagnosticRequest func(domain.AgentDiagnosticRequestMsg)
}

func NewModel(handler *terminalcore.Handler, shellPath string, opts ...Option) *Model {
	input := textinput.New()
	input.Prompt = "> "
	input.Placeholder = "Type a command and press Enter"
	input.Focus()
	input.CharLimit = 0
	input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	markdown, _ := markdownadapter.NewGlamourStrategy(120)

	model := &Model{
		ptyHandler:    handler,
		shellPath:     shellPath,
		blockManager:  blocks.NewManager(),
		markdown:      markdown,
		plain:         markdownadapter.NewPlainStrategy(),
		inputField:    input,
		splits:        []string{"main"},
		activeSplitID: "main",
	}

	for _, opt := range opts {
		opt(model)
	}

	return model
}

func (m *Model) SetProgram(program *tea.Program) {
	m.program = program
}

func (m *Model) Init() tea.Cmd {
	return func() tea.Msg {
		return domain.NewStartPTYMsg()
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case domain.StartPTYMsg:
		return m.handleStartPTY()
	case domain.PTYOutputMsg:
		return m.handlePTYOutput([]byte(msg))
	case domain.PTYExitMsg:
		return m.handlePTYExit(msg)
	case domain.AgentGhostTextMsg:
		m.ghostText = msg.Text
		return m, nil
	case domain.AgentClearGhostTextMsg:
		m.ghostText = ""
		return m, nil
	case domain.AgentPinArtifactMsg:
		m.pinArtifact(msg.Artifact)
		return m, nil
	case domain.AgentSendInputMsg:
		m.executeCommand(msg.Text)
		return m, nil
	case domain.AgentCreateSplitMsg:
		m.createSplit(msg.SplitID)
		return m, nil
	case domain.AgentFocusSplitMsg:
		m.focusSplit(msg.SplitID)
		return m, nil
	case domain.AgentOpenURLMsg:
		m.pinArtifact(domain.Artifact{
			ID:      "url:" + msg.URL,
			Title:   "Browser Snapshot",
			Kind:    domain.ContentTypeMarkdown,
			Content: fmt.Sprintf("Open URL requested:\n\n%s\n\nBrowser snapshot integration is not enabled yet in this build.", msg.URL),
		})
		return m, nil
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	default:
		var cmd tea.Cmd
		m.inputField, cmd = m.inputField.Update(msg)
		return m, cmd
	}
}

func (m *Model) View() string {
	if !m.ready {
		return "initializing terminal..."
	}

	inputStyle := lipgloss.NewStyle().
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("8")).
		Padding(0, 1)

	inputView := inputStyle.Width(max(1, m.mainWidth())).Render(m.renderInputWithGhost())
	mainPane := lipgloss.JoinVertical(lipgloss.Left, m.viewport.View(), inputView)
	if len(m.artifacts) == 0 {
		return mainPane
	}

	sidebarWidth := max(28, m.width/3)
	mainStyled := lipgloss.NewStyle().Width(max(1, m.width-sidebarWidth)).Render(mainPane)
	sidebar := lipgloss.NewStyle().
		Width(sidebarWidth).
		BorderLeft(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("8")).
		PaddingLeft(1).
		Render(m.renderArtifacts())

	return lipgloss.JoinHorizontal(lipgloss.Top, mainStyled, sidebar)
}

func (m *Model) handleStartPTY() (tea.Model, tea.Cmd) {
	if m.program == nil {
		m.blockManager.AppendToActive([]byte("\r\n[error] bubbletea program is not initialized\r\n"))
		m.refreshViewport()
		return m, tea.Quit
	}

	if err := m.ptyHandler.Start(context.Background(), m.shellPath, func(out any) {
		m.program.Send(out)
	}); err != nil {
		m.blockManager.AppendToActive([]byte(fmt.Sprintf("\r\n[error] failed to start shell: %v\r\n", err)))
		m.refreshViewport()
		return m, tea.Quit
	}

	if m.width > 0 && m.height > 0 {
		_ = m.ptyHandler.Resize(m.width, max(1, m.height-2))
	}

	return m, nil
}

func (m *Model) handlePTYOutput(data []byte) (tea.Model, tea.Cmd) {
	m.blockManager.AppendToActive(limitBytes(data))
	m.refreshViewport()
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handlePTYExit(msg domain.PTYExitMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.blockManager.AppendToActive([]byte(fmt.Sprintf("\r\n[pty exited with error] %v\r\n", msg.Err)))
		if msg.ExitCode != 0 {
			m.blockManager.MarkLastBlockExit(msg.ExitCode, true)
			if m.onDiagnosticRequest != nil {
				m.onDiagnosticRequest(domain.AgentDiagnosticRequestMsg{
					Reason:   "pty process exited with non-zero status",
					ExitCode: msg.ExitCode,
				})
			}
		}
	}
	m.refreshViewport()
	return m, tea.Quit
}

func (m *Model) handleWindowSize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.ready = true
	m.width = msg.Width
	m.height = msg.Height

	m.viewport = viewport.New(max(1, m.mainWidth()), max(1, msg.Height-3))
	m.inputField.Width = max(1, m.mainWidth()-4)
	_ = m.ptyHandler.Resize(msg.Width, max(1, msg.Height-3))

	m.refreshViewport()
	m.viewport.GotoBottom()
	return m, nil
}

func (m *Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Type != tea.MouseLeft {
		var cmd tea.Cmd
		m.inputField, cmd = m.inputField.Update(msg)
		return m, cmd
	}

	inputLine := max(0, m.height-1)
	if msg.Y >= inputLine {
		cursor := msg.X - 2
		if cursor < 0 {
			cursor = 0
		}
		if cursor > len(m.inputField.Value()) {
			cursor = len(m.inputField.Value())
		}
		m.inputField.SetCursor(cursor)
		m.inputField.Focus()
		return m, nil
	}

	clickY := m.viewport.YOffset + msg.Y
	block := m.blockManager.FindByRenderY(clickY)
	if block == nil {
		m.highlightedBlockID = ""
		m.refreshViewport()
		return m, nil
	}

	m.highlightedBlockID = block.ID
	if block.Command != "" {
		m.inputField.SetValue(block.Command)
		m.inputField.SetCursor(len(block.Command))
	}
	m.inputField.Focus()
	m.refreshViewport()
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		_ = m.ptyHandler.Close()
		return m, tea.Quit
	case tea.KeyEnter:
		m.executeCommand(m.inputField.Value())
		return m, nil
	default:
		var cmd tea.Cmd
		m.inputField, cmd = m.inputField.Update(msg)
		return m, cmd
	}
}

func (m *Model) refreshViewport() {
	var rendered []string
	currentY := 0

	for _, block := range m.blockManager.Blocks() {
		output := string(block.RawOutput)
		content := output

		if m.shouldRenderMarkdown(block, output) && m.markdown != nil {
			renderedContent, err := m.markdown.Render([]byte(stripANSI(output)))
			if err == nil {
				content = renderedContent
			}
		} else {
			plainContent, err := m.plain.Render(block.RawOutput)
			if err == nil {
				content = plainContent
			}
		}

		header := "$ " + block.Command
		if strings.TrimSpace(block.Command) == "" {
			header = "$"
		}
		if block.HasError {
			header = header + fmt.Sprintf("  [exit=%d]", block.ExitCode)
		}

		borderColor := "8"
		if block.Active {
			borderColor = "10"
		}
		if block.ID == m.highlightedBlockID {
			borderColor = "33"
		}
		if block.HasError {
			borderColor = "196"
		}

		blockStyle := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color(borderColor)).
			PaddingLeft(1).
			MarginBottom(1)

		headerStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("245")).
			Bold(true)

		blockView := blockStyle.Render(
			headerStyle.Render(header) + "\n" + content,
		)

		blockHeight := max(1, lipgloss.Height(blockView))
		m.blockManager.SetRenderBounds(block.ID, currentY, blockHeight)
		currentY += blockHeight
		rendered = append(rendered, blockView)
	}

	m.viewport.Width = max(1, m.mainWidth())
	m.viewport.SetContent(strings.Join(rendered, "\n"))
	m.publishBlocksSnapshot()
}

func (m *Model) publishBlocksSnapshot() {
	if m.onBlocksChanged == nil {
		return
	}

	currentBlocks := m.blockManager.Blocks()
	out := make([]domain.Block, 0, len(currentBlocks))
	for _, block := range currentBlocks {
		out = append(out, *block)
	}
	m.onBlocksChanged(out)
}

func (m *Model) shouldRenderMarkdown(block *domain.Block, content string) bool {
	if block.ContentType == domain.ContentTypeMarkdown {
		return true
	}
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "# ") ||
		strings.HasPrefix(trimmed, "```") ||
		strings.HasPrefix(trimmed, "|")
}

func (m *Model) renderInputWithGhost() string {
	base := m.inputField.View()
	input := m.inputField.Value()

	if m.ghostText == "" || !strings.HasPrefix(m.ghostText, input) {
		return base
	}

	suffix := strings.TrimPrefix(m.ghostText, input)
	ghost := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(suffix)

	return base + ghost
}

func (m *Model) pinArtifact(artifact domain.Artifact) {
	if artifact.ID == "" {
		return
	}
	for i := range m.artifacts {
		if m.artifacts[i].ID == artifact.ID {
			m.artifacts[i] = artifact
			m.activeArtifactIdx = i
			return
		}
	}
	m.artifacts = append(m.artifacts, artifact)
	m.activeArtifactIdx = len(m.artifacts) - 1
}

func (m *Model) renderArtifacts() string {
	if len(m.artifacts) == 0 {
		return ""
	}

	var tabs []string
	for i := range m.artifacts {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
		if i == m.activeArtifactIdx {
			style = style.Foreground(lipgloss.Color("33")).Bold(true)
		}
		tabs = append(tabs, style.Render(m.artifacts[i].Title))
	}

	active := m.artifacts[m.activeArtifactIdx]
	content := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Render(active.Content)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Left, tabs...),
		"",
		content,
	)
}

func (m *Model) mainWidth() int {
	if len(m.artifacts) == 0 {
		return m.width
	}
	return max(1, m.width-max(28, m.width/3))
}

func stripANSI(s string) string {
	return ansiRegexp.ReplaceAllString(s, "")
}

func limitBytes(data []byte) []byte {
	if len(data) <= maxBlockOutputBytes {
		return data
	}
	return data[len(data)-maxBlockOutputBytes:]
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
