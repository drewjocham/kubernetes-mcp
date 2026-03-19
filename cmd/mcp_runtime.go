package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"kube-watcher/mcp/monitoring/history"
	"kube-watcher/mcp/server"
	"kube-watcher/pkg/kube"
	kwatch "kube-watcher/pkg/kube/watch"
	"kube-watcher/pkg/logging"
)

type mcpRuntimeConfig struct {
	debug    bool
	logFile  string
	dbPath   string
	interval time.Duration
}

type mcpRuntime struct {
	server  *server.MCPServer
	history *history.Store
}

func newMCPRuntime(cfg mcpRuntimeConfig) (*mcpRuntime, error) {
	logger, err := logging.New(cfg.debug, cfg.logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	dbPath := expandPath(cfg.dbPath)
	if err := ensureDir(dbPath); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	k8sClient, err := kube.NewClient(logger)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client init failed: %w", err)
	}

	historyStore, err := history.NewStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("storage init failed: %w", err)
	}

	watchManager := kwatch.NewManager(k8sClient, logger, cfg.interval)
	mcpServer, err := server.NewMCPServer(logger, server.Config{
		Version:      version,
		GitCommit:    gitCommit,
		BuildDate:    buildDate,
		K8sClient:    k8sClient,
		HistoryStore: historyStore,
		Watcher:      watchManager,
	})
	if err != nil {
		historyStore.Close()
		return nil, fmt.Errorf("mcp server init failed: %w", err)
	}

	return &mcpRuntime{server: mcpServer, history: historyStore}, nil
}

func (r *mcpRuntime) close() {
	if r != nil && r.history != nil {
		_ = r.history.Close()
	}
}

func runMCPTool(ctx context.Context, srv *server.MCPServer, toolName, rawArgs, output string) error {
	var args map[string]any
	if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
		return fmt.Errorf("invalid JSON in --args: %w", err)
	}

	result, err := srv.ExecuteTool(ctx, toolName, args)
	if err != nil {
		return err
	}
	return renderOutput(result, output)
}

func renderOutput(data any, output string) error {
	switch output {
	case "json":
		out, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	case "yaml", "pretty":
		out, err := yaml.Marshal(data)
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		return nil
	default:
		return fmt.Errorf("unsupported output format %q (expected json|yaml)", output)
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func ensureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0o755)
}
