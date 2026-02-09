package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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
		_           = flag.Bool("server", false, "Run in interactive server mode (default)")
	)
	flag.Parse()

	if *showVersion {
		_, _ = os.Stdout.WriteString(fmt.Sprintf("kube-watcher v%s (commit: %s, built: %s)\n", version, gitCommit, buildDate))
		return
	}

	if *showHelp {
		showUsage()
		return
	}

	logger, err := logging.New(*debug, *logFile)
	if err != nil {
		_, _ = os.Stderr.WriteString(fmt.Sprintf("Failed to create logger: %v\n", err))
		os.Exit(1)
	}

	mcpServer, err := server.NewMCPServer(logger, server.Config{
		Version:   version,
		GitCommit: gitCommit,
		BuildDate: buildDate,
	})
	if err != nil {
		logger.Error("Failed to create MCP server", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
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

func showUsage() {
	_, _ = os.Stdout.WriteString(fmt.Sprintf(`kube-watcher v%s - Kubernetes Monitoring MCP Server

USAGE:
    kube-watcher [OPTIONS]

OPTIONS:
    --version           Show version information
    --help              Show this help message
    --list-tools        List all available tools
    --exec TOOL         Execute a specific tool
    --args JSON         JSON arguments for tool execution (use with --exec)
    --health            Perform health check
    --server            Run in interactive server mode (default)
    --debug             Enable debug logging
    --log-file PATH     Path to log file

EXAMPLES:
    # List available tools
    kube-watcher --list-tools

    # Execute node status check
    kube-watcher --exec get_node_status --args '{"include_metrics":true}'

    # Execute cluster analysis
    kube-watcher --exec analyze_cluster --args '{"include_pods":true,"include_events":true}'

    # Health check
    kube-watcher --health

    # Run as MCP server (interactive mode)
    kube-watcher --server
`, version))
}

func handleListTools(ctx context.Context, mcpServer *server.MCPServer, logger *slog.Logger) {
	toolsInfo := mcpServer.ListTools()
	logger.Info("Available Tools", "tools", toolsInfo)
}

func handleToolExecution(ctx context.Context, mcpServer *server.MCPServer, toolName, argsJSON string, logger *slog.Logger) {
	var args map[string]interface{}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		logger.Error("Invalid JSON arguments", "error", err)
		os.Exit(1)
	}

	logger.Info("Executing tool", "tool", toolName, "args", args)

	result, err := mcpServer.ExecuteTool(ctx, toolName, args)
	if err != nil {
		logger.Error("Tool execution failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Execution completed successfully", "result", result)
}

func handleHealthCheck(ctx context.Context, mcpServer *server.MCPServer, logger *slog.Logger) {
	logger.Info("Performing health check")

	health := mcpServer.HealthCheck(ctx)
	logger.Info("Health check result", "health", health)

	status, _ := health["server_status"].(string)
	k8sStatus, _ := health["k8s_connectivity"].(string)

	if status == "healthy" && k8sStatus == "healthy" {
		logger.Info("All systems operational")
		os.Exit(0)
	} else {
		logger.Warn("Issues detected")
		os.Exit(1)
	}
}

func handleServerMode(ctx context.Context, mcpServer *server.MCPServer, logger *slog.Logger) {
	logger.Info("Starting kube-watcher MCP server", "version", version)

	health := mcpServer.HealthCheck(ctx)
	k8sStatus, _ := health["k8s_connectivity"].(string)

	if k8sStatus != "healthy" {
		logger.Warn("Kubernetes connectivity issue", "status", k8sStatus, "error", health["k8s_error"])
	} else {
		logger.Info("Kubernetes connectivity verified", "cluster_info", health["cluster_info"])
	}

	toolsInfo := mcpServer.ListTools()
	logger.Info("Loaded tools", "count", toolsInfo["tool_count"])
	logger.Info("Server ready for MCP requests")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		select {
		case sig := <-sigCh:
			logger.Info("Shutdown signal received", "signal", sig.String())
			cancel()
		case <-runCtx.Done():
		}
	}()

	if err := mcpServer.Start(runCtx); err != nil {
		logger.Error("Server terminated with error", "error", err)
		os.Exit(1)
	}

	logger.Info("Server stopped gracefully")
}
