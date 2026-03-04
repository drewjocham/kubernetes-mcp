package rules

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
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
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
	celEnv, err := cel.NewEnv(
		cel.Variable("evt", cel.DynType),
		cel.Variable("kind", cel.StringType),
		cel.Variable("ns", cel.StringType),
		cel.Variable("name", cel.StringType),
	)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		cfg          *config.WatchConfig
		store        *stubStore
		event        events.ResourceEvent
		celEnv       *cel.Env
		wantActions  int
		wantStoreSet bool
		wantErr      bool
		check        func(t *testing.T, invs []ActionInvocation, store *stubStore)
	}{
		{
			name: "rule matches with 'all' logic and all conditions met",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name:  "all_match",
						Kind:  "Pod",
						Logic: "all",
						Conditions: []config.Condition{
							{Field: "status.phase", Operator: "eq", Value: "Running"},
							{Field: "metadata.name", Operator: "eq", Value: "test-pod"},
						},
						Actions: []string{"log"},
					},
				},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod", Name: "test-pod",
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{"name": "test-pod"},
					"status":   map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions:  1,
			wantStoreSet: true,
		},
		{
			name: "rule does not match with 'all' logic and one condition fails",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name:  "one_fails",
						Kind:  "Pod",
						Logic: "all",
						Conditions: []config.Condition{
							{Field: "status.phase", Operator: "eq", Value: "Running"},
							{Field: "metadata.name", Operator: "eq", Value: "wrong-name"},
						},
						Actions: []string{"log"},
					},
				},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod", Name: "test-pod",
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{"name": "test-pod"},
					"status":   map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions:  0,
			wantStoreSet: true,
		},
		{
			name: "rule matches with 'any' logic and one condition met",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name:  "any_match",
						Kind:  "Pod",
						Logic: "any",
						Conditions: []config.Condition{
							{Field: "status.phase", Operator: "eq", Value: "Pending"},
							{Field: "metadata.name", Operator: "eq", Value: "test-pod"},
						},
						Actions: []string{"log"},
					},
				},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod", Name: "test-pod",
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{"name": "test-pod"},
					"status":   map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions:  1,
			wantStoreSet: true,
		},
		{
			name: "rule with 'changed' operator is triggered",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name: "changed_op",
						Kind: "Pod",
						Conditions: []config.Condition{
							{Field: "status.phase", Operator: "changed"},
						},
						Actions: []string{"log"},
					},
				},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			store: &stubStore{
				data: map[string]tracker.Snapshot{
					"pod/default/test-pod": {Values: map[string]interface{}{"status.phase": "Pending"}},
				},
			},
			event: events.ResourceEvent{
				Kind: "Pod", Namespace: "default", Name: "test-pod",
				Object: map[string]interface{}{
					"status": map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions:  1,
			wantStoreSet: true,
		},
		{
			name: "rule with CEL expression evaluates to true",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name:       "cel_true",
						Kind:       "Pod",
						Expression: `evt.status.phase == "Running"`,
						Actions:    []string{"log"},
					},
				},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			celEnv: celEnv,
			event: events.ResourceEvent{
				Kind: "Pod", Name: "test-pod",
				Object: map[string]interface{}{
					"status": map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions:  1,
			wantStoreSet: true,
		},
		{
			name: "rule with CEL expression evaluates to false",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name:       "cel_false",
						Kind:       "Pod",
						Expression: `evt.status.phase == "Pending"`,
						Actions:    []string{"log"},
					},
				},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			celEnv: celEnv,
			event: events.ResourceEvent{
				Kind: "Pod", Name: "test-pod",
				Object: map[string]interface{}{
					"status": map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions:  0,
			wantStoreSet: true,
		},
		{
			name: "kind mismatch prevents rule evaluation",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name: "kind_mismatch",
						Kind: "Deployment",
						Conditions: []config.Condition{
							{Field: "status.phase", Operator: "eq", Value: "Running"},
						},
						Actions: []string{"log"},
					},
				},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod", Name: "test-pod",
				Object: map[string]interface{}{
					"status": map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions:  0,
			wantStoreSet: true,
		},
		{
			name: "namespace match allows rule evaluation",
			cfg: &config.WatchConfig{
				Rules: []config.Rule{
					{
						Name:      "ns_match",
						Kind:      "Pod",
						Namespace: "kube-system",
						Conditions: []config.Condition{
							{Field: "status.phase", Operator: "eq", Value: "Running"},
						},
						Actions: []string{"log"},
					},
				},
				Actions: map[string]config.Action{"log": {Type: "log"}},
			},
			event: events.ResourceEvent{
				Kind: "Pod", Namespace: "kube-system", Name: "test-pod",
				Object: map[string]interface{}{
					"status": map[string]interface{}{"phase": "Running"},
				},
			},
			wantActions:  1,
			wantStoreSet: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := tc.store
			if store == nil {
				store = newStubStore()
			}

			engine := NewEngine(logger, tc.cfg, store, tc.celEnv)

			invocations, err := engine.Evaluate(context.Background(), tc.event)

			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.wantActions, len(invocations), "unexpected number of actions invoked")

			if tc.wantStoreSet {
				_, ok := store.Get(tc.event.Key())
				assert.True(t, ok, "expected snapshot to be stored")
			}

			if tc.check != nil {
				tc.check(t, invocations, store)
			}
		})
	}
}

func TestEngine_evaluateCondition(t *testing.T) {
	engine := &Engine{}
	objJSON, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": 5, "size": "100m"})

	testCases := []struct {
		name    string
		cond    config.Condition
		prev    tracker.Snapshot
		wantMet bool
		wantVal interface{}
		wantErr bool
	}{
		{name: "eq string true", cond: config.Condition{Field: "status", Operator: "eq", Value: "ok"}, wantMet: true, wantVal: "ok"},
		{name: "eq string false", cond: config.Condition{Field: "status", Operator: "eq", Value: "fail"}, wantMet: false, wantVal: "ok"},
		{name: "ne string true", cond: config.Condition{Field: "status", Operator: "ne", Value: "fail"}, wantMet: true, wantVal: "ok"},
		{name: "gt numeric true", cond: config.Condition{Field: "count", Operator: "gt", Value: 4}, wantMet: true, wantVal: float64(5)},
		{name: "lt numeric false", cond: config.Condition{Field: "count", Operator: "lt", Value: 4}, wantMet: false, wantVal: float64(5)},
		{name: "changed true", cond: config.Condition{Field: "status", Operator: "changed"}, prev: tracker.Snapshot{Values: map[string]interface{}{"status": "pending"}}, wantMet: true, wantVal: "ok"},
		{name: "changed false", cond: config.Condition{Field: "status", Operator: "changed"}, prev: tracker.Snapshot{Values: map[string]interface{}{"status": "ok"}}, wantMet: false, wantVal: "ok"},
		{name: "bad operator", cond: config.Condition{Field: "status", Operator: "???"}, wantErr: true, wantVal: "ok"},
		{name: "field missing", cond: config.Condition{Field: "nonexistent", Operator: "eq"}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			met, val, err := engine.evaluateCondition(tc.cond, tc.prev, objJSON)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantMet, met, "condition met status")
				assert.Equal(t, tc.wantVal, val, "extracted value")
			}
		})
	}
}

func TestEngine_compareScalar(t *testing.T) {
	testCases := []struct {
		name string
		a, b interface{}
		want int
	}{
		{name: "floats equal", a: 1.0, b: 1.0, want: 0},
		{name: "floats a > b", a: 2.0, b: 1.0, want: 1},
		{name: "floats a < b", a: 1.0, b: 2.0, want: -1},
		{name: "strings equal", a: "abc", b: "abc", want: 0},
		{name: "strings a > b", a: "def", b: "abc", want: 1},
		{name: "strings a < b", a: "abc", b: "def", want: -1},
		{name: "quantity a > b", a: "200m", b: "100m", want: 1},
		{name: "mixed float and int", a: 1.0, b: 1, want: 0},
		{name: "mixed float and quantity", a: 0.2, b: "200m", want: 0},
		{name: "mixed string and float", a: "1.0", b: 1.0, want: 0}, // numeric comparison
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := compareScalar(tc.a, tc.b)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestEngine_extractValue(t *testing.T) {
	objJSON, _ := json.Marshal(map[string]interface{}{
		"a": map[string]interface{}{"b": "c"},
		"d": []interface{}{"e", "f"},
	})

	testCases := []struct {
		name    string
		path    string
		json    []byte
		wantVal interface{}
		wantErr bool
	}{
		{name: "simple path", path: "a.b", json: objJSON, wantVal: "c"},
		{name: "array path", path: "d.1", json: objJSON, wantVal: "f"},
		{name: "path not exists", path: "a.x", json: objJSON, wantErr: true},
		{name: "empty path", path: "", json: objJSON, wantErr: true},
		{name: "empty json", path: "a.b", json: []byte{}, wantErr: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			val, err := extractValue(tc.json, tc.path)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantVal, val)
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
		Kind: "Pod", Namespace: "ns", Name: "worker",
		Object: map[string]interface{}{
			"status": map[string]interface{}{"phase": "Pending"},
		},
	}

	ctx := context.Background()
	inv, err := engine.Evaluate(ctx, event)
	require.NoError(t, err)
	assert.Empty(t, inv, "expected no actions before duration window")

	// Condition is no longer met, timer should be cleared
	event.Object = map[string]interface{}{"status": map[string]interface{}{"phase": "Running"}}
	inv, err = engine.Evaluate(ctx, event)
	require.NoError(t, err)
	assert.Empty(t, inv, "expected no actions when condition is no longer met")

	// Condition is met again, timer should start over
	event.Object = map[string]interface{}{"status": map[string]interface{}{"phase": "Pending"}}
	inv, err = engine.Evaluate(ctx, event)
	require.NoError(t, err)
	assert.Empty(t, inv, "expected no actions when timer restarts")

	time.Sleep(60 * time.Millisecond)
	inv, err = engine.Evaluate(ctx, event)
	require.NoError(t, err)
	assert.Len(t, inv, 1, "expected action after duration window")
}

// stubStore is a test implementation of the tracker.Store interface.
type stubStore struct {
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
	val, ok := s.data[key]
	return val, ok
}
func (s *stubStore) Set(key string, snap tracker.Snapshot) {
	s.data[key] = snap
}
func (s *stubStore) RecordHistory(key string, snap tracker.Snapshot) {
	if s.history == nil {
		s.history = make(map[string][]tracker.Snapshot)
	}
	s.history[key] = append(s.history[key], snap)
}
func (s *stubStore) History(key string, limit int) []tracker.Snapshot {
	h, ok := s.history[key]
	if !ok {
		return nil
	}
	if len(h) > limit && limit > 0 {
		h = h[len(h)-limit:]
	}
	out := make([]tracker.Snapshot, len(h))
	copy(out, h)
	return out
}
func (s *stubStore) Close() error { return nil }
