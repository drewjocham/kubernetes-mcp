// Package agenttelemetry normalizes agent probe telemetry so ingest and processor
// accept both legacy map-shaped metrics and []Metric arrays from cmd/probe.
package agenttelemetry

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NormalizeMetricsJSON parses the JSON "metrics" field (object or array) into a flat map.
func NormalizeMetricsJSON(raw json.RawMessage) map[string]interface{} {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]interface{}{}
	}
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return map[string]interface{}{}
	}
	return NormalizeMetrics(v)
}

// NormalizeMetrics converts metrics from either a JSON object or a probe metric slice
// into a single map[string]interface{} suitable for Pub/Sub and anomaly checks.
func NormalizeMetrics(m interface{}) map[string]interface{} {
	if m == nil {
		return map[string]interface{}{}
	}
	switch v := m.(type) {
	case map[string]interface{}:
		return v
	case []interface{}:
		return normalizeMetricSlice(v)
	default:
		return map[string]interface{}{}
	}
}

func normalizeMetricSlice(items []interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	var cpuMax float64
	var cpuOK bool

	for _, item := range items {
		obj, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := obj["name"].(string)
		if name == "" {
			continue
		}
		val, ok := toFloat64(obj["value"])
		if !ok {
			continue
		}
		labels := extractStringMap(obj["labels"])
		key := metricMapKey(name, labels)
		out[key] = val

		if strings.HasPrefix(name, "cpu_usage_percent") || name == "cpu_used_percent" {
			if !cpuOK || val > cpuMax {
				cpuMax = val
				cpuOK = true
			}
		}
	}
	if cpuOK {
		// detectAnomalies expects a single aggregate key from older probes
		out["cpu_used_percent"] = cpuMax
	}
	return out
}

func metricMapKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	for _, k := range keys {
		_, _ = fmt.Fprintf(&b, "|%s=%s", k, labels[k])
	}
	return b.String()
}

func extractStringMap(v interface{}) map[string]string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil
	}
	out := make(map[string]string)
	for k, val := range m {
		if s, ok := val.(string); ok {
			out[k] = s
		}
	}
	return out
}

func toFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
