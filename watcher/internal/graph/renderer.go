package graph

import (
	"bytes"
	"fmt"

	chart "github.com/wcharczuk/go-chart/v2"
	"k8s.io/apimachinery/pkg/api/resource"

	"kube-watcher/watcher/internal/tracker"
)

func Render(field string, history []tracker.Snapshot) ([]byte, error) {
	if len(history) == 0 {
		return nil, fmt.Errorf("no history available")
	}

	var xValues []float64
	var yValues []float64

	for i, snap := range history {
		if snap.Values == nil {
			continue
		}
		val, ok := valueToFloat(snap.Values[field])
		if !ok {
			continue
		}
		if !snap.Timestamp.IsZero() {
			xValues = append(xValues, float64(snap.Timestamp.Unix()))
		} else {
			xValues = append(xValues, float64(i))
		}
		yValues = append(yValues, val)
	}

	if len(xValues) == 0 {
		return nil, fmt.Errorf("field %s has no numeric history", field)
	}

	graph := chart.Chart{
		Series: []chart.Series{
			chart.ContinuousSeries{
				Name:    field,
				XValues: xValues,
				YValues: yValues,
			},
		},
	}

	var buf bytes.Buffer
	if err := graph.Render(chart.PNG, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func valueToFloat(v interface{}) (float64, bool) {
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
