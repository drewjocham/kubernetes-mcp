package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"kube-watcher/integrations/chatbridge"
	"kube-watcher/pkg/logging"
)

func main() {
	configPath := flag.String("config", "integrations/chatbridge/config.example.yaml", "path to YAML config")
	debug := flag.Bool("debug", false, "enable debug logging")
	logFile := flag.String("log-file", "", "optional log file path")
	flag.Parse()

	logger, err := logging.New(*debug, *logFile)
	if err != nil {
		slog.Error("logger init failed", "error", err)
		os.Exit(1)
	}

	cfg, err := chatbridge.LoadConfig(*configPath)
	if err != nil {
		logger.Error("failed loading config", "error", err, "path", *configPath)
		os.Exit(1)
	}

	bridge, err := chatbridge.NewBridge(logger, cfg)
	if err != nil {
		logger.Error("failed creating bridge", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := bridge.Run(ctx); err != nil {
		logger.Error("bridge exited with error", "error", err)
		os.Exit(1)
	}
}
