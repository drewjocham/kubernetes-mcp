package actions

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/rules"
)

type mockLogger struct {
	buf    bytes.Buffer
	logger *slog.Logger
}

func newMockLogger() *mockLogger {
	var buf bytes.Buffer
	return &mockLogger{
		buf:    buf,
		logger: slog.New(slog.NewTextHandler(&buf, nil)),
	}
}

func (m *mockLogger) String() string {
	return m.buf.String()
}

func TestDispatcher_Dispatch(t *testing.T) {
	testCases := []struct {
		name       string
		actions    map[string]config.Action
		invocation rules.ActionInvocation
		setup      func(d *Dispatcher)
		check      func(t *testing.T, mockLogger *mockLogger)
	}{
		{
			name: "log action with simple template",
			actions: map[string]config.Action{
				"log-action": {Type: "log", Template: "Pod {{ .resource.Name }} is failing."},
			},
			invocation: rules.ActionInvocation{
				RuleName: "test-rule",
				ActionID: "log-action",
				Action:   config.Action{Type: "log", Template: "Pod {{ .resource.Name }} is failing."},
				Context:  map[string]interface{}{"resource": map[string]interface{}{"Name": "test-pod"}},
			},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), `message="Pod test-pod is failing."`)
			},
		},
		{
			name: "log action with invalid template",
			actions: map[string]config.Action{
				"log-action": {Type: "log", Template: "Pod {{ .resource.Name } is failing."},
			},
			invocation: rules.ActionInvocation{
				RuleName: "test-rule",
				ActionID: "log-action",
				Action:   config.Action{Type: "log", Template: "Pod {{ .resource.Name } is failing."},
				Context:  map[string]interface{}{"resource": map[string]interface{}{"Name": "test-pod"}},
			},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), "action template failure")
			},
		},
		{
			name: "throttled action is blocked",
			actions: map[string]config.Action{
				"log-action": {Type: "log", Template: "throttled", Throttle: config.Throttle{MaxPerMinute: 1}},
			},
			invocation: rules.ActionInvocation{
				RuleName: "test-rule",
				ActionID: "log-action",
				Action:   config.Action{Type: "log", Template: "throttled", Throttle: config.Throttle{MaxPerMinute: 1}},
			},
			setup: func(d *Dispatcher) {
				// Simulate a recent run
				key := "test-rule:log-action"
				d.mu.Lock()
				d.lastRun[key] = time.Now()
				d.mu.Unlock()
			},
			check: func(t *testing.T, ml *mockLogger) {
				assert.NotContains(t, ml.String(), "message=throttled")
			},
		},
		{
			name: "throttled action is allowed",
			actions: map[string]config.Action{
				"log-action": {Type: "log", Template: "throttled", Throttle: config.Throttle{MaxPerMinute: 1}},
			},
			invocation: rules.ActionInvocation{
				RuleName: "test-rule",
				ActionID: "log-action",
				Action:   config.Action{Type: "log", Template: "throttled", Throttle: config.Throttle{MaxPerMinute: 1}},
			},
			setup: func(d *Dispatcher) {
				key := "test-rule:log-action"
				d.mu.Lock()
				d.lastRun[key] = time.Now().Add(-2 * time.Minute)
				d.mu.Unlock()
			},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), "message=throttled")
			},
		},
		{
			name: "unknown action type",
			actions: map[string]config.Action{
				"unknown-action": {Type: "unknown"},
			},
			invocation: rules.ActionInvocation{
				RuleName: "test-rule",
				ActionID: "unknown-action",
				Action:   config.Action{Type: "unknown"},
			},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), `type=unknown`)
				assert.Contains(t, ml.String(), `rule=test-rule`)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ml := newMockLogger()
			dispatcher, err := NewDispatcher(ml.logger, tc.actions, 4)
			require.NoError(t, err)
			defer dispatcher.Close()

			if tc.setup != nil {
				tc.setup(dispatcher)
			}

			err = dispatcher.Dispatch(context.Background(), tc.invocation)
			require.NoError(t, err)

			// Allow time for the async pool to execute the task
			time.Sleep(50 * time.Millisecond)

			if tc.check != nil {
				tc.check(t, ml)
			}
		})
	}
}

func TestDispatcher_renderTemplate(t *testing.T) {
	d := &Dispatcher{}
	data := map[string]interface{}{"name": "world"}

	testCases := []struct {
		name    string
		tmplStr string
		data    interface{}
		want    string
		wantErr bool
	}{
		{name: "valid template", tmplStr: "hello {{ .name }}", data: data, want: "hello world"},
		{name: "invalid template", tmplStr: "hello {{ .name", data: data, wantErr: true},
		{name: "template execution error", tmplStr: "hello {{ .missing }}", data: data, want: "hello <no value>"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := d.renderTemplate(tc.tmplStr, tc.data)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
