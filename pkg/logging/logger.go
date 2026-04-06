package logging

import (
	"io"
	"log/slog"
	"os"
)

var (
	otlpCleanup func()
)

// Shutdown cleans up OTLP logging resources.
// Should be called before application exit.
func Shutdown() {
	if otlpCleanup != nil {
		otlpCleanup()
		otlpCleanup = nil
	}
}

func getServiceName() string {
	if name := os.Getenv("OTEL_SERVICE_NAME"); name != "" {
		return name
	}
	return "kube-watcher"
}

func New(debug bool, logFile string) (*slog.Logger, error) {
	var level slog.Level
	if debug {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	var output io.Writer = os.Stdout
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
		if err != nil {
			return nil, err
		}
		output = f
	}

	// Create JSON handler for local output
	jsonHandler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	})

	// Create OTLP handler if endpoint is configured
	otlpHandler, cleanup, err := newOTLPHandler(getServiceName())
	if err != nil {
		return nil, err
	}
	if otlpHandler != nil {
		// Store cleanup for later shutdown
		otlpCleanup = cleanup
		// Apply level filtering to OTLP handler
		filteredOTLPHandler := NewLevelFilterHandler(otlpHandler, level)
		// Combine both handlers
		handler := NewMultiHandler(jsonHandler, filteredOTLPHandler)
		logger := slog.New(handler)
		slog.SetDefault(logger)
		return logger, nil
	}

	// Only JSON handler
	logger := slog.New(jsonHandler)
	slog.SetDefault(logger)
	return logger, nil
}
