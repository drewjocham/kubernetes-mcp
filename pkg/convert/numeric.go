package convert

import (
	"k8s.io/apimachinery/pkg/api/resource"
)

// ToFloat64 converts common numeric types and Kubernetes quantity strings to float64.
func ToFloat64(v interface{}) (float64, bool) {
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
