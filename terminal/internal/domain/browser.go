package domain

import (
	"context"
	"time"
)

type BrowserSnapshot struct {
	URL         string
	Title       string
	TextPreview string
	CapturedAt  time.Time
}

type BrowserAdapter interface {
	Start(ctx context.Context) error
	Navigate(ctx context.Context, rawURL string) error
	Snapshot(ctx context.Context) (BrowserSnapshot, error)
	Close(ctx context.Context) error
}
