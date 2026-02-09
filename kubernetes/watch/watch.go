package watch

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"kube-watcher/kubernetes"
)

type AlertKind string

const (
	AlertKindNode  AlertKind = "node"
	AlertKindPod   AlertKind = "pod"
	AlertKindEvent AlertKind = "event"
)

type Alert struct {
	Kind       AlertKind `json:"kind"`
	Severity   string    `json:"severity"`
	Namespace  string    `json:"namespace,omitempty"`
	Name       string    `json:"name,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Message    string    `json:"message,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// Key returns a unique identifier for deduplication.
func (a Alert) Key() string {
	return fmt.Sprintf("%s:%s:%s:%s", a.Kind, a.Namespace, a.Name, a.Message)
}

type Manager struct {
	client       kubernetes.ClientInterface
	logger       *slog.Logger
	interval     time.Duration
	dedupeWindow time.Duration

	mu         sync.Mutex
	lastAlerts map[string]time.Time
}

func NewManager(client kubernetes.ClientInterface, logger *slog.Logger, interval time.Duration) *Manager {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Manager{
		client:       client,
		logger:       logger,
		interval:     interval,
		dedupeWindow: 5 * time.Minute,
		lastAlerts:   make(map[string]time.Time),
	}
}

func (m *Manager) Start(ctx context.Context) <-chan Alert {
	out := make(chan Alert, 64)

	go func() {
		defer close(out)
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		m.tick(ctx, out)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.tick(ctx, out)
				m.cleanupCache()
			}
		}
	}()

	return out
}

func (m *Manager) tick(ctx context.Context, out chan<- Alert) {
	var wg sync.WaitGroup
	scanners := []func(context.Context, chan<- Alert) error{
		m.scanNodes,
		m.scanPods,
		m.scanEvents,
	}

	for _, scan := range scanners {
		wg.Add(1)
		go func(s func(context.Context, chan<- Alert) error) {
			defer wg.Done()
			if err := s(ctx, out); err != nil {
				m.logger.Error("scan failed", "error", err)
			}
		}(scan)
	}
	wg.Wait()
}

func (m *Manager) scanNodes(ctx context.Context, out chan<- Alert) error {
	nodes, err := m.client.GetNodes(ctx)
	if err != nil {
		return err
	}

	for _, node := range nodes {
		if node.Status == "Ready" {
			continue
		}

		m.emit(out, Alert{
			Kind:       AlertKindNode,
			Severity:   "critical",
			Name:       node.Name,
			Message:    "Node is not Ready",
			OccurredAt: time.Now(),
		})
	}
	return nil
}

func (m *Manager) scanPods(ctx context.Context, out chan<- Alert) error {
	pods, err := m.client.GetPodsAllNamespaces(ctx)
	if err != nil {
		return err
	}

	for _, pod := range pods {
		if pod.RestartCount < 5 && pod.Status == "Running" {
			continue
		}

		severity, msg := "warning", "Pod experiencing issues"
		if pod.RestartCount >= 10 {
			severity, msg = "critical", "Pod restarting frequently"
		} else if pod.Status == "Failed" {
			severity, msg = "critical", "Pod failed"
		} else if pod.Status == "Pending" {
			msg = "Pod pending scheduling"
		}

		m.emit(out, Alert{
			Kind:       AlertKindPod,
			Severity:   severity,
			Namespace:  pod.Namespace,
			Name:       pod.Name,
			Message:    msg,
			OccurredAt: time.Now(),
		})
	}
	return nil
}

func (m *Manager) scanEvents(ctx context.Context, out chan<- Alert) error {
	events, err := m.client.GetEventsAllNamespaces(ctx)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-m.interval * 2) // Dynamically scale cutoff to interval
	for _, event := range events {
		if event.Type != "Warning" || event.LastTimestamp.Before(cutoff) {
			continue
		}

		m.emit(out, Alert{
			Kind:       AlertKindEvent,
			Severity:   "warning",
			Namespace:  event.Namespace,
			Name:       event.ObjectName,
			Reason:     event.Reason,
			Message:    event.Message,
			OccurredAt: event.LastTimestamp,
		})
	}
	return nil
}

func (m *Manager) emit(out chan<- Alert, a Alert) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := a.Key()
	if last, ok := m.lastAlerts[key]; ok && time.Since(last) < m.dedupeWindow {
		return
	}

	m.lastAlerts[key] = time.Now()
	out <- a
}

func (m *Manager) cleanupCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, last := range m.lastAlerts {
		if now.Sub(last) > m.dedupeWindow*2 {
			delete(m.lastAlerts, key)
		}
	}
}
