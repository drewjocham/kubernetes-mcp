package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/cel-go/cel"

	"kube-watcher/internal/actions"
	"kube-watcher/internal/config"
	"kube-watcher/internal/logging"
	"kube-watcher/internal/pipeline"
	"kube-watcher/internal/rules"
	"kube-watcher/internal/source"
	"kube-watcher/internal/tracker"
	"kube-watcher/kubernetes"
)

var (
	errInitLogger = errors.New("event-engine: logger init failed")
	errInitClient = errors.New("event-engine: kubernetes client init failed")
	errLoadConfig = errors.New("event-engine: config load failed")
	errInitStore  = errors.New("event-engine: tracker store init failed")
	errDispatcher = errors.New("event-engine: dispatcher init failed")
)

func main() {
	var (
		configPath string
		debug      bool
		logFile    string
		httpAddr   string
	)
	flag.StringVar(&configPath, "config", "", "path to event engine config")
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.StringVar(&logFile, "log-file", "", "path to log file")
	flag.StringVar(&httpAddr, "listen", ":8085", "http listen address for health/metrics")
	flag.Parse()

	logger, err := logging.New(debug, logFile)
	if err != nil {
		slog.Default().Error(errInitLogger.Error(), "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error(errLoadConfig.Error(), "error", err)
		os.Exit(1)
	}

	k8sClient, err := kubernetes.NewClient(logger)
	if err != nil {
		logger.Error(errInitClient.Error(), "error", err)
		os.Exit(1)
	}

	store, err := initStore(cfg)
	if err != nil {
		logger.Error(errInitStore.Error(), "error", err)
		os.Exit(1)
	}
	defer store.Close()

	var celEnv *cel.Env
	if cfg.Settings.CEL.Enabled {
		celEnv, err = cel.NewEnv()
		if err != nil {
			logger.Error("failed to init CEL environment", "error", err)
			os.Exit(1)
		}
	}

	engine := rules.NewEngine(logger, cfg, store, celEnv)

	dispatcher, err := actions.NewDispatcher(logger, cfg.Actions, 64)
	if err != nil {
		logger.Error(errDispatcher.Error(), "error", err)
		os.Exit(1)
	}

	src := source.NewInformerSource(k8sClient.GetRawInterface(), logger, 30*time.Second)
	filter := pipeline.NewRuleAwareFilter(cfg)
	pipe := pipeline.New(logger, src, filter, nil, engine, dispatcher, cfg.Settings.QueueDepth)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go serveHTTP(ctx, logger, httpAddr)

	logger.Info("event engine starting", "config", configPath)
	pipe.Start(ctx)
	logger.Info("event engine stopped")
}

func initStore(cfg *config.WatchConfig) (tracker.Store, error) {
	switch cfg.ResourceTracking.Storage {
	case "memory":
		return tracker.NewMemoryStore(), nil
	case "badger":
		return tracker.NewBadgerStore(cfg.ResourceTracking.Path)
	default:
		return tracker.NewBadgerStore(cfg.ResourceTracking.Path)
	}
}

func serveHTTP(ctx context.Context, logger *slog.Logger, addr string) {
	if addr == "" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server error", "error", err)
	}
}
