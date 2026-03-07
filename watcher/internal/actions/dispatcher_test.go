package actions

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
	"kube-watcher/watcher/internal/rules"
)

type mockLogger struct {
	buf    bytes.Buffer
	logger *slog.Logger
}

func newMockLogger() *mockLogger {
	var m mockLogger
	m.logger = slog.New(slog.NewTextHandler(&m.buf, nil))
	return &m
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

			time.Sleep(50 * time.Millisecond)

			if tc.check != nil {
				tc.check(t, ml)
			}
		})
	}
}

func TestDispatcher_OzAgent(t *testing.T) {
	testCases := []struct {
		name       string
		action     config.Action
		envVars    map[string]string
		serverFunc func(t *testing.T) http.HandlerFunc
		check      func(t *testing.T, ml *mockLogger)
	}{
		{
			name: "successful oz-agent dispatch",
			action: config.Action{
				Type:     "oz-agent",
				Template: "Investigate {{ .Kind }}/{{ .Name }} in {{ .Namespace }} for rule {{ .RuleName }}",
				Config: map[string]string{
					"environment_id": "ENV123",
					"api_key_env":    "OZ_API_KEY",
				},
			},
			envVars: map[string]string{"OZ_API_KEY": "test-key-123"},
			serverFunc: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "Bearer test-key-123", r.Header.Get("Authorization"))
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					body, _ := io.ReadAll(r.Body)
					var payload map[string]interface{}
					require.NoError(t, json.Unmarshal(body, &payload))
					assert.Contains(t, payload["prompt"], "Investigate Pod/crash-pod")

					cfg := payload["config"].(map[string]interface{})
					assert.Equal(t, "ENV123", cfg["environment_id"])

					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":"run-abc-123"}`))
				}
			},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), "oz-agent spawned")
				assert.Contains(t, ml.String(), "run-abc-123")
			},
		},
		{
			name: "missing API key",
			action: config.Action{
				Type:     "oz-agent",
				Template: "Investigate",
				Config: map[string]string{
					"environment_id": "ENV123",
					"api_key_env":    "OZ_API_KEY",
				},
			},
			envVars: map[string]string{},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), "oz-agent action missing API key")
			},
		},
		{
			name: "missing environment_id",
			action: config.Action{
				Type:     "oz-agent",
				Template: "Investigate",
				Config: map[string]string{
					"api_key_env": "OZ_API_KEY",
				},
			},
			envVars: map[string]string{"OZ_API_KEY": "key"},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), "oz-agent action missing environment_id")
			},
		},
		{
			name: "API returns error status",
			action: config.Action{
				Type:     "oz-agent",
				Template: "Investigate {{ .Kind }}",
				Config: map[string]string{
					"environment_id": "ENV123",
					"api_key_env":    "OZ_API_KEY",
				},
			},
			envVars: map[string]string{"OZ_API_KEY": "key"},
			serverFunc: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"error":"invalid key"}`))
				}
			},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), "oz-agent API returned non-success status")
			},
		},
		{
			name: "template rendering error",
			action: config.Action{
				Type:     "oz-agent",
				Template: "{{ .Invalid }",
				Config: map[string]string{
					"environment_id": "ENV123",
					"api_key_env":    "OZ_API_KEY",
				},
			},
			envVars: map[string]string{"OZ_API_KEY": "key"},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), "oz-agent template failure")
			},
		},
		{
			name: "defaults to OZ_API_KEY env var",
			action: config.Action{
				Type:     "oz-agent",
				Template: "Investigate {{ .Kind }}",
				Config: map[string]string{
					"environment_id": "ENV123",
				},
			},
			envVars: map[string]string{"OZ_API_KEY": "default-key"},
			serverFunc: func(t *testing.T) http.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "Bearer default-key", r.Header.Get("Authorization"))
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"id":"run-default"}`))
				}
			},
			check: func(t *testing.T, ml *mockLogger) {
				assert.Contains(t, ml.String(), "oz-agent spawned")
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ml := newMockLogger()
			actions := map[string]config.Action{"oz-action": tc.action}

			dispatcher, err := NewDispatcher(ml.logger, actions, 4)
			require.NoError(t, err)
			defer dispatcher.Close()

			// Inject env var lookup
			dispatcher.getEnv = func(key string) string {
				return tc.envVars[key]
			}

			// If a test server is provided, set up the mock and override api_url
			if tc.serverFunc != nil {
				srv := httptest.NewServer(tc.serverFunc(t))
				defer srv.Close()
				tc.action.Config["api_url"] = srv.URL
			}

			inv := rules.ActionInvocation{
				RuleName: "Pod-Restarting-Frequently",
				ActionID: "oz-action",
				Action:   tc.action,
				Context: map[string]interface{}{
					"resource": map[string]interface{}{"Name": "crash-pod"},
				},
				Event: events.ResourceEvent{
					Kind:      "Pod",
					Namespace: "production",
					Name:      "crash-pod",
				},
			}

			err = dispatcher.Dispatch(context.Background(), inv)
			require.NoError(t, err)

			time.Sleep(100 * time.Millisecond)

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
