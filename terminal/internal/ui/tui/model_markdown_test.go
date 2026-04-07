package tui

import (
	"testing"

	"kube-watcher/terminal/internal/domain"
)

func TestModel_ShouldRenderMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		block    domain.Block
		content  string
		expected bool
	}{
		{
			name: "Override - render mode markdown forces markdown",
			block: domain.Block{
				RenderMode: domain.RenderModeMarkdown,
			},
			content:  "plain text",
			expected: true,
		},
		{
			name: "Override - render mode plain disables markdown",
			block: domain.Block{
				RenderMode: domain.RenderModePlain,
			},
			content:  "# heading",
			expected: false,
		},
		{
			name: "ContentType - markdown content type renders markdown",
			block: domain.Block{
				RenderMode:  domain.RenderModeAuto,
				ContentType: domain.ContentTypeMarkdown,
			},
			content:  "anything",
			expected: true,
		},
		{
			name: "Heuristic - heading prefix detected",
			block: domain.Block{
				RenderMode:  domain.RenderModeAuto,
				ContentType: domain.ContentTypePlainText,
			},
			content:  "# heading",
			expected: true,
		},
		{
			name: "Heuristic - plain text remains plain",
			block: domain.Block{
				RenderMode:  domain.RenderModeAuto,
				ContentType: domain.ContentTypePlainText,
			},
			content:  "hello world",
			expected: false,
		},
	}

	m := &Model{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := m.shouldRenderMarkdown(&tc.block, tc.content)
			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}
