package rules

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/tracker"
)

type stubStore struct {
	data map[string]tracker.Snapshot
}

func TestEvaluateRuleMatches(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	store := newStubStore()
	cfg := &config.WatchConfig{
		Rules: []config.Rule{
			{
				Name:    "phase_failed",
				Kind:    "Pod",
				Logic:   "all",
				Actions: []string{"log"},
				Conditions: []config.Condition{
					{Field: "status.phase", Operator: "eq", Value: "Failed"},
				},
			},
		},
		Actions: map[string]config.Action{
			"log": {Type: "log", Template: "tmpl"},
		},
	}

	engine := NewEngine(logger, cfg, store, nil)
	rule := cfg.Rules[0]
	evt := events.ResourceEvent{
		Kind:            "Pod",
		Namespace:       "default",
		Name:            "api",
		ResourceVersion: "1",
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":            "api",
				"resourceVersion": "1",
			},
			"status": map[string]interface{}{
				"phase": "Failed",
			},
		},
	}

	for i, cond := range rule.Conditions {
		okCond, _, err := engine.evaluateCondition(cond, evt, tracker.Snapshot{})
		if err != nil {
			t.Fatalf("condition %d error: %v", i, err)
		}
		if !okCond {
			t.Fatalf("condition %d not satisfied", i)
		}
	}

	ok, err := engine.evaluateRule(rule, evt, tracker.Snapshot{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("evaluateRule error: %v", err)
	}
	if !ok {
		t.Fatalf("expected rule to match event")
	}
}

func TestExtractValue(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"phase": "Failed",
		},
	}
	val, err := extractValue(obj, "status.phase")
	if err != nil {
		t.Fatalf("extractValue error: %v", err)
	}
	if val.(string) != "Failed" {
		t.Fatalf("unexpected value %v", val)
	}
}

func newStubStore() *stubStore {
	return &stubStore{data: make(map[string]tracker.Snapshot)}
}

func (s *stubStore) Get(key string) (tracker.Snapshot, bool) {
	val, ok := s.data[key]
	return val, ok
}

func (s *stubStore) Set(key string, snap tracker.Snapshot) {
	s.data[key] = snap
}

func (s *stubStore) Close() error { return nil }

func TestEngineEvaluate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	store := newStubStore()
	cfg := &config.WatchConfig{
		Rules: []config.Rule{
			{
				Name:    "phase_failed",
				Kind:    "Pod",
				Logic:   "all",
				For:     0,
				Actions: []string{"log"},
				Conditions: []config.Condition{
					{Field: "status.phase", Operator: "eq", Value: "Failed"},
				},
			},
			{
				Name:    "status_changed",
				Kind:    "Pod",
				Logic:   "all",
				For:     0,
				Actions: []string{"log"},
				Conditions: []config.Condition{
					{Field: "metadata.resourceVersion", Operator: "changed"},
				},
			},
		},
		Actions: map[string]config.Action{
			"log": {Type: "log", Template: "{{ .resource.metadata.name }}"},
		},
	}

	engine := NewEngine(logger, cfg, store, nil)

	eventsTable := []struct {
		name       string
		event      events.ResourceEvent
		wantCount  int
		shouldSave bool
	}{
		{
			name: "matches_failed_phase",
			event: events.ResourceEvent{
				Kind:            "Pod",
				Namespace:       "default",
				Name:            "api",
				ResourceVersion: "10",
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":            "api",
						"resourceVersion": "10",
					},
					"status": map[string]interface{}{
						"phase": "Failed",
					},
				},
			},
			wantCount:  2,
			shouldSave: true,
		},
		{
			name: "changed_condition_detected",
			event: events.ResourceEvent{
				Kind:            "Pod",
				Namespace:       "default",
				Name:            "api",
				ResourceVersion: "11",
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":            "api",
						"resourceVersion": "11",
					},
					"status": map[string]interface{}{
						"phase": "Running",
					},
				},
			},
			wantCount: 1,
		},
	}

	ctx := context.Background()
	for _, tc := range eventsTable {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			invocations, err := engine.Evaluate(ctx, tc.event)
			if err != nil {
				t.Fatalf("Evaluate returned error: %v", err)
			}
			if len(invocations) != tc.wantCount {
				t.Fatalf("expected %d actions, got %d", tc.wantCount, len(invocations))
			}
			if tc.shouldSave {
				if _, ok := store.data[tc.event.Key()]; !ok {
					t.Fatalf("expected snapshot stored for %s", tc.event.Key())
				}
			}
		})
	}
}

func TestEngineForDuration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	store := newStubStore()
	cfg := &config.WatchConfig{
		Rules: []config.Rule{
			{
				Name:    "damped",
				Kind:    "Pod",
				Logic:   "all",
				For:     50 * time.Millisecond,
				Actions: []string{"alert"},
				Conditions: []config.Condition{
					{Field: "status.phase", Operator: "eq", Value: "Pending"},
				},
			},
		},
		Actions: map[string]config.Action{
			"alert": {Type: "log", Template: "pending"},
		},
	}
	engine := NewEngine(logger, cfg, store, nil)

	event := events.ResourceEvent{
		Kind:            "Pod",
		Namespace:       "ns",
		Name:            "worker",
		ResourceVersion: "1",
		Object: map[string]interface{}{
			"status": map[string]interface{}{
				"phase": "Pending",
			},
		},
	}

	ctx := context.Background()
	if inv, _ := engine.Evaluate(ctx, event); len(inv) != 0 {
		t.Fatalf("expected no actions before duration window")
	}
	time.Sleep(60 * time.Millisecond)
	if inv, _ := engine.Evaluate(ctx, event); len(inv) != 1 {
		t.Fatalf("expected action after duration window")
	}
}
