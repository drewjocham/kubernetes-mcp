package chatbridge

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBackend struct {
	result InvestigationResult
	err    error
	calls  int
	mu     sync.Mutex
}

func (m *mockBackend) Run(_ context.Context, _ InvestigationRequest) (InvestigationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.result, m.err
}

type mockReporter struct {
	posts []string
	mu    sync.Mutex
}

func (m *mockReporter) Post(_ context.Context, _ string, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.posts = append(m.posts, text)
	return nil
}

func TestBridge_HandleChatEvent_Success(t *testing.T) {
	rep := &mockReporter{}
	bridge := &Bridge{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
		cfg: Config{
			Trigger: TriggerConfig{
				KubernetesKeywords: []string{"pod"},
			},
			Investigation: InvestigationConfig{
				Provider: "test-ai",
			},
		},
		backend: &mockBackend{
			result: InvestigationResult{Summary: "Found OOMKill"},
		},
		reporter:    rep,
		idempotency: NewIdempotencyStore(10 * time.Minute),
		now:         time.Now,
	}

	body := `{
       "type":"MESSAGE",
       "space":{"name":"spaces/dev"},
       "message":{
          "name":"m1",
          "text":"The pod is crashing",
          "thread":{"name":"t1"},
          "sender":{"displayName":"alice"}
       }
    }`

	req := httptest.NewRequest(http.MethodPost, "/chat/events", strings.NewReader(body))
	rec := httptest.NewRecorder()

	bridge.handleChatEvent(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	assert.Equal(t, "accepted", resp["status"])

	require.Eventually(t, func() bool {
		rep.mu.Lock()
		defer rep.mu.Unlock()
		return len(rep.posts) > 0
	}, 2*time.Second, 50*time.Millisecond, "Reporter should have received a post")

	assert.Contains(t, rep.posts[0], "OOMKill")
}

func TestBridge_HandleChatEvent_Unauthorized(t *testing.T) {
	os.Setenv("BRIDGE_SECRET", "super-secret")
	defer os.Unsetenv("BRIDGE_SECRET")

	bridge := &Bridge{
		logger: slog.Default(),
		cfg: Config{
			GoogleChat: GoogleChatConfig{
				AuthTokenEnv: "BRIDGE_SECRET",
				AuthHeader:   "X-Token",
			},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/chat/events", strings.NewReader(`{}`))
	req.Header.Set("X-Token", "wrong-password")
	rec := httptest.NewRecorder()

	bridge.handleChatEvent(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBridge_HandleChatEvent_DuplicateIgnored(t *testing.T) {
	backend := &mockBackend{}
	bridge := &Bridge{
		logger: slog.Default(),
		cfg: Config{
			Trigger:       TriggerConfig{KubernetesKeywords: []string{"k8s"}},
			Investigation: InvestigationConfig{Provider: "ai"},
		},
		backend:     backend,
		reporter:    &mockReporter{},
		idempotency: NewIdempotencyStore(10 * time.Minute),
		now:         time.Now,
	}

	body := `{"type":"MESSAGE","space":{"name":"s1"},"message":{"name":"msg-unique-123","text":"k8s error","thread":{"name":"t1"}}}`

	req1 := httptest.NewRequest(http.MethodPost, "/chat/events", strings.NewReader(body))
	rec1 := httptest.NewRecorder()
	bridge.handleChatEvent(rec1, req1)
	assert.Equal(t, http.StatusOK, rec1.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/chat/events", strings.NewReader(body))
	rec2 := httptest.NewRecorder()
	bridge.handleChatEvent(rec2, req2)

	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "duplicate")

	backend.mu.Lock()
	defer backend.mu.Unlock()
	assert.Equal(t, 1, backend.calls)
}
