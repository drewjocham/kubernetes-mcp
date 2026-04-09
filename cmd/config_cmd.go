package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration and examples",
	}

	cmd.AddCommand(newConfigExampleCmd())
	return cmd
}

func newConfigExampleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "example",
		Short: "Print an example configuration file",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(defaultConfigExample())
		},
	}
	return cmd
}
