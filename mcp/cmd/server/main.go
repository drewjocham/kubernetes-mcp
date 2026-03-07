package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"kube-watcher/mcp/monitoring/history"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kube-watcher/mcp/server"
	"kube-watcher/pkg/kube"
	"kube-watcher/pkg/kube/watch"
	"kube-watcher/pkg/logging"
)

var (
	version   = "1.0.0"
	gitCommit = "dev"
	buildDate = "unknown"
)

func main() {
	cfg := parseFlags()

	if cfg.showVersion {
		fmt.Printf("kube-watcher v%s (commit: %s, built: %s)\n", version, gitCommit, buildDate)
		return
	}

	if cfg.showHelp {
		showUsage()
		return
	}

	logger, err := logging.New(cfg.debug, cfg.logFile)
	handleErr(err, "logger init failed")

	k8sClient, err := kube.NewClient(logger)
	handleErr(err, "kubernetes client init failed")

	historyStore, err := history.NewStore(cfg.dbPath)
	handleErr(err, "storage init failed")
	defer func() {
		_ = historyStore.Close()
	}()

	watchManager := watch.NewManager(k8sClient, logger, cfg.interval)

	mcpServer, err := server.NewMCPServer(logger, server.Config{
		Version:      version,
		GitCommit:    gitCommit,
		BuildDate:    buildDate,
		K8sClient:    k8sClient,
		HistoryStore: historyStore,
		Watcher:      watchManager,
	})
	handleErr(err, "mcp server init failed")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	executeAction(ctx, mcpServer, cfg, logger)
}

type config struct {
	showVersion, showHelp, listTools, healthCheck, debug bool
	execTool, toolArgs, logFile, dbPath                  string
	interval                                             time.Duration
}

func parseFlags() config {
	c := config{}
	flag.BoolVar(&c.showVersion, "version", false, "")
	flag.BoolVar(&c.showHelp, "help", false, "")
	flag.BoolVar(&c.listTools, "list-tools", false, "")
	flag.StringVar(&c.execTool, "exec", "", "")
	flag.StringVar(&c.toolArgs, "args", "{}", "")
	flag.BoolVar(&c.healthCheck, "health", false, "")
	flag.BoolVar(&c.debug, "debug", false, "")
	flag.StringVar(&c.logFile, "log-file", "", "")
	flag.StringVar(&c.dbPath, "db-path", "~/.kube-watcher/history.db", "")
	flag.DurationVar(&c.interval, "interval", 30*time.Second, "")
	flag.Parse()
	return c
}

func executeAction(ctx context.Context, s *server.MCPServer, cfg config, logger any) {
	switch {
	case cfg.listTools:
		fmt.Printf("%+v\n", s.ListTools())
	case cfg.execTool != "":
		var args map[string]any
		if err := json.Unmarshal([]byte(cfg.toolArgs), &args); err != nil {
			os.Exit(1)
		}
		res, _ := s.ExecuteTool(ctx, cfg.execTool, args)
		fmt.Println(res)
	case cfg.healthCheck:
		if h := s.HealthCheck(ctx); h["status"] != "healthy" {
			os.Exit(1)
		}
	default:
		if err := s.Start(ctx); err != nil {
			os.Exit(1)
		}
	}
}

func handleErr(err error, msg string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
		os.Exit(1)
	}
}

func showUsage() {
	fmt.Printf("kube-watcher v%s\nusage: kube-watcher [options]\n", version)
}
