package deploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	DefaultImage         = "drewjocham/kube-anomaly-detection:latest"
	DefaultWatcherImage  = "watcher:latest"
	DefaultNamespace     = "kube-anomaly-detection"
	DefaultPrometheusURL = "http://localhost:9090/metrics"
	DefaultDockerConfig  = ".kube-watcher/kad/config.yaml"
	DefaultDataDir       = "kube-anomaly-detection-data"
	DefaultPVCSize       = "1Gi"
	DefaultTailLines     = 100
	DefaultAppName       = "kube-anomaly-detection"
	DefaultConfigPath    = "/app/config.yaml"
	GoogleChatWebhookEnv = "GOOGLE_CHAT_WEBHOOK_URL"
	AlertWebhookEnv      = "ALERT_WEBHOOK_URL"
)

type Config struct {
	Action             string
	Target             string
	Image              string
	SourcePath         string
	Name               string
	ClusterName        string
	Namespace          string
	CreateNamespace    bool
	DeleteNamespace    bool
	PrometheusEndpoint string
	GoogleChatWebhook  string
	AlertWebhook       string
	DashboardWebhook   string
	KubeconfigPath     string
	DockerConfigPath   string
	DockerDataDir      string
	PVCName            string
	PVCSize            string
	TailLines          int
	Follow             bool
	AppType            string
	ComposeFile        string
	ComposeService     string
	ComposeProject     string
}

func DefaultConfig() Config {
	return Config{
		Action:             "deploy",
		Target:             "kube",
		Image:              DefaultImage,
		Namespace:          DefaultNamespace,
		CreateNamespace:    true,
		PrometheusEndpoint: DefaultPrometheusURL,
		KubeconfigPath:     "",
		DockerConfigPath:   DefaultDockerConfig,
		DockerDataDir:      DefaultDataDir,
		PVCSize:            DefaultPVCSize,
		TailLines:          DefaultTailLines,
		AppType:            "kad",
	}
}

func NormalizeAndValidate(cfg Config) (Config, error) {
	switch cfg.Action {
	case "deploy", "cleanup", "status", "logs", "print-manifest":
	default:
		return cfg, fmt.Errorf("unsupported action %q", cfg.Action)
	}

	if cfg.Target != "kube" && cfg.Target != "docker" && cfg.Target != "compose" {
		return cfg, fmt.Errorf("unsupported target %q", cfg.Target)
	}

	if strings.TrimSpace(cfg.ClusterName) == "" {
		return cfg, errors.New("--cluster-name is required")
	}

	if cfg.Action == "deploy" && cfg.AppType != "watcher" && strings.TrimSpace(cfg.PrometheusEndpoint) == "" {
		return cfg, errors.New("--prometheus-endpoint is required for deploy (kad app type)")
	}

	sanitizedCluster := SanitizeForResourceName(cfg.ClusterName)
	if sanitizedCluster == "" {
		return cfg, errors.New("--cluster-name must include at least one alphanumeric character")
	}

	if strings.TrimSpace(cfg.Namespace) == "" {
		cfg.Namespace = DefaultNamespace
	}

	if !IsValidDNSLabel(cfg.Namespace) {
		return cfg, fmt.Errorf("invalid namespace %q: must be a valid DNS label (lowercase alphanumeric "+
			"characters or '-', start and end with alphanumeric, max 63 characters)", cfg.Namespace)
	}

	if strings.TrimSpace(cfg.Name) == "" {
		if cfg.AppType == "watcher" {
			cfg.Name = fmt.Sprintf("watcher-%s", sanitizedCluster)
		} else {
			cfg.Name = fmt.Sprintf("%s-%s", DefaultAppName, sanitizedCluster)
		}
	} else {
		if !IsValidDNSLabel(cfg.Name) {
			return cfg, fmt.Errorf("invalid name %q: must be a valid Kubernetes resource name "+
				"(lowercase alphanumeric characters or '-', start and end with alphanumeric, max 63 characters)", cfg.Name)
		}
	}

	// Adjust default image based on app type
	if cfg.AppType == "watcher" && cfg.Image == DefaultImage {
		cfg.Image = DefaultWatcherImage
	}

	if cfg.DockerDataDir == DefaultDataDir {
		cfg.DockerDataDir = fmt.Sprintf("%s-%s", DefaultDataDir, sanitizedCluster)
	}

	if cfg.DockerConfigPath == DefaultDockerConfig {
		// Separate config path for watcher vs kad
		if cfg.AppType == "watcher" {
			cfg.DockerConfigPath = filepath.Join(".kube-watcher", "watcher", sanitizedCluster, "config.yaml")
		} else {
			cfg.DockerConfigPath = filepath.Join(".kube-watcher", "kad", sanitizedCluster, "config.yaml")
		}
	}

	if strings.TrimSpace(cfg.PVCName) == "" {
		cfg.PVCName = fmt.Sprintf("%s-data", cfg.Name)
	}

	if strings.TrimSpace(cfg.PVCSize) == "" {
		cfg.PVCSize = DefaultPVCSize
	}

	if cfg.TailLines <= 0 {
		cfg.TailLines = DefaultTailLines
	}

	if cfg.GoogleChatWebhook == "" {
		if envWebhook := os.Getenv(GoogleChatWebhookEnv); envWebhook != "" {
			cfg.GoogleChatWebhook = envWebhook
		}
	}
	if cfg.AlertWebhook == "" {
		if envWebhook := os.Getenv(AlertWebhookEnv); envWebhook != "" {
			cfg.AlertWebhook = envWebhook
		}
	}
	if cfg.AlertWebhook == "" && cfg.GoogleChatWebhook != "" {
		cfg.AlertWebhook = cfg.GoogleChatWebhook
	}
	if cfg.Target == "compose" && cfg.Action != "print-manifest" && strings.TrimSpace(cfg.ComposeFile) == "" {
		return cfg, errors.New("--compose-file is required for compose target")
	}

	cfg.ClusterName = sanitizedCluster
	return cfg, nil
}

func Run(ctx context.Context, cfg Config) error {
	switch cfg.Action {
	case "deploy":
		return deploy(ctx, cfg)
	case "cleanup":
		return cleanup(ctx, cfg)
	case "status":
		return status(ctx, cfg)
	case "logs":
		return logs(ctx, cfg)
	case "print-manifest":
		return printManifest(ctx, cfg)
	default:
		return fmt.Errorf("unsupported action %q", cfg.Action)
	}
}

func deploy(ctx context.Context, cfg Config) error {
	if cfg.SourcePath != "" {
		if err := runCmd(ctx, "docker", "build", "-t", cfg.Image, cfg.SourcePath); err != nil {
			return fmt.Errorf("build image from source path %q: %w", cfg.SourcePath, err)
		}
	}

	switch cfg.Target {
	case "kube":
		return deployKube(ctx, cfg)
	case "docker":
		return deployDocker(ctx, cfg)
	case "compose":
		return deployCompose(ctx, cfg)
	default:
		return fmt.Errorf("unsupported target %q", cfg.Target)
	}
}

func cleanup(ctx context.Context, cfg Config) error {
	switch cfg.Target {
	case "kube":
		return cleanupKube(ctx, cfg)
	case "docker":
		return cleanupDocker(ctx, cfg)
	case "compose":
		return cleanupCompose(ctx, cfg)
	default:
		return fmt.Errorf("unsupported target %q", cfg.Target)
	}
}

func status(ctx context.Context, cfg Config) error {
	switch cfg.Target {
	case "kube":
		return statusKube(ctx, cfg)
	case "docker":
		return statusDocker(ctx, cfg)
	case "compose":
		return statusCompose(ctx, cfg)
	default:
		return fmt.Errorf("unsupported target %q", cfg.Target)
	}
}

func logs(ctx context.Context, cfg Config) error {
	switch cfg.Target {
	case "kube":
		return logsKube(ctx, cfg)
	case "docker":
		return logsDocker(ctx, cfg)
	case "compose":
		return logsCompose(ctx, cfg)
	default:
		return fmt.Errorf("unsupported target %q", cfg.Target)
	}
}

func deployKube(ctx context.Context, cfg Config) error {
	if cfg.CreateNamespace {
		if err := createNamespaceIfNotExists(ctx, cfg.Namespace); err != nil {
			return fmt.Errorf("apply namespace %q: %w", cfg.Namespace, err)
		}
	}

	if err := applyKubeManifest(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("deployed %s to namespace %s\n", cfg.Name, cfg.Namespace)
	fmt.Printf("prometheus endpoint: %s\n", cfg.PrometheusEndpoint)
	fmt.Printf("check status: kubectl -n %s get deploy,pods | grep %s\n", cfg.Namespace, cfg.Name)
	return nil
}

func cleanupKube(ctx context.Context, cfg Config) error {
	manifest := KubeManifest(cfg)
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "--ignore-not-found=true", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl delete resources: %w", err)
	}

	if cfg.DeleteNamespace {
		if err := runCmd(ctx, "kubectl", "delete", "namespace", cfg.Namespace, "--ignore-not-found=true"); err != nil {
			return fmt.Errorf("delete namespace %q: %w", cfg.Namespace, err)
		}
	}

	fmt.Printf("cleaned up %s from namespace %s\n", cfg.Name, cfg.Namespace)
	return nil
}

func statusKube(ctx context.Context, cfg Config) error {
	if err := runCmd(ctx, "kubectl", "-n", cfg.Namespace, "get", "deployment", cfg.Name, "--ignore-not-found=true"); err != nil {
		return fmt.Errorf("kube deployment status failed: %w", err)
	}
	if err := runCmd(ctx, "kubectl", "-n", cfg.Namespace, "get", "pods", "-l", "app.kubernetes.io/name="+cfg.Name); err != nil {
		return fmt.Errorf("kube pod status failed: %w", err)
	}
	return nil
}

func logsKube(ctx context.Context, cfg Config) error {
	args := []string{"-n", cfg.Namespace, "logs", "deployment/" + cfg.Name, "--tail", fmt.Sprintf("%d", cfg.TailLines)}
	if cfg.Follow {
		args = append(args, "-f")
	}
	if err := runCmd(ctx, "kubectl", args...); err != nil {
		return fmt.Errorf("kube logs failed: %w", err)
	}
	return nil
}

func deployDocker(ctx context.Context, cfg Config) error {
	configPath, err := materializeDockerConfig(cfg)
	if err != nil {
		return err
	}

	_ = runCmd(ctx, "docker", "rm", "-f", cfg.Name)
	_ = runCmd(ctx, "docker", "volume", "create", cfg.DockerDataDir)

	image := cfg.Image
	if cfg.AppType == "watcher" && image == DefaultImage {
		image = DefaultWatcherImage
	}

	args := []string{"--config", DefaultConfigPath}
	if cfg.AppType == "watcher" {
		args = []string{"--listen", ":8085", "--config", DefaultConfigPath}
	}

	cmdArgs := []string{"run", "-d",
		"--name", cfg.Name,
		"-v", fmt.Sprintf("%s:%s:ro", configPath, DefaultConfigPath),
		"-v", fmt.Sprintf("%s:/data", cfg.DockerDataDir),
	}
	// Mount kubeconfig if provided
	if cfg.KubeconfigPath != "" {
		cmdArgs = append(cmdArgs, "-v", fmt.Sprintf("%s:/home/nonroot/.kube/config:ro", cfg.KubeconfigPath))
		cmdArgs = append(cmdArgs, "-e", "KUBECONFIG=/home/nonroot/.kube/config")
	}
	// Run as root for watcher to avoid permission issues
	if cfg.AppType == "watcher" {
		cmdArgs = append(cmdArgs, "--user", "0:0")
	}
	cmdArgs = append(cmdArgs, image)
	cmdArgs = append(cmdArgs, args...)

	if err := runCmd(
		ctx,
		"docker", cmdArgs...,
	); err != nil {
		return fmt.Errorf("docker run: %w", err)
	}

	fmt.Printf("deployed %s locally in docker\n", cfg.Name)
	fmt.Printf("logs: docker logs -f %s\n", cfg.Name)
	return nil
}

func cleanupDocker(ctx context.Context, cfg Config) error {
	_ = runCmd(ctx, "docker", "rm", "-f", cfg.Name)
	_ = runCmd(ctx, "docker", "volume", "rm", cfg.DockerDataDir)
	fmt.Printf("cleaned up docker container %s and volume %s\n", cfg.Name, cfg.DockerDataDir)
	return nil
}

func statusDocker(ctx context.Context, cfg Config) error {
	if err := runCmd(
		ctx,
		"docker", "ps", "-a",
		"--filter", "name=^/"+cfg.Name+"$",
		"--format", "table {{.Names}}\t{{.Status}}\t{{.Image}}\t{{.RunningFor}}",
	); err != nil {
		return fmt.Errorf("docker status failed: %w", err)
	}
	return nil
}

func logsDocker(ctx context.Context, cfg Config) error {
	args := []string{"logs", "--tail", fmt.Sprintf("%d", cfg.TailLines)}
	if cfg.Follow {
		args = append(args, "-f")
	}
	args = append(args, cfg.Name)
	if err := runCmd(ctx, "docker", args...); err != nil {
		return fmt.Errorf("docker logs failed: %w", err)
	}
	return nil
}

func deployCompose(ctx context.Context, cfg Config) error {
	args := composeBaseArgs(cfg)
	args = append(args, "up", "-d")
	if cfg.ComposeService != "" {
		args = append(args, cfg.ComposeService)
	}
	if err := runComposeCmd(ctx, cfg, args...); err != nil {
		return fmt.Errorf("compose deploy failed: %w", err)
	}
	fmt.Printf("deployed compose stack from %s\n", cfg.ComposeFile)
	return nil
}

func cleanupCompose(ctx context.Context, cfg Config) error {
	args := composeBaseArgs(cfg)
	args = append(args, "down")
	if err := runComposeCmd(ctx, cfg, args...); err != nil {
		return fmt.Errorf("compose cleanup failed: %w", err)
	}
	fmt.Printf("cleaned up compose stack from %s\n", cfg.ComposeFile)
	return nil
}

func statusCompose(ctx context.Context, cfg Config) error {
	args := composeBaseArgs(cfg)
	args = append(args, "ps")
	if err := runComposeCmd(ctx, cfg, args...); err != nil {
		return fmt.Errorf("compose status failed: %w", err)
	}
	return nil
}

func logsCompose(ctx context.Context, cfg Config) error {
	args := composeBaseArgs(cfg)
	args = append(args, "logs", "--tail", fmt.Sprintf("%d", cfg.TailLines))
	if cfg.Follow {
		args = append(args, "-f")
	}
	if cfg.ComposeService != "" {
		args = append(args, cfg.ComposeService)
	}
	if err := runComposeCmd(ctx, cfg, args...); err != nil {
		return fmt.Errorf("compose logs failed: %w", err)
	}
	return nil
}

func printManifest(ctx context.Context, cfg Config) error {
	switch cfg.Target {
	case "kube":
		fmt.Println(KubeManifest(cfg))
	case "docker":
		configPath, err := materializeDockerConfig(cfg)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("read docker config: %w", err)
		}
		fmt.Println(string(data))
	case "compose":
		fmt.Printf("compose file: %s\n", cfg.ComposeFile)
		if cfg.ComposeProject != "" {
			fmt.Printf("compose project: %s\n", cfg.ComposeProject)
		}
		if cfg.ComposeService != "" {
			fmt.Printf("compose service: %s\n", cfg.ComposeService)
		}
	default:
		return fmt.Errorf("unsupported target %q for print-manifest", cfg.Target)
	}
	return nil
}

func applyKubeManifest(ctx context.Context, cfg Config) error {
	manifest := KubeManifest(cfg)
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply: %w", err)
	}
	return nil
}

func materializeDockerConfig(cfg Config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	localPath := cfg.DockerConfigPath
	if strings.HasPrefix(localPath, "~") {
		localPath = filepath.Join(home, strings.TrimPrefix(localPath, "~"))
	} else if !filepath.IsAbs(localPath) {
		localPath = filepath.Join(home, localPath)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	var configYAML string
	if cfg.AppType == "watcher" {
		configYAML = WatcherConfigYAML(cfg)
	} else {
		configYAML = KADConfigYAML(cfg)
	}

	if err := os.WriteFile(localPath, []byte(configYAML), 0o600); err != nil {
		return "", fmt.Errorf("write docker config %s: %w", localPath, err)
	}
	return localPath, nil
}

func KubeManifest(cfg Config) string {
	var configYAML string
	var image string
	var component string
	var args []string

	if cfg.AppType == "watcher" {
		configYAML = WatcherConfigYAML(cfg)
		image = DefaultWatcherImage
		if cfg.Image != DefaultImage {
			image = cfg.Image
		}
		component = "watcher"
		args = []string{"--listen", ":8085", "--config", "/app/config/config.yaml"}
	} else {
		configYAML = KADConfigYAML(cfg)
		image = cfg.Image
		component = "anomaly-detection"
		args = []string{"--config", "/app/config/config.yaml"}
	}

	argsStr := ""
	for i, arg := range args {
		if i > 0 {
			argsStr += ", "
		}
		argsStr += fmt.Sprintf("%q", arg)
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s-config
  namespace: %s
type: Opaque
stringData:
  config.yaml: |
%s
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: %s
    app.kubernetes.io/part-of: kube-watcher
    app.kubernetes.io/component: %s
    app.kubernetes.io/instance: %s
    kube-watcher.io/cluster: %s
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: %s
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: %s
  template:
    metadata:
      labels:
        app.kubernetes.io/name: %s
        app.kubernetes.io/part-of: kube-watcher
        app.kubernetes.io/component: %s
        app.kubernetes.io/instance: %s
        kube-watcher.io/cluster: %s
    spec:
      containers:
        - name: app
          image: %s
          imagePullPolicy: IfNotPresent
          args: [%s]
          env:
            - name: CLUSTER_NAME
              value: %q
          volumeMounts:
            - name: app-config
              mountPath: /app/config
            - name: app-data
              mountPath: /data
      volumes:
        - name: app-config
          secret:
            secretName: %s-config
        - name: app-data
          persistentVolumeClaim:
            claimName: %s
`, cfg.Name, cfg.Namespace, IndentYAML(configYAML, 4), cfg.PVCName, cfg.Namespace, cfg.Name, component, cfg.Name,
		cfg.ClusterName, cfg.PVCSize, cfg.Name, cfg.Namespace, cfg.Name, cfg.Name, component, cfg.Name,
		cfg.ClusterName, image, argsStr, cfg.ClusterName, cfg.Name, cfg.PVCName)
}

func KADConfigYAML(cfg Config) string {
	webhook := cfg.AlertWebhook
	if webhook == "" {
		webhook = cfg.GoogleChatWebhook
	}
	if webhook == "" {
		webhook = `""`
	} else {
		webhook = fmt.Sprintf("%q", webhook)
	}

	return fmt.Sprintf(`scrape:
  endpoint: %q
  interval_seconds: 30
  timeout_seconds: 10
duckdb:
  path: "/data/metrics.duckdb"
  memory_limit_gb: 1
  threads: 2
  lookback_hours: 8
detection:
  anomaly_threshold: 3.0
  warmup_samples: 20
alerts:
  cooldown_seconds: 3600
  max_per_window: 10
  window_seconds: 3600
silences: []
google_chat:
  webhook_url: %s
  timeout_seconds: 10
`, cfg.PrometheusEndpoint, webhook)
}

func WatcherConfigYAML(cfg Config) string {
	dashboardWebhook := cfg.DashboardWebhook
	if dashboardWebhook == "" {
		dashboardWebhook = "http://host.docker.internal:3000/api/alerts/ingest"
	}
	baseDashboardURL := strings.TrimSuffix(dashboardWebhook, "/api/alerts/ingest")
	if baseDashboardURL == "" || baseDashboardURL == dashboardWebhook {
		baseDashboardURL = "http://dashboard:3000"
	}
	clusterName := cfg.ClusterName
	if clusterName == "" {
		clusterName = "local"
	}
	// Basic watcher config based on watcher/internal/config/config.yaml
	return fmt.Sprintf(`resource_tracking:
  enabled: true
  retention: 1h
  path: "/data/event-engine-badger"
  fields:
    - cpu_request
    - cpu_limit
    - memory_request
    - memory_limit
    - ram
    - min_pod_count
    - max_pod_count
    - current_replicas
    - desired_replicas
    - node_memory_pressure
    - node_disk_pressure
    - waiting_reason
rules:
  - name: Pod-Restarting-Frequently
    kind: Pod
    condition: "evt.restart_delta > 3 && evt.waiting_reason == 'CrashLoopBackOff'"
    actions:
      - dashboard-webhook
  - name: Image-Pull-Failure
    kind: Pod
    duration: 2m
    condition: "evt.waiting_reasons.exists(r, r.contains('ImagePull'))"
    actions:
      - dashboard-webhook
  - name: HPA-Stalled-At-Ceiling
    kind: HorizontalPodAutoscaler
    duration: 5m
    condition: "evt.hpa_is_stalled == true"
    actions:
      - dashboard-webhook
  - name: Node-Memory-Pressure
    kind: Node
    condition: "evt.node_memory_pressure == true"
    actions:
      - dashboard-webhook
  - name: Dangerous-Resource-Gap
    kind: Pod
    condition: "evt.node_memory_pressure == true && evt.mem_limit_gap > 2048"
    actions:
      - dashboard-webhook
  - name: Pod-Stuck-In-Pending
    kind: Pod
    duration: 10m
    condition: "evt.is_ready == false && evt.waiting_reason == 'ContainerCreating'"
    actions:
      - dashboard-webhook
actions:
  dashboard-webhook:
    type: webhook
    template: |
      {
        "kind": "Unknown",
        "cluster": %q,
        "namespace": "{{ .Event.Namespace }}",
        "pod": "{{ .Event.Name }}",
        "ruleName": "{{ .RuleName }}",
        "severity": "critical",
        "source": "kube-watcher"
      }
    config:
      url: %q
settings:
  metrics:
    enabled: true
    listen: ":9090"
  heartbeat:
    enabled: true
    dashboard_url: %q
    cluster_name: %q
    interval: 30s
`, clusterName, dashboardWebhook, baseDashboardURL, clusterName)
}

func IndentYAML(content string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func runCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("%s %s exited with code %d", name, strings.Join(args, " "), exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func runComposeCmd(ctx context.Context, cfg Config, args ...string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), composeEnv(cfg)...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("docker %s exited with code %d", strings.Join(args, " "), exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func composeBaseArgs(cfg Config) []string {
	args := []string{"compose", "-f", cfg.ComposeFile}
	if cfg.ComposeProject != "" {
		args = append(args, "-p", cfg.ComposeProject)
	}
	return args
}

func composeEnv(cfg Config) []string {
	env := make([]string, 0)
	if cfg.AlertWebhook != "" {
		env = append(env, AlertWebhookEnv+"="+cfg.AlertWebhook)
		env = append(env, GoogleChatWebhookEnv+"="+cfg.AlertWebhook)
	}
	if cfg.DashboardWebhook != "" {
		env = append(env, "DASHBOARD_WEBHOOK_URL="+cfg.DashboardWebhook)
	}
	if cfg.ClusterName != "" {
		env = append(env, "CLUSTER_NAME="+cfg.ClusterName)
	}
	return env
}

func createNamespaceIfNotExists(ctx context.Context, namespace string) error {
	// Run kubectl create namespace with dry-run to generate manifest
	createCmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", namespace, "--dry-run=client", "-o", "yaml")

	// Capture output of create command
	createOutput, err := createCmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("kubectl create namespace failed: %s", exitErr.Stderr)
		}
		return fmt.Errorf("kubectl create namespace failed: %w", err)
	}

	// Pipe the output to kubectl apply -f -
	applyCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	applyCmd.Stdin = bytes.NewReader(createOutput)
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr

	if err := applyCmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("kubectl apply failed: exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("kubectl apply failed: %w", err)
	}

	return nil
}

func SanitizeForResourceName(input string) string {
	trimmed := strings.TrimSpace(strings.ToLower(input))
	var b strings.Builder
	prevDash := false
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			prevDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 63 {
		out = strings.Trim(out[:63], "-")
	}
	return out
}

// IsValidDNSLabel validates a string as a DNS label according to RFC 1123
// Used for Kubernetes resource names and namespace names
func IsValidDNSLabel(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			// valid character
			continue
		}
		return false
	}
	// Must start and end with alphanumeric
	if (name[0] >= 'a' && name[0] <= 'z') || (name[0] >= '0' && name[0] <= '9') {
	} else {
		return false
	}
	last := name[len(name)-1]
	if (last >= 'a' && last <= 'z') || (last >= '0' && last <= '9') {
		return true
	}
	return false
}
