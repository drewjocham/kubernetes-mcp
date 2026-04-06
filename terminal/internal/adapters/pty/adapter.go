package pty

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/creack/pty"

	terminalcore "kube-watcher/terminal/internal/core/terminal"
)

type Adapter struct{}

func NewAdapter() *Adapter {
	return &Adapter{}
}

type Session struct {
	cmd  *exec.Cmd
	file *os.File
}

func (a *Adapter) Start(shellPath string) (terminalcore.Session, error) {
	cmd := exec.Command(shellPath)
	file, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("pty start shell %q: %w", shellPath, err)
	}

	return &Session{
		cmd:  cmd,
		file: file,
	}, nil
}

func (s *Session) Read(p []byte) (int, error) {
	return s.file.Read(p)
}

func (s *Session) Write(p []byte) (int, error) {
	return s.file.Write(p)
}

func (s *Session) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	if err := pty.Setsize(s.file, &pty.Winsize{
		Cols: uint16(cols),
		Rows: uint16(rows),
	}); err != nil {
		return fmt.Errorf("set pty size to %dx%d: %w", cols, rows, err)
	}
	return nil
}

func (s *Session) Close() error {
	if err := s.file.Close(); err != nil {
		return fmt.Errorf("close pty file: %w", err)
	}
	return nil
}

func (s *Session) Wait() error {
	return s.cmd.Wait()
}
