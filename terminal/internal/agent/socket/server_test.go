package socket

import (
	"encoding/json"
	"net"
	"testing"

	"kube-watcher/terminal/internal/domain"
)

func TestNormalizeAndValidateRequest(t *testing.T) {
	tests := []struct {
		name        string
		input       Request
		wantErr     bool
		errContains string
		check       func(t *testing.T, got Request)
	}{
		{
			name: "Success - send_input with text",
			input: Request{
				Action: "send_input",
				Text:   "go test ./...",
			},
			check: func(t *testing.T, got Request) {
				t.Helper()
				if got.Text != "go test ./..." {
					t.Fatalf("expected normalized text to match, got %q", got.Text)
				}
			},
		},
		{
			name: "Failure - unsupported action",
			input: Request{
				Action: "unknown",
			},
			wantErr:     true,
			errContains: "unsupported action",
		},
		{
			name: "Failure - send_input missing text",
			input: Request{
				Action: "send_input",
			},
			wantErr:     true,
			errContains: "text is required",
		},
		{
			name: "Success - create_split auto split id",
			input: Request{
				Action: "create_split",
			},
			check: func(t *testing.T, got Request) {
				t.Helper()
				if got.SplitID == "" {
					t.Fatalf("expected split_id to be auto-generated")
				}
			},
		},
		{
			name: "Failure - open_url invalid scheme",
			input: Request{
				Action: "open_url",
				URL:    "file:///tmp/x",
			},
			wantErr:     true,
			errContains: "only http/https urls are supported",
		},
		{
			name: "Success - pin_artifact applies defaults",
			input: Request{
				Action: "pin_artifact",
				Artifact: domain.Artifact{
					ID: "artifact-1",
				},
			},
			check: func(t *testing.T, got Request) {
				t.Helper()
				if got.Artifact.Title != "artifact-1" {
					t.Fatalf("expected default title to be artifact id, got %q", got.Artifact.Title)
				}
				if got.Artifact.Kind != domain.ContentTypePlainText {
					t.Fatalf("expected default kind %q, got %q", domain.ContentTypePlainText, got.Artifact.Kind)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.input
			err := normalizeAndValidateRequest(&req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got nil")
				}
				if tc.errContains != "" && err.Error() != tc.errContains {
					t.Fatalf("expected error %q, got %q", tc.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.check != nil {
				tc.check(t, req)
			}
		})
	}
}

func TestHandleConn_RoutesActionsAndEmitsEvents(t *testing.T) {
	tests := []struct {
		name      string
		input     Request
		wantOK    bool
		setup     func(srv *Server)
		checkResp func(t *testing.T, resp Response)
		checkMsg  func(t *testing.T, msg any)
		checkMsgs func(t *testing.T, msgs []any)
		checkEvt  func(t *testing.T, events []EventEnvelope)
	}{
		{
			name: "Success - set_ghost_text emits event",
			input: Request{
				Action: "set_ghost_text",
				Text:   "go test ./...",
			},
			wantOK: true,
			checkMsg: func(t *testing.T, msg any) {
				t.Helper()
				typed, ok := msg.(domain.AgentGhostTextMsg)
				if !ok {
					t.Fatalf("expected AgentGhostTextMsg, got %T", msg)
				}
				if typed.Text != "go test ./..." {
					t.Fatalf("unexpected ghost text: %q", typed.Text)
				}
			},
			checkEvt: func(t *testing.T, events []EventEnvelope) {
				t.Helper()
				if len(events) != 1 || events[0].Type != "ghost_applied" {
					t.Fatalf("expected ghost_applied event, got %+v", events)
				}
			},
		},
		{
			name: "Success - send_input emits command event",
			input: Request{
				Action: "send_input",
				Text:   "kubectl get pods",
			},
			wantOK: true,
			checkMsg: func(t *testing.T, msg any) {
				t.Helper()
				typed, ok := msg.(domain.AgentSendInputMsg)
				if !ok {
					t.Fatalf("expected AgentSendInputMsg, got %T", msg)
				}
				if typed.Text != "kubectl get pods" {
					t.Fatalf("unexpected input text: %q", typed.Text)
				}
			},
			checkEvt: func(t *testing.T, events []EventEnvelope) {
				t.Helper()
				if len(events) != 1 || events[0].Type != "command_executed" {
					t.Fatalf("expected command_executed event, got %+v", events)
				}
			},
		},
		{
			name: "Success - create_split auto id and message",
			input: Request{
				Action: "create_split",
			},
			wantOK: true,
			checkResp: func(t *testing.T, resp Response) {
				t.Helper()
				if resp.Data == nil {
					t.Fatalf("expected response data with split_id")
				}
				if _, ok := resp.Data["split_id"]; !ok {
					t.Fatalf("expected split_id in response data")
				}
			},
			checkMsg: func(t *testing.T, msg any) {
				t.Helper()
				typed, ok := msg.(domain.AgentCreateSplitMsg)
				if !ok {
					t.Fatalf("expected AgentCreateSplitMsg, got %T", msg)
				}
				if typed.SplitID == "" {
					t.Fatalf("expected split id in message")
				}
			},
			checkEvt: func(t *testing.T, events []EventEnvelope) {
				t.Helper()
				if len(events) != 1 || events[0].Type != "split_created" {
					t.Fatalf("expected split_created event, got %+v", events)
				}
			},
		},
		{
			name: "Success - open_url uses snapshot handler and pins artifact",
			input: Request{
				Action: "open_url",
				URL:    "https://example.com",
			},
			wantOK: true,
			setup: func(srv *Server) {
				srv.SetOpenURLHandler(func(rawURL string) (domain.Artifact, error) {
					return domain.Artifact{
						ID:      "a1",
						Title:   "Browser Snapshot",
						Kind:    domain.ContentTypeMarkdown,
						Content: "snapshot for " + rawURL,
					}, nil
				})
			},
			checkResp: func(t *testing.T, resp Response) {
				t.Helper()
				if resp.Data == nil || resp.Data["artifact_id"] != "a1" {
					t.Fatalf("expected artifact_id=a1 in response data, got %+v", resp.Data)
				}
			},
			checkMsgs: func(t *testing.T, msgs []any) {
				t.Helper()
				if len(msgs) != 1 {
					t.Fatalf("expected one message, got %d", len(msgs))
				}
				msg, ok := msgs[0].(domain.AgentPinArtifactMsg)
				if !ok {
					t.Fatalf("expected AgentPinArtifactMsg, got %T", msgs[0])
				}
				if msg.Artifact.ID != "a1" {
					t.Fatalf("expected artifact id a1, got %q", msg.Artifact.ID)
				}
			},
			checkEvt: func(t *testing.T, events []EventEnvelope) {
				t.Helper()
				if len(events) != 2 {
					t.Fatalf("expected two events for open_url snapshot path, got %+v", events)
				}
				if events[0].Type != "artifact_pinned" || events[1].Type != "url_open_requested" {
					t.Fatalf("unexpected events order/types: %+v", events)
				}
			},
		},
		{
			name: "Success - open_url falls back to agent message when handler missing",
			input: Request{
				Action: "open_url",
				URL:    "https://example.com",
			},
			wantOK: true,
			checkMsg: func(t *testing.T, msg any) {
				t.Helper()
				typed, ok := msg.(domain.AgentOpenURLMsg)
				if !ok {
					t.Fatalf("expected AgentOpenURLMsg, got %T", msg)
				}
				if typed.URL != "https://example.com" {
					t.Fatalf("unexpected open url payload: %q", typed.URL)
				}
			},
			checkEvt: func(t *testing.T, events []EventEnvelope) {
				t.Helper()
				if len(events) != 1 || events[0].Type != "url_open_requested" {
					t.Fatalf("expected only url_open_requested event, got %+v", events)
				}
			},
		},
		{
			name: "Failure - open_url invalid rejects request",
			input: Request{
				Action: "open_url",
				URL:    "ftp://example.com",
			},
			wantOK: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sent []any
			srv := NewServer(
				"/tmp/unused.sock",
				func() []domain.Block {
					return []domain.Block{{ID: "b1"}}
				},
				func(msg any) {
					sent = append(sent, msg)
				},
			)
			if tc.setup != nil {
				tc.setup(srv)
			}

			resp := roundTripRequest(t, srv, tc.input)
			if resp.OK != tc.wantOK {
				t.Fatalf("expected OK=%v, got %v (error=%q)", tc.wantOK, resp.OK, resp.Error)
			}
			if !tc.wantOK {
				if len(sent) != 0 {
					t.Fatalf("expected no messages for failed request")
				}
				return
			}

			if tc.checkResp != nil {
				tc.checkResp(t, resp)
			}
			if tc.checkMsg != nil {
				if len(sent) != 1 {
					t.Fatalf("expected one message, got %d", len(sent))
				}
				tc.checkMsg(t, sent[0])
			}
			if tc.checkMsgs != nil {
				tc.checkMsgs(t, sent)
			}
			if tc.checkEvt != nil {
				tc.checkEvt(t, srv.readEvents())
			}
		})
	}
}

func roundTripRequest(t *testing.T, srv *Server, req Request) Response {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()

	done := make(chan struct{})
	go func() {
		srv.handleConn(serverConn)
		close(done)
	}()

	enc := json.NewEncoder(clientConn)
	dec := json.NewDecoder(clientConn)

	if err := enc.Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp Response
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	<-done
	return resp
}
