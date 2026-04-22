package source

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"kube-watcher/watcher/internal/config"
	"kube-watcher/watcher/internal/events"
)

type AnomstackSource struct {
	logger *slog.Logger
	cfg    config.AnomstackConfig
	client *http.Client
}

func NewAnomstackSource(logger *slog.Logger, cfg config.AnomstackConfig) *AnomstackSource {
	if cfg.Interval <= 0 {
		cfg.Interval = 30 * time.Second
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "http://anomstack.kube-watcher.svc.cluster.local:8080/anomalies"
	}
	return &AnomstackSource{
		logger: logger,
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *AnomstackSource) Run(ctx context.Context, out chan<- events.ResourceEvent) {
	if !s.cfg.Enabled {
		s.logger.Info("anomstack source disabled")
		return
	}

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()

	s.logger.Info("anomstack source starting", "endpoint", s.cfg.Endpoint, "interval", s.cfg.Interval)

	// Initial poll
	s.poll(ctx, out)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("anomstack source stopping")
			return
		case <-ticker.C:
			s.poll(ctx, out)
		}
	}
}

func (s *AnomstackSource) poll(ctx context.Context, out chan<- events.ResourceEvent) {
	req, err := http.NewRequestWithContext(ctx, "GET", s.cfg.Endpoint, nil)
	if err != nil {
		s.logger.Warn("failed to create request", "error", err)
		return
	}

	resp, err := s.client.Do(req)
	if err != nil {
		s.logger.Warn("failed to poll anomstack", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Warn("anomstack returned non-OK status", "status", resp.StatusCode)
		return
	}

	var result struct {
		Anomalies []Anomaly `json:"anomalies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		s.logger.Warn("failed to decode anomstack response", "error", err)
		return
	}

	for _, anomaly := range result.Anomalies {
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
		select {
		case out <- evt:
		default:
			s.logger.Warn("event channel full, dropping anomaly", "id", anomaly.ID)
		}
	}
}

type Anomaly struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}
