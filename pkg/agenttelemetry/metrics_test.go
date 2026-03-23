package agenttelemetry

import (
	"encoding/json"
	"testing"
)

func TestNormalizeMetrics_mapPassthrough(t *testing.T) {
	in := map[string]interface{}{
		"cpu_used_percent":    42.0,
		"memory_used_percent": 50.0,
	}
	out := NormalizeMetrics(in)
	if out["cpu_used_percent"] != 42.0 {
		t.Fatalf("cpu: got %v", out["cpu_used_percent"])
	}
}

func TestNormalizeMetrics_sliceToMapAndCPUAggregate(t *testing.T) {
	raw := []interface{}{
		map[string]interface{}{
			"name":  "cpu_usage_percent",
			"value": 12.0,
			"labels": map[string]interface{}{
				"cpu": "cpu0",
			},
		},
		map[string]interface{}{
			"name":  "cpu_usage_percent",
			"value": 88.0,
			"labels": map[string]interface{}{
				"cpu": "cpu1",
			},
		},
		map[string]interface{}{
			"name":  "memory_used_percent",
			"value": 91.0,
		},
	}
	out := NormalizeMetrics(raw)
	if out["cpu_used_percent"] != 88.0 {
		t.Fatalf("expected cpu_used_percent 88, got %v", out["cpu_used_percent"])
	}
	if out["memory_used_percent"] != 91.0 {
		t.Fatalf("memory: got %v", out["memory_used_percent"])
	}
	if _, ok := out["cpu_usage_percent|cpu=cpu1"]; !ok {
		t.Fatalf("expected labeled key, got keys %#v", out)
	}
}

func TestNormalizeMetricsJSON(t *testing.T) {
	m, err := json.Marshal([]map[string]interface{}{
		{"name": "memory_used_percent", "value": 77},
	})
	if err != nil {
		t.Fatal(err)
	}
	out := NormalizeMetricsJSON(m)
	if out["memory_used_percent"] != float64(77) {
		t.Fatalf("got %v", out["memory_used_percent"])
	}
}
