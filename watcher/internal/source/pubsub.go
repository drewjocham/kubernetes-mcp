package source

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"cloud.google.com/go/pubsub" //nolint:staticcheck // SA1019: v1 client; migrate to pubsub/v2 when subscription admin is refactored.
	"golang.org/x/sync/errgroup"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
)

type PubSubSource struct {
	logger *slog.Logger
	cfg    config.PubSubConfig
}

func NewPubSubSource(logger *slog.Logger, cfg config.PubSubConfig) *PubSubSource {
	return &PubSubSource{
		logger: logger,
		cfg:    cfg,
	}
}

func (s *PubSubSource) Run(ctx context.Context, out chan<- events.ResourceEvent) {
	if !s.cfg.Enabled {
		s.logger.Info("pubsub source disabled")
		return
	}

	projectID := s.cfg.ProjectID
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
		if projectID == "" {
			s.logger.Warn("GOOGLE_CLOUD_PROJECT not set, cannot start pubsub source")
			return
		}
	}

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		// If credentials are missing, treat as disabled (like ingest service)
		if err.Error() == "pubsub(publisher): credentials: could not find default credentials. See https://cloud.google.com/docs/authentication/external/set-up-adc for more information" {
			s.logger.Warn("pubsub credentials not found, skipping pubsub source", "project", projectID)
			return
		}
		s.logger.Error("failed to create pubsub client", "error", err)
		return
	}
	defer func() {
		if cerr := client.Close(); cerr != nil {
			s.logger.Warn("failed to close pubsub client", "error", cerr)
		}
	}()

	// Ensure topic exists
	topic := client.Topic(s.cfg.TopicID)
	topicExists, err := topic.Exists(ctx)
	if err != nil {
		s.logger.Error("failed to check topic existence", "error", err)
		return
	}
	if !topicExists {
		s.logger.Info("topic does not exist, creating", "topic", s.cfg.TopicID)
		_, err = client.CreateTopic(ctx, s.cfg.TopicID)
		if err != nil {
			s.logger.Error("failed to create topic", "error", err)
			return
		}
	}

	sub := client.Subscription(s.cfg.SubscriptionID)
	subExists, err := sub.Exists(ctx)
	if err != nil {
		s.logger.Error("failed to check subscription existence", "error", err)
		return
	}
	if !subExists {
		s.logger.Info("subscription does not exist, creating", "subscription", s.cfg.SubscriptionID)
		_, err = client.CreateSubscription(ctx, s.cfg.SubscriptionID, pubsub.SubscriptionConfig{
			Topic:             topic,
			AckDeadline:       30 * time.Second,
			RetentionDuration: 7 * 24 * time.Hour,
		})
		if err != nil {
			s.logger.Error("failed to create subscription", "error", err)
			return
		}
	}

	s.logger.Info("pubsub source starting", "subscription", s.cfg.SubscriptionID, "topic", s.cfg.TopicID)

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			s.processMessage(ctx, msg, out)
		})
		if err != nil && err != context.Canceled {
			s.logger.Error("pubsub receive error", "error", err)
			return err
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		s.logger.Error("pubsub source failed", "error", err)
	}
}

func (s *PubSubSource) processMessage(ctx context.Context, msg *pubsub.Message, out chan<- events.ResourceEvent) {
	s.logger.Info("pubsub message received", "message_id", msg.ID, "data_length", len(msg.Data))
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic recovered processing pubsub message", "panic", r)
			msg.Nack()
		}
	}()

	evts, err := parsePubSubMessageData(msg.Data)
	if err != nil {
		s.logger.Warn("failed to unmarshal pubsub message", "error", err)
		msg.Nack()
		return
	}

	for _, evt := range evts {
		select {
		case out <- evt:
		default:
			s.logger.Warn("event channel full, dropping anomaly", "id", evt.Name)
		}
	}

	msg.Ack()
	s.logger.Debug("processed pubsub message", "anomalies", len(evts))
}

// parsePubSubMessageData parses Pub/Sub message data and returns resource events.
// This is extracted for testability.
func parsePubSubMessageData(data []byte) ([]events.ResourceEvent, error) {
	var payload struct {
		Anomalies []Anomaly `json:"anomalies"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}

	evts := make([]events.ResourceEvent, 0, len(payload.Anomalies))
	for _, anomaly := range payload.Anomalies {
		evt := events.ResourceEvent{
			Kind:            "Anomaly",
			Namespace:       "anomstack",
			Name:            anomaly.ID,
			ResourceVersion: fmt.Sprintf("%d", anomaly.Timestamp),
			Object: map[string]interface{}{
				"id":        anomaly.ID,
				"title":     anomaly.Title,
				"severity":  anomaly.Severity,
				"message":   anomaly.Message,
				"timestamp": anomaly.Timestamp,
			},
			Raw: anomaly,
		}
		evts = append(evts, evt)
	}
	return evts, nil
}
