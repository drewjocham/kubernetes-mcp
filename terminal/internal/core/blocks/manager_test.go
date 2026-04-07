package blocks

import (
	"testing"

	"kube-watcher/terminal/internal/domain"
)

func TestManager_ToggleBlockRenderMode(t *testing.T) {
	tests := []struct {
		name          string
		initialMode   string
		expectedModes []string
	}{
		{
			name:          "Cycles - auto to markdown to plain to auto",
			initialMode:   domain.RenderModeAuto,
			expectedModes: []string{domain.RenderModeMarkdown, domain.RenderModePlain, domain.RenderModeAuto},
		},
		{
			name:          "Cycles - plain to auto",
			initialMode:   domain.RenderModePlain,
			expectedModes: []string{domain.RenderModeAuto},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManager()
			b := m.Blocks()[0]
			b.RenderMode = tc.initialMode

			for _, want := range tc.expectedModes {
				got := m.ToggleBlockRenderMode(b.ID)
				if got != want {
					t.Fatalf("expected mode %q, got %q", want, got)
				}
			}
		})
	}
}

func TestManager_SealAndNew_PrunesAndSetsMetadata(t *testing.T) {
	tests := []struct {
		name       string
		sealCount  int
		wantMaxLen int
	}{
		{
			name:       "Prunes - exceeds max live blocks",
			sealCount:  MaxLiveBlocks + 10,
			wantMaxLen: MaxLiveBlocks,
		},
		{
			name:       "Keeps - under max live blocks",
			sealCount:  5,
			wantMaxLen: 6, // initial + 5 seals
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := NewManager()
			for i := 0; i < tc.sealCount; i++ {
				m.SetActiveCommand("echo test")
				m.SetActiveCWD("/tmp/project")
				m.SealAndNew()
			}

			if len(m.Blocks()) != tc.wantMaxLen {
				t.Fatalf("expected %d blocks, got %d", tc.wantMaxLen, len(m.Blocks()))
			}

			for _, block := range m.Blocks() {
				if block.Active {
					continue
				}
				if block.Command != "" && block.CompletedAt.IsZero() {
					t.Fatalf("expected completed timestamp for sealed block")
				}
				if block.Command != "" && block.Duration < 0 {
					t.Fatalf("expected non-negative duration")
				}
			}
		})
	}
}
