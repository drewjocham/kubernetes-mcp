package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/data/mcp"
	"kube-watcher-app/internal/data/opencode"
	"kube-watcher-app/internal/data/prometheus"
	"kube-watcher-app/internal/workspace"
)

// App struct
type App struct {
	ctx       context.Context
	mcp       *mcp.Client
	prom      *prometheus.Client
	opencode  *opencode.Client
	workspace *workspace.Handler
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.mcp = mcp.New()
	a.prom = prometheus.New()
	a.opencode = opencode.New()
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

// GetHistory returns incident history
func (a *App) GetHistory() ([]data.Incident, error) {
	return a.mcp.History(a.ctx)
}

// GetRecommendations returns remediation suggestions
func (a *App) GetRecommendations() ([]data.Recommendation, error) {
	return a.mcp.Recommendations(a.ctx)
}

// GetStatus returns the current MCP server status, endpoint and cluster
func (a *App) GetStatus() (data.StatusResponse, error) {
	return a.mcp.Status(a.ctx)
}

// GetEndpoint returns the MCP endpoint URL
func (a *App) GetEndpoint() string {
	return a.mcp.Endpoint()
}

// AskAI calls the OpenCode AI API with context
func (a *App) AskAI(prompt string, contextStr string) (string, error) {
	return a.opencode.Ask(a.ctx, prompt, contextStr)
}

// ExecuteTool calls an MCP tool
func (a *App) ExecuteTool(name string, args map[string]any) (data.ToolResult, error) {
	return a.mcp.ExecuteTool(a.ctx, name, args)
}

// GetServiceStatus returns Docker service statuses
func (a *App) GetServiceStatus() ([]data.ServiceStatus, error) {
	return a.mcp.Services(a.ctx)
}

// StartService starts a Docker service
func (a *App) StartService(name string) error {
	return a.mcp.StartService(a.ctx, name)
}

// StopService stops a Docker service
func (a *App) StopService(name string) error {
	return a.mcp.StopService(a.ctx, name)
}

// GetLogs returns actual logs from the platform components
func (a *App) GetLogs() ([]data.LogLine, error) {
	services, err := a.mcp.Services(a.ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch services for logs: %w", err)
	}

	var allLogs []data.LogLine
	for _, s := range services {
		if s.Status != "running" {
			continue
		}
		lines, err := a.mcp.FetchServiceLogs(a.ctx, s.Name, "10")
		if err != nil {
			continue
		}
		for _, line := range lines {
			allLogs = append(allLogs, data.LogLine{
				Timestamp: time.Now(), // FetchServiceLogs currently returns strings, we'll use now for simplicity
				Level:     "INFO",
				Source:    s.Name,
				Message:   line,
			})
		}
	}

	if len(allLogs) == 0 {
		allLogs = append(allLogs, data.LogLine{
			Timestamp: time.Now(),
			Level:     "INFO",
			Source:    "System",
			Message:   "No active logs found for running services.",
		})
	}

	return allLogs, nil
}

// RunSynapseSweep executes popeye CLI for node status
func (a *App) RunSynapseSweep() (string, error) {
	// Check if popeye is available
	bin, err := exec.LookPath("popeye")
	if err != nil {
		return "", fmt.Errorf("popeye binary not found in PATH; install it first (go install github.com/derailed/popeye@v0.21.4 or add it to PATH)")
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
	return string(out), nil
}

// RunCommand executes a shell command and returns its combined output
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
