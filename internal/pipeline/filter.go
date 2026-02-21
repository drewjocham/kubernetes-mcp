package pipeline

import (
	"context"
	"strings"

	"kube-watcher/internal/config"
	"kube-watcher/internal/events"
)

type RuleAwareFilter struct {
	cfg *config.WatchConfig
}

func NewRuleAwareFilter(cfg *config.WatchConfig) *RuleAwareFilter {
	return &RuleAwareFilter{cfg: cfg}
}

func (f *RuleAwareFilter) Allow(ctx context.Context, evt events.ResourceEvent) bool {
	for _, rule := range f.cfg.Rules {
		if rule.Namespace != "" && rule.Namespace != evt.Namespace {
			continue
		}
		if rule.Kind != "" && !strings.EqualFold(rule.Kind, evt.Kind) {
			continue
		}
		return true
	}
	return false
}
