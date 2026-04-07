package domain

import "time"

const (
	ContentTypePlainText = "text/plain"
	ContentTypeMarkdown  = "text/markdown"
	RenderModeAuto       = "auto"
	RenderModeMarkdown   = "markdown"
	RenderModePlain      = "plain"
)

type PTYOutputMsg []byte

type PTYExitMsg struct {
	Err      error
	ExitCode int
}

type Artifact struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
}

type AgentGhostTextMsg struct {
	Text string
}

type AgentClearGhostTextMsg struct{}

type AgentPinArtifactMsg struct {
	Artifact Artifact
}
type AgentSendInputMsg struct {
	Text string
}

type AgentCreateSplitMsg struct {
	SplitID string
}

type AgentFocusSplitMsg struct {
	SplitID string
}

type AgentOpenURLMsg struct {
	URL string
}

type AgentDiagnosticRequestMsg struct {
	Reason   string
	ExitCode int
}

type StartPTYMsg struct{}

func NewStartPTYMsg() StartPTYMsg {
	return StartPTYMsg{}
}

type Block struct {
	ID          string
	Command     string
	RawOutput   []byte
	ContentType string
	RenderMode  string
	Active      bool
	Timestamp   time.Time
	StartedAt   time.Time
	CompletedAt time.Time
	Duration    time.Duration
	CWD         string
	RenderY     int
	Height      int
	ExitCode    int
	HasError    bool
}
