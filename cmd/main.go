package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kube-watcher/internal/history"
	"kube-watcher/internal/logging"
	"kube-watcher/kubernetes"
	"kube-watcher/kubernetes/watch"
	"kube-watcher/server"
)

var (
	version   = "1.0.0"
	gitCommit = "dev"
	buildDate = "unknown"
)

func main() {
	var (
		showVersion = flag.Bool("version", false, "")
		showHelp    = flag.Bool("help", false, "")
		listTools   = flag.Bool("list-tools", false, "")
		execTool    = flag.String("exec", "", "")
		toolArgs    = flag.String("args", "{}", "")
		healthCheck = flag.Bool("health", false, "")
		debug       = flag.Bool("debug", false, "")
		logFile     = flag.String("log-file", "", "")
		dbPath      = flag.String("db-path", "~/.kube-watcher/history.db", "")
		interval    = flag.Duration("interval", 30*time.Second, "")
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
		fmt.Fprintf(os.Stderr, "logger init failed: %v\n", err)
		os.Exit(1)
	}

	k8sClient, err := kubernetes.NewClient(logger)
	if err != nil {
		logger.Error("kubernetes client init failed", "error", err)
		os.Exit(1)
	}

	historyStore, err := history.NewStore(*dbPath)
	if err != nil {
		logger.Error("storage init failed", "error", err)
		os.Exit(1)
	}
	defer historyStore.Close()

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
		logger.Error("mcp server init failed", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch {
	case *listTools:
		fmt.Printf("%+v\n", mcpServer.ListTools())
	case *execTool != "":
		var args map[string]any
		if err := json.Unmarshal([]byte(*toolArgs), &args); err != nil {
			logger.Error("invalid json args", "error", err)
			os.Exit(1)
		}
		result, err := mcpServer.ExecuteTool(ctx, *execTool, args)
		if err != nil {
			logger.Error("execution failed", "error", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n", result)
	case *healthCheck:
		health := mcpServer.HealthCheck(ctx)
		if health["status"] == "healthy" {
			os.Exit(0)
		}
		os.Exit(1)
	default:
		if err := mcpServer.Start(ctx); err != nil {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}
}

func showUsage() {
	fmt.Printf("kube-watcher v%s\nusage: kube-watcher [options]\n", version)
}
