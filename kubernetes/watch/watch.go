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

type Manager struct {
	client       kubernetes.ClientInterface
	logger       *slog.Logger
	interval     time.Duration
	dedupeWindow time.Duration
	cache        sync.Map
	out          chan Alert
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
		out:          make(chan Alert, 128),
	}
}

func (m *Manager) Start(ctx context.Context) <-chan Alert {
	m.logger.Info("starting watch manager", "interval", m.interval)
	if raw := m.client.GetRawInterface(); raw != nil {
		factory := informers.NewSharedInformerFactory(raw, 10*time.Minute)

		podInformer := factory.Core().V1().Pods().Informer()
		podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(old, new interface{}) {
				p, ok := new.(*corev1.Pod)
				if ok {
					m.checkPodRealtime(p)
				}
			},
		})

		eventInformer := factory.Core().V1().Events().Informer()
		eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				e, ok := obj.(*corev1.Event)
				if ok && e.Type == corev1.EventTypeWarning {
					m.emitEventRealtime(e)
				}
			},
		})

	} else {
		m.logger.Warn("raw kubernetes interface unavailable; realtime watchers disabled")
	}
	go m.runScanner(ctx)

	return m.out
}

func (m *Manager) runScanner(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		m.scan(ctx)
		select {
		case <-ctx.Done():
			close(m.out)
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) scan(ctx context.Context) {
	var wg sync.WaitGroup
	wg.Add(2)

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

	go func() {
		defer wg.Done()
		if pods, err := m.client.GetPodsAllNamespaces(ctx); err == nil {
			for _, p := range pods {
				if a := evaluatePodState(p); a != nil {
					m.emit(a)
				}
			}
		}
	}()

	wg.Wait()
}

func (m *Manager) checkPodRealtime(p *corev1.Pod) {
	for _, s := range p.Status.ContainerStatuses {
		isCrashLoop := s.State.Waiting != nil && s.State.Waiting.Reason == "CrashLoopBackOff"
		if s.RestartCount > 5 || isCrashLoop {
			reason := "PodIssue"
			if isCrashLoop {
				reason = "CrashLoopBackOff"
			}
			m.emit(&Alert{
				Kind:       AlertKindPod,
				Severity:   "high",
				Namespace:  p.Namespace,
				Name:       p.Name,
				Reason:     reason,
				Message:    fmt.Sprintf("container %s has %d restarts", s.Name, s.RestartCount),
				OccurredAt: time.Now(),
			})
		}
	}
}

func (m *Manager) emitEventRealtime(e *corev1.Event) {
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

func (m *Manager) emit(a *Alert) {
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
	select {
	case m.out <- *a:
	default:
		m.logger.Warn("alert channel full, dropping alert", "name", a.Name)
	}
}

func evaluatePodState(pod kubernetes.PodInfo) *Alert {
	if pod.RestartCount < 5 && (pod.Status == "Running" || pod.Status == "Succeeded") {
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
