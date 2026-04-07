package terminal

import (
	"context"
	"io"
	"sync"
	"testing"
)

type mockAdapter struct {
	session Session
	err     error
}

func (a *mockAdapter) Start(_ string) (Session, error) {
	if a.err != nil {
		return nil, a.err
	}
	return a.session, nil
}

type mockSession struct {
	mu         sync.Mutex
	closeCalls int
	waitErr    error
}

func (s *mockSession) Read(_ []byte) (int, error)  { return 0, io.EOF }
func (s *mockSession) Write(p []byte) (int, error) { return len(p), nil }
func (s *mockSession) Resize(_, _ int) error       { return nil }
func (s *mockSession) Wait() error                 { return s.waitErr }
func (s *mockSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closeCalls++
	return nil
}

func (s *mockSession) CloseCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeCalls
}

func TestHandler_Close_GracefulAndIdempotent(t *testing.T) {
	tests := []struct {
		name           string
		startFirst     bool
		expectedCloses int
	}{
		{
			name:           "Close without start is noop",
			startFirst:     false,
			expectedCloses: 0,
		},
		{
			name:           "Close after start closes session once",
			startFirst:     true,
			expectedCloses: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			session := &mockSession{}
			h := NewHandler(&mockAdapter{session: session})

			if tc.startFirst {
				err := h.Start(context.Background(), "/bin/sh", func(any) {})
				if err != nil {
					t.Fatalf("unexpected start error: %v", err)
				}
			}

			if err := h.Close(); err != nil {
				t.Fatalf("unexpected close error: %v", err)
			}
			if err := h.Close(); err != nil {
				t.Fatalf("second close should be idempotent: %v", err)
			}

			if got := session.CloseCalls(); got != tc.expectedCloses {
				t.Fatalf("expected close calls %d, got %d", tc.expectedCloses, got)
			}
		})
	}
}
