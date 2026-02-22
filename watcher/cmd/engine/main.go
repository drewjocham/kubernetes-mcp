package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/cel-go/cel"

	"kube-watcher/pkg/kube"
	"kube-watcher/pkg/logging"
	"kube-watcher/watcher/internal/actions"
	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/pipeline"
	"kube-watcher/watcher/internal/rules"
	"kube-watcher/watcher/internal/source"
	"kube-watcher/watcher/internal/tracker"
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
		health     bool
	)
	flag.StringVar(&configPath, "config", "", "path to config")
	flag.BoolVar(&debug, "debug", false, "enable debug")
	flag.StringVar(&logFile, "log-file", "", "path to log file")
	flag.StringVar(&httpAddr, "listen", ":8085", "http listen address")
	flag.BoolVar(&health, "health", false, "run health probe and exit")
	flag.Parse()

	logger, err := logging.New(debug, logFile)
	if err != nil {
		slog.Default().Error(errInitLogger.Error(), "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fatal(logger, errLoadConfig, err)
	}
	loadedPaths := config.ResolvedConfigPaths(configPath)

	if health {
		logger.Info("config loaded", "paths", loadedPaths)
		return
	}

	k8sClient, err := kube.NewClient(logger)
	if err != nil {
		fatal(logger, errInitClient, err)
	}

	store, err := initStore(cfg)
	if err != nil {
		fatal(logger, errInitStore, err)
	}
	defer store.Close()

	var celEnv *cel.Env
	if cfg.Settings.CEL.Enabled {
		if celEnv, err = cel.NewEnv(); err != nil {
			fatal(logger, errors.New("CEL init failed"), err)
		}
	}

	dispatcher, err := actions.NewDispatcher(logger, cfg.Actions, 64)
	if err != nil {
		fatal(logger, errDispatcher, err)
	}

	engine := rules.NewEngine(logger, cfg, store, celEnv)
	src := source.NewInformerSource(k8sClient.GetRawInterface(), logger, 30*time.Second)
	enricher := pipeline.NewPodEnricher(enrichmentFields(cfg))
	pipe := pipeline.New(
		logger,
		src,
		pipeline.NewRuleAwareFilter(cfg),
		enricher,
		engine,
		dispatcher,
		cfg.Settings.QueueDepth,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go serveHTTP(ctx, logger, httpAddr)

	logger.Info("event engine starting", "config_paths", loadedPaths)
	pipe.Start(ctx)
	logger.Info("event engine stopped")
}

func initStore(cfg *config.WatchConfig) (tracker.Store, error) {
	if cfg.ResourceTracking.Storage == "memory" {
		return tracker.NewMemoryStore(), nil
	}
	return tracker.NewBadgerStore(cfg.ResourceTracking.Path)
}

func fatal(l *slog.Logger, msg, err error) {
	l.Error(msg.Error(), "error", err)
	os.Exit(1)
}

func serveHTTP(ctx context.Context, logger *slog.Logger, addr string) {
	if addr == "" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", healthHandler)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server error", "error", err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func enrichmentFields(cfg *config.WatchConfig) []string {
	fields := make(map[string]struct{})

	for _, f := range cfg.ResourceTracking.Fields {
		if f != "" {
			fields[strings.ToLower(f)] = struct{}{}
		}
	}

	for _, rule := range cfg.Rules {
		for _, cond := range rule.Conditions {
			if cond.Field != "" {
				fields[strings.ToLower(cond.Field)] = struct{}{}
			}
		}
	}

	fields["restart_count"] = struct{}{}

	out := make([]string, 0, len(fields))
	for f := range fields {
		out = append(out, f)
	}
	return out
}
