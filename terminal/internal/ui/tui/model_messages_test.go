package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	markdownadapter "kube-watcher/terminal/internal/adapters/markdown"

	"kube-watcher/terminal/internal/core/blocks"
	"kube-watcher/terminal/internal/domain"
)

func TestModel_Update_GhostAndArtifactMessages(t *testing.T) {
	tests := []struct {
		name      string
		start     func(*Model)
		msg       any
		checkFunc func(t *testing.T, m *Model)
	}{
		{
			name: "Ghost - set ghost text",
			msg:  domain.AgentGhostTextMsg{Text: "go test ./..."},
			checkFunc: func(t *testing.T, m *Model) {
				t.Helper()
				if m.ghostText != "go test ./..." {
					t.Fatalf("expected ghost text to be set, got %q", m.ghostText)
				}
			},
		},
		{
			name: "Ghost - clear ghost text",
			start: func(m *Model) {
				m.ghostText = "go test ./..."
			},
			msg: domain.AgentClearGhostTextMsg{},
			checkFunc: func(t *testing.T, m *Model) {
				t.Helper()
				if m.ghostText != "" {
					t.Fatalf("expected ghost text to be cleared, got %q", m.ghostText)
				}
			},
		},
		{
			name: "Artifact - pin and activate",
			msg: domain.AgentPinArtifactMsg{Artifact: domain.Artifact{
				ID:      "a1",
				Title:   "Snapshot",
				Kind:    domain.ContentTypeMarkdown,
				Content: "content",
			}},
			checkFunc: func(t *testing.T, m *Model) {
				t.Helper()
				if len(m.artifacts) != 1 {
					t.Fatalf("expected one artifact, got %d", len(m.artifacts))
				}
				if m.activeArtifactIdx != 0 {
					t.Fatalf("expected active artifact index 0, got %d", m.activeArtifactIdx)
				}
				if m.artifacts[0].ID != "a1" {
					t.Fatalf("expected artifact id a1, got %q", m.artifacts[0].ID)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newMessageTestModel()
			if tc.start != nil {
				tc.start(m)
			}
			_, _ = m.Update(tc.msg)
			tc.checkFunc(t, m)
		})
	}
}

func TestModel_Update_PTYExit_DiagnosticEmission(t *testing.T) {
	tests := []struct {
		name             string
		msg              domain.PTYExitMsg
		expectDiagnostic bool
		expectedExitCode int
	}{
		{
			name: "Diagnostic - non zero exit with error emits diagnostic",
			msg: domain.PTYExitMsg{
				Err:      errString("pty failed"),
				ExitCode: 2,
			},
			expectDiagnostic: true,
			expectedExitCode: 2,
		},
		{
			name: "No diagnostic - zero exit",
			msg: domain.PTYExitMsg{
				Err:      nil,
				ExitCode: 0,
			},
			expectDiagnostic: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got *domain.AgentDiagnosticRequestMsg
			m := newMessageTestModel()
			m.onDiagnosticRequest = func(msg domain.AgentDiagnosticRequestMsg) {
				clone := msg
				got = &clone
			}
			_, _ = m.Update(tc.msg)

			if tc.expectDiagnostic {
				if got == nil {
					t.Fatalf("expected diagnostic callback to be invoked")
				}
				if got.ExitCode != tc.expectedExitCode {
					t.Fatalf("expected diagnostic exit code %d, got %d", tc.expectedExitCode, got.ExitCode)
				}
			} else if got != nil {
				t.Fatalf("expected no diagnostic callback, got %+v", *got)
			}
		})
	}
}

func newMessageTestModel() *Model {
	input := textinput.New()
	return &Model{
		blockManager: blocks.NewManager(),
		inputField:   input,
		plain:        markdownadapter.NewPlainStrategy(),
	}
}

type errString string

func (e errString) Error() string { return string(e) }
