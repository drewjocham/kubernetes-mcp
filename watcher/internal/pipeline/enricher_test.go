package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"kube-watcher/watcher/internal/events"
)

func TestPodEnricher_Enrich(t *testing.T) {
	testCases := []struct {
		name   string
		kind   string
		fields []string
		object map[string]interface{}
		want   map[string]interface{}
	}{
		{
			name:   "enrich pod with restart count and crash loop",
			kind:   "Pod",
			fields: []string{"restart_count", "crash_looping"},
			object: map[string]interface{}{
				"status": map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{
							"restartCount": 2,
							"state": map[string]interface{}{
								"waiting": map[string]interface{}{"reason": "CrashLoopBackOff"},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"restart_count": 2,
				"crash_looping": true,
			},
		},
		{
			name:   "enrich pod with resource requests and limits",
			kind:   "Pod",
			fields: []string{"cpu_request", "memory_limit"},
			object: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"resources": map[string]interface{}{
								"requests": map[string]interface{}{"cpu": "100m"},
								"limits":   map[string]interface{}{"memory": "512Mi"},
							},
						},
					},
				},
			},
			want: map[string]interface{}{
				"cpu_request":  "100m",
				"memory_limit": "512Mi",
			},
		},
		{
			name:   "enrich node with pressure and readiness",
			kind:   "Node",
			fields: []string{"node_memory_pressure", "node_ready"},
			object: map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{"type": "MemoryPressure", "status": "True"},
						map[string]interface{}{"type": "Ready", "status": "False"},
					},
				},
			},
			want: map[string]interface{}{
				"node_memory_pressure": true,
				"node_ready":           false,
			},
		},
		{
			name:   "enrich hpa with replica counts and saturation",
			kind:   "HorizontalPodAutoscaler",
			fields: []string{"min_pod_count", "max_pod_count", "current_pod_count", "hpa_saturation_ratio"},
			object: map[string]interface{}{
				"spec":   map[string]interface{}{"minReplicas": 2, "maxReplicas": 10},
				"status": map[string]interface{}{"currentReplicas": 5},
			},
			want: map[string]interface{}{
				"min_pod_count":        2,
				"max_pod_count":        10,
				"current_pod_count":    5,
				"hpa_saturation_ratio": 0.5,
			},
		},
		{
			name:   "enrich with no specific fields (all supported fields)",
			kind:   "Pod",
			fields: nil, // This should enable all fields
			object: map[string]interface{}{
				"status": map[string]interface{}{
					"containerStatuses": []interface{}{
						map[string]interface{}{"restartCount": 1},
					},
				},
			},
			want: map[string]interface{}{
				"restart_count": 1,
			},
		},
		{
			name:   "enrich with empty object",
			kind:   "Pod",
			fields: []string{"restart_count"},
			object: nil,
			want:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			enricher := NewPodEnricher(tc.fields)
			event := events.ResourceEvent{
				Kind:   tc.kind,
				Object: tc.object,
			}

			enrichedEvent, err := enricher.Enrich(context.Background(), event)
			assert.NoError(t, err)

			if tc.want == nil {
				assert.Nil(t, enrichedEvent.Object)
				return
			}

			for key, expectedValue := range tc.want {
				assert.Contains(t, enrichedEvent.Object, key)
				assert.Equal(t, expectedValue, enrichedEvent.Object[key], "field: %s", key)
			}

			if tc.fields == nil && tc.kind == "Pod" {
				assert.Contains(t, enrichedEvent.Object, "is_ready")
			}
		})
	}
}
