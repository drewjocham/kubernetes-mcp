package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"kube-watcher/terminal/app"
)

func newTerminalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "terminal",
		Short: "Launch interactive terminal TUI",
		Long: `Launch the interactive terminal TUI (Bubble Tea) for kube-watcher.
This provides a terminal interface with widgets, shell integration, and agent communication.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.Run(); err != nil {
				return fmt.Errorf("terminal TUI failed: %w", err)
			}
			return nil
		},
	}

	return cmd
}
