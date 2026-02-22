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
				Name:    "crash_loop",
				Kind:    "Pod",
				Logic:   "all",
				Actions: []string{"log"},
				Conditions: []config.Condition{
					{Field: "status.phase", Operator: "eq", Value: "CrashLoopBackOff"},
				},
			},
		},
		Actions: map[string]config.Action{
			"log": {Type: "log", Template: "triggered {{ .rule.Name }} on {{ .resource.metadata.name }}"},
		},
		Settings: config.Settings{QueueDepth: 4},
	}

	store := tracker.NewMemoryStore()
	defer store.Close()

	engine := rules.NewEngine(logger, cfg, store, nil)

	dispatcher, err := actions.NewDispatcher(logger, cfg.Actions, 4)
	if err != nil {
		t.Fatalf("failed to init dispatcher: %v", err)
	}

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

	src := eventSource{events: []events.ResourceEvent{event}}
	filter := pipeline.NewRuleAwareFilter(cfg)
	pipe := pipeline.New(logger, src, filter, nil, engine, dispatcher, cfg.Settings.QueueDepth)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		pipe.Start(ctx)
		close(done)
	}()

	assertLogged := func() bool {
		msg := buf.String()
		return strings.Contains(msg, "rule action") && strings.Contains(msg, "crash_loop")
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	for {
		if assertLogged() {
			break
		}
		select {
		case <-waitCtx.Done():
			t.Fatal("timed out waiting for dispatcher log output")
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pipeline did not stop")
	}
}
