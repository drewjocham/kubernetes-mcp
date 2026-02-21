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

func (a Alert) Key() string {
	return fmt.Sprintf("%s:%s:%s:%s", a.Kind, a.Namespace, a.Name, a.Message)
}

func EvaluateNode(node kubernetes.NodeInfo) *Alert {
	if node.Status == "Ready" {
		return nil
	}
	return &Alert{
		Kind:       AlertKindNode,
		Severity:   "critical",
		Name:       node.Name,
		Message:    "Node is not Ready",
		OccurredAt: time.Now(),
	}
}

func EvaluatePod(pod kubernetes.PodInfo) *Alert {
	if pod.RestartCount < 5 && pod.Status == "Running" {
		return nil
	}

	severity, msg := "warning", "Pod experiencing issues"
	if pod.RestartCount >= 10 {
		severity, msg = "critical", "Pod restarting frequently"
	} else if pod.Status == "Failed" {
		severity, msg = "critical", "Pod failed"
	} else if pod.Status == "Pending" {
		msg = "Pod pending scheduling"
	}

	return &Alert{
		Kind:       AlertKindPod,
		Severity:   severity,
		Namespace:  pod.Namespace,
		Name:       pod.Name,
		Message:    msg,
		OccurredAt: time.Now(),
	}
}

func EvaluateEvent(event kubernetes.EventInfo, cutoff time.Time) *Alert {
	if event.Type != "Warning" || event.LastTimestamp.Before(cutoff) {
		return nil
	}
	return &Alert{
		Kind:       AlertKindEvent,
		Severity:   "warning",
		Namespace:  event.Namespace,
		Name:       event.ObjectName,
		Reason:     event.Reason,
		Message:    event.Message,
		OccurredAt: event.LastTimestamp,
	}
}

type Manager struct {
	client       kubernetes.ClientInterface
	logger       *slog.Logger
	interval     time.Duration
	dedupeWindow time.Duration
	cache        sync.Map
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
	}
}

func (m *Manager) Start(ctx context.Context) <-chan Alert {
	out := make(chan Alert, 64)
	go m.run(ctx, out)
	return out
}

func (m *Manager) run(ctx context.Context, out chan<- Alert) {
	defer close(out)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		m.scan(ctx, out)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) scan(ctx context.Context, out chan<- Alert) {
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		if nodes, err := m.client.GetNodes(ctx); err == nil {
			for _, n := range nodes {
				m.emit(out, EvaluateNode(n))
			}
		}
	}()

	go func() {
		defer wg.Done()
		if pods, err := m.client.GetPodsAllNamespaces(ctx); err == nil {
			for _, p := range pods {
				m.emit(out, EvaluatePod(p))
			}
		}
	}()

	go func() {
		defer wg.Done()
		cutoff := time.Now().Add(-m.interval * 2)
		if events, err := m.client.GetEventsAllNamespaces(ctx); err == nil {
			for _, e := range events {
				m.emit(out, EvaluateEvent(e, cutoff))
			}
		}
	}()

	wg.Wait()
}

func (m *Manager) emit(out chan<- Alert, a *Alert) {
	if a == nil {
		return
	}

	key := a.Key()
	now := time.Now()

	if last, ok := m.cache.Load(key); ok {
		if now.Sub(last.(time.Time)) < m.dedupeWindow {
			return
		}
	}

	m.cache.Store(key, now)
	out <- *a
}
