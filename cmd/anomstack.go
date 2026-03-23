package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func newAnomstackCmd() *cobra.Command {
	anomstackCmd := &cobra.Command{
		Use:   "anomstack",
		Short: "Launch Anomstack for anomaly detection",
		Long:  "Start the Anomstack anomaly detection system with dashboard and Dagster UI.",
	}

	anomstackCmd.AddCommand(newAnomstackStartCmd())
	anomstackCmd.AddCommand(newAnomstackStopCmd())
	anomstackCmd.AddCommand(newAnomstackStatusCmd())
	anomstackCmd.AddCommand(newAnomstackConfigCmd())

	return anomstackCmd
}

func newAnomstackStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start Anomstack services",
		RunE: func(cmd *cobra.Command, args []string) error {
			anomstackPath := rootViper.GetString("anomstack.path")
			if anomstackPath == "" {
				// Try to find anomstack in common locations
				candidates := []string{
					"../anomstack",
					"../../anomstack",
					filepath.Join(os.Getenv("HOME"), "programming", "anomstack"),
					"/Users/jocham/programming/anomstack",
				}
				for _, candidate := range candidates {
					if _, err := os.Stat(filepath.Join(candidate, "docker-compose.yaml")); err == nil {
						anomstackPath = candidate
						break
					}
				}
				if anomstackPath == "" {
					return fmt.Errorf("anomstack not found. Set anomstack.path in config or ensure it's in a standard location")
				}
			}

			fmt.Printf("Starting Anomstack from %s\n", anomstackPath)

			// Check if Docker Compose is available
			if err := exec.Command("docker-compose", "version").Run(); err != nil {
				// Try docker compose (v2)
				if err := exec.Command("docker", "compose", "version").Run(); err != nil {
					return fmt.Errorf("docker compose not available: %v", err)
				}
			}

			// Change to anomstack directory
			originalDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(anomstackPath); err != nil {
				return fmt.Errorf("change to anomstack directory: %w", err)
			}

			// Copy example metrics config if needed
			metricsDir := filepath.Join(anomstackPath, "metrics", "kube-watcher")
			if _, err := os.Stat(metricsDir); os.IsNotExist(err) {
				if err := createAnomstackMetricsConfig(anomstackPath); err != nil {
					fmt.Printf("Warning: Could not create metrics config: %v\n", err)
				}
			}

			// Start services
			fmt.Println("Starting Anomstack services...")
			dockerCmd := exec.Command("docker-compose", "up", "-d")
			if exec.Command("docker", "compose", "version").Run() == nil {
				dockerCmd = exec.Command("docker", "compose", "up", "-d")
			}

			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr
			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("failed to start docker compose: %w", err)
			}

			fmt.Println("\n✅ Anomstack started!")
			fmt.Println("📊 Dashboard: http://localhost:5001")
			fmt.Println("⚙️  Dagster UI: http://localhost:3000")
			fmt.Println("\nTo stop: kw anomstack stop")
			fmt.Println("To view logs: docker-compose logs -f")

			return nil
		},
	}

	cmd.Flags().String("path", "", "Path to anomstack directory")
	rootViper.BindPFlag("anomstack.path", cmd.Flags().Lookup("path"))

	return cmd
}

func newAnomstackStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop Anomstack services",
		RunE: func(cmd *cobra.Command, args []string) error {
			anomstackPath := rootViper.GetString("anomstack.path")
			if anomstackPath == "" {
				return fmt.Errorf("anomstack.path not set in config")
			}

			originalDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(anomstackPath); err != nil {
				return fmt.Errorf("change to anomstack directory: %w", err)
			}

			fmt.Println("Stopping Anomstack services...")
			dockerCmd := exec.Command("docker-compose", "down")
			if exec.Command("docker", "compose", "version").Run() == nil {
				dockerCmd = exec.Command("docker", "compose", "down")
			}

			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr
			if err := dockerCmd.Run(); err != nil {
				return fmt.Errorf("failed to stop docker compose: %w", err)
			}

			fmt.Println("✅ Anomstack stopped")
			return nil
		},
	}
}

func newAnomstackStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check Anomstack service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			anomstackPath := rootViper.GetString("anomstack.path")
			if anomstackPath == "" {
				return fmt.Errorf("anomstack.path not set in config")
			}

			originalDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			defer os.Chdir(originalDir)

			if err := os.Chdir(anomstackPath); err != nil {
				return fmt.Errorf("change to anomstack directory: %w", err)
			}

			dockerCmd := exec.Command("docker-compose", "ps")
			if exec.Command("docker", "compose", "version").Run() == nil {
				dockerCmd = exec.Command("docker", "compose", "ps")
			}

			dockerCmd.Stdout = os.Stdout
			dockerCmd.Stderr = os.Stderr
			return dockerCmd.Run()
		},
	}
}

func newAnomstackConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Generate Anomstack metric configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			anomstackPath := rootViper.GetString("anomstack.path")
			if anomstackPath == "" {
				return fmt.Errorf("anomstack.path not set in config")
			}

			if err := createAnomstackMetricsConfig(anomstackPath); err != nil {
				return fmt.Errorf("create metrics config: %w", err)
			}

			fmt.Println("✅ Anomstack metric configuration created")
			fmt.Printf("📁 Location: %s\n", filepath.Join(anomstackPath, "metrics", "kube-watcher"))
			return nil
		},
	}
}

func createAnomstackMetricsConfig(anomstackPath string) error {
	// Copy system_metrics configuration from our project to anomstack metrics directory
	sourceDir := filepath.Join("anomstack-metrics", "system_metrics")
	targetDir := filepath.Join(anomstackPath, "metrics", "system_metrics")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}
	// Copy YAML file
	sourceYAML := filepath.Join(sourceDir, "system_metrics.yaml")
	targetYAML := filepath.Join(targetDir, "system_metrics.yaml")
	data, err := os.ReadFile(sourceYAML)
	if err != nil {
		return fmt.Errorf("read source YAML: %w", err)
	}
	if err := os.WriteFile(targetYAML, data, 0644); err != nil {
		return fmt.Errorf("write target YAML: %w", err)
	}
	log.Printf("Copied metric configuration to %s", targetYAML)
	return nil
}
