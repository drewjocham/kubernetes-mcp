// Package cli provides the kube-watcher command-line interface.
// This package re-exports the cmd package for backward compatibility.
package cli

import "kube-watcher/cli/cmd"

// Execute runs the CLI. This is the main entry point for the CLI.
func Execute() {
	cmd.Execute()
}
