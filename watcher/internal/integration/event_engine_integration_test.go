package integration

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"kube-watcher/watcher/internal/actions"
	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/pipeline"
	"kube-watcher/watcher/internal/rules"
	"kube-watcher/watcher/internal/tracker"
)

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type eventSource struct {
	events []events.ResourceEvent
}

func (e eventSource) Run(ctx context.Context, out chan<- events.ResourceEvent) {
	for _, evt := range e.events {
		select {
		case <-ctx.Done():
			return
		case out <- evt:
		}
	}
	<-ctx.Done()
}

func TestEventEngineIntegration(t *testing.T) {
	buf := &safeBuffer{}
	logger := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg := &config.WatchConfig{
		Rules: []config.Rule{
			{
				Name:    "high_restarts",
				Kind:    "Pod",
				Logic:   "all",
				Actions: []string{"log"},
				Conditions: []config.Condition{
					{Field: "restart_delta", Operator: "gt", Value: 0},
				},
			},
			{
				Name:    "node_pressure",
				Kind:    "Node",
				Actions: []string{"log"},
				Conditions: []config.Condition{
					{Field: "node_memory_pressure", Operator: "eq", Value: true},
				},
			},
		},
		Actions: map[string]config.Action{
			"log": {Type: "log", Template: "Alert: {{ .rule.Name }} on {{ .resource.metadata.name }}"},
		},
		Settings: config.Settings{QueueDepth: 10},
	}

	store := tracker.NewMemoryStore()
	defer func() {
		_ = store.Close()
	}()

	engine := rules.NewEngine(logger, cfg, store, nil)
	dispatcher, _ := actions.NewDispatcher(logger, cfg.Actions, 4)

	podEvent := events.ResourceEvent{
		Kind: "Pod", Namespace: "default", Name: "api",
		Object: map[string]interface{}{
			"metadata":      map[string]interface{}{"name": "api"},
			"restart_count": 5,
			"restart_delta": 1,
		},
	}

	nodeEvent := events.ResourceEvent{
		Kind: "Node", Name: "worker-01",
		Object: map[string]interface{}{
			"metadata":             map[string]interface{}{"name": "worker-01"},
			"node_memory_pressure": true,
		},
	}

	src := eventSource{events: []events.ResourceEvent{podEvent, nodeEvent}}
	filter := pipeline.NewRuleAwareFilter(cfg)

	pipe := pipeline.New(logger, src, filter, nil, engine, dispatcher,
		store, nil, cfg.Settings.QueueDepth, 2, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pipe.Start(ctx)

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer waitCancel()

	for {
		msg := buf.String()
		if strings.Contains(msg, "high_restarts") && strings.Contains(msg, "node_pressure") {
			break
		}
		select {
		case <-waitCtx.Done():
			t.Fatalf("timed out. Log output: %s", msg)
		default:
			time.Sleep(50 * time.Millisecond)
		}
	}
}
