package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"kube-watcher/terminal/internal/domain"
)

var (
	titleRegexp = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	tagRegexp   = regexp.MustCompile(`(?is)<[^>]+>`)
	spaceRegexp = regexp.MustCompile(`\s+`)
)

type Adapter struct {
	httpClient *http.Client

	mu         sync.Mutex
	started    bool
	currentURL string
}

func NewAdapter() *Adapter {
	return &Adapter{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (a *Adapter) Start(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.started = true
	return nil
}

func (a *Adapter) Navigate(_ context.Context, rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported url scheme %q", parsed.Scheme)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.started {
		return fmt.Errorf("adapter not started")
	}

	a.currentURL = parsed.String()
	return nil
}

func (a *Adapter) Snapshot(ctx context.Context) (domain.BrowserSnapshot, error) {
	a.mu.Lock()
	currentURL := a.currentURL
	started := a.started
	a.mu.Unlock()

	if !started {
		return domain.BrowserSnapshot{}, fmt.Errorf("adapter not started")
	}
	if currentURL == "" {
		return domain.BrowserSnapshot{}, fmt.Errorf("no url is currently loaded")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
	if err != nil {
		return domain.BrowserSnapshot{}, fmt.Errorf("build snapshot request: %w", err)
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return domain.BrowserSnapshot{}, fmt.Errorf("fetch page: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return domain.BrowserSnapshot{}, fmt.Errorf("read page response: %w", err)
	}

	html := string(body)
	title := extractTitle(html)
	preview := extractPreview(html, 1200)

	return domain.BrowserSnapshot{
		URL:         currentURL,
		Title:       title,
		TextPreview: preview,
		CapturedAt:  time.Now().UTC(),
	}, nil
}

func (a *Adapter) Close(_ context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.started = false
	a.currentURL = ""
	return nil
}

func extractTitle(html string) string {
	matches := titleRegexp.FindStringSubmatch(html)
	if len(matches) < 2 {
		return ""
	}
	return normalizeSpaces(tagRegexp.ReplaceAllString(matches[1], " "))
}

func extractPreview(html string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 1200
	}
	preview := normalizeSpaces(tagRegexp.ReplaceAllString(html, " "))
	if preview == "" {
		return ""
	}
	if len(preview) <= maxLen {
		return preview
	}
	return strings.TrimSpace(preview[:maxLen]) + "..."
}

func normalizeSpaces(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	return spaceRegexp.ReplaceAllString(trimmed, " ")
}
