package middleware

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// ErrorHandlerConfig holds configuration for error handling middleware
type ErrorHandlerConfig struct {
	ExitOnError bool
	ShowStack   bool
	Format      string // "text" or "json"
}

// DefaultErrorHandlerConfig returns default error handling configuration
func DefaultErrorHandlerConfig() ErrorHandlerConfig {
	return ErrorHandlerConfig{
		ExitOnError: true,
		ShowStack:   false,
		Format:      "text",
	}
}

// NewErrorHandlerMiddleware creates error handling middleware for cobra commands
func NewErrorHandlerMiddleware(config ErrorHandlerConfig) func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		// Wrap the command's RunE function
		originalRun := cmd.RunE
		if originalRun != nil {
			cmd.RunE = func(cmd *cobra.Command, args []string) error {
				err := originalRun(cmd, args)
				if err != nil {
					return handleError(err, config, cmd)
				}
				return nil
			}
		}

		// Also wrap Run (non-error returning version)
		originalRunNoError := cmd.Run
		if originalRunNoError != nil {
			cmd.Run = func(cmd *cobra.Command, args []string) {
				// Can't catch panics in Run, but we can wrap execution
				// For now, just call original
				originalRunNoError(cmd, args)
			}
		}
	}
}

// handleError processes errors according to configuration
func handleError(err error, config ErrorHandlerConfig, cmd *cobra.Command) error {
	if config.Format == "json" {
		// JSON error output
		fmt.Printf(`{"error": %q, "command": %q}\n`, err.Error(), cmd.CommandPath())
	} else {
		// Text error output
		fmt.Fprintf(os.Stderr, "Error executing command %q: %v\n", cmd.CommandPath(), err)
	}

	if config.ExitOnError {
		os.Exit(1)
	}

	return err // Return error for further handling
}

// PanicRecoveryMiddleware recovers from panics and converts them to errors
func PanicRecoveryMiddleware() func(*cobra.Command, []string) {
	return func(cmd *cobra.Command, args []string) {
		originalRun := cmd.RunE
		if originalRun != nil {
			cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("panic recovered: %v", r)
					}
				}()
				return originalRun(cmd, args)
			}
		}
	}
}
