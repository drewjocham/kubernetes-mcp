package logging

import (
	"context"
	"fmt"
	"log/slog"
)

// multiHandler forwards log records to multiple slog.Handler instances.
type multiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a slog.Handler that forwards logs to all provided handlers.
func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	// Filter out nil handlers
	filtered := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			filtered = append(filtered, h)
		}
	}
	return &multiHandler{handlers: filtered}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var errs []error
	for _, h := range m.handlers {
		if h.Enabled(ctx, record.Level) {
			if err := h.Handle(ctx, record); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors in handlers: %v", errs)
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

// levelFilterHandler wraps a slog.Handler to apply level filtering.
type levelFilterHandler struct {
	handler slog.Handler
	level   slog.Level
}

// NewLevelFilterHandler creates a slog.Handler that filters log records by level.
func NewLevelFilterHandler(handler slog.Handler, level slog.Level) slog.Handler {
	return &levelFilterHandler{
		handler: handler,
		level:   level,
	}
}

func (h *levelFilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *levelFilterHandler) Handle(ctx context.Context, record slog.Record) error {
	return h.handler.Handle(ctx, record)
}

func (h *levelFilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelFilterHandler{
		handler: h.handler.WithAttrs(attrs),
		level:   h.level,
	}
}

func (h *levelFilterHandler) WithGroup(name string) slog.Handler {
	return &levelFilterHandler{
		handler: h.handler.WithGroup(name),
		level:   h.level,
	}
}
