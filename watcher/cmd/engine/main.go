package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"kube-watcher/pkg/kube"
	"kube-watcher/pkg/logging"
	"kube-watcher/watcher/internal/actions"
	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/monitoring/metrics"
	"kube-watcher/watcher/internal/pipeline"
	"kube-watcher/watcher/internal/rules"
	"kube-watcher/watcher/internal/source"
	"kube-watcher/watcher/internal/tracker"
)

type engineApp struct {
	cfg    *config.WatchConfig
	logger *slog.Logger
}

func main() {
	var (
		configPath string
		debug      bool
		logFile    string
		httpAddr   string
		healthOnly bool
	)

	flag.StringVar(&configPath, "config", "", "path to config")
	flag.BoolVar(&debug, "debug", false, "enable debug")
	flag.StringVar(&logFile, "log-file", "", "path to log file")
	flag.StringVar(&httpAddr, "listen", ":8085", "http listen address")
	flag.BoolVar(&healthOnly, "health", false, "run health probe and exit")
	flag.Parse()

	logger, err := logging.New(debug, logFile)
	if err != nil {
		slog.Default().Error("failed to initialize logger", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fatal(logger, "config load failed", err)
	}

	if healthOnly {
		logger.Info("config validation successful", "paths", config.ResolvedConfigPaths(configPath))
		return
	}

	app := &engineApp{cfg: cfg, logger: logger}
	if err := app.run(configPath, httpAddr); err != nil {
		fatal(logger, "application failed", err)
	}
}

func (a *engineApp) run(configPath, httpAddr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	k8sClient, err := kube.NewClient(a.logger)
	if err != nil {
		return fmt.Errorf("k8s client: %w", err)
	}

	store, err := a.initStore()
	if err != nil {
		return fmt.Errorf("store init: %w", err)
	}
	defer func() {
		_ = store.Close()
	}()

	celEnv, err := a.initCEL()
	if err != nil {
		return fmt.Errorf("cel init: %w", err)
	}

	dispatcher, err := actions.NewDispatcher(a.logger, a.cfg.Actions, 64)
	if err != nil {
		return fmt.Errorf("dispatcher: %w", err)
	}

	metricStore := tracker.NewMetricStore(time.Hour)
	go metricStore.CleanupLoop(ctx, 5*time.Minute)

	pipe := a.buildPipeline(k8sClient, store, metricStore, dispatcher, celEnv)

	a.startServer(ctx, "internal-api", httpAddr, a.apiMux())
	if a.cfg.Settings.Metrics.Enabled {
		a.startServer(ctx, "metrics", a.cfg.Settings.Metrics.Listen, a.metricsMux())
	}

	a.logger.Info("event engine starting",
		"config_paths", config.ResolvedConfigPaths(configPath))
	pipe.Start(ctx)
	a.logger.Info("event engine stopped")
	return nil
}

func (a *engineApp) initStore() (tracker.Store, error) {
	if a.cfg.ResourceTracking.Storage == "memory" {
		return tracker.NewMemoryStore(), nil
	}
	return tracker.NewBadgerStore(a.cfg.ResourceTracking.Path, a.cfg.ResourceTracking.Retention)
}

func (a *engineApp) initCEL() (*cel.Env, error) {
	if !a.cfg.Settings.CEL.Enabled {
		return nil, nil
	}
	return cel.NewEnv(
		cel.Variable("evt", cel.DynType),
		cel.Variable("meta", cel.DynType),
	)
}

func (a *engineApp) buildPipeline(client *kube.Client, st tracker.Store, ms *tracker.MetricStore, dp *actions.Dispatcher, env *cel.Env) *pipeline.Pipeline {
	engine := rules.NewEngine(a.logger, a.cfg, st, env)
	src := source.NewInformerSource(client.GetRawInterface(), a.logger, 30*time.Second)
	enricher := pipeline.NewPodEnricher(a.getEnrichmentFields())

	pipe := pipeline.New(
		a.logger,
		src,
		pipeline.NewRuleAwareFilter(a.cfg),
		enricher,
		engine,
		dp,
		st,
		ms,
		a.cfg.Settings.QueueDepth,
		a.cfg.Settings.QueueDepth,
		30,
	)

	if a.cfg.Settings.Metrics.Enabled {
		pipe.AddObserver(metrics.NewExporter())
	}

	return pipe
}

func (a *engineApp) apiMux() *http.ServeMux {
	mux := http.NewServeMux()
	h := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	mux.HandleFunc("/health", h)
	mux.HandleFunc("/ready", h)
	return mux
}

func (a *engineApp) metricsMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func (a *engineApp) startServer(ctx context.Context, name, addr string, handler http.Handler) {
	if addr == "" && name == "metrics" {
		addr = ":9095"
	}
	if addr == "" {
		return
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("server error", "name", name, "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		sCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(sCtx)
		a.logger.Debug("server shutdown complete", "name", name)
	}()
}

func (a *engineApp) getEnrichmentFields() []string {
	fieldSet := map[string]struct{}{"restart_count": {}}

	for _, f := range a.cfg.ResourceTracking.Fields {
		if f != "" {
			fieldSet[strings.ToLower(f)] = struct{}{}
		}
	}

	for _, rule := range a.cfg.Rules {
		for _, cond := range rule.Conditions {
			if cond.Field != "" {
				fieldSet[strings.ToLower(cond.Field)] = struct{}{}
			}
		}
	}

	out := make([]string, 0, len(fieldSet))
	for f := range fieldSet {
		out = append(out, f)
	}
	return out
}

func fatal(l *slog.Logger, msg string, err error) {
	l.Error(msg, "error", err)
	os.Exit(1)
}
