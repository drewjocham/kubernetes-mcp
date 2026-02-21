package logging

import (
	"io"
	"log/slog"
	"os"
)

func New(debug bool, logFile string) (*slog.Logger, error) {
	var level slog.Level
	if debug {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	var output io.Writer = os.Stdout
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, err
		}
		output = f
	}

	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{
		AddSource: true,
		Level:     level,
	})

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger, nil
}
