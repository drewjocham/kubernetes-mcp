package chatbridge

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBackend struct {
	result InvestigationResult
	err    error
	calls  int
}

func (m *mockBackend) Run(_ context.Context, _ InvestigationRequest) (InvestigationResult, error) {
	m.calls++
	return m.result, m.err
}

type mockReporter struct {
	posts []string
}

func (m *mockReporter) Post(_ context.Context, _ string, text string) error {
	m.posts = append(m.posts, text)
	return nil
}

func TestBridge_HandleChatEvent_Success(t *testing.T) {
	bridge := &Bridge{
		logger: slog.Default(),
		cfg: Config{
			GoogleChat: GoogleChatConfig{},
			Trigger: TriggerConfig{
				KubernetesKeywords: []string{"pod", "kubernetes"},
			},
			Investigation: InvestigationConfig{
				Provider: "oz",
			},
		},
		backend: &mockBackend{
			result: InvestigationResult{
				Provider: "oz",
				RunID:    "run-123",
				State:    "SUCCEEDED",
				Summary:  "Pod restarted due to image pull issue.",
			},
		},
		reporter:    &mockReporter{},
		idempotency: NewIdempotencyStore(10 * time.Minute),
		now:         time.Now,
	}

	body := `{
		"type":"MESSAGE",
		"eventTime":"2026-03-07T20:00:00Z",
		"space":{"name":"spaces/AAA"},
		"message":{
			"name":"spaces/AAA/messages/123",
			"text":"pod is failing in kubernetes",
			"thread":{"name":"spaces/AAA/threads/xyz"},
			"sender":{"displayName":"alice"}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/chat/events", strings.NewReader(body))
	rec := httptest.NewRecorder()

	bridge.handleChatEvent(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"status":"ok"`)

	mockRep := bridge.reporter.(*mockReporter)
	require.Len(t, mockRep.posts, 1)
	assert.Contains(t, mockRep.posts[0], "Investigation report (KUBERNETES)")
}

func TestBridge_HandleChatEvent_DuplicateIgnored(t *testing.T) {
	backend := &mockBackend{
		result: InvestigationResult{
			Provider: "openapi",
			RunID:    "run-1",
			State:    "SUCCEEDED",
			Summary:  "ok",
		},
	}
	bridge := &Bridge{
		logger: slog.Default(),
		cfg: Config{
			GoogleChat: GoogleChatConfig{},
			Trigger: TriggerConfig{
				KubernetesKeywords: []string{"k8s"},
			},
			Investigation: InvestigationConfig{
				Provider: "openapi",
			},
		},
		backend:     backend,
		reporter:    &mockReporter{},
		idempotency: NewIdempotencyStore(10 * time.Minute),
		now:         time.Now,
	}
	body := `{
		"type":"MESSAGE",
		"eventTime":"2026-03-07T20:00:00Z",
		"space":{"name":"spaces/BBB"},
		"message":{"name":"m1","text":"k8s issue","thread":{"name":"t1"}}
	}`

	req1 := httptest.NewRequest(http.MethodPost, "/chat/events", strings.NewReader(body))
	rec1 := httptest.NewRecorder()
	bridge.handleChatEvent(rec1, req1)
	require.Equal(t, http.StatusOK, rec1.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/chat/events", strings.NewReader(body))
	rec2 := httptest.NewRecorder()
	bridge.handleChatEvent(rec2, req2)
	require.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), `"status":"duplicate_ignored"`)
	assert.Equal(t, 1, backend.calls)
}
