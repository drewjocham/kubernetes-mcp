package popecli

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Message types emitted to AppModel via p.Send()

// OutputLineMsg carries a raw stdout/stderr line for viewport display.
type OutputLineMsg string

// FindingMsg carries a parsed finding from popecli output.
type FindingMsg struct {
	Raw      string
	Severity string
	Resource string
	Message  string
}

// StartedMsg signals that the process started successfully.
type StartedMsg struct{}

// ExitMsg signals that the process has finished.
type ExitMsg struct{ Err error }

// findingRegex matches lines like:
//
//	FINDING: message
//	FINDING[HIGH]: resource: message
var findingRegex = regexp.MustCompile(`^FINDING(?:\[(\w+)\])?:\s*(?:(\S+):\s*)?(.+)$`)

// Runner executes the popecli binary and streams its output.
type Runner struct {
	BinaryPath string
}

// New returns a Runner after resolving the popecli binary on PATH.
func New() (*Runner, error) {
	path, err := exec.LookPath("popecli")
	if err != nil {
		return nil, fmt.Errorf("popecli not found on PATH: %w", err)
	}
	return &Runner{BinaryPath: path}, nil
}

// NewWithPath returns a Runner with an explicit binary path.
func NewWithPath(path string) *Runner {
	return &Runner{BinaryPath: path}
}

// Run starts popecli with the given arguments and streams output via p.Send().
// It returns a tea.Cmd that emits StartedMsg (or ExitMsg on immediate failure).
// Further output arrives as OutputLineMsg and FindingMsg messages.
func (r *Runner) Run(p *tea.Program, namespace, kind, name string) tea.Cmd {
	return func() tea.Msg {
		args := []string{}
		if namespace != "" {
			args = append(args, "-n", namespace)
		}
		if kind != "" {
			args = append(args, kind)
		}
		if name != "" {
			args = append(args, name)
		}

		cmd := exec.Command(r.BinaryPath, args...)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return ExitMsg{Err: fmt.Errorf("stdout pipe: %w", err)}
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return ExitMsg{Err: fmt.Errorf("stderr pipe: %w", err)}
		}

		if err := cmd.Start(); err != nil {
			return ExitMsg{Err: fmt.Errorf("start: %w", err)}
		}

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				p.Send(OutputLineMsg(line))
				if f, ok := parseFindings(line); ok {
					p.Send(f)
				}
			}
		}()

		go func() {
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				p.Send(OutputLineMsg("[stderr] " + scanner.Text()))
			}
		}()

		go func() {
			err := cmd.Wait()
			p.Send(ExitMsg{Err: err})
		}()

		return StartedMsg{}
	}
}

func parseFindings(line string) (FindingMsg, bool) {
	m := findingRegex.FindStringSubmatch(strings.TrimSpace(line))
	if m == nil {
		return FindingMsg{}, false
	}
	severity := strings.ToLower(m[1])
	if severity == "" {
		severity = "medium"
	}
	return FindingMsg{
		Raw:      line,
		Severity: severity,
		Resource: m[2],
		Message:  m[3],
	}, true
}
