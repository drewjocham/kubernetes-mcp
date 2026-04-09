package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"kube-watcher/pkg/profile"
)

var (
	version   = "1.0.0"
	gitCommit = "dev"
	buildDate = "unknown"
	cfgFile   string
	rootViper = viper.New()

	profileManager         *profile.Manager
	activeProfileOverrides map[string]string
)

var rootCmd = &cobra.Command{
	Use:   "kw",
	Short: "Kube-Watcher CLI - Manage MCP Agents and K8s Monitoring",
	Long:  "A unified interface to manage MCP servers, watcher engines, deployment operations, and AI agent definitions.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.kube-watcher.yaml)")

	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newAgentCmd())
	rootCmd.AddCommand(newOpsCmd())
	rootCmd.AddCommand(newViewCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newAnomstackCmd())
	rootCmd.AddCommand(newProfileCmd())
}

func initConfig() {
	if cfgFile != "" {
		rootViper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			rootViper.AddConfigPath(home)
		}
		rootViper.SetConfigName(".kube-watcher")
		rootViper.SetConfigType("yaml")
	}

	rootViper.SetEnvPrefix("KW")
	rootViper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	rootViper.AutomaticEnv()
	_ = rootViper.ReadInConfig()

	home, err := os.UserHomeDir()
	if err == nil {
		profileManager = profile.NewManager(filepath.Join(home, ".kube-watcher", "profiles.yaml"))
		_ = refreshActiveProfileOverrides()
	}
}

func writeRootConfig() error {
	if rootViper.ConfigFileUsed() != "" {
		return rootViper.WriteConfig()
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}

	target := filepath.Join(home, ".kube-watcher.yaml")
	rootViper.SetConfigFile(target)

	if _, err := os.Stat(target); errors.Is(err, os.ErrNotExist) {
		return rootViper.SafeWriteConfigAs(target)
	}

	return rootViper.WriteConfigAs(target)
}

func bindCommandViper(cmd *cobra.Command, prefix string) *viper.Viper {
	v := viper.New()
	v.SetEnvPrefix(prefix)
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	v.AutomaticEnv()
	_ = v.BindPFlags(cmd.Flags())
	return v
}

func defaultDBPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube-watcher", "history.db")
}

func defaultRuntimeTimeout() time.Duration {
	return 2 * time.Minute
}

func configString(key, fallback string) string {
	if activeProfileOverrides != nil {
		if v := activeProfileOverrides[key]; v != "" {
			return v
		}
	}
	if rootViper.IsSet(key) {
		if v := rootViper.GetString(key); v != "" {
			return v
		}
	}
	return fallback
}

func configBool(key string, fallback bool) bool {
	if activeProfileOverrides != nil {
		if v, ok := activeProfileOverrides[key]; ok {
			parsed, err := strconv.ParseBool(strings.TrimSpace(v))
			if err == nil {
				return parsed
			}
		}
	}
	if rootViper.IsSet(key) {
		return rootViper.GetBool(key)
	}
	return fallback
}

func refreshActiveProfileOverrides() error {
	if profileManager == nil {
		activeProfileOverrides = map[string]string{}
		return nil
	}
	overrides, err := profileManager.ActiveConfigOverrides()
	if err != nil {
		return err
	}
	activeProfileOverrides = overrides
	return nil
}
