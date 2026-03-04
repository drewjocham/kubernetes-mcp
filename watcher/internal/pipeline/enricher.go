package pipeline

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

	"kube-watcher/watcher/internal/events"
)

var supportedFields = map[string]struct{}{
	"restart_count":          {},
	"restart_delta":          {},
	"cpu_request":            {},
	"memory_request":         {},
	"ram":                    {},
	"cpu_limit":              {},
	"memory_limit":           {},
	"cpu_limit_gap":          {},
	"mem_limit_gap":          {},
	"cpu_usage_ratio":        {},
	"mem_usage_ratio":        {},
	"replicas":               {},
	"min_pod_count":          {},
	"max_pod_count":          {},
	"crash_looping":          {},
	"crash_reason":           {},
	"containers_not_ready":   {},
	"container_ready_ratio":  {},
	"oom_killed":             {},
	"waiting_reason":         {},
	"waiting_reasons":        {},
	"waiting_message":        {},
	"is_ready":               {},
	"is_terminating":         {},
	"current_pod_count":      {},
	"desired_pod_count":      {},
	"current_replicas":       {},
	"current_replicas_delta": {},
	"desired_replicas":       {},
	"desired_replicas_delta": {},
	"hpa_at_max_capacity":    {},
	"hpa_at_min_capacity":    {},
	"hpa_saturation_ratio":   {},
	"hpa_is_stalled":         {},
	"node_memory_pressure":   {},
	"node_disk_pressure":     {},
	"node_pid_pressure":      {},
	"node_ready":             {},
	"cpu_allocatable_m":      {},
	"mem_allocatable_mi":     {},
	"cpu_capacity_m":         {},
	"mem_capacity_mi":        {},
}

type PodEnricher struct {
	fields map[string]struct{}
}

var defaultTrackedFields = []string{
	"restart_count",
	"waiting_reason",
	"waiting_reasons",
	"waiting_message",
	"is_ready",
	"is_terminating",
	"crash_looping",
	"crash_reason",
	"hpa_at_max_capacity",
	"hpa_saturation_ratio",
	"hpa_is_stalled",
	"node_memory_pressure",
	"node_disk_pressure",
	"node_ready",
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
	} else {
		for _, f := range defaultTrackedFields {
			set[f] = struct{}{}
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
	case "node":
		p.enrichNode(evt.Object)
	}

	return evt, nil
}

func (p *PodEnricher) enabled(field string) bool {
	_, ok := p.fields[field]
	return ok
}

func aggregateResource(obj map[string]interface{}, resourceName, resourceType string) (string, bool) {
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
		target, _ := resources[resourceType].(map[string]interface{})
		if target == nil {
			continue
		}
		val, ok := target[resourceName]
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
	stats := gatherContainerStats(obj)

	if p.enabled("restart_count") {
		obj["restart_count"] = stats.restarts
	}
	if p.enabled("crash_looping") {
		obj["crash_looping"] = stats.crashLooping
	}
	if p.enabled("crash_reason") && stats.crashReason != "" {
		obj["crash_reason"] = stats.crashReason
	}
	if p.enabled("containers_not_ready") {
		obj["containers_not_ready"] = stats.notReady
	}
	if p.enabled("container_ready_ratio") {
		obj["container_ready_ratio"] = stats.readyRatio
	}
	if p.enabled("oom_killed") {
		obj["oom_killed"] = stats.oomKilled
	}
	if p.enabled("waiting_reasons") && len(stats.waitingReasons) > 0 {
		obj["waiting_reasons"] = stats.waitingReasons
	}
	if p.enabled("waiting_reason") && len(stats.waitingReasons) > 0 {
		obj["waiting_reason"] = stats.waitingReasons[0]
	}
	if p.enabled("waiting_message") && len(stats.waitingMessages) > 0 {
		obj["waiting_message"] = strings.Join(stats.waitingMessages, " | ")
	}
	if p.enabled("is_ready") {
		obj["is_ready"] = stats.notReady == 0
	}
	if p.enabled("is_terminating") {
		obj["is_terminating"] = isTerminating(obj)
	}

	if p.enabled("cpu_request") {
		if val, ok := aggregateResource(obj, "cpu", "requests"); ok {
			obj["cpu_request"] = val
		} else {
			obj["cpu_request"] = "0"
		}
	}
	if p.enabled("memory_request") || p.enabled("ram") {
		val, ok := aggregateResource(obj, "memory", "requests")
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
	if p.enabled("cpu_limit") || p.enabled("cpu_limit_gap") || p.enabled("cpu_usage_ratio") {
		if val, ok := aggregateResource(obj, "cpu", "limits"); ok {
			if p.enabled("cpu_limit") {
				obj["cpu_limit"] = val
			}
		}
	}
	if p.enabled("memory_limit") || p.enabled("mem_limit_gap") || p.enabled("mem_usage_ratio") {
		if val, ok := aggregateResource(obj, "memory", "limits"); ok {
			if p.enabled("memory_limit") {
				obj["memory_limit"] = val
			}
		}
	}
	p.applyResourceGaps(obj)

	if p.enabled("replicas") {
		if val, ok := replicas(obj); ok {
			obj["replicas"] = val
		} else {
			obj["replicas"] = 0
		}
	}
}

func (p *PodEnricher) applyResourceGaps(obj map[string]interface{}) {
	reqCPU := toMillicores(obj["cpu_request"])
	limitCPU := toMillicores(obj["cpu_limit"])
	reqMem := toMiB(obj["memory_request"])
	limitMem := toMiB(obj["memory_limit"])

	if p.enabled("cpu_limit_gap") && limitCPU > 0 {
		obj["cpu_limit_gap"] = limitCPU - reqCPU
	}
	if p.enabled("mem_limit_gap") && limitMem > 0 {
		obj["mem_limit_gap"] = limitMem - reqMem
	}
	if p.enabled("cpu_usage_ratio") && limitCPU > 0 {
		obj["cpu_usage_ratio"] = ratio(reqCPU, limitCPU)
	}
	if p.enabled("mem_usage_ratio") && limitMem > 0 {
		obj["mem_usage_ratio"] = ratio(reqMem, limitMem)
	}
}

func ratio(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func toMillicores(v interface{}) int64 {
	if v == nil {
		return 0
	}
	q, err := resource.ParseQuantity(toString(v))
	if err != nil {
		return 0
	}
	return q.MilliValue()
}

func toMiB(v interface{}) int64 {
	if v == nil {
		return 0
	}
	q, err := resource.ParseQuantity(toString(v))
	if err != nil {
		return 0
	}
	return q.Value() / (1024 * 1024)
}

func isTerminating(obj map[string]interface{}) bool {
	metadata, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return false
	}
	_, terminating := metadata["deletionTimestamp"]
	return terminating
}

func (p *PodEnricher) enrichNode(obj map[string]interface{}) {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return
	}

	if conditions, ok := status["conditions"].([]interface{}); ok {
		for _, raw := range conditions {
			cond, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			condType := strings.ToLower(toString(cond["type"]))
			isTrue := strings.EqualFold(toString(cond["status"]), "true")

			switch condType {
			case "memorypressure":
				if p.enabled("node_memory_pressure") {
					obj["node_memory_pressure"] = isTrue
				}
			case "diskpressure":
				if p.enabled("node_disk_pressure") {
					obj["node_disk_pressure"] = isTrue
				}
			case "pidpressure":
				if p.enabled("node_pid_pressure") {
					obj["node_pid_pressure"] = isTrue
				}
			case "ready":
				if p.enabled("node_ready") {
					obj["node_ready"] = isTrue
				}
			}
		}
	}

	if allocatable, ok := status["allocatable"].(map[string]interface{}); ok {
		if p.enabled("cpu_allocatable_m") {
			obj["cpu_allocatable_m"] = toMillicores(allocatable["cpu"])
		}
		if p.enabled("mem_allocatable_mi") {
			obj["mem_allocatable_mi"] = toMiB(allocatable["memory"])
		}
	}
	if capacity, ok := status["capacity"].(map[string]interface{}); ok {
		if p.enabled("cpu_capacity_m") {
			obj["cpu_capacity_m"] = toMillicores(capacity["cpu"])
		}
		if p.enabled("mem_capacity_mi") {
			obj["mem_capacity_mi"] = toMiB(capacity["memory"])
		}
	}
}
func (p *PodEnricher) enrichHPA(obj map[string]interface{}) {
	min, minOK := hpaReplicaBound(obj, "min")
	max, maxOK := hpaReplicaBound(obj, "max")
	status, _ := obj["status"].(map[string]interface{})

	current, currentOK := hpaStatusReplicas(obj, "currentReplicas")
	desired, desiredOK := hpaStatusReplicas(obj, "desiredReplicas")

	if p.enabled("min_pod_count") && minOK {
		obj["min_pod_count"] = min
	}
	if p.enabled("max_pod_count") && maxOK {
		obj["max_pod_count"] = max
	}

	if p.enabled("current_pod_count") {
		obj["current_pod_count"] = current
	}
	if p.enabled("desired_pod_count") {
		if desiredOK {
			obj["desired_pod_count"] = desired
		} else {
			obj["desired_pod_count"] = current
		}
	}
	if p.enabled("current_replicas") {
		obj["current_replicas"] = current
	}
	if p.enabled("desired_replicas") && desiredOK {
		obj["desired_replicas"] = desired
	}

	if p.enabled("hpa_at_max_capacity") && maxOK && currentOK {
		obj["hpa_at_max_capacity"] = current >= max
	}
	if p.enabled("hpa_at_min_capacity") {
		minThreshold := 1
		if minOK && min > 0 {
			minThreshold = min
		}
		obj["hpa_at_min_capacity"] = current <= minThreshold
	}
	if p.enabled("hpa_saturation_ratio") && maxOK && max > 0 {
		obj["hpa_saturation_ratio"] = float64(current) / float64(max)
	}
	if p.enabled("hpa_is_stalled") && maxOK && desiredOK && currentOK && max > 0 {
		obj["hpa_is_stalled"] = current >= max && desired > current
	}

	if p.enabled("current_pod_count") && !currentOK && status != nil {
		if val, ok := toInt(status["currentReplicas"]); ok {
			obj["current_pod_count"] = val
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

func hpaStatusReplicas(obj map[string]interface{}, key string) (int, bool) {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return 0, false
	}
	return toInt(status[key])
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

type containerStats struct {
	restarts        int
	notReady        int
	readyRatio      float64
	crashLooping    bool
	crashReason     string
	oomKilled       bool
	waitingReasons  []string
	waitingMessages []string
}

func gatherContainerStats(obj map[string]interface{}) containerStats {
	statuses := collectContainerStatuses(obj)
	if len(statuses) == 0 {
		return containerStats{readyRatio: 1}
	}

	var stats containerStats

	for _, cs := range statuses {
		stats.restarts += restartValue(cs)

		if !isContainerReady(cs) {
			stats.notReady++
		}

		if reason := waitingReason(cs); reason != "" {
			stats.waitingReasons = appendUnique(stats.waitingReasons, reason)
			if isCrashLoopReason(reason) {
				stats.crashLooping = true
				if stats.crashReason == "" {
					stats.crashReason = reason
				}
			}
		}
		if msg := waitingMessage(cs); msg != "" {
			stats.waitingMessages = appendUnique(stats.waitingMessages, msg)
		}

		if termReason := terminationReason(cs); strings.EqualFold(termReason, "oomkilled") {
			stats.oomKilled = true
			if stats.crashReason == "" {
				stats.crashReason = termReason
			}
		}
	}

	total := len(statuses)
	stats.readyRatio = 1
	if total > 0 {
		stats.readyRatio = float64(total-stats.notReady) / float64(total)
	}

	return stats
}

func collectContainerStatuses(obj map[string]interface{}) []map[string]interface{} {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return nil
	}
	var out []map[string]interface{}
	for _, key := range []string{"containerStatuses", "initContainerStatuses"} {
		raw, _ := status[key].([]interface{})
		for _, item := range raw {
			if cs, ok := item.(map[string]interface{}); ok {
				out = append(out, cs)
			}
		}
	}
	return out
}

func appendUnique(list []string, value string) []string {
	if value == "" {
		return list
	}
	for _, existing := range list {
		if existing == value {
			return list
		}
	}
	return append(list, value)
}

func restartValue(cs map[string]interface{}) int {
	if rc, ok := toInt(cs["restartCount"]); ok {
		return rc
	}
	return 0
}

func isContainerReady(cs map[string]interface{}) bool {
	ready, ok := cs["ready"].(bool)
	return ok && ready
}

func waitingReason(cs map[string]interface{}) string {
	state, ok := cs["state"].(map[string]interface{})
	if !ok {
		return ""
	}
	if waiting, ok := state["waiting"].(map[string]interface{}); ok {
		if reason, _ := waiting["reason"].(string); reason != "" {
			return reason
		}
	}
	return ""
}

func waitingMessage(cs map[string]interface{}) string {
	state, ok := cs["state"].(map[string]interface{})
	if !ok {
		return ""
	}
	if waiting, ok := state["waiting"].(map[string]interface{}); ok {
		if msg, _ := waiting["message"].(string); msg != "" {
			return msg
		}
	}
	return ""
}

func terminationReason(cs map[string]interface{}) string {
	if lastState, ok := cs["lastState"].(map[string]interface{}); ok {
		if term, ok := lastState["terminated"].(map[string]interface{}); ok {
			if reason, _ := term["reason"].(string); reason != "" {
				return reason
			}
		}
	}
	if state, ok := cs["state"].(map[string]interface{}); ok {
		if term, ok := state["terminated"].(map[string]interface{}); ok {
			if reason, _ := term["reason"].(string); reason != "" {
				return reason
			}
		}
	}
	return ""
}

func isCrashLoopReason(reason string) bool {
	if reason == "" {
		return false
	}
	switch strings.ToLower(reason) {
	case "crashloopbackoff", "imagepullbackoff", "errimagepull":
		return true
	default:
		return strings.Contains(strings.ToLower(reason), "backoff")
	}
}
