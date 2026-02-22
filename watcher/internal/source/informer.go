package source

import (
	"context"
	"log/slog"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"kube-watcher/watcher/internal/events"
)

type InformerSource struct {
	client clientset.Interface
	logger *slog.Logger
	resync time.Duration
}

func NewInformerSource(client clientset.Interface, logger *slog.Logger, resync time.Duration) *InformerSource {
	if resync <= 0 {
		resync = 30 * time.Second
	}
	return &InformerSource{
		client: client,
		logger: logger,
		resync: resync,
	}
}

func (s *InformerSource) Run(ctx context.Context, out chan<- events.ResourceEvent) {
	factory := informers.NewSharedInformerFactory(s.client, s.resync)
	inf := factory.Core().V1().Pods().Informer()

	emit := func(obj interface{}) { s.emit(obj, out, "Pod") }

	inf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    emit,
		UpdateFunc: func(_, newObj interface{}) { emit(newObj) },
		DeleteFunc: emit,
	})

	factory.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), inf.HasSynced) {
		s.logger.Warn("pod informer cache sync failed")
		return
	}

	<-ctx.Done()
}

func (s *InformerSource) emit(obj interface{}, out chan<- events.ResourceEvent, kind string) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
			pod, _ = tombstone.Obj.(*corev1.Pod)
		}
	}

	if pod == nil {
		return
	}

	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		s.logger.Warn("failed to convert resource", "error", err)
		return
	}

	evt := events.ResourceEvent{
		Kind:            kind,
		Namespace:       pod.Namespace,
		Name:            pod.Name,
		ResourceVersion: pod.ResourceVersion,
		Object:          data,
		Raw:             pod,
	}

	select {
	case out <- evt:
	default:
		s.logger.Warn("event channel full, dropping", "key", evt.Key())
	}
}
