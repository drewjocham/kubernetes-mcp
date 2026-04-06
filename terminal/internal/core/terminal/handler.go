package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"kube-watcher/terminal/internal/domain"
)

type Session interface {
	Read(p []byte) (int, error)
	Write(p []byte) (int, error)
	Resize(cols, rows int) error
	Close() error
	Wait() error
}

type Adapter interface {
	Start(shellPath string) (Session, error)
}

type Handler struct {
	adapter Adapter

	mu      sync.Mutex
	session Session
	cancel  context.CancelFunc
}

func NewHandler(adapter Adapter) *Handler {
	return &Handler{
		adapter: adapter,
	}
}

func (h *Handler) Start(ctx context.Context, shellPath string, send func(any)) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session != nil {
		return nil
	}

	session, err := h.adapter.Start(shellPath)
	if err != nil {
		return fmt.Errorf("start pty session: %w", err)
	}

	loopCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	h.session = session

	go h.readLoop(loopCtx, session, send)
	go h.waitLoop(session, send)

	return nil
}

func (h *Handler) Write(data []byte) error {
	h.mu.Lock()
	session := h.session
	h.mu.Unlock()

	if session == nil {
		return nil
	}

	if _, err := session.Write(data); err != nil {
		return fmt.Errorf("write to pty: %w", err)
	}

	return nil
}

func (h *Handler) Resize(cols, rows int) error {
	h.mu.Lock()
	session := h.session
	h.mu.Unlock()

	if session == nil {
		return nil
	}

	if err := session.Resize(cols, rows); err != nil {
		return fmt.Errorf("resize pty: %w", err)
	}

	return nil
}

func (h *Handler) Close() error {
	h.mu.Lock()
	session := h.session
	cancel := h.cancel
	h.session = nil
	h.cancel = nil
	h.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if session == nil {
		return nil
	}

	if err := session.Close(); err != nil {
		return fmt.Errorf("close pty session: %w", err)
	}

	return nil
}

func (h *Handler) readLoop(ctx context.Context, session Session, send func(any)) {
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := session.Read(buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			send(domain.PTYOutputMsg(chunk))
		}

		if err != nil {
			if err == io.EOF {
				send(domain.PTYExitMsg{Err: nil, ExitCode: 0})
				return
			}
			send(domain.PTYExitMsg{Err: fmt.Errorf("pty read loop: %w", err), ExitCode: 1})
			return
		}
	}
}

func (h *Handler) waitLoop(session Session, send func(any)) {
	if err := session.Wait(); err != nil {
		send(domain.PTYExitMsg{
			Err:      fmt.Errorf("pty process exited: %w", err),
			ExitCode: exitCodeFromErr(err),
		})
		return
	}
	send(domain.PTYExitMsg{Err: nil, ExitCode: 0})
}

func exitCodeFromErr(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
