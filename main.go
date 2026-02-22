package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	cmd := exec.Command("go", append([]string{"run", filepath.Join(dir, "mcp", "cmd", "server")}, os.Args[1:]...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			os.Exit(exitError.ExitCode())
		}
		panic(err)
	}
}
