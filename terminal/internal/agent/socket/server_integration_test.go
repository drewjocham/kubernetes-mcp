package socket

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"kube-watcher/terminal/internal/domain"
)

func TestServer_StartAndSocketActions_Integration(t *testing.T) {
	socketPath := shortSocketPath(t)

	var sent []any
	srv := NewServer(
		socketPath,
		func() []domain.Block {
			return []domain.Block{{ID: "b1", Command: "echo hi"}}
		},
		func(msg any) {
			sent = append(sent, msg)
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		t.Fatalf("start server: %v", err)
	}

	waitForSocket(t, socketPath)
	stat, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if gotPerm := stat.Mode().Perm(); gotPerm != 0o600 {
		t.Fatalf("expected socket permissions 0600, got %o", gotPerm)
	}

	resp := callSocketRequest(t, socketPath, Request{Action: "get_blocks"})
	if !resp.OK || len(resp.Blocks) != 1 {
		t.Fatalf("expected blocks response, got %+v", resp)
	}

	resp = callSocketRequest(t, socketPath, Request{Action: "set_ghost_text", Text: "go test ./..."})
	if !resp.OK {
		t.Fatalf("expected set_ghost_text OK response, got %+v", resp)
	}
	if len(sent) == 0 {
		t.Fatalf("expected at least one outbound message")
	}
	if _, ok := sent[len(sent)-1].(domain.AgentGhostTextMsg); !ok {
		t.Fatalf("expected latest message to be AgentGhostTextMsg, got %T", sent[len(sent)-1])
	}

	resp = callSocketRequest(t, socketPath, Request{Action: "get_events"})
	if !resp.OK || len(resp.Events) == 0 {
		t.Fatalf("expected events response with entries, got %+v", resp)
	}

	if err := srv.Close(); err != nil {
		t.Fatalf("close server: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("second close should be idempotent, got: %v", err)
	}
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Fatalf("expected socket path to be removed after close, stat err=%v", err)
	}
}

func waitForSocket(t *testing.T, socketPath string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(socketPath); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("socket was not created within deadline: %s", socketPath)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func callSocketRequest(t *testing.T, socketPath string, req Request) Response {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial unix socket: %v", err)
	}
	defer func() { _ = conn.Close() }()

	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	if err := enc.Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp.OK && resp.Error == "" {
		t.Fatalf("response not ok but empty error payload: %+v", resp)
	}
	return resp
}

func TestServer_CloseWithoutStart_IsNoop(t *testing.T) {
	srv := NewServer(
		filepath.Join(t.TempDir(), "kw-terminal-agent.sock"),
		func() []domain.Block { return nil },
		func(any) {},
	)
	if err := srv.Close(); err != nil {
		t.Fatalf("close without start should be noop, got %v", err)
	}
}

func TestServer_UnsupportedAction_ReturnsError(t *testing.T) {
	socketPath := shortSocketPath(t)
	srv := NewServer(
		socketPath,
		func() []domain.Block { return nil },
		func(any) {},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() { _ = srv.Close() }()
	waitForSocket(t, socketPath)

	resp := callSocketRequest(t, socketPath, Request{Action: "bogus_action"})
	if resp.OK {
		t.Fatalf("expected unsupported action response to fail, got %+v", resp)
	}
	if resp.Error == "" {
		t.Fatalf("expected non-empty error for unsupported action")
	}
	if got, want := resp.Error, "unsupported action"; got != want {
		t.Fatalf("expected error %q, got %q", want, got)
	}
}

func TestServer_OpenURLHandlerError_FallsBackToAgentMessage(t *testing.T) {
	socketPath := shortSocketPath(t)
	var sent []any

	srv := NewServer(
		socketPath,
		func() []domain.Block { return nil },
		func(msg any) {
			sent = append(sent, msg)
		},
	)
	srv.SetOpenURLHandler(func(rawURL string) (domain.Artifact, error) {
		return domain.Artifact{}, fmt.Errorf("snapshot failure for %s", rawURL)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := srv.Start(ctx); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() { _ = srv.Close() }()
	waitForSocket(t, socketPath)

	resp := callSocketRequest(t, socketPath, Request{Action: "open_url", URL: "https://example.com"})
	if !resp.OK {
		t.Fatalf("expected open_url response OK with fallback, got %+v", resp)
	}
	if len(sent) != 1 {
		t.Fatalf("expected one fallback outbound message, got %d", len(sent))
	}
	if _, ok := sent[0].(domain.AgentOpenURLMsg); !ok {
		t.Fatalf("expected fallback AgentOpenURLMsg, got %T", sent[0])
	}
}

func shortSocketPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(os.TempDir(), fmt.Sprintf("kwt-%d.sock", time.Now().UnixNano()))
}
