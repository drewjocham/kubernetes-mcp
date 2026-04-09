package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show build version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("kube-watcher v%s (commit: %s, built: %s)\n", version, gitCommit, buildDate)
		},
	}
}
