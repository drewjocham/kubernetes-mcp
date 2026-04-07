package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// newOTLPHandler initializes the OpenTelemetry log provider if OTEL_EXPORTER_OTLP_ENDPOINT is set.
// Returns a slog.Handler that sends logs to OTLP and a cleanup function.
func newOTLPHandler(serviceName string) (slog.Handler, func(), error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// OTLP not configured
		return nil, func() {}, nil
	}

	// Create resource with service name
	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	)

	// Create OTLP log exporter
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OTLP log exporter: %w", err)
	}

	// Create logger provider with batch processor
	processor := log.NewBatchProcessor(exporter)
	provider := log.NewLoggerProvider(
		log.WithResource(res),
		log.WithProcessor(processor),
	)

	// Create slog handler using otelslog bridge
	handler := otelslog.NewHandler(
		serviceName,
		otelslog.WithLoggerProvider(provider),
		otelslog.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)

	cleanup := func() {
		// Shutdown provider with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = provider.Shutdown(ctx)
	}

	return handler, cleanup, nil
}

// NewOTLPLogger creates a new slog.Logger that exports logs via OpenTelemetry OTLP.
// If OTEL_EXPORTER_OTLP_ENDPOINT is not set, returns nil handler and no-op cleanup.
func NewOTLPLogger(serviceName string, debug bool) (*slog.Logger, func(), error) {
	handler, cleanup, err := newOTLPHandler(serviceName)
	if err != nil {
		return nil, nil, err
	}
	if handler == nil {
		// OTLP not configured, return no-op logger
		return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})), func() {}, nil
	}

	// Set log level based on debug flag
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	// Wrap handler to apply level filtering
	levelHandler := NewLevelFilterHandler(handler, level)

	logger := slog.New(levelHandler)
	return logger, cleanup, nil
}
