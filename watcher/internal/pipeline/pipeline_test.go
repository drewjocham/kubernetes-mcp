package pipeline

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/rules"
	"kube-watcher/watcher/internal/tracker"
)

type fakeSource struct {
	events []events.ResourceEvent
}

func (f *fakeSource) Run(ctx context.Context, out chan<- events.ResourceEvent) {
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

func newRecordingDispatcher(size int) *recordingDispatcher {
	return &recordingDispatcher{signal: make(chan struct{}, size)}
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

type mockObserver struct {
	mu     sync.Mutex
	events []events.ResourceEvent
}

func (m *mockObserver) Observe(evt events.ResourceEvent) {
	m.mu.Lock()
	m.events = append(m.events, evt)
	m.mu.Unlock()
}

func TestPipelineIntegration(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	cfg := &config.WatchConfig{
		Rules: []config.Rule{
			{
				Name:       "pod_failure",
				Kind:       "Pod",
				Expression: `evt.status.phase == "CrashLoopBackOff"`,
				Actions:    []string{"log"},
			},
		},
		Actions:  map[string]config.Action{"log": {Type: "log"}},
		Settings: config.Settings{QueueDepth: 4},
	}

	store := tracker.NewMemoryStore()
	engine := rules.NewEngine(logger, cfg, store, nil)
	dispatcher := newRecordingDispatcher(1)

	event := events.ResourceEvent{
		Kind: "Pod",
		Object: map[string]interface{}{
			"status": map[string]interface{}{"phase": "CrashLoopBackOff"},
		},
	}

	src := &fakeSource{events: []events.ResourceEvent{event}}
	filter := NewRuleAwareFilter(cfg)
	enricher := NewPodEnricher(nil)

	pipe := New(logger, src, filter, enricher, engine, dispatcher, store, nil, cfg.Settings.QueueDepth, 2, 5)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go pipe.Start(ctx)

	<-dispatcher.signal

	assert.Len(t, dispatcher.invocations, 1)
	assert.Equal(t, "pod_failure", dispatcher.invocations[0].RuleName)
}

func TestRuleAwareFilter_Allow(t *testing.T) {
	cfg := &config.WatchConfig{
		Rules: []config.Rule{
			{Kind: "Pod", Namespace: "default"},
			{Kind: "Node"},
		},
	}
	filter := NewRuleAwareFilter(cfg)

	testCases := []struct {
		name  string
		event events.ResourceEvent
		want  bool
	}{
		{name: "allowed pod in namespace", event: events.ResourceEvent{Kind: "Pod", Namespace: "default"}, want: true},
		{name: "disallowed pod in other namespace", event: events.ResourceEvent{Kind: "Pod", Namespace: "kube-system"}, want: false},
		{name: "allowed node", event: events.ResourceEvent{Kind: "Node"}, want: true},
		{name: "disallowed kind", event: events.ResourceEvent{Kind: "Deployment"}, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := filter.Allow(context.Background(), tc.event)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestPipeline_recordMetrics(t *testing.T) {
	p := &Pipeline{
		metrics:    tracker.NewMetricStore(time.Minute),
		fields:     []string{"restart_count"},
		deltaNames: map[string]string{"restart_count": "restart_delta"},
	}

	evt := events.ResourceEvent{
		Object: map[string]interface{}{"restart_count": 5},
	}

	// First run, no delta
	p.recordMetrics(evt)
	assert.NotContains(t, evt.Object, "restart_delta")

	// Second run, with delta
	evt.Object["restart_count"] = 7
	p.recordMetrics(evt)
	assert.Contains(t, evt.Object, "restart_delta")
	assert.Equal(t, 2.0, evt.Object["restart_delta"])
}

func TestPipeline_maybeAttachGraph(t *testing.T) {
	store := tracker.NewMemoryStore()
	key := "pod/default/p1"
	store.RecordHistory(key, tracker.Snapshot{Values: map[string]interface{}{"restart_count": 1}})

	p := &Pipeline{
		store:        store,
		logger:       slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{})),
		historyLimit: 10,
	}

	testCases := []struct {
		name   string
		inv    rules.ActionInvocation
		hasImg bool
	}{
		{
			name: "attach graph true",
			inv: rules.ActionInvocation{
				Action: config.Action{Config: map[string]string{"attach_graph": "true", "graph_field": "restart_count"}},
			},
			hasImg: true,
		},
		{
			name: "attach graph false",
			inv: rules.ActionInvocation{
				Action: config.Action{Config: map[string]string{"attach_graph": "false"}},
			},
			hasImg: false,
		},
		{
			name:   "no config",
			inv:    rules.ActionInvocation{},
			hasImg: false,
		},
		{
			name: "graph render fails",
			inv: rules.ActionInvocation{
				Action: config.Action{Config: map[string]string{"attach_graph": "true", "graph_field": "non_existent_field"}},
			},
			hasImg: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := p.maybeAttachGraph(events.ResourceEvent{Kind: "Pod", Namespace: "default", Name: "p1"}, tc.inv)
			_, ok := result.Context["graph_image"]
			assert.Equal(t, tc.hasImg, ok)
		})
	}
}

func TestPipeline_notifyObservers(t *testing.T) {
	obs1 := &mockObserver{}
	obs2 := &mockObserver{}
	p := &Pipeline{}
	p.AddObserver(obs1)
	p.AddObserver(obs2)
	p.AddObserver(nil) // Should be ignored

	evt := events.ResourceEvent{Name: "test-event"}
	p.notifyObservers(evt)

	assert.Len(t, obs1.events, 1)
	assert.Equal(t, "test-event", obs1.events[0].Name)
	assert.Len(t, obs2.events, 1)
}
