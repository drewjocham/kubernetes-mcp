package middleware

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slog"
)

// LoggingConfig holds configuration for logging middleware
type LoggingConfig struct {
	Level  slog.Level
	Format string // "text" or "json"
}

// DefaultLoggingConfig returns default logging configuration
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:  slog.LevelInfo,
		Format: "text",
	}
}

// NewLoggingMiddleware creates logging middleware for cobra commands
func NewLoggingMiddleware(config LoggingConfig) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		// Setup logger based on config
		var handler slog.Handler
		opts := &slog.HandlerOptions{
			Level: config.Level,
		}

		if config.Format == "json" {
			handler = slog.NewJSONHandler(os.Stdout, opts)
		} else {
			handler = slog.NewTextHandler(os.Stdout, opts)
		}

		logger := slog.New(handler)

		// Store logger in command context
		ctx := cmd.Context()
		if ctx == nil {
			ctx = cmd.Context()
		}
		// Note: In real implementation, we'd set logger in context
		// For now, we'll just log command execution

		start := time.Now()
		logger.Info("command started",
			"command", cmd.CommandPath(),
			"args", args,
			"start_time", start.Format(time.RFC3339),
		)

		// Set up post-run logging
		originalRun := cmd.RunE
		if originalRun != nil {
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				err := originalRun(cmd, args)
				duration := time.Since(start)

				if err != nil {
					logger.Error("command failed",
						"command", cmd.CommandPath(),
						"duration", duration.String(),
						"error", err.Error(),
					)
				} else {
					logger.Info("command completed",
						"command", cmd.CommandPath(),
						"duration", duration.String(),
						"success", true,
					)
				}
				return err
			}
		}
	}
}

// SimpleLogger provides a simple logging wrapper
func SimpleLogger(level slog.Level) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		fmt.Printf("[%s] Executing command: %s\n", time.Now().Format("15:04:05"), cmd.CommandPath())
		if len(args) > 0 {
			fmt.Printf("[%s] Args: %v\n", time.Now().Format("15:04:05"), args)
		}
	}
}
