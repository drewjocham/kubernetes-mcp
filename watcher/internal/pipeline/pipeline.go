package pipeline

import (
	"context"
	"log/slog"
	"runtime"
	"strings"

	"github.com/panjf2000/ants/v2"
	"k8s.io/apimachinery/pkg/api/resource"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/graph"
	"kube-watcher/watcher/internal/rules"
	"kube-watcher/watcher/internal/tracker"
)

type Source interface {
	Run(ctx context.Context, out chan<- events.ResourceEvent)
}

type Filter interface {
	Allow(ctx context.Context, evt events.ResourceEvent) bool
}

type Enricher interface {
	Enrich(ctx context.Context, evt events.ResourceEvent) (events.ResourceEvent, error)
}

type Dispatcher interface {
	Dispatch(ctx context.Context, inv rules.ActionInvocation) error
}

type RuleAwareFilter struct {
	cfg *config.WatchConfig
}

func NewRuleAwareFilter(cfg *config.WatchConfig) *RuleAwareFilter {
	return &RuleAwareFilter{cfg: cfg}
}

func (f *RuleAwareFilter) Allow(_ context.Context, evt events.ResourceEvent) bool {
	for _, r := range f.cfg.Rules {
		nsMatch := r.Namespace == "" || r.Namespace == evt.Namespace
		kindMatch := r.Kind == "" || strings.EqualFold(r.Kind, evt.Kind)
		if nsMatch && kindMatch {
			return true
		}
	}
	return false
}

type Pipeline struct {
	logger       *slog.Logger
	source       Source
	filter       Filter
	enricher     Enricher
	evaluator    *rules.Engine
	dispatcher   Dispatcher
	store        tracker.Store
	metrics      *tracker.MetricStore
	queue        chan events.ResourceEvent
	fields       []string
	deltaNames   map[string]string
	workers      int
	observers    []Observer
	historyLimit int
}

func New(logger *slog.Logger, source Source, filter Filter, enricher Enricher,
	evaluator *rules.Engine, dispatcher Dispatcher, store tracker.Store, metrics *tracker.MetricStore, depth int,
	workers int, historyLimit int) *Pipeline {
	if depth <= 0 {
		depth = 128
	}
	if workers <= 0 {
		workers = runtime.NumCPU()
		if workers < 1 {
			workers = 1
		}
	}
	if historyLimit <= 0 {
		historyLimit = 30
	}
	return &Pipeline{
		logger:     logger,
		source:     source,
		filter:     filter,
		enricher:   enricher,
		evaluator:  evaluator,
		dispatcher: dispatcher,
		store:      store,
		metrics:    metrics,
		queue:      make(chan events.ResourceEvent, depth),
		fields: []string{
			"restart_count",
			"current_pod_count",
			"desired_pod_count",
			"current_replicas",
			"desired_replicas",
		},
		deltaNames: map[string]string{
			"restart_count":    "restart_delta",
			"current_replicas": "current_replicas_delta",
			"desired_replicas": "desired_replicas_delta",
		},
		workers:      workers,
		historyLimit: historyLimit,
	}
}

func (p *Pipeline) Start(ctx context.Context) {
	go p.source.Run(ctx, p.queue)

	pool, err := ants.NewPoolWithFunc(p.workers, func(task interface{}) {
		evt, ok := task.(events.ResourceEvent)
		if !ok {
			return
		}
		p.processEvent(ctx, evt)
	})
	if err != nil {
		p.logger.Error("failed to start worker pool", "error", err)
		return
	}
	defer pool.Release()

	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-p.queue:
			if err := pool.Invoke(evt); err != nil {
				p.logger.Warn("worker pool invoke failed", "error", err)
			}
		}
	}
}

func (p *Pipeline) processEvent(ctx context.Context, evt events.ResourceEvent) {
	if p.filter != nil && !p.filter.Allow(ctx, evt) {
		return
	}

	if p.enricher != nil {
		enriched, err := p.enricher.Enrich(ctx, evt)
		if err != nil {
			p.logger.Warn("enrichment failed", "key", evt.Key(), "error", err)
			return
		}
		evt = enriched
	}
	p.recordMetrics(evt)
	p.notifyObservers(evt)

	actions, err := p.evaluator.Evaluate(ctx, evt)
	if err != nil {
		p.logger.Warn("evaluation failed", "key", evt.Key(), "error", err)
		return
	}

	for i := range actions {
		act := p.maybeAttachGraph(evt, actions[i])
		if err := p.dispatcher.Dispatch(ctx, act); err != nil {
			p.logger.Error("dispatch failed", "rule", act.RuleName, "action", act.ActionID, "error", err)
		}
	}
}

func (p *Pipeline) recordMetrics(evt events.ResourceEvent) {
	if p.metrics == nil || evt.Object == nil {
		return
	}
	for _, field := range p.fields {
		val, ok := extractNumeric(evt.Object[field])
		if !ok {
			continue
		}
		delta := p.metrics.Observe(evt.Key(), field, val)
		if !delta.HasPrevious {
			continue
		}
		deltaField := field + "_delta"
		if name, ok := p.deltaNames[field]; ok {
			deltaField = name
		}
		rateField := deltaField + "_rate_per_sec"
		evt.Object[deltaField] = delta.Delta
		evt.Object[rateField] = delta.RatePerSec
	}
}

func extractNumeric(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		q, err := resource.ParseQuantity(val)
		if err != nil {
			return 0, false
		}
		return q.AsApproximateFloat64(), true
	}
	return 0, false
}

func (p *Pipeline) maybeAttachGraph(evt events.ResourceEvent, inv rules.ActionInvocation) rules.ActionInvocation {
	if p.store == nil || inv.Action.Config == nil {
		return inv
	}
	if !strings.EqualFold(inv.Action.Config["attach_graph"], "true") {
		return inv
	}
	field := inv.Action.Config["graph_field"]
	if field == "" {
		field = "restart_count"
	}
	history := p.store.History(evt.Key(), p.historyLimit)
	if len(history) == 0 {
		return inv
	}
	img, err := graph.Render(field, history)
	if err != nil {
		p.logger.Warn("graph render failed", "rule", inv.RuleName, "field", field, "error", err)
		return inv
	}
	if inv.Context == nil {
		inv.Context = make(map[string]interface{})
	}
	inv.Context["graph_image"] = img
	inv.Context["graph_field"] = field
	return inv
}

func (p *Pipeline) AddObserver(obs Observer) {
	if obs == nil {
		return
	}
	p.observers = append(p.observers, obs)
}

func (p *Pipeline) notifyObservers(evt events.ResourceEvent) {
	for _, obs := range p.observers {
		obs.Observe(evt)
	}
}
