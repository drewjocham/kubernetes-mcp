package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"kube-watcher/pkg/audit"
	"kube-watcher/pkg/kube"
	"kube-watcher/pkg/logging"
	"kube-watcher/watcher/internal/actions"
	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/monitoring/metrics"
	"kube-watcher/watcher/internal/pipeline"
	"kube-watcher/watcher/internal/rules"
	"kube-watcher/watcher/internal/security"
	"kube-watcher/watcher/internal/source"
	"kube-watcher/watcher/internal/tracker"
)

type engineApp struct {
	cfg          *config.WatchConfig
	logger       *slog.Logger
	store        tracker.Store
	sanitizer    *security.Sanitizer
	listeners    map[chan string]struct{}
	listenersMu  sync.RWMutex
	rulesMu      sync.RWMutex
	dynamicRules []config.Rule
	k8s          kube.ClientInterface
}

func (a *engineApp) handlePostRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var rule config.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	if rule.Name == "" {
		http.Error(w, "rule name is required", http.StatusBadRequest)
		return
	}
	a.logger.Info("adding dynamic rule", "name", rule.Name)
	a.rulesMu.Lock()
	defer a.rulesMu.Unlock()
	a.dynamicRules = append(a.dynamicRules, rule)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rule)
}
func (a *engineApp) Close() error {
	var errs []error

	a.listenersMu.Lock()
	for ch := range a.listeners {
		close(ch)
		delete(a.listeners, ch)
	}
	a.listenersMu.Unlock()

	if a.store != nil {
		if err := a.store.Close(); err != nil {
			errs = append(errs, fmt.Errorf("store close: %w", err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	a.logger.Info("engine shutdown complete")
	return nil
}

func main() {
	fs := flag.NewFlagSet("watcher", flag.ExitOnError)
	var (
		cfgPath  = fs.String("config", "", "")
		debug    = fs.Bool("debug", false, "")
		logFile  = fs.String("log-file", "", "")
		httpAddr = fs.String("listen", ":8085", "")
		health   = fs.Bool("health", false, "")
	)
	if err := fs.Parse(os.Args[1:]); err != nil {
		slog.Error("failed to parse flags", "err", err)
		os.Exit(1)
	}

	if err := run(*cfgPath, *debug, *logFile, *httpAddr, *health); err != nil {
		slog.Error("application failed", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath string, debug bool, logFile, addr string, healthOnly bool) error {
	logger, _ := logging.New(debug, logFile)
	defer logging.Shutdown()

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if healthOnly {
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	base, err := kube.NewClient(logger)
	if err != nil {
		return err
	}

	app := &engineApp{
		cfg:       cfg,
		logger:    logger,
		sanitizer: security.NewSanitizer(true),
		listeners: make(map[chan string]struct{}),
		k8s:       kube.NewAuditClient(base, audit.NewSlogLogger(logger), logger, kube.AuditOptionsFromEnv()...),
	}

	return app.start(ctx, addr)
}

func (a *engineApp) start(ctx context.Context, addr string) error {
	store, err := a.setupStore()
	if err != nil {
		return err
	}
	a.store = store
	defer store.Close()

	env, _ := cel.NewEnv(
		cel.Variable("evt", cel.DynType),
		cel.Variable("meta", cel.DynType),
		cel.Variable("kind", cel.StringType),
		cel.Variable("ns", cel.StringType),
		cel.Variable("name", cel.StringType),
	)

	disp, _ := actions.NewDispatcher(a.logger, a.cfg.Actions, 64)
	mStore := tracker.NewMetricStore(time.Hour)
	go mStore.CleanupLoop(ctx, 5*time.Minute)

	if a.cfg.Settings.Heartbeat.Enabled {
		go a.heartbeatLoop(ctx)
	}

	pipe := a.buildPipeline(env, disp, mStore)
	defer pipe.Close()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { pipe.Start(gctx); return nil })
	g.Go(func() error { return a.serve(gctx, "api", addr, a.apiMux()) })

	if a.cfg.Settings.Metrics.Enabled {
		g.Go(func() error { return a.serve(gctx, "metrics", a.cfg.Settings.Metrics.Listen, a.metricsMux()) })
	}

	return g.Wait()
}

func (a *engineApp) buildPipeline(env *cel.Env, dp *actions.Dispatcher, ms *tracker.MetricStore) *pipeline.Pipeline {
	srcs := []pipeline.Source{source.NewInformerSource(a.k8s.GetRawInterface(), a.logger, 30*time.Second)}
	if a.cfg.Settings.Anomstack.Enabled {
		srcs = append(srcs, source.NewAnomstackSource(a.logger, a.cfg.Settings.Anomstack))
	}
	if a.cfg.Settings.PubSub.Enabled {
		a.logger.Warn("PubSub source not implemented")
		// srcs = append(srcs, source.NewPubSubSource(a.logger, a.cfg.Settings.PubSub))
	}

	enricher := pipeline.NewPodEnricher(getEnrichmentFields(a.cfg))
	pipe := pipeline.New(a.logger, source.NewMultiSource(a.logger, srcs...),
		pipeline.NewRuleAwareFilter(a.cfg), enricher, rules.NewEngine(a.logger, a.cfg, a.store, env),
		dp, a.store, ms, a.cfg.Settings.QueueDepth, a.cfg.Settings.QueueDepth, 30)

	pipe.AddObserver(a)
	if a.cfg.Settings.Metrics.Enabled {
		pipe.AddObserver(metrics.NewExporter())
	}
	return pipe
}

func (a *engineApp) serve(ctx context.Context, name, addr string, h http.Handler) error {
	if addr == "" && name == "metrics" {
		addr = ":9095"
	}
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		sCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(sCtx); err != nil {
			a.logger.Warn("server shutdown error", "name", name, "error", err)
		}
	}()
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("%s server: %w", name, err)
	}
	return nil
}

func (a *engineApp) apiMux() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	m.HandleFunc("GET /api/config", a.jsonHandler(func() interface{} { return a.cfg }))
	m.HandleFunc("/api/rules", func(w http.ResponseWriter, r *http.Request) {
		a.logger.Warn("api/rules request", "method", r.Method, "path", r.URL.Path)
		switch r.Method {
		case "GET":
			a.rulesMu.RLock()
			defer a.rulesMu.RUnlock()
			allRules := make([]config.Rule, 0, len(a.cfg.Rules)+len(a.dynamicRules))
			allRules = append(allRules, a.cfg.Rules...)
			allRules = append(allRules, a.dynamicRules...)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(allRules)
		case "POST":
			a.handlePostRule(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	m.HandleFunc("GET /api/resources", a.jsonHandler(func() interface{} { return a.store.List() }))
	m.HandleFunc("GET /api/status", a.jsonHandler(func() interface{} {
		return map[string]interface{}{
			"ready":   true,
			"store":   a.store != nil,
			"config":  a.cfg != nil,
			"started": time.Now().UTC().Format(time.RFC3339),
		}
	}))
	m.HandleFunc("GET /api/logs/stream", a.handleLogStream)
	return m
}

func (a *engineApp) jsonHandler(provider func() interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(provider())
	}
}

func (a *engineApp) handleLogStream(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 100)
	a.addListener(ch)
	defer a.removeListener(ch)

	for {
		select {
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (a *engineApp) Observe(evt events.ResourceEvent) {
	var data interface{} = evt
	if a.sanitizer != nil {
		data = a.sanitizer.SanitizeEvent(&evt)
	}
	b, _ := json.Marshal(data)
	a.broadcast(string(b))
}

func (a *engineApp) broadcast(msg string) {
	a.listenersMu.RLock()
	defer a.listenersMu.RUnlock()
	for ch := range a.listeners {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (a *engineApp) addListener(ch chan string) {
	a.listenersMu.Lock()
	defer a.listenersMu.Unlock()
	a.listeners[ch] = struct{}{}
}
func (a *engineApp) removeListener(ch chan string) {
	a.listenersMu.Lock()
	defer a.listenersMu.Unlock()
	delete(a.listeners, ch)
}

func (a *engineApp) setupStore() (tracker.Store, error) {
	if a.cfg.ResourceTracking.Storage == "memory" {
		return tracker.NewMemoryStore(), nil
	}
	return tracker.NewBadgerStore(a.cfg.ResourceTracking.Path, a.cfg.ResourceTracking.Retention)
}

func (a *engineApp) heartbeatLoop(ctx context.Context) {
	h := a.cfg.Settings.Heartbeat
	t := time.NewTicker(h.Interval)
	defer t.Stop()
	url := fmt.Sprintf("%s/api/clusters/%s/heartbeat", h.DashboardURL, h.ClusterName)

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			payload, err := json.Marshal(map[string]interface{}{"cluster": h.ClusterName, "status": "active", "timestamp": time.Now().UTC()})
			if err != nil {
				a.logger.Warn("heartbeat json marshal error", "error", err)
				continue
			}
			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
			if err != nil {
				a.logger.Warn("heartbeat request creation error", "error", err)
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				a.logger.Warn("heartbeat request failed", "error", err)
				continue
			}
			defer resp.Body.Close()
			if _, err := io.Copy(io.Discard, resp.Body); err != nil {
				a.logger.Warn("heartbeat response drain error", "error", err)
			}
		}
	}
}

func getEnrichmentFields(cfg *config.WatchConfig) []string {
	unique := map[string]struct{}{"restart_count": {}}
	for _, f := range cfg.ResourceTracking.Fields {
		unique[strings.ToLower(f)] = struct{}{}
	}
	for _, r := range cfg.Rules {
		for _, c := range r.Conditions {
			unique[strings.ToLower(c.Field)] = struct{}{}
		}
	}
	var out []string
	for k := range unique {
		if k != "" {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func (a *engineApp) metricsMux() *http.ServeMux {
	m := http.NewServeMux()
	m.Handle("/metrics", promhttp.Handler())
	return m
}
