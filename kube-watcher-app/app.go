package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kube-watcher-app/internal/data"
	"kube-watcher-app/internal/data/mcp"
	"kube-watcher-app/internal/data/opencode"
	"kube-watcher-app/internal/data/prometheus"
	"kube-watcher-app/internal/workspace"
)

type App struct {
	ctx       context.Context
	mcp       *mcp.Client
	prom      *prometheus.Client
	opencode  *opencode.Client
	workspace *workspace.Handler
}

func NewApp() *App {
	return &App{}
}

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

func (a *App) GetRecommendations() ([]data.Recommendation, error) {
	return a.mcp.Recommendations(a.ctx)
}

func (a *App) GetStatus() (data.StatusResponse, error) {
	return a.mcp.Status(a.ctx)
}

func (a *App) GetEndpoint() string {
	return a.mcp.Endpoint()
}

func (a *App) AskAI(prompt string, contextStr string) (string, error) {
	return a.opencode.Ask(a.ctx, prompt, contextStr)
}

func (a *App) ExecuteTool(name string, args map[string]any) (data.ToolResult, error) {
	return a.mcp.ExecuteTool(a.ctx, name, args)
}

func (a *App) GetServiceStatus() ([]data.ServiceStatus, error) {
	return a.mcp.Services(a.ctx)
}

func (a *App) StartService(name string) error {
	return a.mcp.StartService(a.ctx, name)
}

func (a *App) StopService(name string) error {
	return a.mcp.StopService(a.ctx, name)
}

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

// GetAnomstackAnomalies returns anomalies from the anomstack service
func (a *App) GetAnomstackAnomalies() ([]data.AlertRecord, error) {
	return a.mcp.GetAnomstackAnomalies(a.ctx)
}

// RunSynapseSweep executes popeye CLI for node status
func (a *App) RunSynapseSweep() (string, error) {
	// Check if popeye is available
	bin, err := exec.LookPath("popeye")
	if err != nil {
		// Debug: check PATH and try direct path
		pathEnv := os.Getenv("PATH")
		directPath := "/opt/homebrew/bin/popeye"
		if _, directErr := os.Stat(directPath); directErr == nil {
			// Try using direct path
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
	return string(out), nil
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
