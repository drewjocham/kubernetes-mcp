package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultImage         = "kube-anomaly-detection:latest"
	defaultAppName       = "kube-anomaly-detection"
	defaultNamespace     = "kubewatcher"
	defaultPrometheusURL = ""
	defaultConfigPath    = "/app/config/config.yaml"
	defaultDataDir       = "kad-data"
	defaultDockerConfig  = ".kube-watcher/kad/config.yaml"
	defaultPVCSize       = "5Gi"
)

type config struct {
	action             string
	target             string
	image              string
	sourcePath         string
	name               string
	clusterName        string
	namespace          string
	createNamespace    bool
	deleteNamespace    bool
	prometheusEndpoint string
	googleChatWebhook  string
	dockerConfigPath   string
	dockerDataDir      string
	pvcName            string
	pvcSize            string
}

func normalizeAndValidate(cfg config) (config, error) {
	if cfg.action != "deploy" && cfg.action != "cleanup" {
		return cfg, fmt.Errorf("unsupported action %q", cfg.action)
	}
	if cfg.target != "kube" && cfg.target != "docker" {
		return cfg, fmt.Errorf("unsupported target %q", cfg.target)
	}
	if strings.TrimSpace(cfg.clusterName) == "" {
		return cfg, errors.New("--cluster-name is required")
	}
	if cfg.action == "deploy" && strings.TrimSpace(cfg.prometheusEndpoint) == "" {
		return cfg, errors.New("--prometheus-endpoint is required for deploy")
	}
	sanitizedCluster := sanitizeForResourceName(cfg.clusterName)
	if sanitizedCluster == "" {
		return cfg, errors.New("--cluster-name must include at least one alphanumeric character")
	}
	if strings.TrimSpace(cfg.name) == "" {
		cfg.name = fmt.Sprintf("%s-%s", defaultAppName, sanitizedCluster)
	}
	if cfg.dockerDataDir == defaultDataDir {
		cfg.dockerDataDir = fmt.Sprintf("%s-%s", defaultDataDir, sanitizedCluster)
	}
	if cfg.dockerConfigPath == defaultDockerConfig {
		cfg.dockerConfigPath = filepath.Join(".kube-watcher", "kad", sanitizedCluster, "config.yaml")
	}
	if strings.TrimSpace(cfg.pvcName) == "" {
		cfg.pvcName = fmt.Sprintf("%s-data", cfg.name)
	}
	if strings.TrimSpace(cfg.pvcSize) == "" {
		cfg.pvcSize = defaultPVCSize
	}
	cfg.clusterName = sanitizedCluster
	return cfg, nil
}

func main() {
	cfg := parseFlags()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var err error
	switch cfg.action {
	case "deploy":
		err = deploy(ctx, cfg)
	case "cleanup":
		err = cleanup(ctx, cfg)
	default:
		err = fmt.Errorf("unsupported action %q", cfg.action)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	cfg := config{}
	flag.StringVar(&cfg.action, "action", "deploy", "action to execute: deploy|cleanup")
	flag.StringVar(&cfg.target, "target", "kube", "runtime target: kube|docker")
	flag.StringVar(&cfg.image, "image", defaultImage, "container image for kube-anomaly-detection")
	flag.StringVar(&cfg.sourcePath, "source-path", "", "optional local path to build image before deploy")
	flag.StringVar(&cfg.name, "name", "", "application name / deployment name (defaults to kube-anomaly-detection-<cluster-name>)")
	flag.StringVar(&cfg.clusterName, "cluster-name", "", "cluster identifier (required)")
	flag.StringVar(&cfg.namespace, "namespace", defaultNamespace, "kubernetes namespace")
	flag.BoolVar(&cfg.createNamespace, "create-namespace", true, "create namespace when deploying to kubernetes")
	flag.BoolVar(&cfg.deleteNamespace, "delete-namespace", false, "delete namespace on cleanup (kubernetes only)")
	flag.StringVar(&cfg.prometheusEndpoint, "prometheus-endpoint", defaultPrometheusURL, "prometheus scrape endpoint (full /metrics URL, required for deploy)")
	flag.StringVar(&cfg.googleChatWebhook, "google-chat-webhook-url", "", "optional Google Chat webhook URL")
	flag.StringVar(&cfg.dockerConfigPath, "docker-config-path", defaultDockerConfig, "local config path for docker target")
	flag.StringVar(&cfg.dockerDataDir, "docker-data-dir", defaultDataDir, "docker volume name for DuckDB data")
	flag.StringVar(&cfg.pvcName, "pvc-name", "", "kubernetes persistent volume claim name (defaults to <deployment-name>-data)")
	flag.StringVar(&cfg.pvcSize, "pvc-size", defaultPVCSize, "kubernetes persistent volume claim size")
	flag.Parse()

	normalized, err := normalizeAndValidate(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		flag.Usage()
		os.Exit(2)
	}
	return normalized
}

func deploy(ctx context.Context, cfg config) error {
	if cfg.sourcePath != "" {
		if err := runCmd(ctx, "docker", "build", "-t", cfg.image, cfg.sourcePath); err != nil {
			return fmt.Errorf("build image from source path %q: %w", cfg.sourcePath, err)
		}
	}
	switch cfg.target {
	case "kube":
		return deployKube(ctx, cfg)
	case "docker":
		return deployDocker(ctx, cfg)
	default:
		return fmt.Errorf("unsupported target %q", cfg.target)
	}
}

func cleanup(ctx context.Context, cfg config) error {
	switch cfg.target {
	case "kube":
		return cleanupKube(ctx, cfg)
	case "docker":
		return cleanupDocker(ctx, cfg)
	default:
		return fmt.Errorf("unsupported target %q", cfg.target)
	}
}

func deployKube(ctx context.Context, cfg config) error {
	if cfg.createNamespace {
		applyNamespace := fmt.Sprintf("kubectl create namespace %s --dry-run=client -o yaml | kubectl apply -f -", cfg.namespace)
		if err := runShellCmd(ctx, applyNamespace); err != nil {
			return fmt.Errorf("apply namespace %q: %w", cfg.namespace, err)
		}
	}

	if err := applyKubeManifest(ctx, cfg); err != nil {
		return err
	}

	fmt.Printf("deployed %s to namespace %s\n", cfg.name, cfg.namespace)
	fmt.Printf("prometheus endpoint: %s\n", cfg.prometheusEndpoint)
	fmt.Printf("check status: kubectl -n %s get deploy,pods | grep %s\n", cfg.namespace, cfg.name)
	return nil
}

func applyKubeManifest(ctx context.Context, cfg config) error {
	manifest := kubeManifest(cfg)
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl apply: %w", err)
	}
	return nil
}

func cleanupKube(ctx context.Context, cfg config) error {
	manifest := kubeManifest(cfg)
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "--ignore-not-found=true", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl delete resources: %w", err)
	}

	if cfg.deleteNamespace {
		if err := runCmd(ctx, "kubectl", "delete", "namespace", cfg.namespace, "--ignore-not-found=true"); err != nil {
			return fmt.Errorf("delete namespace %q: %w", cfg.namespace, err)
		}
	}

	fmt.Printf("cleaned up %s from namespace %s\n", cfg.name, cfg.namespace)
	return nil
}

func deployDocker(ctx context.Context, cfg config) error {
	configPath, err := materializeDockerConfig(cfg)
	if err != nil {
		return err
	}

	_ = runCmd(ctx, "docker", "rm", "-f", cfg.name)
	_ = runCmd(ctx, "docker", "volume", "create", cfg.dockerDataDir)

	if err := runCmd(
		ctx,
		"docker", "run", "-d",
		"--name", cfg.name,
		"-v", fmt.Sprintf("%s:%s:ro", configPath, defaultConfigPath),
		"-v", fmt.Sprintf("%s:/data", cfg.dockerDataDir),
		cfg.image,
		"--config", defaultConfigPath,
	); err != nil {
		return fmt.Errorf("docker run: %w", err)
	}

	fmt.Printf("deployed %s locally in docker\n", cfg.name)
	fmt.Printf("logs: docker logs -f %s\n", cfg.name)
	return nil
}

func cleanupDocker(ctx context.Context, cfg config) error {
	_ = runCmd(ctx, "docker", "rm", "-f", cfg.name)
	_ = runCmd(ctx, "docker", "volume", "rm", cfg.dockerDataDir)
	fmt.Printf("cleaned up docker container %s and volume %s\n", cfg.name, cfg.dockerDataDir)
	return nil
}

func materializeDockerConfig(cfg config) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}

	localPath := cfg.dockerConfigPath
	if strings.HasPrefix(localPath, "~") {
		localPath = filepath.Join(home, strings.TrimPrefix(localPath, "~"))
	} else if !filepath.IsAbs(localPath) {
		localPath = filepath.Join(home, localPath)
	}

	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return "", fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(localPath, []byte(kadConfigYAML(cfg)), 0o600); err != nil {
		return "", fmt.Errorf("write docker config %s: %w", localPath, err)
	}
	return localPath, nil
}

func kubeManifest(cfg config) string {
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
    app.kubernetes.io/component: anomaly-detection
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
        app.kubernetes.io/component: anomaly-detection
        app.kubernetes.io/instance: %s
        kube-watcher.io/cluster: %s
    spec:
      containers:
        - name: app
          image: %s
          imagePullPolicy: IfNotPresent
          args: ["--config", "/app/config/config.yaml"]
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
`, cfg.name, cfg.namespace, indentYAML(kadConfigYAML(cfg), 4), cfg.pvcName, cfg.namespace, cfg.name, cfg.name, cfg.clusterName, cfg.pvcSize, cfg.name, cfg.namespace, cfg.name, cfg.name, cfg.name, cfg.clusterName, cfg.image, cfg.clusterName, cfg.name, cfg.pvcName)
}

func kadConfigYAML(cfg config) string {
	webhook := cfg.googleChatWebhook
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
`, cfg.prometheusEndpoint, webhook)
}

func indentYAML(content string, spaces int) string {
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

func runShellCmd(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func sanitizeForResourceName(input string) string {
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
