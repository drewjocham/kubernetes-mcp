package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/api/resource"

	"kube-watcher/watcher/internal/events"
)

type Exporter struct {
	restarts *prometheus.GaugeVec
	cpuGap   *prometheus.GaugeVec
}

func NewExporter() *Exporter {
	e := &Exporter{
		restarts: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "kube_watcher",
			Name:      "pod_restart_count",
			Help:      "Current restart_count observed for pods",
		}, []string{"namespace", "pod"}),
		cpuGap: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "kube_watcher",
			Name:      "pod_cpu_limit_gap_milli",
			Help:      "Difference between CPU limit and request in millicores",
		}, []string{"namespace", "pod"}),
	}
	prometheus.MustRegister(e.restarts, e.cpuGap)
	return e
}

func (e *Exporter) Observe(evt events.ResourceEvent) {
	if evt.Object == nil || !strings.EqualFold(evt.Kind, "Pod") {
		return
	}
	if val, ok := toFloat(evt.Object["restart_count"]); ok {
		e.restarts.WithLabelValues(evt.Namespace, evt.Name).Set(val)
	}
	if val, ok := toFloat(evt.Object["cpu_limit_gap"]); ok {
		e.cpuGap.WithLabelValues(evt.Namespace, evt.Name).Set(val)
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		q, err := resource.ParseQuantity(val)
		if err != nil {
			return 0, false
		}
		return q.AsApproximateFloat64(), true
	}
	return 0, false
}
