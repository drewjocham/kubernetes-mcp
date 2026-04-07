package watch

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// PodTracker tracks pod existence and provides pod status information
type PodTracker struct {
	client    kubernetes.Interface
	logger    *slog.Logger
	podExists map[string]bool   // key: namespace/name
	podUIDs   map[string]string // key: namespace/name, value: UID
	mu        sync.RWMutex
	stopCh    chan struct{}
}

// NewPodTracker creates a new pod tracker
func NewPodTracker(client kubernetes.Interface, logger *slog.Logger) *PodTracker {
	return &PodTracker{
		client:    client,
		logger:    logger,
		podExists: make(map[string]bool),
		podUIDs:   make(map[string]string),
		stopCh:    make(chan struct{}),
	}
}

// Start starts the pod tracker informer
func (t *PodTracker) Start(ctx context.Context) error {
	factory := informers.NewSharedInformerFactory(t.client, 10*time.Minute)
	podInformer := factory.Core().V1().Pods().Informer()

	_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				t.handlePodAdd(pod)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			if pod, ok := newObj.(*corev1.Pod); ok {
				t.handlePodUpdate(pod)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if pod, ok := obj.(*corev1.Pod); ok {
				t.handlePodDelete(pod)
			} else if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				if pod, ok := tombstone.Obj.(*corev1.Pod); ok {
					t.handlePodDelete(pod)
				}
			}
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add pod informer handler: %w", err)
	}

	go podInformer.Run(t.stopCh)
	factory.Start(ctx.Done())

	t.logger.Info("pod tracker started")
	return nil
}

// Stop stops the pod tracker
func (t *PodTracker) Stop() {
	close(t.stopCh)
	t.logger.Info("pod tracker stopped")
}

// PodExists checks if a pod currently exists
func (t *PodTracker) PodExists(namespace, name string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	key := podKey(namespace, name)
	exists, ok := t.podExists[key]
	return ok && exists
}

// GetPodUID returns the UID of a pod if it exists
func (t *PodTracker) GetPodUID(namespace, name string) (string, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	key := podKey(namespace, name)
	uid, ok := t.podUIDs[key]
	return uid, ok
}

// ListPods returns a list of all tracked pods
func (t *PodTracker) ListPods() []PodInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var pods []PodInfo
	for key, exists := range t.podExists {
		if exists {
			namespace, name := parsePodKey(key)
			uid := t.podUIDs[key]
			pods = append(pods, PodInfo{
				Namespace: namespace,
				Name:      name,
				UID:       uid,
			})
		}
	}
	return pods
}

// PodInfo contains basic pod information
type PodInfo struct {
	Namespace string
	Name      string
	UID       string
}

func (t *PodTracker) handlePodAdd(pod *corev1.Pod) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := podKey(pod.Namespace, pod.Name)
	t.podExists[key] = true
	t.podUIDs[key] = string(pod.UID)
	t.logger.Debug("pod added", "namespace", pod.Namespace, "name", pod.Name, "uid", pod.UID)
}

func (t *PodTracker) handlePodUpdate(pod *corev1.Pod) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := podKey(pod.Namespace, pod.Name)
	// Check if pod is being deleted
	if pod.DeletionTimestamp != nil {
		t.podExists[key] = false
		t.logger.Debug("pod marked for deletion", "namespace", pod.Namespace, "name", pod.Name)
	} else {
		t.podExists[key] = true
		t.podUIDs[key] = string(pod.UID)
		t.logger.Debug("pod updated", "namespace", pod.Namespace, "name", pod.Name)
	}
}

func (t *PodTracker) handlePodDelete(pod *corev1.Pod) {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := podKey(pod.Namespace, pod.Name)
	delete(t.podExists, key)
	delete(t.podUIDs, key)
	t.logger.Debug("pod deleted", "namespace", pod.Namespace, "name", pod.Name)
}

func podKey(namespace, name string) string {
	return namespace + "/" + name
}

func parsePodKey(key string) (namespace, name string) {
	// Simple parsing, assuming no "/" in namespace or name
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '/' {
			return key[:i], key[i+1:]
		}
	}
	return "", key
}
