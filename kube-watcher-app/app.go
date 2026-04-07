package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/data/k8sgpt"
	"kube-watcher-app/internal/data/mcp"
	"kube-watcher-app/internal/data/opencode"
	"kube-watcher-app/internal/data/prometheus"
	"kube-watcher-app/internal/data/watcher"
	"kube-watcher-app/internal/data/widgets"
)

type App struct {
	ctx         context.Context
	mcp         *mcp.Client
	prom        *prometheus.Client
	opencode    *opencode.Client
	k8sgpt      *k8sgpt.Client
	watcher     *watcher.Client
	workspace   *workspace.Handler
	widgetStore *widgets.Store
	aiConfig    AIConfig
}

type AIConfig struct {
	Provider string
	APIKey   string
	BaseURL  string
	Model    string
	Backend  string
}

// kwBinaryPath returns the path to the kw binary (prefer kw-cli for CLI commands).
// It looks for the binary in the following order:
// 1. KW_CLI_BINARY_PATH environment variable (for kw-cli)
// 2. KW_BINARY_PATH environment variable (for kw)
// 3. ../bin/kw-cli relative to the executable
// 4. ../bin/kw relative to the executable
// 5. kw-cli in PATH
// 6. kw in PATH
func (a *App) kwBinaryPath() (string, error) {
	// 1. KW_CLI_BINARY_PATH environment variable
	if envPath := os.Getenv("KW_CLI_BINARY_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	// 2. KW_BINARY_PATH environment variable
	if envPath := os.Getenv("KW_BINARY_PATH"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
	}

	// 3. Relative to executable (for development)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		// Try multiple relative paths
		relPaths := []string{
			"../bin/kw-cli",
			"../bin/kw",
			"../../bin/kw-cli",
			"../../bin/kw",
		}
		for _, rel := range relPaths {
			devPath := filepath.Join(exeDir, rel)
			if absPath, err := filepath.Abs(devPath); err == nil {
				if _, err := os.Stat(absPath); err == nil {
					return absPath, nil
				}
			}
		}
	}

	// 4. Look in PATH for kw-cli
	if path, err := exec.LookPath("kw-cli"); err == nil {
		return path, nil
	}
	// 5. Look in PATH for kw
	if path, err := exec.LookPath("kw"); err == nil {
		return path, nil
	}

	return "", fmt.Errorf("kw binary not found. Please install kw using 'make install-local' or set KW_CLI_BINARY_PATH/KW_BINARY_PATH")
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.mcp = mcp.New()
	a.prom = prometheus.New()
	a.opencode = opencode.New()
	a.k8sgpt = k8sgpt.New()
	a.watcher = watcher.New()
	// Initialize widget store (ignore error for now, will be lazy-loaded)
	widgetStore, err := widgets.DefaultStore()
	if err != nil {
		// Log error but continue (store will be nil)
		fmt.Printf("Failed to initialize widget store: %v\n", err)
	} else {
		a.widgetStore = widgetStore
	}
	adapter := workspace.NewMCPAdapter(a.mcp)
	a.workspace = workspace.NewHandler(
		adapter,
		adapter,
		adapter,
		adapter,
		workspace.DefaultDeploymentStrategies(),
	)
}

// Greet returns a greeting for the given name
func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}

// GetAIWorkspace returns the AI-first desktop control plane state.
func (a *App) GetAIWorkspace() (data.AIWorkspace, error) {
	return a.workspace.Build(a.ctx)
}

// GetAlerts returns the current alerts
func (a *App) GetAlerts() ([]data.AlertRecord, error) {
	return a.mcp.Alerts(a.ctx)
}

// UpdateAlertState updates the state of an alert
func (a *App) UpdateAlertState(id string, state string) error {
	return a.mcp.UpdateAlertState(a.ctx, id, state)
}

// AddAlertComment adds a comment to an alert
func (a *App) AddAlertComment(id string, author string, content string) error {
	return a.mcp.AddAlertComment(a.ctx, id, author, content)
}

// GetHistory returns incident history
func (a *App) GetHistory() ([]data.Incident, error) {
	return a.mcp.History(a.ctx)
}

func (a *App) GetRecommendations() ([]data.Recommendation, error) {
	return a.mcp.Recommendations(a.ctx)
}

func (a *App) GetStatus() (data.StatusResponse, error) {
	return a.mcp.Status(a.ctx)
}

func (a *App) GetEndpoint() string {
	return a.mcp.Endpoint()
}

func (a *App) GetWatcherConfig() (map[string]interface{}, error) {
	if a.watcher == nil {
		return nil, fmt.Errorf("watcher client not initialized")
	}
	return a.watcher.GetConfig(a.ctx)
}

func (a *App) GetWatcherRules() ([]interface{}, error) {
	if a.watcher == nil {
		return nil, fmt.Errorf("watcher client not initialized")
	}
	return a.watcher.GetRules(a.ctx)
}

func (a *App) GetWatcherResources() ([]string, error) {
	if a.watcher == nil {
		return nil, fmt.Errorf("watcher client not initialized")
	}
	return a.watcher.GetResources(a.ctx)
}

func (a *App) GetWatcherStatus() (map[string]interface{}, error) {
	if a.watcher == nil {
		return nil, fmt.Errorf("watcher client not initialized")
	}
	return a.watcher.GetStatus(a.ctx)
}

func (a *App) StreamWatcherLogs() (*http.Response, error) {
	if a.watcher == nil {
		return nil, fmt.Errorf("watcher client not initialized")
	}
	return a.watcher.StreamLogs(a.ctx)
}

func (a *App) AskAI(prompt string, contextStr string) (string, error) {
	// Try K8sGPT for Kubernetes-related questions
	if a.k8sgpt != nil {
		response, err := a.k8sgpt.Ask(a.ctx, prompt, contextStr)
		if err == nil {
			return response, nil
		}
		// If error is about non-Kubernetes question, fall back to opencode
		if !strings.Contains(err.Error(), "not Kubernetes-related") {
			// For other errors, we could still fall back to opencode
			// but for now, let's log and try opencode
		}
	}
	// Fall back to opencode for non-Kubernetes questions or if K8sGPT fails
	return a.opencode.Ask(a.ctx, prompt, contextStr)
}

func (a *App) UpdateAIConfig(provider, apiKey, baseURL, model, backend string) error {
	fmt.Printf("UpdateAIConfig called: provider=%q, apiKey=%q (len=%d), baseURL=%q, model=%q, backend=%q\n",
		provider, maskAPIKey(apiKey), len(apiKey), baseURL, model, backend)
	a.aiConfig = AIConfig{
		Provider: provider,
		APIKey:   apiKey,
		BaseURL:  baseURL,
		Model:    model,
		Backend:  backend,
	}
	// Update opencode client (general AI agent)
	if a.opencode != nil {
		a.opencode.SetConfig(baseURL, apiKey, provider, model, backend)
	}
	// Note: K8sGPT client uses separate environment variables (K8SGPT_*)
	// and is configured independently
	return nil
}

func maskAPIKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "***" + key[len(key)-4:]
}

func (a *App) ExecuteTool(name string, args map[string]any) (data.ToolResult, error) {
	return a.mcp.ExecuteTool(a.ctx, name, args)
}

// GetAnomstackAnomalies returns anomalies from the anomstack service
func (a *App) GetAnomstackAnomalies() ([]data.AlertRecord, error) {
	return a.mcp.GetAnomstackAnomalies(a.ctx)
}

// GetNamespaces returns a list of all namespaces in the cluster
func (a *App) GetNamespaces() ([]string, error) {
	result, err := a.mcp.ExecuteTool(a.ctx, "list_namespaces", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("failed to execute namespace list tool: %w", err)
	}
	if result.Err != "" {
		return nil, fmt.Errorf("tool error: %s", result.Err)
	}
	namespacesRaw, ok := result.Output["namespaces"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing namespaces array")
	}
	namespaces := make([]string, 0, len(namespacesRaw))
	for _, ns := range namespacesRaw {
		nsMap, ok := ns.(map[string]any)
		if !ok {
			continue
		}
		name, ok := nsMap["name"].(string)
		if !ok {
			continue
		}
		namespaces = append(namespaces, name)
	}
	return namespaces, nil
}

// GetPods returns pods in the specified namespace (empty for all namespaces)
func (a *App) GetPods(namespace string) ([]data.PodInfo, error) {
	args := map[string]any{}
	if namespace != "" {
		args["namespace"] = namespace
	}
	result, err := a.mcp.ExecuteTool(a.ctx, "get_pod_resources", args)
	if err != nil {
		return nil, fmt.Errorf("failed to execute pod resources tool: %w", err)
	}
	if result.Err != "" {
		return nil, fmt.Errorf("tool error: %s", result.Err)
	}
	podsRaw, ok := result.Output["pods"].([]any)
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing pods array")
	}
	pods := make([]data.PodInfo, 0, len(podsRaw))
	for _, p := range podsRaw {
		podMap, ok := p.(map[string]any)
		if !ok {
			continue
		}
		podInfo, err := mapToPodInfo(podMap)
		if err != nil {
			// skip invalid pod entry
			continue
		}
		pods = append(pods, podInfo)
	}
	return pods, nil
}

func mapToPodInfo(podMap map[string]any) (data.PodInfo, error) {
	var pod data.PodInfo
	var err error

	// Basic fields
	if name, ok := podMap["name"].(string); ok {
		pod.Name = name
	}
	if namespace, ok := podMap["namespace"].(string); ok {
		pod.Namespace = namespace
	}
	if status, ok := podMap["status"].(string); ok {
		pod.Status = status
	}
	if node, ok := podMap["node"].(string); ok {
		pod.NodeName = node
	}
	if phaseRaw, ok := podMap["phase"]; ok {
		if phase, ok := phaseRaw.(string); ok {
			pod.Phase = phase
		}
	}
	if ageStr, ok := podMap["age"].(string); ok {
		duration, err := time.ParseDuration(ageStr)
		if err == nil {
			pod.Age = duration
		}
	}
	if restartsRaw, ok := podMap["restarts"]; ok {
		if restarts, ok := restartsRaw.(float64); ok {
			pod.RestartCount = int(restarts)
		}
	}
	// Labels and annotations not provided by tool, leave empty
	pod.Labels = make(map[string]string)
	pod.Annotations = make(map[string]string)

	// Containers
	if containersRaw, ok := podMap["containers"].([]any); ok {
		containers := make([]data.ContainerInfo, 0, len(containersRaw))
		for _, cRaw := range containersRaw {
			if cMap, ok := cRaw.(map[string]any); ok {
				var container data.ContainerInfo
				if name, ok := cMap["name"].(string); ok {
					container.Name = name
				}
				if image, ok := cMap["image"].(string); ok {
					container.Image = image
				}
				if readyRaw, ok := cMap["ready"]; ok {
					if ready, ok := readyRaw.(bool); ok {
						container.Ready = ready
					}
				}
				if restartsRaw, ok := cMap["restarts"]; ok {
					if restarts, ok := restartsRaw.(float64); ok {
						container.RestartCount = int32(restarts)
					}
				}
				if health, ok := cMap["health"].(string); ok {
					container.State = health
				}
				containers = append(containers, container)
			}
		}
		pod.Containers = containers
	}
	return pod, err
}

// GetPodLogs returns logs for a specific pod and container
func (a *App) GetPodLogs(namespace, podName, container string) (string, error) {
	args := map[string]any{
		"namespace": namespace,
		"pod_name":  podName,
	}
	if container != "" {
		args["container"] = container
	}
	result, err := a.mcp.ExecuteTool(a.ctx, "get_pod_logs", args)
	if err != nil {
		return "", fmt.Errorf("failed to execute pod logs tool: %w", err)
	}
	if result.Err != "" {
		return "", fmt.Errorf("tool error: %s", result.Err)
	}
	logsRaw, ok := result.Output["logs"].(string)
	if !ok {
		return "", fmt.Errorf("invalid response format: missing logs string")
	}
	return logsRaw, nil
}

// DeployAnomstack deploys anomstack using the specified profile (local, minikube, cluster)
func (a *App) DeployAnomstack(profile string) (string, error) {
	return a.runKwCommand("anomstack", "start", "--profile", profile)
}

// GetPodYAML returns the YAML manifest of a pod
func (a *App) GetPodYAML(namespace, podName string) (string, error) {
	return a.runKubectlCommand("get", "pod", "-n", namespace, podName, "-o", "yaml")
}

// DescribePod returns the kubectl describe output for a pod
func (a *App) DescribePod(namespace, podName string) (string, error) {
	return a.runKubectlCommand("describe", "pod", "-n", namespace, podName)
}

// GetPodAIHelp returns AI-generated help for a pod using k8sgpt or opencode
func (a *App) GetPodAIHelp(namespace, podName string) (string, error) {
	// Use opencode to analyze pod issues
	query := fmt.Sprintf("Analyze pod %s in namespace %s. Provide troubleshooting steps.", podName, namespace)
	response, err := a.opencode.Ask(a.ctx, query, "")
	if err != nil {
		return "", fmt.Errorf("failed to get AI help: %w", err)
	}
	return response, nil
}

// ExecPodCommand executes a command inside a pod container using kubectl exec
func (a *App) ExecPodCommand(namespace, podName, container, command string) (string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	args := []string{"exec", "-n", namespace, podName}
	if container != "" {
		args = append(args, "-c", container)
	}
	args = append(args, "--", "sh", "-c", command)

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Env = os.Environ()
	if kPath := resolveKubeconfigPath(); kPath != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+kPath)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("kubectl exec failed: %w (output: %s)", err, string(out))
	}
	return string(out), nil
}

// RunSynapseSweep executes popeye CLI for node status
func (a *App) RunSynapseSweep() (string, error) {
	// Check if popeye is available
	bin, err := exec.LookPath("popeye")
	if err != nil {
		pathEnv := os.Getenv("PATH")
		directPath := "/opt/homebrew/bin/popeye"
		if _, directErr := os.Stat(directPath); directErr == nil {
			bin = directPath
		} else {
			return "", fmt.Errorf("popeye binary not found in PATH (%s); install it first (go install github.com/derailed/popeye@v0.21.4 or add it to PATH). Direct path check failed: %v", pathEnv, directErr)
		}
	}

	// Run popeye with -A -o json
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-A", "-o", "json")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+resolveKubeconfigPath())
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return "", fmt.Errorf("Synapse Sweep (popeye) error: %v, output: %s", runErr, string(out))
	}
	result := string(out)
	fmt.Printf("[RunSynapseSweep] raw output length: %d\n", len(result))
	if len(result) > 200 {
		fmt.Printf("[RunSynapseSweep] first 200 chars: %s\n", result[:200])
	}
	return result, nil
}

func (a *App) GetCurrentContext() (string, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", "config", "current-context")
	cmd.Env = os.Environ()
	if kPath := resolveKubeconfigPath(); kPath != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+kPath)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get current context: %w (output: %s)", err, string(out))
	}

	context := strings.TrimSpace(string(out))
	if context == "" {
		return "unknown", nil
	}
	return context, nil
}

func (a *App) RunCommand(command string) (string, error) {
	// For security in this specific context, we'll use a shell but wrap it.
	// In a production app, you might want to whitelist or use more robust parsing.
	ctx, cancel := context.WithTimeout(a.ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Env = os.Environ()
	// You might want to set specific env vars here like KUBECONFIG
	if kPath := resolveKubeconfigPath(); kPath != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+kPath)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("command failed: %w (output: %s)", err, string(out))
	}
	return string(out), nil
}

func resolveKubeconfigPath() string {
	if value := os.Getenv("KUBECONFIG_PATH"); value != "" {
		return value
	}
	if value := os.Getenv("KUBECONFIG"); value != "" {
		return value
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".kube", "config")
}

// runKubectlCommand executes a kubectl command with the given arguments.
func (a *App) runKubectlCommand(args ...string) (string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", args...)
	cmd.Env = os.Environ()
	if kPath := resolveKubeconfigPath(); kPath != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+kPath)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("kubectl command failed: %w (output: %s)", err, string(out))
	}
	return string(out), nil
}

// runKwCommand executes a kw command with the given arguments.
func (a *App) runKwCommand(args ...string) (string, error) {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	binaryPath, err := a.kwBinaryPath()
	if err != nil {
		return "", fmt.Errorf("failed to find kw binary: %w", err)
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Env = os.Environ()
	if kPath := resolveKubeconfigPath(); kPath != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+kPath)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("kw command failed: %w (output: %s)", err, string(out))
	}
	return string(out), nil
}

// GetWidgets returns all widgets.
func (a *App) GetWidgets() ([]data.Widget, error) {
	if a.widgetStore == nil {
		return []data.Widget{}, nil
	}
	return a.widgetStore.All(a.ctx)
}

// SaveWidget creates or updates a widget.
func (a *App) SaveWidget(widget data.Widget) error {
	if a.widgetStore == nil {
		return fmt.Errorf("widget store not initialized")
	}
	return a.widgetStore.Save(a.ctx, widget)
}

// DeleteWidget removes a widget by ID.
func (a *App) DeleteWidget(id string) error {
	if a.widgetStore == nil {
		return fmt.Errorf("widget store not initialized")
	}
	return a.widgetStore.Delete(a.ctx, id)
}
