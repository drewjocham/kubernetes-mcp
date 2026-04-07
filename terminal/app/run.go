package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	browseradapter "kube-watcher/terminal/internal/adapters/browser"

	ptyadapter "kube-watcher/terminal/internal/adapters/pty"
	agentsocket "kube-watcher/terminal/internal/agent/socket"
	browsercore "kube-watcher/terminal/internal/core/browser"
	terminalcore "kube-watcher/terminal/internal/core/terminal"
	"kube-watcher/terminal/internal/domain"
	"kube-watcher/terminal/internal/ui/tui"
	"kube-watcher/terminal/internal/widgets"
)

func Run() error {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}

	adapter := ptyadapter.NewAdapter()
	handler := terminalcore.NewHandler(adapter)
	browserHandler := browsercore.NewHandler(browseradapter.NewAdapter())
	repoPath, _ := os.Getwd()
	widgetSet := []domain.Widget{
		widgets.NewSystemStatsWidget(),
		widgets.NewGitStatusWidget(repoPath),
	}
	var (
		blocksMu sync.Mutex
		blocks   []domain.Block
	)

	socketPath := filepath.Join(os.TempDir(), "kw-terminal-agent.sock")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var socketSrv *agentsocket.Server

	model := tui.NewModel(
		handler,
		shellPath,
		tui.WithBlocksChangedCallback(func(next []domain.Block) {
			blocksMu.Lock()
			blocks = next
			blocksMu.Unlock()
		}),
		tui.WithDiagnosticRequestCallback(func(diag domain.AgentDiagnosticRequestMsg) {
			if socketSrv == nil {
				return
			}
			socketSrv.RecordEvent(agentsocket.EventEnvelope{
				Type:      "diagnostic_request",
				Timestamp: time.Now().UTC(),
				Payload: map[string]interface{}{
					"reason":    diag.Reason,
					"exit_code": diag.ExitCode,
				},
			})
		}),
		tui.WithWidgets(widgetSet),
	)
	program := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	model.SetProgram(program)
	socketSrv = agentsocket.NewServer(
		socketPath,
		func() []domain.Block {
			blocksMu.Lock()
			defer blocksMu.Unlock()
			out := make([]domain.Block, len(blocks))
			copy(out, blocks)
			return out
		},
		func(msg any) {
			program.Send(msg)
		},
	)
	socketSrv.SetOpenURLHandler(func(rawURL string) (domain.Artifact, error) {
		return browserHandler.OpenURL(ctx, rawURL)
	})
	if err := socketSrv.Start(ctx); err != nil {
		return fmt.Errorf("start agent socket server: %w", err)
	}
	defer func() { _ = socketSrv.Close() }()
	defer func() { _ = browserHandler.Close(context.Background()) }()

	if _, err := program.Run(); err != nil {
		return fmt.Errorf("run terminal tui: %w", err)
	}

	return nil
}
