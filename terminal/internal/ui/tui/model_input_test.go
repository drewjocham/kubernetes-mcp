package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	"kube-watcher/terminal/internal/domain"
)

func TestModel_AcceptGhostText(t *testing.T) {
	tests := []struct {
		name          string
		currentInput  string
		ghostText     string
		expectApplied bool
		expectedInput string
	}{
		{
			name:          "Success - accepts matching ghost suffix",
			currentInput:  "go te",
			ghostText:     "go test ./...",
			expectApplied: true,
			expectedInput: "go test ./...",
		},
		{
			name:          "NoOp - ghost is empty",
			currentInput:  "go te",
			ghostText:     "",
			expectApplied: false,
			expectedInput: "go te",
		},
		{
			name:          "NoOp - ghost does not match prefix",
			currentInput:  "kubectl",
			ghostText:     "go test ./...",
			expectApplied: false,
			expectedInput: "kubectl",
		},
		{
			name:          "NoOp - ghost equals current input",
			currentInput:  "go test",
			ghostText:     "go test",
			expectApplied: false,
			expectedInput: "go test",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := textinput.New()
			input.SetValue(tc.currentInput)
			m := &Model{
				inputField: input,
				ghostText:  tc.ghostText,
			}

			applied := m.acceptGhostText()
			if applied != tc.expectApplied {
				t.Fatalf("expected applied=%v, got %v", tc.expectApplied, applied)
			}
			if m.inputField.Value() != tc.expectedInput {
				t.Fatalf("expected input %q, got %q", tc.expectedInput, m.inputField.Value())
			}
		})
	}
}

func TestModel_SelectArtifactTabs(t *testing.T) {
	tests := []struct {
		name              string
		startIdx          int
		artifacts         []domain.Artifact
		call              func(*Model)
		expectedActiveIdx int
	}{
		{
			name: "Success - next cycles forward",
			artifacts: []domain.Artifact{
				{ID: "a1", Title: "A"},
				{ID: "a2", Title: "B"},
				{ID: "a3", Title: "C"},
			},
			startIdx:          0,
			call:              (*Model).selectNextArtifactTab,
			expectedActiveIdx: 1,
		},
		{
			name: "Success - next wraps",
			artifacts: []domain.Artifact{
				{ID: "a1", Title: "A"},
				{ID: "a2", Title: "B"},
			},
			startIdx:          1,
			call:              (*Model).selectNextArtifactTab,
			expectedActiveIdx: 0,
		},
		{
			name: "Success - previous wraps",
			artifacts: []domain.Artifact{
				{ID: "a1", Title: "A"},
				{ID: "a2", Title: "B"},
			},
			startIdx:          0,
			call:              (*Model).selectPreviousArtifactTab,
			expectedActiveIdx: 1,
		},
		{
			name:              "NoOp - empty artifact list",
			artifacts:         nil,
			startIdx:          0,
			call:              (*Model).selectPreviousArtifactTab,
			expectedActiveIdx: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &Model{
				artifacts:         tc.artifacts,
				activeArtifactIdx: tc.startIdx,
			}
			tc.call(m)
			if m.activeArtifactIdx != tc.expectedActiveIdx {
				t.Fatalf("expected active index %d, got %d", tc.expectedActiveIdx, m.activeArtifactIdx)
			}
		})
	}
}
