package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/assert"

	"kube-watcher/watcher/internal/events"
)

func TestExporter_Observe(t *testing.T) {
	exporter := NewExporter()
	handler := promhttp.Handler()

	testCases := []struct {
		name           string
		event          events.ResourceEvent
		expectedMetric string
	}{
		{
			name: "pod restart metric",
			event: events.ResourceEvent{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod-1",
				Object:    map[string]interface{}{"restart_count": 5},
			},
			expectedMetric: `kube_watcher_pod_restart_count{namespace="default",pod="test-pod-1"} 5`,
		},
		{
			name: "pod cpu gap metric",
			event: events.ResourceEvent{
				Kind:      "Pod",
				Namespace: "kube-system",
				Name:      "test-pod-2",
				Object:    map[string]interface{}{"cpu_limit_gap": "200m"},
			},
			expectedMetric: `kube_watcher_pod_cpu_limit_gap_milli{namespace="kube-system",pod="test-pod-2"} 200`,
		},
		{
			name: "non-pod event is ignored",
			event: events.ResourceEvent{
				Kind: "Node",
				Name: "node-1",
			},
			expectedMetric: "",
		},
		{
			name: "event with no relevant metrics",
			event: events.ResourceEvent{
				Kind:      "Pod",
				Namespace: "default",
				Name:      "test-pod-3",
				Object:    map[string]interface{}{"other_field": "value"},
			},
			expectedMetric: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			exporter.Observe(tc.event)

			req := httptest.NewRequest("GET", "/metrics", nil)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusOK, rr.Code)
			body, _ := io.ReadAll(rr.Body)

			if tc.expectedMetric != "" {
				assert.Contains(t, string(body), tc.expectedMetric)
			} else {
				if tc.event.Name != "" {
					assert.NotContains(t, string(body), tc.event.Name)
				}
			}
		})
	}
}

func Test_toFloat(t *testing.T) {
	testCases := []struct {
		name   string
		value  interface{}
		want   float64
		wantOk bool
	}{
		{name: "float64", value: 1.23, want: 1.23, wantOk: true},
		{name: "int", value: 123, want: 123.0, wantOk: true},
		{name: "quantity string", value: "100m", want: 0.1, wantOk: true},
		{name: "invalid string", value: "abc", want: 0, wantOk: false},
		{name: "nil value", value: nil, want: 0, wantOk: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := toFloat(tc.value)
			assert.Equal(t, tc.wantOk, ok)
			if ok {
				assert.InDelta(t, tc.want, got, 0.001)
			}
		})
	}
}
