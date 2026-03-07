package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    float64
		wantOK  bool
	}{
		{"float64", 42.5, 42.5, true},
		{"float32", float32(3.14), 3.140000104904175, true},
		{"int", 10, 10.0, true},
		{"int32", int32(20), 20.0, true},
		{"int64", int64(30), 30.0, true},
		{"k8s_cpu_milli", "500m", 0.5, true},
		{"k8s_memory_gi", "1Gi", 1073741824.0, true},
		{"k8s_plain_number", "100", 100.0, true},
		{"invalid_string", "not-a-number", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
		{"slice", []int{1}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToFloat64(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.InDelta(t, tt.want, got, 0.01)
			}
		})
	}
}
