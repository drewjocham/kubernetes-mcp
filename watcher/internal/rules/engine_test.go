package rules

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/tracker"
)

func TestEngine_Evaluate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	celEnv, _ := cel.NewEnv(
		cel.Variable("evt", cel.DynType),
		cel.Variable("kind", cel.StringType),
		cel.Variable("ns", cel.StringType),
		cel.Variable("name", cel.StringType),
	)

	testCases := []struct {
		name        string
		cfg         *config.WatchConfig
		storeData   map[string]tracker.Snapshot
		event       events.ResourceEvent
		wantActions int
	}{
		{
			name: "all_logic_success",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{{
					Name: "test", Kind: "Pod", Logic: "all",
					Conditions: []config.Condition{
						{Field: "status.phase", Operator: "eq", Value: "Running"},
						{Field: "metadata.name", Operator: "eq", Value: "test-pod"},
					},
					Actions: []string{"log"},
				}},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod", Name: "test-pod",
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{"name": "test-pod"},
					"status":   map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions: 1,
		},
		{
			name: "any_logic_short_circuit",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{{
					Name: "test", Kind: "Pod", Logic: "any",
					Conditions: []config.Condition{
						{Field: "status.phase", Operator: "eq", Value: "Running"},
						{Field: "status.phase", Operator: "eq", Value: "Failed"},
					},
					Actions: []string{"log"},
				}},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod",
				Object: map[string]interface{}{
					"status": map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions: 1,
		},
		{
			name: "changed_operator_detection",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{{
					Name: "delta", Kind: "Pod",
					Conditions: []config.Condition{{Field: "status.phase", Operator: "changed"}},
					Actions:    []string{"log"},
				}},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			storeData: map[string]tracker.Snapshot{
				"pod/test-pod": {Values: map[string]interface{}{"status.phase": "Pending"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod", Name: "test-pod",
				Object: map[string]interface{}{"status": map[string]interface{}{"phase": "Running"}},
			},
			wantActions: 1,
		},
		{
			name: "cel_expression_evaluation",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{{
					Name: "cel", Kind: "Pod", Expression: `evt.status.phase == "Running"`,
					Actions: []string{"log"},
				}},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod",
				Object: map[string]interface{}{
					"status": map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := newStubStore()
			for k, v := range tc.storeData {
				store.Set(k, v)
			}

			engine := NewEngine(logger, tc.cfg, store, celEnv)
			invs, err := engine.Evaluate(context.Background(), tc.event)

			require.NoError(t, err)
			assert.Equal(t, tc.wantActions, len(invs))
		})
	}
}

func TestEngine_CompareScalar(t *testing.T) {
	cases := []struct {
		name string
		a, b interface{}
		want int
	}{
		{"float_eq", 10.0, 10.0, 0},
		{"k8s_milli_cpu", "500m", 0.5, 0},
		{"k8s_memory_gi", "1Gi", "1024Mi", 0},
		{"numeric_string", "100", 100.0, 0},
		{"string_lex", "a", "b", -1},
	}

	engine := &Engine{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, engine.compare(tc.a, tc.b))
		})
	}
}

func TestEngine_TimerLogic(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.WatchConfig{
		Rules: []config.Rule{{
			Name: "damped", Kind: "Pod", For: 20 * time.Millisecond,
			Conditions: []config.Condition{{Field: "status", Operator: "eq", Value: "Error"}},
			Actions:    []string{"act"},
		}},
		Actions: map[string]config.Action{"act": {Type: "log"}},
	}
	engine := NewEngine(logger, cfg, newStubStore(), nil)
	ctx := context.Background()
	evt := events.ResourceEvent{Kind: "Pod", Name: "pod-1", Object: map[string]interface{}{"status": "Error"}}

	inv, _ := engine.Evaluate(ctx, evt)
	assert.Empty(t, inv, "should not trigger before duration")

	time.Sleep(25 * time.Millisecond)

	inv, _ = engine.Evaluate(ctx, evt)
	assert.Len(t, inv, 1, "should trigger after duration")

	inv, _ = engine.Evaluate(ctx, evt)
	assert.Empty(t, inv, "should reset timer after triggering")
}

func TestEngine_Concurrency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.WatchConfig{
		Rules: []config.Rule{{
			Name: "rapid", Kind: "Pod", For: 5 * time.Millisecond,
			Conditions: []config.Condition{{Field: "status", Operator: "eq", Value: "Fail"}},
			Actions:    []string{"act"},
		}},
		Actions: map[string]config.Action{"act": {Type: "log"}},
	}
	engine := NewEngine(logger, cfg, newStubStore(), nil)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			evt := events.ResourceEvent{
				Kind: "Pod", Name: string(rune(id)),
				Object: map[string]interface{}{"status": "Fail"},
			}
			_, _ = engine.Evaluate(context.Background(), evt)
			time.Sleep(10 * time.Millisecond)
			inv, _ := engine.Evaluate(context.Background(), evt)
			assert.Len(t, inv, 1)
		}(i)
	}
	wg.Wait()
}

type stubStore struct {
	mu      sync.RWMutex
	data    map[string]tracker.Snapshot
	history map[string][]tracker.Snapshot
}

func newStubStore() *stubStore {
	return &stubStore{
		data:    make(map[string]tracker.Snapshot),
		history: make(map[string][]tracker.Snapshot),
	}
}

func (s *stubStore) Get(key string) (tracker.Snapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *stubStore) Set(key string, snap tracker.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = snap
}

func (s *stubStore) RecordHistory(key string, snap tracker.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history[key] = append(s.history[key], snap)
}

func (s *stubStore) History(key string, limit int) []tracker.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h, ok := s.history[key]
	if !ok {
		return nil
	}

	if limit <= 0 || limit >= len(h) {
		out := make([]tracker.Snapshot, len(h))
		copy(out, h)
		return out
	}

	start := len(h) - limit
	out := make([]tracker.Snapshot, limit)
	copy(out, h[start:])
	return out
}

func (s *stubStore) Close() error { return nil }
