package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"kube-watcher/pkg/profile"
)

func newProfileCmd() *cobra.Command {
	profileCmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage runtime profiles for config, environment, and secret references",
	}

	profileCmd.AddCommand(newProfileListCmd())
	profileCmd.AddCommand(newProfileCreateCmd())
	profileCmd.AddCommand(newProfileShowCmd())
	profileCmd.AddCommand(newProfileUseCmd())
	profileCmd.AddCommand(newProfileSetConfigCmd())
	profileCmd.AddCommand(newProfileSetEnvCmd())
	profileCmd.AddCommand(newProfileSetSecretRefCmd())
	profileCmd.AddCommand(newProfileEnvCmd())
	return profileCmd
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := profileManager.Load()
			if err != nil {
				return err
			}
			if len(cfg.Profiles) == 0 {
				fmt.Println("no profiles configured")
				return nil
			}
			names := make([]string, 0, len(cfg.Profiles))
			for name := range cfg.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				activeMarker := " "
				if cfg.Active == name {
					activeMarker = "*"
				}
				desc := ""
				if cfg.Profiles[name] != nil {
					desc = cfg.Profiles[name].Description
				}
				fmt.Printf("%s %s\t%s\n", activeMarker, name, desc)
			}
			return nil
		},
	}
}

func newProfileCreateCmd() *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := profileManager.Create(args[0], description); err != nil {
				return err
			}
			fmt.Printf("created profile %q\n", args[0])
			return refreshActiveProfileOverrides()
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "human-readable profile description")
	return cmd
}

func newProfileShowCmd() *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "show [name]",
		Short: "Show a profile (defaults to active profile)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := profileManager.Load()
			if err != nil {
				return err
			}
			name := cfg.Active
			if len(args) > 0 {
				name = args[0]
			}
			prof, ok := cfg.Profiles[name]
			if !ok || prof == nil {
				return profile.ErrProfileNotFound
			}
			switch strings.ToLower(output) {
			case "yaml":
				out, err := yaml.Marshal(prof)
				if err != nil {
					return err
				}
				fmt.Print(string(out))
				return nil
			case "json":
				out, err := json.MarshalIndent(prof, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			default:
				return fmt.Errorf("unsupported output %q (expected yaml|json)", output)
			}
		},
	}
	cmd.Flags().StringVar(&output, "output", "yaml", "output format: yaml|json")
	return cmd
}

func newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use [name]",
		Short: "Set active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := profileManager.SetActive(args[0]); err != nil {
				return err
			}
			fmt.Printf("active profile set to %q\n", args[0])
			return refreshActiveProfileOverrides()
		},
	}
}

func newProfileSetConfigCmd() *cobra.Command {
	var key, value string
	cmd := &cobra.Command{
		Use:   "set-config [name]",
		Short: "Set profile config override (example key: ops.target)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			if err := profileManager.SetConfigOverride(args[0], key, value); err != nil {
				return err
			}
			fmt.Printf("updated profile %q config %q\n", args[0], key)
			return refreshActiveProfileOverrides()
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "dot-notation config key override")
	cmd.Flags().StringVar(&value, "value", "", "override value")
	return cmd
}

func newProfileSetEnvCmd() *cobra.Command {
	var key, value string
	cmd := &cobra.Command{
		Use:   "set-env [name]",
		Short: "Set profile environment variable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			if err := profileManager.SetEnvValue(args[0], key, value); err != nil {
				return err
			}
			fmt.Printf("updated profile %q env %q\n", args[0], key)
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "environment variable name")
	cmd.Flags().StringVar(&value, "value", "", "environment variable value")
	return cmd
}

func newProfileSetSecretRefCmd() *cobra.Command {
	var key, provider, ref string
	cmd := &cobra.Command{
		Use:   "set-secret-ref [name]",
		Short: "Set profile secret reference (provider: env|keychain)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if key == "" {
				return fmt.Errorf("--key is required")
			}
			if provider == "" {
				return fmt.Errorf("--provider is required")
			}
			if ref == "" {
				return fmt.Errorf("--ref is required")
			}
			if err := profileManager.SetSecretRef(args[0], key, provider, ref); err != nil {
				return err
			}
			fmt.Printf("updated profile %q secret ref %q\n", args[0], key)
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "target environment variable name")
	cmd.Flags().StringVar(&provider, "provider", "env", "secret provider: env|keychain")
	cmd.Flags().StringVar(&ref, "ref", "", "provider-specific secret reference")
	return cmd
}

func newProfileEnvCmd() *cobra.Command {
	var resolveSecrets bool
	var output string
	cmd := &cobra.Command{
		Use:   "env [name]",
		Short: "Print profile environment values (masked by default)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) > 0 {
				name = args[0]
			}
			envValues, err := profileManager.ResolveEnv(name)
			if err != nil {
				return err
			}
			if !resolveSecrets {
				envValues = profile.MaskedEnv(envValues)
			}

			switch strings.ToLower(output) {
			case "shell":
				keys := make([]string, 0, len(envValues))
				for k := range envValues {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Printf("export %s=%q\n", k, envValues[k])
				}
				return nil
			case "json":
				out, err := json.MarshalIndent(envValues, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			case "yaml":
				out, err := yaml.Marshal(envValues)
				if err != nil {
					return err
				}
				fmt.Print(string(out))
				return nil
			default:
				return fmt.Errorf("unsupported output %q (expected shell|json|yaml)", output)
			}
		},
	}
	cmd.Flags().BoolVar(&resolveSecrets, "resolve-secrets", false, "resolve and print secret values (handle with care)")
	cmd.Flags().StringVar(&output, "output", "yaml", "output format: shell|json|yaml")
	return cmd
}
