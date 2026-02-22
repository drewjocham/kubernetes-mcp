package pipeline

import (
	"context"
	"log/slog"
	"strings"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/rules"
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
	logger     *slog.Logger
	source     Source
	filter     Filter
	enricher   Enricher
	evaluator  *rules.Engine
	dispatcher Dispatcher
	queue      chan events.ResourceEvent
}

func New(logger *slog.Logger, source Source, filter Filter, enricher Enricher,
	evaluator *rules.Engine, dispatcher Dispatcher, depth int) *Pipeline {
	if depth <= 0 {
		depth = 128
	}
	return &Pipeline{
		logger:     logger,
		source:     source,
		filter:     filter,
		enricher:   enricher,
		evaluator:  evaluator,
		dispatcher: dispatcher,
		queue:      make(chan events.ResourceEvent, depth),
	}
}

func (p *Pipeline) Start(ctx context.Context) {
	go p.source.Run(ctx, p.queue)
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-p.queue:
			p.handle(ctx, evt)
		}
	}
}

func (p *Pipeline) handle(ctx context.Context, evt events.ResourceEvent) {
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

	actions, err := p.evaluator.Evaluate(ctx, evt)
	if err != nil {
		p.logger.Warn("evaluation failed", "key", evt.Key(), "error", err)
		return
	}

	for _, act := range actions {
		if err := p.dispatcher.Dispatch(ctx, act); err != nil {
			p.logger.Error("dispatch failed", "rule", act.RuleName, "action", act.ActionID, "error", err)
		}
	}
}
