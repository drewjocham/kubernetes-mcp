package chatbridge

import (
	"fmt"
	"strings"
	"time"
)

type GoogleChatEvent struct {
	Type      string            `json:"type"`
	EventTime string            `json:"eventTime"`
	Space     GoogleChatSpace   `json:"space"`
	Message   GoogleChatMessage `json:"message"`
}

type GoogleChatSpace struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type GoogleChatMessage struct {
	Name   string           `json:"name"`
	Text   string           `json:"text"`
	Thread GoogleChatThread `json:"thread"`
	Sender GoogleChatSender `json:"sender"`
}

type GoogleChatThread struct {
	Name string `json:"name"`
}

type GoogleChatSender struct {
	DisplayName string `json:"displayName"`
}

type IncidentKind string

const (
	IncidentKubernetes IncidentKind = "kubernetes"
	IncidentSolace     IncidentKind = "solace"
)

type InvestigationRequest struct {
	EventID       string
	Kind          IncidentKind
	MessageText   string
	SpaceName     string
	ThreadName    string
	SenderName    string
	Prompt        string
	CorrelationID string
}

type InvestigationResult struct {
	Provider string
	RunID    string
	State    string
	Summary  string
}

func (e GoogleChatEvent) EventID() string {
	parts := []string{
		strings.TrimSpace(e.EventTime),
		strings.TrimSpace(e.Space.Name),
		strings.TrimSpace(e.Message.Name),
		strings.TrimSpace(e.Message.Thread.Name),
	}
	out := strings.Join(parts, ":")
	if out == ":::" {
		return fmt.Sprintf("fallback:%d", time.Now().UnixNano())
	}
	return out
}
