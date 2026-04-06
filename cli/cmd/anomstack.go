package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
			profile := rootViper.GetString("anomstack.profile")
			if profile == "" {
				profile = "local"
			}

			if anomstackPath == "" {
				// Try to find anomstack in common locations
				candidates := []string{
					"anomstack",
					"./anomstack",
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

			fmt.Printf("Starting Anomstack from %s with profile %s\n", anomstackPath, profile)

			absAnomstackPath, err := filepath.Abs(anomstackPath)
			if err != nil {
				return fmt.Errorf("get absolute path: %w", err)
			}

			originalDir, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get current directory: %w", err)
			}
			defer func() {
				if rerr := os.Chdir(originalDir); rerr != nil {
					log.Printf("restore working directory: %v", rerr)
				}
			}()

			if err := os.Chdir(absAnomstackPath); err != nil {
				return fmt.Errorf("change to anomstack directory: %w", err)
			}

			metricsDir := filepath.Join(absAnomstackPath, "metrics", "kube-watcher")
			if _, err := os.Stat(metricsDir); os.IsNotExist(err) {
				if err := createAnomstackMetricsConfig(absAnomstackPath); err != nil {
					fmt.Printf("Warning: Could not create metrics config: %v\n", err)
				}
			}

			switch profile {
			case "local":
				if err := exec.Command("docker-compose", "version").Run(); err != nil {
					if err := exec.Command("docker", "compose", "version").Run(); err != nil {
						return fmt.Errorf("docker compose not available: %v", err)
					}
				}

				fmt.Println("Starting Anomstack services with docker-compose...")
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

			case "minikube":
				// Minikube deployment
				fmt.Println("🚀 Starting Minikube deployment...")

				skipMinikubeStart := rootViper.GetBool("anomstack.skipMinikubeStart")
				if skipMinikubeStart {
					// Only check, don't start
					if err := checkMinikubeRunning(); err != nil {
						return fmt.Errorf("minikube is not running: %w", err)
					}
				} else {
					if err := ensureMinikubeRunning(); err != nil {
						return fmt.Errorf("minikube failed: %w", err)
					}
				}
				if err := setMinikubeDockerEnv(); err != nil {
					return fmt.Errorf("failed to set minikube docker environment: %w", err)
				}

				fmt.Println("Building dagster image...")
				buildDagster := exec.Command("docker", "build", "-t", "anomstack_dagster_image:kw-local", "-f", "docker/Dockerfile.dagster", ".")
				buildDagster.Stdout = os.Stdout
				buildDagster.Stderr = os.Stderr
				if err := buildDagster.Run(); err != nil {
					return fmt.Errorf("failed to build dagster image: %w", err)
				}

				fmt.Println("Building dashboard image...")
				buildDashboard := exec.Command("docker", "build", "-t", "anomstack_dashboard_image:kw-local", "-f", "docker/Dockerfile.anomstack_dashboard", ".")
				buildDashboard.Stdout = os.Stdout
				buildDashboard.Stderr = os.Stderr
				if err := buildDashboard.Run(); err != nil {
					return fmt.Errorf("failed to build dashboard image: %w", err)
				}

				fmt.Println("Preparing manifests with built images...")
				tmpDir, err := copyMinikubeDir()
				if err != nil {
					return fmt.Errorf("failed to copy minikube manifests: %w", err)
				}
				defer func() {
					if err := os.RemoveAll(tmpDir); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to remove temp directory %s: %v\n", tmpDir, err)
					}
				}()

				if err := updateManifestImages(tmpDir, "anomstack_dagster_image:kw-local", "anomstack_dashboard_image:kw-local"); err != nil {
					return fmt.Errorf("failed to update manifest images: %w", err)
				}

				// Apply kustomization from temp directory
				fmt.Println("Applying Kubernetes manifests...")
				kubectlCmd := exec.Command("kubectl", "apply", "-k", tmpDir)
				kubectlCmd.Stdout = os.Stdout
				kubectlCmd.Stderr = os.Stderr
				if err := kubectlCmd.Run(); err != nil {
					return fmt.Errorf("failed to apply kustomization: %w", err)
				}

				fmt.Println("\n✅ Anomstack deployed to Minikube!")
				fmt.Println("📊 Dashboard: kubectl -n kw-anomaly port-forward svc/anomstack-dashboard 5001:8080")
				fmt.Println("⚙️  Dagster UI: kubectl -n kw-anomaly port-forward svc/anomstack-webserver 3000:3000")
				fmt.Println("\nTo stop: kw anomstack stop (for local profile only) or kubectl delete -k deploy/anomaly/minikube")
				fmt.Println("To view logs: kubectl -n kw-anomaly logs -l app=anomstack-webserver -f")

			case "cluster":
				// Cluster deployment with registry
				fmt.Println("🚀 Starting Cluster deployment...")

				registry := rootViper.GetString("anomstack.registry")
				tag := rootViper.GetString("anomstack.tag")
				if registry == "" {
					return fmt.Errorf("registry is required for cluster deployment. Use --registry flag")
				}

				fmt.Printf("Using registry: %s, tag: %s\n", registry, tag)

				// First, ensure local images are built (same as minikube)
				fmt.Println("Building local images...")
				buildDagster := exec.Command("docker", "build", "-t", "anomstack_dagster_image:kw-local", "-f", "docker/Dockerfile.dagster", ".")
				buildDagster.Stdout = os.Stdout
				buildDagster.Stderr = os.Stderr
				if err := buildDagster.Run(); err != nil {
					return fmt.Errorf("failed to build dagster image: %w", err)
				}

				buildDashboard := exec.Command("docker", "build", "-t", "anomstack_dashboard_image:kw-local", "-f", "docker/Dockerfile.anomstack_dashboard", ".")
				buildDashboard.Stdout = os.Stdout
				buildDashboard.Stderr = os.Stderr
				if err := buildDashboard.Run(); err != nil {
					return fmt.Errorf("failed to build dashboard image: %w", err)
				}

				// Tag images with registry/tag
				dagsterRemote := fmt.Sprintf("%s/anomstack_dagster_image:%s", registry, tag)
				dashboardRemote := fmt.Sprintf("%s/anomstack_dashboard_image:%s", registry, tag)

				fmt.Printf("Tagging dagster image as %s\n", dagsterRemote)
				tagDagster := exec.Command("docker", "tag", "anomstack_dagster_image:kw-local", dagsterRemote)
				tagDagster.Stdout = os.Stdout
				tagDagster.Stderr = os.Stderr
				if err := tagDagster.Run(); err != nil {
					return fmt.Errorf("failed to tag dagster image: %w", err)
				}

				fmt.Printf("Tagging dashboard image as %s\n", dashboardRemote)
				tagDashboard := exec.Command("docker", "tag", "anomstack_dashboard_image:kw-local", dashboardRemote)
				tagDashboard.Stdout = os.Stdout
				tagDashboard.Stderr = os.Stderr
				if err := tagDashboard.Run(); err != nil {
					return fmt.Errorf("failed to tag dashboard image: %w", err)
				}

				// Push images
				fmt.Printf("Pushing dagster image to %s\n", dagsterRemote)
				pushDagster := exec.Command("docker", "push", dagsterRemote)
				pushDagster.Stdout = os.Stdout
				pushDagster.Stderr = os.Stderr
				if err := pushDagster.Run(); err != nil {
					return fmt.Errorf("failed to push dagster image: %w", err)
				}

				fmt.Printf("Pushing dashboard image to %s\n", dashboardRemote)
				pushDashboard := exec.Command("docker", "push", dashboardRemote)
				pushDashboard.Stdout = os.Stdout
				pushDashboard.Stderr = os.Stderr
				if err := pushDashboard.Run(); err != nil {
					return fmt.Errorf("failed to push dashboard image: %w", err)
				}

				// Copy minikube manifests to temporary directory and update images
				fmt.Println("Preparing manifests with registry images...")
				tmpDir, err := copyMinikubeDir()
				if err != nil {
					return fmt.Errorf("failed to copy minikube manifests: %w", err)
				}
				defer func() {
					if err := os.RemoveAll(tmpDir); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to remove temp directory %s: %v\n", tmpDir, err)
					}
				}()

				// Update images to use remote images
				if err := updateKustomizationImages(tmpDir, registry, tag); err != nil {
					return fmt.Errorf("failed to update manifest images: %w", err)
				}

				// Apply kustomization from temp directory
				fmt.Println("Applying Kubernetes manifests...")
				kubectlCmd := exec.Command("kubectl", "apply", "-k", tmpDir)
				kubectlCmd.Stdout = os.Stdout
				kubectlCmd.Stderr = os.Stderr
				if err := kubectlCmd.Run(); err != nil {
					return fmt.Errorf("failed to apply kustomization: %w", err)
				}

				fmt.Println("\n✅ Anomstack deployed to Cluster!")
				fmt.Printf("📊 Dashboard: kubectl -n kw-anomaly port-forward svc/anomstack-dashboard 5001:8080\n")
				fmt.Printf("⚙️  Dagster UI: kubectl -n kw-anomaly port-forward svc/anomstack-webserver 3000:3000\n")
				fmt.Println("\nTo delete: kubectl delete -k deploy/anomaly/minikube")
				fmt.Println("To view logs: kubectl -n kw-anomaly logs -l app=anomstack-webserver -f")

			default:
				return fmt.Errorf("unknown profile: %s", profile)
			}

			return nil
		},
	}

	cmd.Flags().String("path", "", "Path to anomstack directory")
	if err := rootViper.BindPFlag("anomstack.path", cmd.Flags().Lookup("path")); err != nil {
		panic(fmt.Errorf("bind anomstack.path flag: %w", err))
	}
	cmd.Flags().String("profile", "local", "Deployment profile: local, minikube, cluster")
	if err := rootViper.BindPFlag("anomstack.profile", cmd.Flags().Lookup("profile")); err != nil {
		panic(fmt.Errorf("bind anomstack.profile flag: %w", err))
	}
	cmd.Flags().String("registry", "", "Container registry for cluster deployment (e.g., ghcr.io/username)")
	cmd.Flags().String("tag", "latest", "Image tag for cluster deployment")
	if err := rootViper.BindPFlag("anomstack.registry", cmd.Flags().Lookup("registry")); err != nil {
		panic(fmt.Errorf("bind anomstack.registry flag: %w", err))
	}
	if err := rootViper.BindPFlag("anomstack.tag", cmd.Flags().Lookup("tag")); err != nil {
		panic(fmt.Errorf("bind anomstack.tag flag: %w", err))
	}
	cmd.Flags().Bool("skip-minikube-start", false, "Skip automatic start of minikube (minikube profile only)")
	if err := rootViper.BindPFlag("anomstack.skipMinikubeStart", cmd.Flags().Lookup("skip-minikube-start")); err != nil {
		panic(fmt.Errorf("bind anomstack.skipMinikubeStart flag: %w", err))
	}

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
			defer func() {
				if rerr := os.Chdir(originalDir); rerr != nil {
					log.Printf("restore working directory: %v", rerr)
				}
			}()

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
			defer func() {
				if rerr := os.Chdir(originalDir); rerr != nil {
					log.Printf("restore working directory: %v", rerr)
				}
			}()

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
	// anomstack-metrics is in the parent directory of anomstackPath (kube-watcher root)
	parentDir := filepath.Dir(anomstackPath)
	sourceDir := filepath.Join(parentDir, "anomstack-metrics", "system_metrics")
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

// setMinikubeDockerEnv sets the current process environment variables
// to use minikube's Docker daemon by running `minikube docker-env` and
// parsing its output.
func setMinikubeDockerEnv() error {
	cmd := exec.Command("minikube", "docker-env")
	output, err := cmd.Output()
	if err != nil {
		// Capture stderr for better error message
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if stderr != "" {
				return fmt.Errorf("failed to get minikube docker-env: %s", stderr)
			}
			// Check exit code
			if exitErr.ExitCode() == 85 {
				return fmt.Errorf("minikube is not running. Please start minikube with 'minikube start'")
			}
		}
		return fmt.Errorf("failed to get minikube docker-env: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "export") {
			// Parse export VAR="value"
			parts := strings.SplitN(line[len("export "):], "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := parts[0]
			value := strings.Trim(parts[1], `"`)
			if err := os.Setenv(key, value); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to set environment variable %s: %v\n", key, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning minikube docker-env output: %w", err)
	}
	return nil
}

// ensureMinikubeRunning checks if minikube is running and starts it if not.
// It waits for minikube to be ready up to 5 minutes.
func ensureMinikubeRunning() error {
	// First check if minikube is already running
	if err := checkMinikubeRunning(); err == nil {
		return nil
	}

	fmt.Println("Minikube is not running. Starting minikube (this may take a few minutes)...")

	// Start minikube
	startCmd := exec.Command("minikube", "start")
	startCmd.Stdout = os.Stdout
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		return fmt.Errorf("failed to start minikube: %w", err)
	}

	// Wait for minikube to be ready
	fmt.Println("Waiting for minikube to be ready...")
	timeout := 5 * time.Minute
	pollInterval := 10 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if err := checkMinikubeRunning(); err == nil {
			fmt.Println("Minikube is ready!")
			return nil
		}
		fmt.Print(".")
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("minikube failed to become ready within %v", timeout)
}

// checkMinikubeRunning verifies minikube is running and ready.
func checkMinikubeRunning() error {
	// Check if minikube command exists
	if _, err := exec.LookPath("minikube"); err != nil {
		return fmt.Errorf("minikube is not installed. Please install minikube from https://minikube.sigs.k8s.io/docs/start/")
	}

	// Check if minikube is running by trying to get its IP
	if err := exec.Command("minikube", "ip").Run(); err != nil {
		// Try to get more detailed error from minikube status
		statusCmd := exec.Command("minikube", "status", "--output=json")
		if output, err := statusCmd.Output(); err == nil {
			// Parse JSON to get status
			var status struct {
				Host string `json:"Host"`
			}
			if json.Unmarshal(output, &status) == nil && status.Host != "Running" {
				return fmt.Errorf("minikube is not running (host status: %s). Please start minikube with 'minikube start'", status.Host)
			}
		}
		return fmt.Errorf("minikube is not running. Please start minikube with 'minikube start'")
	}
	return nil
}

// replaceImageInYAML replaces all occurrences of oldImage with newImage in a YAML file.
// It does a simple string replacement; assumes the image line format "image: oldImage".
func replaceImageInYAML(filePath, oldImage, newImage string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", filePath, err)
	}
	content := string(data)
	// Replace "image: oldImage" (with possible whitespace)
	lines := strings.Split(content, "\n")
	updated := false
	for i, line := range lines {
		if strings.Contains(line, "image:") && strings.Contains(line, oldImage) {
			lines[i] = strings.ReplaceAll(line, oldImage, newImage)
			updated = true
		}
	}
	if !updated {
		return fmt.Errorf("image %s not found in %s", oldImage, filePath)
	}
	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write file %s: %w", filePath, err)
	}
	return nil
}

// updateManifestImages updates the image references in the copied minikube manifests
// with the provided dagster and dashboard image names.
func updateManifestImages(dir, dagsterImage, dashboardImage string) error {
	// Update deploy-webserver.yaml (dagster image)
	webserverPath := filepath.Join(dir, "deploy-webserver.yaml")
	if err := replaceImageInYAML(webserverPath, "nginx:alpine", dagsterImage); err != nil {
		return fmt.Errorf("update webserver image: %w", err)
	}
	// Update deploy-daemon.yaml (dagster image)
	daemonPath := filepath.Join(dir, "deploy-daemon.yaml")
	if err := replaceImageInYAML(daemonPath, "nginx:alpine", dagsterImage); err != nil {
		return fmt.Errorf("update daemon image: %w", err)
	}
	// Update deploy-dashboard.yaml (dashboard image)
	dashboardPath := filepath.Join(dir, "deploy-dashboard.yaml")
	if err := replaceImageInYAML(dashboardPath, "nginx:alpine", dashboardImage); err != nil {
		return fmt.Errorf("update dashboard image: %w", err)
	}
	// Note: mock-feeder.yaml uses curlimages/curl, leave unchanged
	return nil
}

// updateKustomizationImages updates the image references in the copied minikube manifests
// to use the provided registry and tag.
func updateKustomizationImages(dir, registry, tag string) error {
	dagsterImage := fmt.Sprintf("%s/anomstack_dagster_image:%s", registry, tag)
	dashboardImage := fmt.Sprintf("%s/anomstack_dashboard_image:%s", registry, tag)
	return updateManifestImages(dir, dagsterImage, dashboardImage)
}

// copyMinikubeDir copies the deploy/anomaly/minikube directory to a temporary directory
// and returns the temporary directory path.
func copyMinikubeDir() (string, error) {
	tmpDir, err := os.MkdirTemp("", "anomstack-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	minikubeDir := "deploy/anomaly/minikube"
	err = filepath.Walk(minikubeDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(minikubeDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(tmpDir, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, info.Mode())
	})
	if err != nil {
		if rerr := os.RemoveAll(tmpDir); rerr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to remove temp directory %s: %v\n", tmpDir, rerr)
		}
		return "", fmt.Errorf("failed to copy kustomization files: %w", err)
	}
	return tmpDir, nil
}
