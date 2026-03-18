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
		return nil, fmt.Errorf("provider %q not configured", providerName)
	}

	reporter, err := NewGoogleChatWebhookReporter(cfg.Reporting)
	if err != nil {
		return nil, fmt.Errorf("reporter init failed: %w", err)
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

	event, err := b.decodeAndLogEvent(r.Body)
	if err != nil {
		b.logger.Warn("payload error", "error", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

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
		b.logger.Debug("duplicate event skipped", "event_id", eventID)
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
		b.logger.Error("prompt render failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	go b.processInvestigation(context.Background(), req)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (b *Bridge) processInvestigation(ctx context.Context, req InvestigationRequest) {
	result, err := b.backend.Run(ctx, req)
	if err != nil {
		b.logger.Error("investigation failed", "error", err, "event_id", req.EventID)
		_ = b.reporter.Post(ctx, req.ThreadName, b.failureReport(req, err))
		return
	}

	report := FormatReport(req, result, b.cfg.Investigation.FallbackActionPlan)
	if err := b.reporter.Post(ctx, req.ThreadName, report); err != nil {
		b.logger.Error("reporting failed", "error", err, "event_id", req.EventID)
	}
}

func (b *Bridge) decodeAndLogEvent(body io.ReadCloser) (GoogleChatEvent, error) {
	defer body.Close()
	raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return GoogleChatEvent{}, err
	}
	var event GoogleChatEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return GoogleChatEvent{}, fmt.Errorf("json error: %w (body: %s)", err, string(raw))
	}
	return event, nil
}

func (b *Bridge) failureReport(req InvestigationRequest, err error) string {
	return fmt.Sprintf(
		"⚠️ Investigation failed for %s\nProvider: %s\nError: %s",
		strings.ToUpper(string(req.Kind)),
		b.cfg.Investigation.Provider,
		err.Error(),
	)
}

func (b *Bridge) isAuthorized(r *http.Request) bool {
	tokenEnv := strings.TrimSpace(b.cfg.GoogleChat.AuthTokenEnv)
	if tokenEnv == "" {
		return true
	}
	expected := os.Getenv(tokenEnv)
	if expected == "" {
		return false
	}
	headerName := b.cfg.GoogleChat.AuthHeader
	if headerName == "" {
		headerName = "X-Bridge-Token"
	}
	return r.Header.Get(headerName) == expected
}

func (b *Bridge) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:         b.cfg.Server.Listen,
		Handler:      b.Routes(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		b.logger.Info("bridge listening", "addr", b.cfg.Server.Listen)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		b.logger.Info("shutting down bridge...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
