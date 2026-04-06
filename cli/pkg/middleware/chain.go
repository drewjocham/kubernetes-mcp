package middleware

import "github.com/spf13/cobra"

// MiddlewareFunc is a function that can modify cobra command execution
type MiddlewareFunc func(*cobra.Command, []string)

// Chain combines multiple middleware functions into a single function
func Chain(middlewares ...MiddlewareFunc) MiddlewareFunc {
	return func(cmd *cobra.Command, args []string) {
		// Store original RunE
		originalRunE := cmd.RunE
		originalRun := cmd.Run

		// Apply middlewares in reverse order so first in chain executes first
		for i := len(middlewares) - 1; i >= 0; i-- {
			middleware := middlewares[i]

			// Create a wrapper that preserves the middleware chain
			currentRunE := cmd.RunE
			currentRun := cmd.Run

			wrapper := func(cmd *cobra.Command, args []string) {
				middleware(cmd, args)
			}

			// Apply wrapper to command
			wrapper(cmd, args)

			// Restore the run functions that might have been modified by middleware
			if cmd.RunE == nil {
				cmd.RunE = currentRunE
			}
			if cmd.Run == nil {
				cmd.Run = currentRun
			}
		}

		// Ensure we preserve the original execution functions
		if cmd.RunE == nil {
			cmd.RunE = originalRunE
		}
		if cmd.Run == nil {
			cmd.Run = originalRun
		}
	}
}

// ApplyToCommand applies middleware to a cobra command
func ApplyToCommand(cmd *cobra.Command, middleware MiddlewareFunc) {
	// Store current PreRun functions
	currentPreRun := cmd.PersistentPreRunE
	currentPreRunNoError := cmd.PersistentPreRun

	if currentPreRun != nil {
		cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			middleware(cmd, args)
			return currentPreRun(cmd, args)
		}
	} else if currentPreRunNoError != nil {
		cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			middleware(cmd, args)
			currentPreRunNoError(cmd, args)
		}
	} else {
		cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
			middleware(cmd, args)
		}
	}
}

// ApplyToCommandE applies middleware to a cobra command with error handling
func ApplyToCommandE(cmd *cobra.Command, middleware MiddlewareFunc) {
	// Store current PreRun function
	currentPreRun := cmd.PersistentPreRunE

	if currentPreRun != nil {
		cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			middleware(cmd, args)
			return currentPreRun(cmd, args)
		}
	} else {
		cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			middleware(cmd, args)
			return nil
		}
	}
}
