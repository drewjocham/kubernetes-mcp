package audit

import (
	"context"
	"log/slog"
	"time"
)

type EventType string

const (
	// EventTypeAuthn represents authentication events (login, token validation)
	EventTypeAuthn EventType = "authentication"
	// EventTypeAuthz represents authorization events (permission checks)
	EventTypeAuthz EventType = "authorization"
	// EventTypeAccess represents resource access events (read, write operations)
	EventTypeAccess EventType = "access"
	// EventTypeConfig represents configuration changes
	EventTypeConfig EventType = "configuration"
	// EventTypeSecurity represents security-related events
	EventTypeSecurity EventType = "security"
)

type Action string

const (
	ActionGet     Action = "get"
	ActionList    Action = "list"
	ActionWatch   Action = "watch"
	ActionCreate  Action = "create"
	ActionUpdate  Action = "update"
	ActionPatch   Action = "patch"
	ActionDelete  Action = "delete"
	ActionExecute Action = "execute"
)

type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomeDenied  Outcome = "denied"
	OutcomeError   Outcome = "error"
)

type AuditEvent struct {
	Timestamp      time.Time         `json:"timestamp"`
	EventType      EventType         `json:"event_type"`
	Action         Action            `json:"action"`
	Resource       string            `json:"resource,omitempty"`
	Namespace      string            `json:"namespace,omitempty"`
	Name           string            `json:"name,omitempty"`
	User           string            `json:"user,omitempty"`
	ServiceAccount string            `json:"service_account,omitempty"`
	Outcome        Outcome           `json:"outcome"`
	Reason         string            `json:"reason,omitempty"`
	Message        string            `json:"message,omitempty"`
	ClientIP       string            `json:"client_ip,omitempty"`
	UserAgent      string            `json:"user_agent,omitempty"`
	RequestID      string            `json:"request_id,omitempty"`
	Extra          map[string]string `json:"extra,omitempty"`
}

type Logger interface {
	Log(ctx context.Context, event AuditEvent)
}

type SlogLogger struct {
	logger *slog.Logger
}

func NewSlogLogger(logger *slog.Logger) *SlogLogger {
	return &SlogLogger{logger: logger}
}

func (l *SlogLogger) Log(ctx context.Context, event AuditEvent) {
	attrs := []slog.Attr{
		slog.String("timestamp", event.Timestamp.Format(time.RFC3339Nano)),
		slog.String("event_type", string(event.EventType)),
		slog.String("action", string(event.Action)),
		slog.String("outcome", string(event.Outcome)),
	}

	if event.Resource != "" {
		attrs = append(attrs, slog.String("resource", event.Resource))
	}
	if event.Namespace != "" {
		attrs = append(attrs, slog.String("namespace", event.Namespace))
	}
	if event.Name != "" {
		attrs = append(attrs, slog.String("name", event.Name))
	}
	if event.User != "" {
		attrs = append(attrs, slog.String("user", event.User))
	}
	if event.ServiceAccount != "" {
		attrs = append(attrs, slog.String("service_account", event.ServiceAccount))
	}
	if event.Reason != "" {
		attrs = append(attrs, slog.String("reason", event.Reason))
	}
	if event.Message != "" {
		attrs = append(attrs, slog.String("message", event.Message))
	}
	if event.ClientIP != "" {
		attrs = append(attrs, slog.String("client_ip", event.ClientIP))
	}
	if event.UserAgent != "" {
		attrs = append(attrs, slog.String("user_agent", event.UserAgent))
	}
	if event.RequestID != "" {
		attrs = append(attrs, slog.String("request_id", event.RequestID))
	}

	for k, v := range event.Extra {
		attrs = append(attrs, slog.String(k, v))
	}

	var level slog.Level
	switch event.Outcome {
	case OutcomeFailure, OutcomeDenied, OutcomeError:
		level = slog.LevelWarn
	default:
		level = slog.LevelInfo
	}

	anyAttrs := make([]any, len(attrs))
	for i, attr := range attrs {
		anyAttrs[i] = attr
	}

	l.logger.Log(ctx, level, "audit_event", slog.Group("audit", anyAttrs...))
}

func LogAccess(ctx context.Context, logger Logger, action Action, resource, namespace, name string, outcome Outcome, reason string) {
	logger.Log(ctx, AuditEvent{
		Timestamp: time.Now(),
		EventType: EventTypeAccess,
		Action:    action,
		Resource:  resource,
		Namespace: namespace,
		Name:      name,
		Outcome:   outcome,
		Reason:    reason,
	})
}

func LogAuthz(ctx context.Context, logger Logger, action Action, resource, namespace, name, user string, outcome Outcome, reason string) {
	logger.Log(ctx, AuditEvent{
		Timestamp: time.Now(),
		EventType: EventTypeAuthz,
		Action:    action,
		Resource:  resource,
		Namespace: namespace,
		Name:      name,
		User:      user,
		Outcome:   outcome,
		Reason:    reason,
	})
}

func LogAuthn(ctx context.Context, logger Logger, user, serviceAccount string, outcome Outcome, reason string) {
	logger.Log(ctx, AuditEvent{
		Timestamp:      time.Now(),
		EventType:      EventTypeAuthn,
		User:           user,
		ServiceAccount: serviceAccount,
		Outcome:        outcome,
		Reason:         reason,
	})
}
