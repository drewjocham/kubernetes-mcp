package cmd

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newAgentCmd() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Manage AI agent personas",
	}

	agentCmd.AddCommand(newAgentCreateCmd())
	agentCmd.AddCommand(newAgentListCmd())
	agentCmd.AddCommand(newAgentShowCmd())
	return agentCmd
}

func newAgentCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [name] [description]",
		Short: "Define a new agent persona",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			description := args[1]

			agentKey := fmt.Sprintf("agents.%s", name)
			rootViper.Set(agentKey+".description", description)
			rootViper.Set(agentKey+".role", "k8s-specialist")
			rootViper.Set(agentKey+".capabilities", []string{"analyze_cluster", "get_pod_resources", "history_insights"})
			rootViper.Set(agentKey+".created_at", time.Now().Format("2006-01-02"))

			if err := writeRootConfig(); err != nil {
				return fmt.Errorf("persist agent config: %w", err)
			}

			fmt.Printf("agent %q configured successfully\n", name)
			return nil
		},
	}
	return cmd
}

func newAgentListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured agent personas",
		Run: func(cmd *cobra.Command, args []string) {
			agents := rootViper.GetStringMap("agents")
			if len(agents) == 0 {
				fmt.Println("no agents configured")
				return
			}

			names := make([]string, 0, len(agents))
			for name := range agents {
				names = append(names, name)
			}
			sort.Strings(names)

			for _, name := range names {
				desc := rootViper.GetString(fmt.Sprintf("agents.%s.description", name))
				role := rootViper.GetString(fmt.Sprintf("agents.%s.role", name))
				fmt.Printf("- %s [%s]: %s\n", name, role, desc)
			}
		},
	}
	return cmd
}

func newAgentShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Show one agent configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			key := fmt.Sprintf("agents.%s", name)
			if !rootViper.IsSet(key) {
				return fmt.Errorf("agent %q not found", name)
			}

			fmt.Printf("name: %s\n", name)
			fmt.Printf("role: %s\n", rootViper.GetString(key+".role"))
			fmt.Printf("description: %s\n", rootViper.GetString(key+".description"))
			fmt.Printf("capabilities: %v\n", rootViper.GetStringSlice(key+".capabilities"))
			return nil
		},
	}
	return cmd
}
