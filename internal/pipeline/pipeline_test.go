package pipeline

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"kube-watcher/internal/config"
	"kube-watcher/internal/events"
	"kube-watcher/internal/rules"
	"kube-watcher/internal/tracker"
)

type fakeSource struct {
	events []events.ResourceEvent
}

func (f fakeSource) Run(ctx context.Context, out chan<- events.ResourceEvent) {
	for _, evt := range f.events {
		select {
		case <-ctx.Done():
			return
		case out <- evt:
		}
	}
	<-ctx.Done()
}

type recordingDispatcher struct {
	mu          sync.Mutex
	invocations []rules.ActionInvocation
	signal      chan struct{}
}

func (r *recordingDispatcher) Dispatch(ctx context.Context, inv rules.ActionInvocation) error {
	r.mu.Lock()
	r.invocations = append(r.invocations, inv)
	r.mu.Unlock()
	select {
	case r.signal <- struct{}{}:
	default:
	}
	return nil
}

func TestPipelineIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	cfg := &config.WatchConfig{
		Rules: []config.Rule{
			{
				Name:    "pod_failure",
				Kind:    "Pod",
				Logic:   "all",
				For:     0,
				Actions: []string{"log"},
				Conditions: []config.Condition{
					{Field: "status.phase", Operator: "eq", Value: "CrashLoopBackOff"},
				},
			},
		},
		Actions: map[string]config.Action{
			"log": {Type: "log", Template: "{{ .resource.metadata.name }}"},
		},
		Settings: config.Settings{QueueDepth: 4},
	}

	store := tracker.NewMemoryStore()
	defer store.Close()
	engine := rules.NewEngine(logger, cfg, store, nil)

	dispatcher := &recordingDispatcher{signal: make(chan struct{}, 1)}

	event := events.ResourceEvent{
		Kind:            "Pod",
		Namespace:       "default",
		Name:            "api",
		ResourceVersion: "1",
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "api",
			},
			"status": map[string]interface{}{
				"phase": "CrashLoopBackOff",
			},
		},
	}

	src := fakeSource{events: []events.ResourceEvent{event}}
	filter := NewRuleAwareFilter(cfg)

	pipe := New(logger, src, filter, nil, engine, dispatcher, cfg.Settings.QueueDepth)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pipe.Start(ctx)

	select {
	case <-dispatcher.signal:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for dispatcher")
	}
	cancel()

	if len(dispatcher.invocations) != 1 {
		t.Fatalf("expected 1 invocation, got %d", len(dispatcher.invocations))
	}
	if dispatcher.invocations[0].RuleName != "pod_failure" {
		t.Fatalf("unexpected rule %s", dispatcher.invocations[0].RuleName)
	}
}
