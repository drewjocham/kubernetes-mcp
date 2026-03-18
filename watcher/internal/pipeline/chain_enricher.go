package pipeline

import (
	"context"

	"kube-watcher/watcher/internal/events"
)

type ChainEnricher struct {
	enrichers []Enricher
}

func NewChainEnricher(enrichers ...Enricher) *ChainEnricher {
	out := make([]Enricher, 0, len(enrichers))
	for _, enricher := range enrichers {
		if enricher == nil {
			continue
		}
		out = append(out, enricher)
	}
	return &ChainEnricher{enrichers: out}
}

func (c *ChainEnricher) Enrich(ctx context.Context, evt events.ResourceEvent) (events.ResourceEvent, error) {
	current := evt
	for _, enricher := range c.enrichers {
		next, err := enricher.Enrich(ctx, current)
		if err != nil {
			return evt, err
		}
		current = next
	}
	return current, nil
}
