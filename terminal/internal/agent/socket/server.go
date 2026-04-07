package socket

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"kube-watcher/terminal/internal/domain"
)

type Command struct {
	Action   string          `json:"action"`
	Text     string          `json:"text,omitempty"`
	URL      string          `json:"url,omitempty"`
	SplitID  string          `json:"split_id,omitempty"`
	Artifact domain.Artifact `json:"artifact,omitempty"`
}

type Request struct {
	Action   string          `json:"action"`
	Text     string          `json:"text,omitempty"`
	URL      string          `json:"url,omitempty"`
	SplitID  string          `json:"split_id,omitempty"`
	Artifact domain.Artifact `json:"artifact,omitempty"`
}

type Response struct {
	OK     bool            `json:"ok"`
	Error  string          `json:"error,omitempty"`
	Blocks []domain.Block  `json:"blocks,omitempty"`
	Events []EventEnvelope `json:"events,omitempty"`
	Data   map[string]any  `json:"data,omitempty"`
}

type EventEnvelope struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

type Server struct {
	socketPath string
	listener   net.Listener

	getBlocks func() []domain.Block
	sendMsg   func(any)
	openURL   func(rawURL string) (domain.Artifact, error)

	mu     sync.Mutex
	events []EventEnvelope
}

func NewServer(socketPath string, getBlocks func() []domain.Block, sendMsg func(any)) *Server {
	return &Server{
		socketPath: socketPath,
		getBlocks:  getBlocks,
		sendMsg:    sendMsg,
		events:     make([]EventEnvelope, 0, 32),
	}
}

func (s *Server) SetOpenURLHandler(handler func(rawURL string) (domain.Artifact, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.openURL = handler
}

func (s *Server) captureOpenURLArtifact(rawURL string) (domain.Artifact, error) {
	s.mu.Lock()
	handler := s.openURL
	s.mu.Unlock()
	if handler == nil {
		return domain.Artifact{}, fmt.Errorf("open url handler is not set")
	}
	return handler(rawURL)
}

const (
	socketFileMode = 0o600
	maxTextLength  = 4096
	maxURLLength   = 2048
)

var supportedActions = map[string]struct{}{
	"get_blocks":       {},
	"get_events":       {},
	"set_ghost_text":   {},
	"clear_ghost_text": {},
	"pin_artifact":     {},
	"create_split":     {},
	"focus_split":      {},
	"send_input":       {},
	"open_url":         {},
}

func (s *Server) Start(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(s.socketPath), 0o755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	_ = os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listen on unix socket: %w", err)
	}
	s.listener = listener
	if err := os.Chmod(s.socketPath, socketFileMode); err != nil {
		return fmt.Errorf("set socket permission: %w", err)
	}

	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	go s.acceptLoop()
	return nil
}

func (s *Server) Close() error {
	s.mu.Lock()
	listener := s.listener
	s.listener = nil
	s.mu.Unlock()

	if listener == nil {
		return nil
	}
	if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
		return fmt.Errorf("close unix listener: %w", err)
	}
	_ = os.Remove(s.socketPath)
	return nil
}

func (s *Server) RecordEvent(evt EventEnvelope) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, evt)
	if len(s.events) > 200 {
		s.events = s.events[len(s.events)-200:]
	}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	decoder := json.NewDecoder(bufio.NewReader(conn))
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		_ = encoder.Encode(Response{OK: false, Error: fmt.Sprintf("decode request: %v", err)})
		return
	}
	if err := normalizeAndValidateRequest(&req); err != nil {
		_ = encoder.Encode(Response{OK: false, Error: err.Error()})
		return
	}

	switch req.Action {
	case "get_blocks":
		_ = encoder.Encode(Response{OK: true, Blocks: s.getBlocks()})
	case "get_events":
		_ = encoder.Encode(Response{OK: true, Events: s.readEvents()})
	case "set_ghost_text":
		s.sendMsg(domain.AgentGhostTextMsg{Text: req.Text})
		s.RecordEvent(newEvent("ghost_applied", map[string]any{
			"text_length": len(req.Text),
		}))
		_ = encoder.Encode(Response{OK: true})
	case "clear_ghost_text":
		s.sendMsg(domain.AgentClearGhostTextMsg{})
		_ = encoder.Encode(Response{OK: true})
	case "pin_artifact":
		s.sendMsg(domain.AgentPinArtifactMsg{Artifact: req.Artifact})
		s.RecordEvent(newEvent("artifact_pinned", map[string]any{
			"artifact_id": req.Artifact.ID,
			"title":       req.Artifact.Title,
			"kind":        req.Artifact.Kind,
		}))
		_ = encoder.Encode(Response{OK: true})
	case "create_split":
		s.sendMsg(domain.AgentCreateSplitMsg{SplitID: req.SplitID})
		s.RecordEvent(newEvent("split_created", map[string]any{
			"split_id": req.SplitID,
		}))
		_ = encoder.Encode(Response{OK: true, Data: map[string]any{"split_id": req.SplitID}})
	case "focus_split":
		s.sendMsg(domain.AgentFocusSplitMsg{SplitID: req.SplitID})
		s.RecordEvent(newEvent("split_focused", map[string]any{
			"split_id": req.SplitID,
		}))
		_ = encoder.Encode(Response{OK: true})
	case "send_input":
		s.sendMsg(domain.AgentSendInputMsg{Text: req.Text})
		s.RecordEvent(newEvent("command_executed", map[string]any{
			"command": req.Text,
		}))
		_ = encoder.Encode(Response{OK: true})
	case "open_url":
		if artifact, err := s.captureOpenURLArtifact(req.URL); err == nil {
			s.sendMsg(domain.AgentPinArtifactMsg{Artifact: artifact})
			s.RecordEvent(newEvent("artifact_pinned", map[string]any{
				"artifact_id": artifact.ID,
				"title":       artifact.Title,
				"kind":        artifact.Kind,
			}))
			s.RecordEvent(newEvent("url_open_requested", map[string]any{
				"url": req.URL,
			}))
			_ = encoder.Encode(Response{OK: true, Data: map[string]any{
				"artifact_id": artifact.ID,
			}})
			return
		}
		s.sendMsg(domain.AgentOpenURLMsg{URL: req.URL})
		s.RecordEvent(newEvent("url_open_requested", map[string]any{
			"url": req.URL,
		}))
		_ = encoder.Encode(Response{OK: true})
	default:
		_ = encoder.Encode(Response{OK: false, Error: "unsupported action"})
	}
}

func (s *Server) readEvents() []EventEnvelope {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]EventEnvelope, len(s.events))
	copy(out, s.events)
	return out
}

func normalizeAndValidateRequest(req *Request) error {
	req.Action = strings.TrimSpace(req.Action)
	req.Text = strings.TrimSpace(req.Text)
	req.URL = strings.TrimSpace(req.URL)
	req.SplitID = strings.TrimSpace(req.SplitID)

	if req.Action == "" {
		return fmt.Errorf("action is required")
	}
	if _, ok := supportedActions[req.Action]; !ok {
		return fmt.Errorf("unsupported action")
	}

	switch req.Action {
	case "set_ghost_text":
		if len(req.Text) > maxTextLength {
			req.Text = req.Text[:maxTextLength]
		}
	case "send_input":
		if req.Text == "" {
			return fmt.Errorf("text is required")
		}
		if len(req.Text) > maxTextLength {
			req.Text = req.Text[:maxTextLength]
		}
	case "pin_artifact":
		if req.Artifact.ID == "" {
			return fmt.Errorf("artifact.id is required")
		}
		if req.Artifact.Title == "" {
			req.Artifact.Title = req.Artifact.ID
		}
		if req.Artifact.Kind == "" {
			req.Artifact.Kind = domain.ContentTypePlainText
		}
	case "create_split":
		if req.SplitID == "" {
			req.SplitID = fmt.Sprintf("split-%d", time.Now().UnixNano())
		}
	case "focus_split":
		if req.SplitID == "" {
			return fmt.Errorf("split_id is required")
		}
	case "open_url":
		if req.URL == "" {
			return fmt.Errorf("url is required")
		}
		if len(req.URL) > maxURLLength {
			return fmt.Errorf("url is too long")
		}
		u, err := url.Parse(req.URL)
		if err != nil {
			return fmt.Errorf("invalid url")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("only http/https urls are supported")
		}
	}

	return nil
}

func newEvent(eventType string, payload map[string]any) EventEnvelope {
	return EventEnvelope{
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		Payload:   payload,
	}
}
