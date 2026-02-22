package source

import (
	"context"
	"log/slog"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	type entry struct {
		kind     string
		informer cache.SharedIndexInformer
	}

	informersToWatch := []entry{
		{kind: "Pod", informer: factory.Core().V1().Pods().Informer()},
		{kind: "HorizontalPodAutoscaler", informer: factory.Autoscaling().V2().HorizontalPodAutoscalers().Informer()},
	}

	for _, e := range informersToWatch {
		entry := e
		emit := func(obj interface{}) { s.emit(obj, out, entry.kind) }
		entry.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    emit,
			UpdateFunc: func(_, newObj interface{}) { emit(newObj) },
			DeleteFunc: emit,
		})
	}

	factory.Start(ctx.Done())
	for _, e := range informersToWatch {
		if !cache.WaitForCacheSync(ctx.Done(), e.informer.HasSynced) {
			s.logger.Warn("informer cache sync failed", "kind", e.kind)
			return
		}
	}

	<-ctx.Done()
}

func (s *InformerSource) emit(obj interface{}, out chan<- events.ResourceEvent, kind string) {
	rtObj, meta := objectMeta(obj)
	if rtObj == nil || meta == nil {
		return
	}

	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(rtObj)
	if err != nil {
		s.logger.Warn("failed to convert resource", "kind", kind, "error", err)
		return
	}

	evt := events.ResourceEvent{
		Kind:            kind,
		Namespace:       meta.GetNamespace(),
		Name:            meta.GetName(),
		ResourceVersion: meta.GetResourceVersion(),
		Object:          data,
		Raw:             rtObj,
	}

	select {
	case out <- evt:
	default:
		s.logger.Warn("event channel full, dropping", "key", evt.Key())
	}
}

func objectMeta(obj interface{}) (runtime.Object, metav1.Object) {
	switch typed := obj.(type) {
	case cache.DeletedFinalStateUnknown:
		return objectMeta(typed.Obj)
	case runtime.Object:
		meta, _ := typed.(metav1.Object)
		return typed, meta
	default:
		return nil, nil
	}
}
