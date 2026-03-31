package popecliscreen

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"kube-watcher-app/internal/popecli"
	"kube-watcher-app/internal/theme"
	"kube-watcher-app/internal/ui"
)

// Screen invokes popecli and streams its output.
type Screen struct {
	runner    *popecli.Runner
	program   *tea.Program // required for streaming
	nsInput   textinput.Model
	kindInput textinput.Model
	nameInput textinput.Model
	focusIdx  int // 0=ns 1=kind 2=name
	viewport  viewport.Model
	spinner   spinner.Model
	running   bool
	lines     []string
	findings  []popecli.FindingMsg
	width     int
	height    int
}

// New creates a Popecli screen. program is needed for subprocess streaming.
func New(width, height int, program *tea.Program) *Screen {
	nsIn := textinput.New()
	nsIn.Placeholder = "namespace (default)"
	nsIn.Width = 20

	kindIn := textinput.New()
	kindIn.Placeholder = "kind (pods)"
	kindIn.Width = 16

	nameIn := textinput.New()
	nameIn.Placeholder = "name (all)"
	nameIn.Width = 20

	nsIn.Focus()

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.AccentBright)

	return &Screen{
		runner:    nil, // set via SetRunner
		program:   program,
		nsInput:   nsIn,
		kindInput: kindIn,
		nameInput: nameIn,
		viewport:  viewport.New(width-2, height-12),
		spinner:   sp,
		width:     width,
		height:    height,
	}
}

// SetRunner attaches a popecli runner (may be nil if binary not found).
func (s *Screen) SetRunner(r *popecli.Runner) {
	s.runner = r
}

// SetProgram sets the tea.Program reference for subprocess streaming.
func (s *Screen) SetProgram(p *tea.Program) {
	s.program = p
}

func (s Screen) Init() tea.Cmd { return nil }

func (s *Screen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			s.focusIdx = (s.focusIdx + 1) % 3
			s.nsInput.Blur()
			s.kindInput.Blur()
			s.nameInput.Blur()
			switch s.focusIdx {
			case 0:
				s.nsInput.Focus()
			case 1:
				s.kindInput.Focus()
			case 2:
				s.nameInput.Focus()
			}
			return s, nil
		case "ctrl+r", "f5":
			if !s.running && s.runner != nil && s.program != nil {
				s.lines = nil
				s.findings = nil
				s.running = true
				s.viewport.SetContent("")
				cmd := s.runner.Run(s.program,
					s.nsInput.Value(),
					s.kindInput.Value(),
					s.nameInput.Value(),
				)
				return s, tea.Batch(cmd, s.spinner.Tick)
			}
		case "ctrl+g":
			if len(s.findings) > 0 {
				return s, func() tea.Msg {
					return ui.SendToAgentMsg{
						Source:  "popecli",
						Content: buildFindingsContext(s.findings),
					}
				}
			}
		}

	case popecli.StartedMsg:
		return s, s.spinner.Tick

	case popecli.OutputLineMsg:
		s.lines = append(s.lines, string(msg))
		s.viewport.SetContent(s.renderOutput())
		s.viewport.GotoBottom()
		return s, nil

	case popecli.FindingMsg:
		s.findings = append(s.findings, msg)
		s.viewport.SetContent(s.renderOutput())
		s.viewport.GotoBottom()
		return s, nil

	case popecli.ExitMsg:
		s.running = false
		if msg.Err != nil {
			s.lines = append(s.lines, fmt.Sprintf("[exit error] %v", msg.Err))
		} else {
			s.lines = append(s.lines, "[popecli exited successfully]")
		}
		s.viewport.SetContent(s.renderOutput())
		s.viewport.GotoBottom()

		// Auto-send findings to agent
		if len(s.findings) > 0 {
			cmds = append(cmds, func() tea.Msg {
				return ui.SendToAgentMsg{
					Source:  "popecli",
					Content: buildFindingsContext(s.findings),
				}
			})
		}
		return s, tea.Batch(cmds...)

	case spinner.TickMsg:
		if s.running {
			var spCmd tea.Cmd
			s.spinner, spCmd = s.spinner.Update(msg)
			cmds = append(cmds, spCmd)
		}
	}

	switch s.focusIdx {
	case 0:
		var c tea.Cmd
		s.nsInput, c = s.nsInput.Update(msg)
		cmds = append(cmds, c)
	case 1:
		var c tea.Cmd
		s.kindInput, c = s.kindInput.Update(msg)
		cmds = append(cmds, c)
	case 2:
		var c tea.Cmd
		s.nameInput, c = s.nameInput.Update(msg)
		cmds = append(cmds, c)
	}

	var vpCmd tea.Cmd
	s.viewport, vpCmd = s.viewport.Update(msg)
	cmds = append(cmds, vpCmd)

	return s, tea.Batch(cmds...)
}

func (s Screen) View() string {
	// Form
	nsLabel := lipgloss.NewStyle().Foreground(theme.TextSecondary).Render("Namespace:")
	kindLabel := lipgloss.NewStyle().Foreground(theme.TextSecondary).Render("Kind:")
	nameLabel := lipgloss.NewStyle().Foreground(theme.TextSecondary).Render("Name:")

	form := lipgloss.JoinHorizontal(lipgloss.Top,
		nsLabel+" ", s.nsInput.View(),
		"  ", kindLabel+" ", s.kindInput.View(),
		"  ", nameLabel+" ", s.nameInput.View(),
	)

	// Status / run button
	var statusLine string
	if s.runner == nil {
		statusLine = lipgloss.NewStyle().Foreground(theme.ColorCritical).
			Render("⚠ popecli not found on PATH")
	} else if s.running {
		statusLine = s.spinner.View() + lipgloss.NewStyle().Foreground(theme.AccentBright).
			Render(" Running popecli…")
	} else {
		statusLine = lipgloss.NewStyle().Foreground(theme.TextSecondary).
			Render("Press ctrl+r to run")
	}

	// Findings badge
	var findingsBadge string
	if len(s.findings) > 0 {
		findingsBadge = "  " + lipgloss.NewStyle().
			Background(theme.ColorCritical).Foreground(theme.TextInverse).
			Padding(0, 1).Bold(true).
			Render(fmt.Sprintf("%d findings", len(s.findings)))
		findingsBadge += lipgloss.NewStyle().Foreground(theme.TextMuted).
			Render("  ctrl+g → send to agent")
	}

	hint := lipgloss.NewStyle().Foreground(theme.TextMuted).
		Render("  tab=next field  ctrl+r=run  ctrl+g=send findings to agent")

	divider := lipgloss.NewStyle().Foreground(theme.BorderSubtle).
		Render(strings.Repeat("─", s.width-4))

	header := lipgloss.NewStyle().Foreground(theme.AccentBright).Bold(true).
		Width(s.width).Padding(0, 1).Render("⌘ Popecli Runner")

	return theme.PanelStyle().Width(s.width).Height(s.height - 2).
		Render(lipgloss.JoinVertical(lipgloss.Left,
			header,
			form,
			statusLine+findingsBadge,
			divider,
			s.viewport.View(),
			hint,
		))
}

func (s *Screen) Resize(width, height int) {
	s.width = width
	s.height = height
	s.viewport = viewport.New(width-2, height-12)
	s.viewport.SetContent(s.renderOutput())
}

func (s Screen) HelpBindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "run popecli")),
		key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field")),
		key.NewBinding(key.WithKeys("ctrl+g"), key.WithHelp("ctrl+g", "send findings to agent")),
	}
}

func (s Screen) Title() string { return "Popecli" }

func (s Screen) renderOutput() string {
	if len(s.lines) == 0 {
		return lipgloss.NewStyle().Foreground(theme.TextMuted).Italic(true).
			Padding(1, 2).Render("No output yet.")
	}
	var sb strings.Builder
	for _, line := range s.lines {
		var styled string
		if strings.HasPrefix(line, "FINDING") {
			styled = lipgloss.NewStyle().Foreground(theme.ColorCritical).Render(line)
		} else if strings.HasPrefix(line, "[exit") {
			styled = lipgloss.NewStyle().Foreground(theme.ColorSuccess).Render(line)
		} else if strings.HasPrefix(line, "[stderr]") {
			styled = lipgloss.NewStyle().Foreground(theme.ColorMedium).Render(line)
		} else {
			styled = lipgloss.NewStyle().Foreground(theme.TextPrimary).Render(line)
		}
		sb.WriteString(styled + "\n")
	}
	return sb.String()
}

func buildFindingsContext(findings []popecli.FindingMsg) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("popecli found %d issues:\n\n", len(findings)))
	for i, f := range findings {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s: %s\n", i+1, strings.ToUpper(f.Severity), f.Resource, f.Message))
	}
	sb.WriteString("\nPlease analyze these findings and suggest remediation steps.")
	return sb.String()
}
