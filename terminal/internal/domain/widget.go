package domain

import "context"

type Widget interface {
	ID() string
	Title() string
	Refresh(ctx context.Context) error
	View() string
}

type WidgetRefreshMsg struct{}
