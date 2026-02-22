package pipeline

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	"kube-watcher/watcher/internal/events"
)

var supportedFields = map[string]struct{}{
	"restart_count":  {},
	"cpu_request":    {},
	"memory_request": {},
	"ram":            {},
	"replicas":       {},
	"min_pod_count":  {},
	"max_pod_count":  {},
}

type PodEnricher struct {
	fields map[string]struct{}
}

func NewPodEnricher(fields []string) *PodEnricher {
	set := make(map[string]struct{})
	for _, f := range fields {
		if f == "" {
			continue
		}
		name := strings.ToLower(f)
		if _, ok := supportedFields[name]; ok {
			set[name] = struct{}{}
		}
	}
	if len(set) == 0 {
		for k := range supportedFields {
			set[k] = struct{}{}
		}
	}
	return &PodEnricher{fields: set}
}

func (p *PodEnricher) Enrich(_ context.Context, evt events.ResourceEvent) (events.ResourceEvent, error) {
	if evt.Object == nil {
		return evt, nil
	}

	switch strings.ToLower(evt.Kind) {
	case "pod":
		p.enrichPod(evt.Object)
	case "horizontalpodautoscaler":
		p.enrichHPA(evt.Object)
	}

	return evt, nil
}

func (p *PodEnricher) enabled(field string) bool {
	_, ok := p.fields[field]
	return ok
}

func restartCount(obj map[string]interface{}) (int, bool) {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return 0, false
	}
	rawStatuses, ok := status["containerStatuses"].([]interface{})
	if !ok {
		return 0, false
	}
	total := 0
	for _, item := range rawStatuses {
		cs, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if rc, ok := toInt(cs["restartCount"]); ok {
			total += rc
		}
	}
	return total, true
}

func aggregateResource(obj map[string]interface{}, resourceName string) (string, bool) {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return "", false
	}
	rawContainers, ok := spec["containers"].([]interface{})
	if !ok {
		return "", false
	}

	var total resource.Quantity
	found := false
	for _, item := range rawContainers {
		container, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		resources, _ := container["resources"].(map[string]interface{})
		requests, _ := resources["requests"].(map[string]interface{})
		if requests == nil {
			continue
		}
		val, ok := requests[resourceName]
		if !ok {
			continue
		}
		q, err := resource.ParseQuantity(toString(val))
		if err != nil {
			continue
		}
		if !found {
			total = q
			found = true
		} else {
			total.Add(q)
		}
	}
	if !found {
		return "", false
	}
	return total.String(), true
}

func replicas(obj map[string]interface{}) (int, bool) {
	if spec, ok := obj["spec"].(map[string]interface{}); ok {
		if val, ok := toInt(spec["replicas"]); ok {
			return val, true
		}
	}
	if status, ok := obj["status"].(map[string]interface{}); ok {
		if val, ok := toInt(status["replicas"]); ok {
			return val, true
		}
	}
	return 0, false
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int32:
		return int(val), true
	case int64:
		return int(val), true
	case float64:
		return int(val), true
	case float32:
		return int(val), true
	case string:
		if parsed, err := resource.ParseQuantity(val); err == nil {
			return int(parsed.Value()), true
		}
	}
	return 0, false
}

func (p *PodEnricher) enrichPod(obj map[string]interface{}) {
	if p.enabled("restart_count") {
		if val, ok := restartCount(obj); ok {
			obj["restart_count"] = val
		} else {
			obj["restart_count"] = 0
		}
	}

	if p.enabled("cpu_request") {
		val, ok := aggregateResource(obj, "cpu")
		if !ok {
			val = "0"
		}
		obj["cpu_request"] = val
	}

	if p.enabled("memory_request") || p.enabled("ram") {
		val, ok := aggregateResource(obj, "memory")
		if !ok {
			val = "0"
		}
		if p.enabled("memory_request") {
			obj["memory_request"] = val
		}
		if p.enabled("ram") {
			obj["ram"] = val
		}
	}

	if p.enabled("replicas") {
		if val, ok := replicas(obj); ok {
			obj["replicas"] = val
		} else {
			obj["replicas"] = 0
		}
	}
}

func (p *PodEnricher) enrichHPA(obj map[string]interface{}) {
	if p.enabled("min_pod_count") {
		if val, ok := hpaReplicaBound(obj, "min"); ok {
			obj["min_pod_count"] = val
		} else {
			obj["min_pod_count"] = 1
		}
	}

	if p.enabled("max_pod_count") {
		if val, ok := hpaReplicaBound(obj, "max"); ok {
			obj["max_pod_count"] = val
		} else {
			obj["max_pod_count"] = 0
		}
	}
}
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return ""
	}
}

func hpaReplicaBound(obj map[string]interface{}, bound string) (int, bool) {
	spec, ok := obj["spec"].(map[string]interface{})
	if !ok {
		return 0, false
	}
	switch bound {
	case "min":
		if val, ok := toInt(spec["minReplicas"]); ok {
			return val, true
		}
		return 1, true
	case "max":
		if val, ok := toInt(spec["maxReplicas"]); ok {
			return val, true
		}
	}
	return 0, false
}
