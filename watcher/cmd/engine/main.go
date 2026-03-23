package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sort"
	"strings"
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

var (
	evtDotFieldPattern   = regexp.MustCompile(`\bevt\.([a-zA-Z_][a-zA-Z0-9_]*)`)
	evtIndexFieldPattern = regexp.MustCompile(`\bevt\[['"]([^'"]+)['"]\]`)
)

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

	if err := runApplication(configPath, debug, logFile, httpAddr, healthOnly); err != nil {
		slog.Default().Error("application failed", "error", err)
		os.Exit(1)
	}
}

func runApplication(configPath string, debug bool, logFile string, httpAddr string, healthOnly bool) error {
	logger, err := logging.New(debug, logFile)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	if healthOnly {
		logger.Info("config validation successful", "paths", config.ResolvedConfigPaths(configPath))
		return nil
	}

	app := &engineApp{cfg: cfg, logger: logger}
	return app.run(configPath, httpAddr)
}

func (a *engineApp) run(configPath, httpAddr string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := snapshotWatchConfig(a.cfg)

	// Create base Kubernetes client
	baseClient, err := kube.NewClient(a.logger)
	if err != nil {
		return fmt.Errorf("k8s client: %w", err)
	}

	// Wrap with audit logging
	auditLogger := audit.NewSlogLogger(a.logger)
	k8sClient := kube.NewAuditClient(baseClient, auditLogger, a.logger, kube.AuditOptionsFromEnv()...)

	store, err := a.initStore(cfg)
	if err != nil {
		return fmt.Errorf("store init: %w", err)
	}
	defer func() {
		_ = store.Close()
	}()

	celEnv, err := a.initCEL(cfg)
	if err != nil {
		return fmt.Errorf("cel init: %w", err)
	}
	if err := a.validateCELRules(cfg, celEnv); err != nil {
		return fmt.Errorf("cel rule validation: %w", err)
	}

	dispatcher, err := actions.NewDispatcher(a.logger, cfg.Actions, 64)
	if err != nil {
		return fmt.Errorf("dispatcher: %w", err)
	}

	metricStore := tracker.NewMetricStore(time.Hour)
	go metricStore.CleanupLoop(ctx, 5*time.Minute)

	if cfg.Settings.Heartbeat.Enabled {
		go a.startHeartbeatLoop(ctx, cfg.Settings.Heartbeat)
	}

	pipe := a.buildPipeline(cfg, k8sClient, store, metricStore, dispatcher, celEnv)
	defer func() {
		if err := pipe.Close(); err != nil {
			a.logger.Warn("pipeline close failed", "error", err)
		}
	}()

	a.logger.Info("event engine starting", "config_paths", config.ResolvedConfigPaths(configPath))

	eg, gctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		pipe.Start(gctx)
		return nil
	})
	eg.Go(func() error {
		return a.startServer(gctx, "internal-api", httpAddr, a.apiMux())
	})
	if cfg.Settings.Metrics.Enabled {
		metricsAddr := cfg.Settings.Metrics.Listen
		eg.Go(func() error {
			return a.startServer(gctx, "metrics", metricsAddr, a.metricsMux())
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	a.logger.Info("event engine stopped")
	return nil
}

func (a *engineApp) initStore(cfg *config.WatchConfig) (tracker.Store, error) {
	if cfg.ResourceTracking.Storage == "memory" {
		return tracker.NewMemoryStore(), nil
	}
	return tracker.NewBadgerStore(cfg.ResourceTracking.Path, cfg.ResourceTracking.Retention)
}

func (a *engineApp) initCEL(cfg *config.WatchConfig) (*cel.Env, error) {
	if !cfg.Settings.CEL.Enabled {
		return nil, nil
	}
	return cel.NewEnv(
		cel.Variable("evt", cel.DynType),
		cel.Variable("meta", cel.DynType),
		cel.Variable("kind", cel.StringType),
		cel.Variable("ns", cel.StringType),
		cel.Variable("name", cel.StringType),
	)
}

func (a *engineApp) buildPipeline(cfg *config.WatchConfig, client kube.ClientInterface, st tracker.Store, ms *tracker.MetricStore, dp *actions.Dispatcher, env *cel.Env) *pipeline.Pipeline {
	engine := rules.NewEngine(a.logger, cfg, st, env)
	src := source.NewInformerSource(client.GetRawInterface(), a.logger, 30*time.Second)
	podEnricher := pipeline.NewPodEnricher(getEnrichmentFields(cfg.ResourceTracking.Fields, cfg.Rules))

	enricher := pipeline.Enricher(podEnricher)
	if cfg.Settings.Model.Enabled {
		enricher = pipeline.NewChainEnricher(
			podEnricher,
			pipeline.NewModelEnricher(a.logger, cfg.Settings.Model),
		)
	}

	pipe := pipeline.New(
		a.logger,
		src,
		pipeline.NewRuleAwareFilter(cfg),
		enricher,
		engine,
		dp,
		st,
		ms,
		cfg.Settings.QueueDepth,
		cfg.Settings.QueueDepth,
		30,
	)
	if cfg.Settings.Metrics.Enabled {
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

func (a *engineApp) startServer(ctx context.Context, name, addr string, handler http.Handler) error {
	if addr == "" && name == "metrics" {
		addr = ":9095"
	}
	if addr == "" {
		return nil
	}

	a.logger.Info("starting server", "name", name, "addr", addr)

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("%s server failed: %w", name, err)
		}
		return nil
	case <-ctx.Done():
		sCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(sCtx); err != nil && !errors.Is(err, context.Canceled) {
			a.logger.Warn("server shutdown error", "name", name, "error", err)
		}
		if err := <-errCh; err != nil {
			return fmt.Errorf("%s server failed during shutdown: %w", name, err)
		}
		a.logger.Debug("server shutdown complete", "name", name)
		return nil
	}
}

func getEnrichmentFields(resourceFields []string, rulesCfg []config.Rule) []string {
	fieldSet := map[string]struct{}{"restart_count": {}}

	for _, f := range resourceFields {
		if f != "" {
			fieldSet[strings.ToLower(f)] = struct{}{}
		}
	}
	for _, rule := range rulesCfg {
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
	sort.Strings(out)
	return out
}

func (a *engineApp) validateCELRules(cfg *config.WatchConfig, env *cel.Env) error {
	if env == nil {
		return nil
	}
	for _, rule := range cfg.Rules {
		expr := strings.TrimSpace(rule.Expression)
		if expr == "" {
			continue
		}

		ast, issues := env.Compile(expr)
		if issues != nil && issues.Err() != nil {
			return fmt.Errorf("rule %q has invalid expression %q: %w", rule.Name, expr, issues.Err())
		}
		if !ast.OutputType().IsExactType(cel.BoolType) {
			return fmt.Errorf("rule %q expression must return bool, got %s", rule.Name, ast.OutputType())
		}
		allowed := knownCELFieldsForKind(cfg, rule.Kind)

		for _, field := range referencedEvtFields(expr) {
			if _, ok := allowed[strings.ToLower(field)]; !ok {
				return fmt.Errorf("rule %q references unknown evt field %q for kind %q", rule.Name, field, rule.Kind)
			}
		}

		a.logger.Debug("validated CEL rule", "rule", rule.Name)
	}

	return nil
}
func knownCELFieldsForKind(cfg *config.WatchConfig, kind string) map[string]struct{} {
	kindKey := strings.ToLower(strings.TrimSpace(kind))
	fields := map[string]struct{}{
		"metadata":               {},
		"spec":                   {},
		"status":                 {},
		"kind":                   {},
		"apiversion":             {},
		"model_issue_detected":   {},
		"model_issue_confidence": {},
		"model_checked_at":       {},
		"model_issue_severity":   {},
		"model_issue_summary":    {},
		"model_issue_signals":    {},
		"model_analysis_error":   {},
	}

	add := func(keys ...string) {
		for _, k := range keys {
			fields[strings.ToLower(k)] = struct{}{}
		}
	}
	podFields := []string{
		"restart_count",
		"restart_delta",
		"cpu_request",
		"memory_request",
		"ram",
		"cpu_limit",
		"memory_limit",
		"cpu_limit_gap",
		"mem_limit_gap",
		"cpu_usage_ratio",
		"mem_usage_ratio",
		"replicas",
		"crash_looping",
		"crash_reason",
		"containers_not_ready",
		"container_ready_ratio",
		"oom_killed",
		"waiting_reason",
		"waiting_reasons",
		"waiting_message",
		"is_ready",
		"is_terminating",
	}
	hpaFields := []string{
		"min_pod_count",
		"max_pod_count",
		"current_pod_count",
		"desired_pod_count",
		"current_replicas",
		"desired_replicas",
		"current_replicas_delta",
		"desired_replicas_delta",
		"hpa_at_max_capacity",
		"hpa_at_min_capacity",
		"hpa_saturation_ratio",
		"hpa_is_stalled",
	}
	nodeFields := []string{
		"node_memory_pressure",
		"node_disk_pressure",
		"node_pid_pressure",
		"node_ready",
		"cpu_allocatable_m",
		"mem_allocatable_mi",
		"cpu_capacity_m",
		"mem_capacity_mi",
	}

	switch kindKey {
	case "pod":
		add(podFields...)
	case "horizontalpodautoscaler":
		add(hpaFields...)
	case "node":
		add(nodeFields...)
	case "":
		add(podFields...)
		add(hpaFields...)
		add(nodeFields...)
	default:
		// unknown kind: keep common fields plus explicit config/rule fields below
	}

	for _, f := range cfg.ResourceTracking.Fields {
		if trimmed := strings.TrimSpace(strings.ToLower(f)); trimmed != "" {
			fields[trimmed] = struct{}{}
		}
	}
	for _, r := range cfg.Rules {
		if kindKey != "" && strings.ToLower(strings.TrimSpace(r.Kind)) != kindKey {
			continue
		}
		for _, c := range r.Conditions {
			if trimmed := strings.TrimSpace(strings.ToLower(c.Field)); trimmed != "" {
				fields[trimmed] = struct{}{}
			}
		}
	}
	return fields
}

func referencedEvtFields(expr string) []string {
	seen := make(map[string]struct{})
	var out []string

	for _, m := range evtDotFieldPattern.FindAllStringSubmatch(expr, -1) {
		if len(m) < 2 {
			continue
		}
		field := m[1]
		key := strings.ToLower(field)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, field)
	}

	for _, m := range evtIndexFieldPattern.FindAllStringSubmatch(expr, -1) {
		if len(m) < 2 {
			continue
		}
		field := m[1]
		key := strings.ToLower(field)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, field)
	}

	return out
}

func (a *engineApp) startHeartbeatLoop(ctx context.Context, cfg config.HeartbeatConfig) {
	if !cfg.Enabled || cfg.DashboardURL == "" || cfg.ClusterName == "" {
		a.logger.Debug("heartbeat disabled or missing configuration")
		return
	}
	endpoint := fmt.Sprintf("%s/api/clusters/%s/heartbeat", cfg.DashboardURL, cfg.ClusterName)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()

	a.logger.Info("heartbeat loop started", "endpoint", endpoint, "interval", cfg.Interval)
	for {
		select {
		case <-ctx.Done():
			a.logger.Debug("heartbeat loop stopping")
			return
		case <-ticker.C:
			go a.sendHeartbeat(endpoint, cfg.ClusterName)
		}
	}
}

func (a *engineApp) sendHeartbeat(endpoint, clusterName string) {
	payload := map[string]interface{}{
		"cluster":   clusterName,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"status":    "active",
	}
	data, err := json.Marshal(payload)
	if err != nil {
		a.logger.Error("failed to marshal heartbeat", "error", err)
		return
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(data))
	if err != nil {
		a.logger.Error("failed to create heartbeat request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		a.logger.Warn("heartbeat request failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		a.logger.Warn("heartbeat received non-2xx response", "status", resp.StatusCode)
		return
	}
	a.logger.Debug("heartbeat sent successfully")
}

func snapshotWatchConfig(in *config.WatchConfig) *config.WatchConfig {
	if in == nil {
		return &config.WatchConfig{}
	}

	out := *in
	out.ResourceTracking.Fields = append([]string(nil), in.ResourceTracking.Fields...)

	out.Rules = make([]config.Rule, 0, len(in.Rules))
	for _, r := range in.Rules {
		rc := r
		rc.Actions = append([]string(nil), r.Actions...)
		rc.Conditions = append([]config.Condition(nil), r.Conditions...)
		if r.Selector.MatchLabels != nil {
			labels := make(map[string]string, len(r.Selector.MatchLabels))
			for k, v := range r.Selector.MatchLabels {
				labels[k] = v
			}
			rc.Selector.MatchLabels = labels
		}
		out.Rules = append(out.Rules, rc)
	}

	out.Actions = make(map[string]config.Action, len(in.Actions))
	for id, act := range in.Actions {
		ac := act
		if act.Config != nil {
			cfgCopy := make(map[string]string, len(act.Config))
			for k, v := range act.Config {
				cfgCopy[k] = v
			}
			ac.Config = cfgCopy
		}
		out.Actions[id] = ac
	}

	return &out
}
