package pipeline

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
)

func TestModelEnricher_Enrich(t *testing.T) {
	t.Run("disabled model", func(t *testing.T) {
		enricher := NewModelEnricher(
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			config.ModelSettings{Enabled: false},
		)
		evt := events.ResourceEvent{Kind: "Pod", Object: map[string]interface{}{"x": "y"}}
		out, err := enricher.Enrich(context.Background(), evt)
		require.NoError(t, err)
		assert.Equal(t, evt.Object, out.Object)
	})

	t.Run("issue detected", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issue_detected":true,"severity":"CRITICAL","summary":"CrashLoop anomaly","confidence":0.91,"signals":["restart_delta","waiting_reason"]}`))
		}))
		defer server.Close()

		enricher := NewModelEnricher(
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			config.ModelSettings{
				Enabled:       true,
				Endpoint:      server.URL,
				Timeout:       0,
				MinConfidence: 0.65,
			},
		)
		evt := events.ResourceEvent{
			Kind:      "Pod",
			Namespace: "default",
			Name:      "api-0",
			Object:    map[string]interface{}{"restart_delta": 4.0},
		}
		out, err := enricher.Enrich(context.Background(), evt)
		require.NoError(t, err)
		assert.Equal(t, true, out.Object["model_issue_detected"])
		assert.Equal(t, "critical", out.Object["model_issue_severity"])
		assert.Equal(t, "CrashLoop anomaly", out.Object["model_issue_summary"])
		assert.Equal(t, 0.91, out.Object["model_issue_confidence"])
	})

	t.Run("confidence below threshold", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issue_detected":true,"severity":"warning","summary":"Weak signal","confidence":0.20}`))
		}))
		defer server.Close()

		enricher := NewModelEnricher(
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			config.ModelSettings{
				Enabled:       true,
				Endpoint:      server.URL,
				Timeout:       0,
				MinConfidence: 0.65,
			},
		)
		evt := events.ResourceEvent{Kind: "Pod", Object: map[string]interface{}{"restart_delta": 1.0}}
		out, err := enricher.Enrich(context.Background(), evt)
		require.NoError(t, err)
		assert.Equal(t, false, out.Object["model_issue_detected"])
		assert.Equal(t, 0.20, out.Object["model_issue_confidence"])
	})

	t.Run("backend failure is non-fatal", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "oops", http.StatusInternalServerError)
		}))
		defer server.Close()

		enricher := NewModelEnricher(
			slog.New(slog.NewTextHandler(io.Discard, nil)),
			config.ModelSettings{
				Enabled:  true,
				Endpoint: server.URL,
				Timeout:  0,
			},
		)
		evt := events.ResourceEvent{Kind: "Pod", Object: map[string]interface{}{"restart_delta": 1.0}}
		out, err := enricher.Enrich(context.Background(), evt)
		require.NoError(t, err)
		assert.Equal(t, false, out.Object["model_issue_detected"])
		assert.Contains(t, out.Object, "model_analysis_error")
	})
}
