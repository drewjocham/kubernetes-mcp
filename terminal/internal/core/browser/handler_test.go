package browser

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"kube-watcher/terminal/internal/domain"
)

type mockBrowserAdapter struct {
	startErr    error
	navigateErr error
	snapshotErr error
	closeErr    error
	snapshot    domain.BrowserSnapshot

	startCalls    int
	navigateCalls int
	snapshotCalls int
	closeCalls    int
	lastURL       string
}

func (m *mockBrowserAdapter) Start(_ context.Context) error {
	m.startCalls++
	return m.startErr
}

func (m *mockBrowserAdapter) Navigate(_ context.Context, rawURL string) error {
	m.navigateCalls++
	m.lastURL = rawURL
	return m.navigateErr
}

func (m *mockBrowserAdapter) Snapshot(_ context.Context) (domain.BrowserSnapshot, error) {
	m.snapshotCalls++
	if m.snapshotErr != nil {
		return domain.BrowserSnapshot{}, m.snapshotErr
	}
	return m.snapshot, nil
}

func (m *mockBrowserAdapter) Close(_ context.Context) error {
	m.closeCalls++
	return m.closeErr
}

func TestHandler_OpenURL(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name        string
		adapter     *mockBrowserAdapter
		url         string
		wantErr     bool
		errContains string
		check       func(t *testing.T, adapter *mockBrowserAdapter, artifact domain.Artifact)
	}{
		{
			name: "Success - start navigate snapshot and normalize markdown",
			adapter: &mockBrowserAdapter{
				snapshot: domain.BrowserSnapshot{
					URL:         "https://example.com",
					Title:       "Example Domain",
					TextPreview: "Example content",
					CapturedAt:  now,
				},
			},
			url: "https://example.com",
			check: func(t *testing.T, adapter *mockBrowserAdapter, artifact domain.Artifact) {
				t.Helper()
				if adapter.startCalls != 1 || adapter.navigateCalls != 1 || adapter.snapshotCalls != 1 {
					t.Fatalf("unexpected call counts: start=%d navigate=%d snapshot=%d", adapter.startCalls, adapter.navigateCalls, adapter.snapshotCalls)
				}
				if artifact.Kind != domain.ContentTypeMarkdown {
					t.Fatalf("expected markdown artifact kind, got %q", artifact.Kind)
				}
				if !strings.Contains(artifact.Content, "## Example Domain") {
					t.Fatalf("expected title in normalized content, got %q", artifact.Content)
				}
				if !strings.Contains(artifact.Content, "### Preview") {
					t.Fatalf("expected preview header in normalized content")
				}
			},
		},
		{
			name: "Failure - start returns error",
			adapter: &mockBrowserAdapter{
				startErr: fmt.Errorf("boom"),
			},
			url:         "https://example.com",
			wantErr:     true,
			errContains: "start browser adapter: boom",
		},
		{
			name: "Failure - navigate returns error",
			adapter: &mockBrowserAdapter{
				navigateErr: fmt.Errorf("bad url"),
			},
			url:         "https://example.com",
			wantErr:     true,
			errContains: "navigate browser: bad url",
		},
		{
			name: "Failure - snapshot returns error",
			adapter: &mockBrowserAdapter{
				snapshotErr: fmt.Errorf("snapshot failed"),
			},
			url:         "https://example.com",
			wantErr:     true,
			errContains: "capture browser snapshot: snapshot failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHandler(tc.adapter)
			artifact, err := h.OpenURL(context.Background(), tc.url)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if tc.errContains != "" && err.Error() != tc.errContains {
					t.Fatalf("expected error %q, got %q", tc.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, tc.adapter, artifact)
			}
		})
	}
}

func TestHandler_OpenURL_ReusesStartedLifecycle(t *testing.T) {
	adapter := &mockBrowserAdapter{
		snapshot: domain.BrowserSnapshot{
			URL:         "https://example.com",
			Title:       "Title",
			TextPreview: "Preview",
			CapturedAt:  time.Now().UTC(),
		},
	}
	h := NewHandler(adapter)

	_, err := h.OpenURL(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected first open error: %v", err)
	}
	_, err = h.OpenURL(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected second open error: %v", err)
	}

	if adapter.startCalls != 1 {
		t.Fatalf("expected start once across two opens, got %d", adapter.startCalls)
	}
	if adapter.navigateCalls != 2 || adapter.snapshotCalls != 2 {
		t.Fatalf("expected navigate/snapshot twice, got navigate=%d snapshot=%d", adapter.navigateCalls, adapter.snapshotCalls)
	}
}

func TestHandler_Close(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *mockBrowserAdapter
		wantErr   bool
		checkFunc func(t *testing.T, adapter *mockBrowserAdapter)
	}{
		{
			name: "Success - close after open",
			setup: func() *mockBrowserAdapter {
				return &mockBrowserAdapter{
					snapshot: domain.BrowserSnapshot{
						URL:         "https://example.com",
						Title:       "Example",
						TextPreview: "Preview",
						CapturedAt:  time.Now().UTC(),
					},
				}
			},
			checkFunc: func(t *testing.T, adapter *mockBrowserAdapter) {
				t.Helper()
				if adapter.closeCalls != 1 {
					t.Fatalf("expected close to be called once, got %d", adapter.closeCalls)
				}
			},
		},
		{
			name: "Success - close before start is noop",
			setup: func() *mockBrowserAdapter {
				return &mockBrowserAdapter{}
			},
			checkFunc: func(t *testing.T, adapter *mockBrowserAdapter) {
				t.Helper()
				if adapter.closeCalls != 0 {
					t.Fatalf("expected close not called before start, got %d", adapter.closeCalls)
				}
			},
		},
		{
			name: "Failure - adapter close error propagates",
			setup: func() *mockBrowserAdapter {
				return &mockBrowserAdapter{
					closeErr: fmt.Errorf("close failed"),
					snapshot: domain.BrowserSnapshot{
						URL:         "https://example.com",
						Title:       "Example",
						TextPreview: "Preview",
						CapturedAt:  time.Now().UTC(),
					},
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			adapter := tc.setup()
			h := NewHandler(adapter)

			if tc.name != "Success - close before start is noop" {
				_, _ = h.OpenURL(context.Background(), "https://example.com")
			}

			err := h.Close(context.Background())
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
			} else if err != nil {
				t.Fatalf("unexpected close error: %v", err)
			}

			if tc.checkFunc != nil {
				tc.checkFunc(t, adapter)
			}
		})
	}
}
