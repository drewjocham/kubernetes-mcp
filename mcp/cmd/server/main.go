package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"kube-watcher/mcp/monitoring/history"
	"kube-watcher/mcp/server"
	"kube-watcher/pkg/audit"
	"kube-watcher/pkg/kube"
	"kube-watcher/pkg/kube/watch"
	"kube-watcher/pkg/logging"
)

var (
	version   = "1.0.0"
	gitCommit = "dev"
	buildDate = "unknown"
)

type config struct {
	showVersion, showHelp, listTools, healthCheck, debug bool
	execTool, toolArgs, logFile, dbPath                  string
	interval                                             time.Duration
}

func main() {
	cfg := parseFlags()

	if cfg.showVersion {
		fmt.Printf("kube-watcher v%s (commit: %s, built: %s)\n", version, gitCommit, buildDate)
		return
	}

	logger, err := logging.New(cfg.debug, cfg.logFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	dbPath := expandPath(cfg.dbPath)
	handleErr(ensureDir(dbPath), "failed to create database directory", logger)

	// Create base Kubernetes client
	baseClient, err := kube.NewClient(logger)
	handleErr(err, "kubernetes client init failed", logger)

	// Wrap with audit logging
	auditLogger := audit.NewSlogLogger(logger)
	k8sClient := kube.NewAuditClient(baseClient, auditLogger, logger, kube.AuditOptionsFromEnv()...)

	historyStore, err := history.NewStore(dbPath)
	handleErr(err, "storage init failed", logger)
	defer func() { _ = historyStore.Close() }()

	watchManager := watch.NewManager(k8sClient, logger, cfg.interval)

	mcpServer, err := server.NewMCPServer(logger, server.Config{
		Version:      version,
		GitCommit:    gitCommit,
		BuildDate:    buildDate,
		K8sClient:    k8sClient,
		HistoryStore: historyStore,
		Watcher:      watchManager,
	})
	handleErr(err, "mcp server init failed", logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	executeAction(ctx, mcpServer, cfg, logger)
}

func parseFlags() config {
	c := config{}
	home, _ := os.UserHomeDir()
	defaultDB := filepath.Join(home, ".kube-watcher", "history.db")

	flag.BoolVar(&c.showVersion, "version", false, "display version information")
	flag.BoolVar(&c.showHelp, "help", false, "display help message")
	flag.BoolVar(&c.listTools, "list-tools", false, "list available MCP tools and exit")
	flag.StringVar(&c.execTool, "exec", "", "execute a specific tool by name")
	flag.StringVar(&c.toolArgs, "args", "{}", "JSON arguments for the tool execution")
	flag.BoolVar(&c.healthCheck, "health", false, "run a health check and exit")
	flag.BoolVar(&c.debug, "debug", false, "enable verbose debug logging")
	flag.StringVar(&c.logFile, "log-file", "", "path to write logs (defaults to stderr)")
	flag.StringVar(&c.dbPath, "db-path", defaultDB, "path to the history database")
	flag.DurationVar(&c.interval, "interval", 30*time.Second, "polling interval for watchers")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "kube-watcher v%s\n", version)
		fmt.Fprintf(os.Stderr, "Usage: kube-watcher [options]\n\nOptions:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if c.showHelp {
		flag.Usage()
		os.Exit(0)
	}

	return c
}

func executeAction(ctx context.Context, s *server.MCPServer, cfg config, logger *slog.Logger) {
	switch {
	case cfg.listTools:
		tools := s.ListTools()
		out, _ := json.MarshalIndent(tools, "", "  ")
		fmt.Println(string(out))

	case cfg.execTool != "":
		var args map[string]any
		if err := json.Unmarshal([]byte(cfg.toolArgs), &args); err != nil {
			logger.Error("invalid tool args JSON", "error", err)
			os.Exit(1)
		}
		res, err := s.ExecuteTool(ctx, cfg.execTool, args)
		if err != nil {
			logger.Error("tool execution failed", "tool", cfg.execTool, "error", err)
			os.Exit(1)
		}
		out, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(out))

	case cfg.healthCheck:
		h := s.HealthCheck(ctx)
		status, _ := h["status"].(string)
		if status != "healthy" {
			logger.Error("system unhealthy", "details", h)
			os.Exit(1)
		}
		logger.Info("system healthy")

	default:
		logger.Info("starting kube-watcher server", "version", version)
		if err := s.Start(ctx); err != nil {
			logger.Error("server exit with error", "error", err)
			os.Exit(1)
		}
	}
}

func handleErr(err error, msg string, logger *slog.Logger) {
	if err != nil {
		if logger != nil {
			logger.Error(msg, "error", err)
		} else {
			fmt.Fprintf(os.Stderr, "FATAL: %s: %v\n", msg, err)
		}
		os.Exit(1)
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
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
