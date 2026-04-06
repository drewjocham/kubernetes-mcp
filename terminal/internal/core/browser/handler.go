package browser

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"kube-watcher/terminal/internal/domain"
)

type Handler struct {
	adapter domain.BrowserAdapter

	mu      sync.Mutex
	started bool
}

func NewHandler(adapter domain.BrowserAdapter) *Handler {
	return &Handler{adapter: adapter}
}

func (h *Handler) OpenURL(ctx context.Context, rawURL string) (domain.Artifact, error) {
	if err := h.ensureStarted(ctx); err != nil {
		return domain.Artifact{}, fmt.Errorf("start browser adapter: %w", err)
	}

	if err := h.adapter.Navigate(ctx, rawURL); err != nil {
		return domain.Artifact{}, fmt.Errorf("navigate browser: %w", err)
	}

	snapshot, err := h.adapter.Snapshot(ctx)
	if err != nil {
		return domain.Artifact{}, fmt.Errorf("capture browser snapshot: %w", err)
	}

	return domain.Artifact{
		ID:      "browser-" + uuid.NewString(),
		Title:   "Browser Snapshot",
		Kind:    domain.ContentTypeMarkdown,
		Content: normalizeSnapshot(snapshot),
	}, nil
}

func (h *Handler) Close(ctx context.Context) error {
	h.mu.Lock()
	started := h.started
	h.started = false
	h.mu.Unlock()

	if !started {
		return nil
	}

	if err := h.adapter.Close(ctx); err != nil {
		return fmt.Errorf("close browser adapter: %w", err)
	}
	return nil
}

func (h *Handler) ensureStarted(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.started {
		return nil
	}
	if err := h.adapter.Start(ctx); err != nil {
		return err
	}
	h.started = true
	return nil
}

func normalizeSnapshot(snapshot domain.BrowserSnapshot) string {
	title := strings.TrimSpace(snapshot.Title)
	if title == "" {
		title = "Untitled Page"
	}
	preview := strings.TrimSpace(snapshot.TextPreview)
	if preview == "" {
		preview = "_No preview text available._"
	}
	capturedAt := snapshot.CapturedAt
	if capturedAt.IsZero() {
		capturedAt = time.Now().UTC()
	}

	return fmt.Sprintf(
		"## %s\n- URL: %s\n- Captured: %s\n\n### Preview\n%s\n",
		title,
		snapshot.URL,
		capturedAt.Format(time.RFC3339),
		preview,
	)
}
