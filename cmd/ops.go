package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

const (
	defaultOpsImage = "drewjocham/kube-watcher:latest"
)

func newOpsCmd() *cobra.Command {
	opsCmd := &cobra.Command{
		Use:   "ops",
		Short: "Deploy and manage watcher/MCP operational components",
	}

	opsCmd.AddCommand(newOpsDeployCmd())
	opsCmd.AddCommand(newOpsCleanupCmd())
	opsCmd.AddCommand(newOpsStatusCmd())
	opsCmd.AddCommand(newOpsLogsCmd())
	return opsCmd
}

func newOpsDeployCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "deploy [component]",
		Short:     "Deploy component as a container",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"mcp", "watcher"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getOpsConfig(cmd)
			return runDeploy(cmd.Context(), args[0], cfg)
		},
	}
	addOpsFlags(cmd)
	return cmd
}

func newOpsCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "cleanup [component]",
		Short:     "Stop and remove component container",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"mcp", "watcher"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getOpsConfig(cmd)
			cli, err := newDockerClient()
			if err != nil {
				return err
			}
			defer func() { _ = cli.Close() }()
			return removeContainer(cmd.Context(), cli, containerName(args[0], cfg))
		},
	}
	addOpsFlags(cmd)
	return cmd
}

func newOpsStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "status [component]",
		Short:     "Show component container status",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"mcp", "watcher"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getOpsConfig(cmd)
			cli, err := newDockerClient()
			if err != nil {
				return err
			}
			defer func() { _ = cli.Close() }()
			return printContainerStatus(cmd.Context(), cli, containerName(args[0], cfg))
		},
	}
	addOpsFlags(cmd)
	return cmd
}

func newOpsLogsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:       "logs [component]",
		Short:     "Stream component container logs",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"mcp", "watcher"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := getOpsConfig(cmd)
			cli, err := newDockerClient()
			if err != nil {
				return err
			}
			defer func() { _ = cli.Close() }()
			return streamContainerLogs(cmd.Context(), cli, containerName(args[0], cfg), cfg.followLogs, cfg.tailLines)
		},
	}
	addOpsFlags(cmd)
	return cmd
}

type opsConfig struct {
	target          string
	image           string
	namePrefix      string
	kubeconfigPath  string
	dbPath          string
	watcherConfig   string
	networkMode     string
	pollInterval    string
	buildIfMissing  bool
	followLogs      bool
	tailLines       string
	kubeNamespace   string
	kubeStorage     string
	kubeClass       string
	binaryMCPPath   string
	binaryWatchPath string
}

func addOpsFlags(cmd *cobra.Command) {
	home, _ := os.UserHomeDir()
	cmd.Flags().String("target", configString("ops.target", "docker"), "deploy target: docker|binary|kube")
	cmd.Flags().String("image", configString("ops.image", defaultOpsImage), "container image for components")
	cmd.Flags().String("name-prefix", configString("ops.name_prefix", "kw"), "container name prefix")
	cmd.Flags().String("kubeconfig", configString("ops.kubeconfig", filepath.Join(home, ".kube", "config")), "host kubeconfig path to mount read-only")
	cmd.Flags().String("db-path", configString("ops.db_path", filepath.Join(home, ".kube-watcher")), "host path for persistent history/badger data")
	cmd.Flags().String("watcher-config", configString("ops.watcher_config", filepath.Join(home, ".kube-watcher", "watcher.yaml")), "host watcher config path to mount into container")
	cmd.Flags().String("network-mode", configString("ops.network_mode", "host"), "docker network mode")
	cmd.Flags().String("interval", configString("ops.interval", "30s"), "MCP poll interval passed to MCP server")
	cmd.Flags().Bool("build-if-missing", configBool("ops.build_if_missing", true), "build image via `make docker` when image is missing locally")
	cmd.Flags().Bool("follow", true, "follow logs")
	cmd.Flags().String("tail", "200", "number of lines to show in logs")
	cmd.Flags().String("kube-namespace", configString("ops.kube_namespace", "kubewatcher"), "kubernetes namespace for component deployment")
	cmd.Flags().String("kube-history-storage", configString("ops.kube_history_storage", "1Gi"), "PVC size for history storage")
	cmd.Flags().String("kube-storage-class", configString("ops.kube_storage_class", ""), "PVC storage class for history storage")
	cmd.Flags().String("binary-mcp", configString("ops.binary_mcp", ""), "path to local mcp-server binary")
	cmd.Flags().String("binary-watcher", configString("ops.binary_watcher", ""), "path to local watcher-engine binary")
}

func getOpsConfig(cmd *cobra.Command) opsConfig {
	v := bindCommandViper(cmd, "KW_OPS")
	return opsConfig{
		target:          v.GetString("target"),
		image:           v.GetString("image"),
		namePrefix:      v.GetString("name-prefix"),
		kubeconfigPath:  expandPath(v.GetString("kubeconfig")),
		dbPath:          expandPath(v.GetString("db-path")),
		watcherConfig:   expandPath(v.GetString("watcher-config")),
		networkMode:     v.GetString("network-mode"),
		pollInterval:    v.GetString("interval"),
		buildIfMissing:  v.GetBool("build-if-missing"),
		followLogs:      v.GetBool("follow"),
		tailLines:       v.GetString("tail"),
		kubeNamespace:   v.GetString("kube-namespace"),
		kubeStorage:     v.GetString("kube-history-storage"),
		kubeClass:       v.GetString("kube-storage-class"),
		binaryMCPPath:   v.GetString("binary-mcp"),
		binaryWatchPath: v.GetString("binary-watcher"),
	}
}

func runDeploy(ctx context.Context, component string, cfg opsConfig) error {
	switch cfg.target {
	case "docker":
		return runDeployDocker(ctx, component, cfg)
	case "binary":
		return runDeployBinary(ctx, component, cfg)
	case "kube":
		return runDeployKube(ctx, component, cfg)
	default:
		return fmt.Errorf("unsupported target %q (expected docker|binary|kube)", cfg.target)
	}
}

func runDeployDocker(ctx context.Context, component string, cfg opsConfig) error {
	cli, err := newDockerClient()
	if err != nil {
		return err
	}
	defer func() { _ = cli.Close() }()

	fmt.Printf("checking for image: %s\n", cfg.image)
	if _, err := cli.ImageInspect(ctx, cfg.image); err != nil {
		if !cfg.buildIfMissing {
			return fmt.Errorf("image %q not available locally: %w", cfg.image, err)
		}
		fmt.Printf("image not found locally, building with make docker\n")
		build := exec.CommandContext(ctx, "make", "docker")
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr
		if err := build.Run(); err != nil {
			return fmt.Errorf("build image with make docker: %w", err)
		}
	}

	name := containerName(component, cfg)
	_ = removeContainer(ctx, cli, name)

	if err := os.MkdirAll(cfg.dbPath, 0o755); err != nil {
		return fmt.Errorf("create db path: %w", err)
	}

	env := []string{"KUBECONFIG=/root/.kube/config"}
	cmd := componentCommand(component, cfg)

	binds := []string{
		fmt.Sprintf("%s:/root/.kube/config:ro", cfg.kubeconfigPath),
		fmt.Sprintf("%s:/data/badger", cfg.dbPath),
	}
	if component == "watcher" {
		binds = append(binds, fmt.Sprintf("%s:/etc/kw/config.yaml:ro", cfg.watcherConfig))
	}

	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: cfg.image,
		Cmd:   cmd,
		Env:   env,
	}, &container.HostConfig{
		Binds:       binds,
		NetworkMode: container.NetworkMode(cfg.networkMode),
	}, &network.NetworkingConfig{}, nil, name)
	if err != nil {
		return fmt.Errorf("container create failed: %w", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("container start failed: %w", err)
	}

	fmt.Printf("deployed %s as container %s\n", component, name)
	return nil
}

func runDeployBinary(ctx context.Context, component string, cfg opsConfig) error {
	var path string
	var args []string
	switch component {
	case "mcp":
		path = cfg.binaryMCPPath
		args = []string{"--db-path", cfg.dbPath, "--interval", cfg.pollInterval}
	case "watcher":
		path = cfg.binaryWatchPath
		args = []string{"--config", cfg.watcherConfig}
	default:
		return fmt.Errorf("unsupported component %q", component)
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("binary path required for %s (set --binary-%s or ops.binary_%s in config)", component, component, component)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "KUBECONFIG="+cfg.kubeconfigPath)
	fmt.Printf("starting %s binary: %s %s\n", component, path, strings.Join(args, " "))
	return cmd.Run()
}

func runDeployKube(ctx context.Context, component string, cfg opsConfig) error {
	manifest, err := kubeManifest(component, cfg)
	if err != nil {
		return err
	}
	apply := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	apply.Stdin = strings.NewReader(manifest)
	apply.Stdout = os.Stdout
	apply.Stderr = os.Stderr
	if err := apply.Run(); err != nil {
		return fmt.Errorf("kubectl apply failed: %w", err)
	}
	fmt.Printf("deployed %s to namespace %s\n", component, cfg.kubeNamespace)
	return nil
}

func componentCommand(component string, cfg opsConfig) []string {
	if component == "mcp" {
		return []string{"/app/mcp-server", "--db-path", "/data/badger/history.db", "--interval", cfg.pollInterval}
	}
	return []string{"/app/watcher-engine", "--config", "/etc/kw/config.yaml"}
}

func kubeManifest(component string, cfg opsConfig) (string, error) {
	name := containerName(component, cfg)
	namespace := cfg.kubeNamespace
	if namespace == "" {
		namespace = "kubewatcher"
	}
	pvcName := fmt.Sprintf("%s-history", name)
	storageClass := ""
	if cfg.kubeClass != "" {
		storageClass = fmt.Sprintf("  storageClassName: %s\n", cfg.kubeClass)
	}

	var configMap string
	if component == "watcher" {
		data, err := os.ReadFile(cfg.watcherConfig)
		if err != nil {
			return "", fmt.Errorf("read watcher config: %w", err)
		}
		configMap = fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-config
  namespace: %s
data:
  config.yaml: |
%s
---
`, name, namespace, indentBlock(string(data), 4))
	}

	return fmt.Sprintf(`%sapiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s
  namespace: %s
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: %s
%s---
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
    spec:
      containers:
        - name: %s
          image: %s
          imagePullPolicy: IfNotPresent
          args: %s
          volumeMounts:
            - name: history
              mountPath: /data/badger
%s      volumes:
        - name: history
          persistentVolumeClaim:
            claimName: %s
%s`,
		configMap,
		pvcName,
		namespace,
		cfg.kubeStorage,
		storageClass,
		name,
		namespace,
		name,
		name,
		component,
		cfg.image,
		formatArgs(componentCommand(component, cfg)),
		watcherVolumeMount(component, name),
		pvcName,
		watcherVolume(component, name),
	), nil
}

func watcherVolumeMount(component, name string) string {
	if component != "watcher" {
		return ""
	}
	return "            - name: watcher-config\n              mountPath: /etc/kw/config.yaml\n              subPath: config.yaml\n"
}

func watcherVolume(component, name string) string {
	if component != "watcher" {
		return ""
	}
	return fmt.Sprintf("        - name: watcher-config\n          configMap:\n            name: %s-config\n", name)
}

func formatArgs(args []string) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, fmt.Sprintf("%q", a))
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func indentBlock(content string, spaces int) string {
	prefix := strings.Repeat(" ", spaces)
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func containerName(component string, cfg opsConfig) string {
	return fmt.Sprintf("%s-%s", strings.TrimSpace(cfg.namePrefix), component)
}

func newDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("initialize docker client: %w", err)
	}
	return cli, nil
}

func removeContainer(ctx context.Context, cli *client.Client, name string) error {
	timeout := int(defaultRuntimeTimeout().Seconds())
	_ = cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &timeout})
	if err := cli.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}); err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("remove container %q: %w", name, err)
	}
	fmt.Printf("removed container %s\n", name)
	return nil
}

func printContainerStatus(ctx context.Context, cli *client.Client, name string) error {
	info, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			fmt.Printf("container %s not found\n", name)
			return nil
		}
		return fmt.Errorf("inspect container %q: %w", name, err)
	}

	status := "unknown"
	started := ""
	if info.State != nil {
		status = info.State.Status
		started = info.State.StartedAt
	}

	fmt.Printf("name: %s\n", name)
	fmt.Printf("image: %s\n", info.Config.Image)
	fmt.Printf("status: %s\n", status)
	fmt.Printf("started_at: %s\n", started)
	return nil
}

func streamContainerLogs(ctx context.Context, cli *client.Client, name string, follow bool, tail string) error {
	reader, err := cli.ContainerLogs(ctx, name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Tail:       tail,
		Timestamps: false,
	})
	if err != nil {
		return fmt.Errorf("read logs for %q: %w", name, err)
	}
	defer func() { _ = reader.Close() }()

	_, err = io.Copy(os.Stdout, reader)
	if err != nil {
		return fmt.Errorf("stream logs for %q: %w", name, err)
	}
	return nil
}
