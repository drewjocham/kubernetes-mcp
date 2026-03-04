package watch

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"kube-watcher/pkg/kube"
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
	return fmt.Sprintf("%s:%s:%s:%s", a.Kind, a.Namespace, a.Name, a.Reason)
}

type Evaluator[T any] func(T) *Alert

var PodEvaluator Evaluator[*corev1.Pod] = func(p *corev1.Pod) *Alert {
	for _, s := range p.Status.ContainerStatuses {
		waiting := s.State.Waiting
		isCrash := waiting != nil && waiting.Reason == "CrashLoopBackOff"
		if s.RestartCount > 5 || isCrash {
			severity := "high"
			if s.RestartCount > 10 {
				severity = "critical"
			}
			reason := "restart_threshold"
			if waiting != nil && waiting.Reason != "" {
				reason = waiting.Reason
			}
			return &Alert{
				Kind:       AlertKindPod,
				Severity:   severity,
				Namespace:  p.Namespace,
				Name:       p.Name,
				Reason:     reason,
				Message:    fmt.Sprintf("container %s: %d restarts", s.Name, s.RestartCount),
				OccurredAt: time.Now(),
			}
		}
	}
	return nil
}

type Manager struct {
	client       kube.ClientInterface
	logger       *slog.Logger
	interval     time.Duration
	dedupeWindow time.Duration
	history      sync.Map
	out          chan Alert
}

func NewManager(client kube.ClientInterface, logger *slog.Logger, interval time.Duration) *Manager {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Manager{
		client:       client,
		logger:       logger,
		interval:     interval,
		dedupeWindow: 5 * time.Minute,
		out:          make(chan Alert, 128),
	}
}

func (m *Manager) Start(ctx context.Context) <-chan Alert {
	m.logger.Info("starting watch manager", "interval", m.interval)

	if raw := m.client.GetRawInterface(); raw != nil {
		factory := informers.NewSharedInformerFactory(raw, 10*time.Minute)
		m.bindInformer(factory)
		factory.Start(ctx.Done())
	}

	go m.runScanner(ctx)

	return m.out
}

func (m *Manager) bindInformer(f informers.SharedInformerFactory) {
	f.Core().V1().Pods().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(_, newObj interface{}) {
			if p, ok := newObj.(*corev1.Pod); ok {
				m.emit(PodEvaluator(p))
			}
		},
	})

	f.Core().V1().Events().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if e, ok := obj.(*corev1.Event); ok && e.Type == corev1.EventTypeWarning {
				m.emit(&Alert{
					Kind:       AlertKindEvent,
					Severity:   "warning",
					Namespace:  e.Namespace,
					Name:       e.InvolvedObject.Name,
					Reason:     e.Reason,
					Message:    e.Message,
					OccurredAt: e.LastTimestamp.Time,
				})
			}
		},
	})
}

func (m *Manager) runScanner(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	defer close(m.out)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.performScan(ctx)
		}
	}
}

func (m *Manager) performScan(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if nodes, err := m.client.GetNodes(ctx); err == nil {
			for _, n := range nodes {
				if n.Status != "Ready" {
					m.emit(&Alert{
						Kind:       AlertKindNode,
						Severity:   "critical",
						Name:       n.Name,
						Message:    "Node is not Ready",
						OccurredAt: time.Now(),
					})
				}
			}
		}
	}()

	wg.Wait()
}

func (m *Manager) emit(a *Alert) {
	if a == nil || m.isDuplicate(a) {
		return
	}

	select {
	case m.out <- *a:
		m.history.Store(a.Key(), time.Now())
	default:
		m.logger.Warn("buffer full, dropping alert", "name", a.Name)
	}
}

func (m *Manager) isDuplicate(a *Alert) bool {
	if val, ok := m.history.Load(a.Key()); ok {
		return time.Since(val.(time.Time)) < m.dedupeWindow
	}
	return false
}
