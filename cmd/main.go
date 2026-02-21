package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"kube-watcher/kubernetes"
	"kube-watcher/kubernetes/watch"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kube-watcher/internal/history"
	"kube-watcher/internal/logging"
	"kube-watcher/server"
)

var (
	version   = "1.0.0"
	gitCommit = "dev"
	buildDate = "unknown"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "Show version information")
		showHelp    = flag.Bool("help", false, "Show help information")
		listTools   = flag.Bool("list-tools", false, "List available tools and exit")
		execTool    = flag.String("exec", "", "Execute a specific tool with JSON args")
		toolArgs    = flag.String("args", "{}", "JSON arguments for tool execution")
		healthCheck = flag.Bool("health", false, "Perform health check and exit")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		logFile     = flag.String("log-file", "", "Path to log file")
		dbPath      = flag.String("db-path", "~/.kube-watcher/history.db", "Path to BadgerDB storage")
		interval    = flag.Duration("interval", 30*time.Second, "Scan interval")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("kube-watcher v%s (commit: %s, built: %s)\n", version, gitCommit, buildDate)
		return
	}

	if *showHelp {
		showUsage()
		return
	}

	logger, err := logging.New(*debug, *logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	// Kubernetes & BadgerDB
	k8sClient, err := kubernetes.NewClient(logger)
	if err != nil {
		logger.Error("Failed to initialize Kubernetes client", "error", err)
		os.Exit(1)
	}

	historyStore, err := history.NewStore(*dbPath)
	if err != nil {
		logger.Error("Failed to initialize BadgerDB store", "error", err)
		os.Exit(1)
	}
	defer historyStore.Close()

	// Watcher
	watchManager := watch.NewManager(k8sClient, logger, *interval)

	mcpServer, err := server.NewMCPServer(logger, server.Config{
		Version:      version,
		GitCommit:    gitCommit,
		BuildDate:    buildDate,
		K8sClient:    k8sClient,
		HistoryStore: historyStore,
		Watcher:      watchManager,
	})
	if err != nil {
		logger.Error("Failed to create MCP server", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch {
	case *listTools:
		handleListTools(ctx, mcpServer, logger)
	case *execTool != "":
		handleToolExecution(ctx, mcpServer, *execTool, *toolArgs, logger)
	case *healthCheck:
		handleHealthCheck(ctx, mcpServer, logger)
	default:
		handleServerMode(ctx, mcpServer, logger)
	}
}

func handleListTools(ctx context.Context, s *server.MCPServer, logger *slog.Logger) {
	fmt.Printf("%+v\n", s.ListTools())
}

func handleToolExecution(ctx context.Context, s *server.MCPServer, name, argsJSON string, logger *slog.Logger) {
	var args map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		logger.Error("Invalid JSON arguments", "error", err)
		os.Exit(1)
	}

	result, err := s.ExecuteTool(ctx, name, args)
	if err != nil {
		logger.Error("Tool execution failed", "error", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", result)
}

func handleHealthCheck(ctx context.Context, s *server.MCPServer, logger *slog.Logger) {
	health := s.HealthCheck(ctx)
	if health["status"] == "healthy" {
		logger.Info("Systems operational", "details", health)
		os.Exit(0)
	}
	logger.Warn("System unhealthy", "details", health)
	os.Exit(1)
}

func handleServerMode(ctx context.Context, s *server.MCPServer, logger *slog.Logger) {
	logger.Info("Starting kube-watcher", "version", version)

	if err := s.Start(ctx); err != nil {
		logger.Error("Server error", "error", err)
		os.Exit(1)
	}
	logger.Info("Shutdown complete")
}

func showUsage() {
	fmt.Printf(`kube-watcher v%s - Kubernetes Monitoring MCP Server

USAGE:
    kube-watcher [OPTIONS]

OPTIONS:
    --version           Show version
    --db-path PATH      Path to history database (BadgerDB)
    --interval DUR      Scan interval (e.g. 1m, 30s)
    --exec TOOL         Execute tool
    --args JSON         Tool arguments
`, version)
}
