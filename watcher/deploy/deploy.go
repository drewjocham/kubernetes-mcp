package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	DefaultImage         = "kube-anomaly-detection:latest"
	DefaultAppName       = "kube-anomaly-detection"
	DefaultNamespace     = "kubewatcher"
	DefaultPrometheusURL = ""
	DefaultConfigPath    = "/app/config/config.yaml"
	DefaultDataDir       = "kad-data"
	DefaultDockerConfig  = ".kube-watcher/kad/config.yaml"
	DefaultPVCSize       = "5Gi"
	DefaultTailLines     = 200
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
	DockerConfigPath   string
	DockerDataDir      string
	PVCName            string
	PVCSize            string
	TailLines          int
	Follow             bool
}

func DefaultConfig() Config {
	return Config{
		Action:             "deploy",
		Target:             "kube",
		Image:              DefaultImage,
		Namespace:          DefaultNamespace,
		CreateNamespace:    true,
		PrometheusEndpoint: DefaultPrometheusURL,
		DockerConfigPath:   DefaultDockerConfig,
		DockerDataDir:      DefaultDataDir,
		PVCSize:            DefaultPVCSize,
		TailLines:          DefaultTailLines,
	}
}

func NormalizeAndValidate(cfg Config) (Config, error) {
	switch cfg.Action {
	case "deploy", "cleanup", "status", "logs":
	default:
		return cfg, fmt.Errorf("unsupported action %q", cfg.Action)
	}

	if cfg.Target != "kube" && cfg.Target != "docker" {
		return cfg, fmt.Errorf("unsupported target %q", cfg.Target)
	}

	if strings.TrimSpace(cfg.ClusterName) == "" {
		return cfg, errors.New("--cluster-name is required")
	}

	if cfg.Action == "deploy" && strings.TrimSpace(cfg.PrometheusEndpoint) == "" {
		return cfg, errors.New("--prometheus-endpoint is required for deploy")
	}

	sanitizedCluster := SanitizeForResourceName(cfg.ClusterName)
	if sanitizedCluster == "" {
		return cfg, errors.New("--cluster-name must include at least one alphanumeric character")
	}

	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = fmt.Sprintf("%s-%s", DefaultAppName, sanitizedCluster)
	}

	if cfg.DockerDataDir == DefaultDataDir {
		cfg.DockerDataDir = fmt.Sprintf("%s-%s", DefaultDataDir, sanitizedCluster)
	}

	if cfg.DockerConfigPath == DefaultDockerConfig {
		cfg.DockerConfigPath = filepath.Join(".kube-watcher", "kad", sanitizedCluster, "config.yaml")
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
	default:
		return fmt.Errorf("unsupported target %q", cfg.Target)
	}
}

func deployKube(ctx context.Context, cfg Config) error {
	if cfg.CreateNamespace {
		applyNamespace := fmt.Sprintf("kubectl create namespace %s --dry-run=client -o yaml | kubectl apply -f -", cfg.Namespace)
		if err := runShellCmd(ctx, applyNamespace); err != nil {
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

	if err := runCmd(
		ctx,
		"docker", "run", "-d",
		"--name", cfg.Name,
		"-v", fmt.Sprintf("%s:%s:ro", configPath, DefaultConfigPath),
		"-v", fmt.Sprintf("%s:/data", cfg.DockerDataDir),
		cfg.Image,
		"--config", DefaultConfigPath,
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

	if err := os.WriteFile(localPath, []byte(KADConfigYAML(cfg)), 0o600); err != nil {
		return "", fmt.Errorf("write docker config %s: %w", localPath, err)
	}
	return localPath, nil
}

func KubeManifest(cfg Config) string {
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
`, cfg.Name, cfg.Namespace, IndentYAML(KADConfigYAML(cfg), 4), cfg.PVCName, cfg.Namespace, cfg.Name, cfg.Name, cfg.ClusterName, cfg.PVCSize, cfg.Name, cfg.Namespace, cfg.Name, cfg.Name, cfg.Name, cfg.ClusterName, cfg.Image, cfg.ClusterName, cfg.Name, cfg.PVCName)
}

func KADConfigYAML(cfg Config) string {
	webhook := cfg.GoogleChatWebhook
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

func runShellCmd(ctx context.Context, command string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
