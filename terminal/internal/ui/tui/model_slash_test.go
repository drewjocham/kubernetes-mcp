package tui

import (
	"context"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"

	markdownadapter "kube-watcher/terminal/internal/adapters/markdown"
	"kube-watcher/terminal/internal/core/blocks"
	"kube-watcher/terminal/internal/domain"
)

func TestParseSlashCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantName string
		wantArgs []string
	}{
		{name: "help no args", input: "/help", wantName: "help"},
		{name: "widgets refresh", input: "/widgets refresh", wantName: "widgets", wantArgs: []string{"refresh"}},
		{name: "trim and lower", input: "   /Agent   ", wantName: "agent"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotArgs := parseSlashCommand(tc.input)
			if gotName != tc.wantName {
				t.Fatalf("expected name %q, got %q", tc.wantName, gotName)
			}
			if strings.Join(gotArgs, ",") != strings.Join(tc.wantArgs, ",") {
				t.Fatalf("expected args %v, got %v", tc.wantArgs, gotArgs)
			}
		})
	}
}

func TestModel_ExecuteCommand_SlashCommands(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		widgets        []domain.Widget
		expectContains string
		expectError    bool
	}{
		{
			name:           "help command renders help output",
			command:        "/help",
			expectContains: "Terminal Commands",
		},
		{
			name:           "agent command renders agent output",
			command:        "/agent",
			expectContains: "Agent Controls",
		},
		{
			name:           "widgets command lists widgets",
			command:        "/widgets",
			widgets:        []domain.Widget{widgetStub{id: "w1", title: "System Stats"}},
			expectContains: "System Stats",
		},
		{
			name:           "unknown slash command marks error",
			command:        "/doesnotexist",
			expectContains: "Unknown slash command",
			expectError:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := newSlashTestModel(tc.widgets)
			m.executeCommand(tc.command)

			if len(m.blockManager.Blocks()) < 2 {
				t.Fatalf("expected sealed block plus active block, got %d blocks", len(m.blockManager.Blocks()))
			}

			previous := m.blockManager.Blocks()[len(m.blockManager.Blocks())-2]
			if !strings.Contains(string(previous.RawOutput), tc.expectContains) {
				t.Fatalf("expected output to contain %q, got %q", tc.expectContains, string(previous.RawOutput))
			}
			if tc.expectError && !previous.HasError {
				t.Fatalf("expected previous block to be marked as error")
			}
		})
	}
}

type widgetStub struct {
	id    string
	title string
}

func (w widgetStub) ID() string                      { return w.id }
func (w widgetStub) Title() string                   { return w.title }
func (w widgetStub) Refresh(_ context.Context) error { return nil }
func (w widgetStub) View() string                    { return "ok" }

func newSlashTestModel(widgets []domain.Widget) *Model {
	input := textinput.New()
	return &Model{
		blockManager: blocks.NewManager(),
		inputField:   input,
		plain:        markdownadapter.NewPlainStrategy(),
		widgets:      widgets,
	}
}
