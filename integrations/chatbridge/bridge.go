package chatbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type Bridge struct {
	logger      *slog.Logger
	cfg         Config
	backend     InvestigationBackend
	reporter    Reporter
	idempotency *IdempotencyStore
	now         func() time.Time
}

func NewBridge(logger *slog.Logger, cfg Config) (*Bridge, error) {
	if logger == nil {
		logger = slog.Default()
	}

	providerName := strings.TrimSpace(cfg.Investigation.Provider)
	if providerName == "" {
		return nil, fmt.Errorf("investigation.provider is required")
	}
	providerCfg, ok := cfg.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("providers.%s is not configured", providerName)
	}

	reporter, err := NewGoogleChatWebhookReporter(cfg.Reporting)
	if err != nil {
		return nil, err
	}

	backend := NewHTTPPollingBackend(
		providerName,
		providerCfg,
		cfg.Investigation.Timeout,
		cfg.Investigation.PollInterval,
		cfg.Reliability.RetryCount,
		cfg.Reliability.RetryBackoff,
	)

	return &Bridge{
		logger:      logger,
		cfg:         cfg,
		backend:     backend,
		reporter:    reporter,
		idempotency: NewIdempotencyStore(cfg.Reliability.IdempotencyTTL),
		now:         time.Now,
	}, nil
}

func (b *Bridge) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", b.handleHealth)
	mux.HandleFunc("/chat/events", b.handleChatEvent)
	return mux
}

func (b *Bridge) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (b *Bridge) handleChatEvent(w http.ResponseWriter, r *http.Request) {
	if !b.isAuthorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	event, err := decodeEvent(r.Body)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.ToUpper(strings.TrimSpace(event.Type)) != "MESSAGE" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored_non_message"}`))
		return
	}

	if !isAllowedSpace(b.cfg.Trigger, event.Space.Name) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored_space"}`))
		return
	}

	eventID := event.EventID()
	if b.idempotency.SeenOrAdd(eventID, b.now()) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"duplicate_ignored"}`))
		return
	}

	kind, ok := DetectIncidentKind(b.cfg.Trigger, event.Message.Text)
	if !ok {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored_no_matching_intent"}`))
		return
	}

	req := InvestigationRequest{
		EventID:       eventID,
		Kind:          kind,
		MessageText:   strings.TrimSpace(event.Message.Text),
		SpaceName:     event.Space.Name,
		ThreadName:    event.Message.Thread.Name,
		SenderName:    event.Message.Sender.DisplayName,
		CorrelationID: eventID,
	}
	req.Prompt, err = RenderPrompt(b.cfg.Investigation.PromptTemplate, req)
	if err != nil {
		http.Error(w, "invalid prompt template", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	result, err := b.backend.Run(ctx, req)
	if err != nil {
		b.logger.Error("investigation failed", "error", err, "event_id", eventID, "provider", b.cfg.Investigation.Provider)
		reportErr := b.reporter.Post(ctx, req.ThreadName, b.failureReport(req, err))
		if reportErr != nil {
			b.logger.Error("failed posting failure report", "error", reportErr, "event_id", eventID)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"investigation_failed_reported"}`))
		return
	}

	report := FormatReport(req, result, b.cfg.Investigation.FallbackActionPlan)
	if err := b.reporter.Post(ctx, req.ThreadName, report); err != nil {
		b.logger.Error("failed posting report", "error", err, "event_id", eventID)
		http.Error(w, "failed to post report", http.StatusBadGateway)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (b *Bridge) failureReport(req InvestigationRequest, err error) string {
	return fmt.Sprintf(
		"Investigation failed (%s)\nProvider: %s\nError: %s\n\nPlan of action:\n1. Validate provider configuration and credentials.\n2. Re-run investigation for the same thread after config fix.\n3. Escalate to on-call if incident impact is active.",
		strings.ToUpper(string(req.Kind)),
		b.cfg.Investigation.Provider,
		strings.TrimSpace(err.Error()),
	)
}

func (b *Bridge) isAuthorized(r *http.Request) bool {
	tokenEnv := strings.TrimSpace(b.cfg.GoogleChat.AuthTokenEnv)
	if tokenEnv == "" {
		return true
	}
	expected := strings.TrimSpace(os.Getenv(tokenEnv))
	if expected == "" {
		return false
	}
	headerName := strings.TrimSpace(b.cfg.GoogleChat.AuthHeader)
	if headerName == "" {
		headerName = "X-Bridge-Token"
	}
	actual := strings.TrimSpace(r.Header.Get(headerName))
	return actual == expected
}

func decodeEvent(body io.ReadCloser) (GoogleChatEvent, error) {
	defer func() {
		_ = body.Close()
	}()
	raw, err := io.ReadAll(io.LimitReader(body, 2<<20))
	if err != nil {
		return GoogleChatEvent{}, err
	}
	var event GoogleChatEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return GoogleChatEvent{}, err
	}
	return event, nil
}

func (b *Bridge) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:    b.cfg.Server.Listen,
		Handler: b.Routes(),
	}
	errCh := make(chan error, 1)
	go func() {
		b.logger.Info("chat bridge listening", "addr", b.cfg.Server.Listen, "provider", b.cfg.Investigation.Provider)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
