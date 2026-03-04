package graph

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"

	"kube-watcher/watcher/internal/tracker"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name        string
		field       string
		history     []tracker.Snapshot
		expectError bool
		errorMsg    string
	}{
		{
			name:        "empty history",
			field:       "cpu",
			history:     []tracker.Snapshot{},
			expectError: true,
			errorMsg:    "no history available",
		},
		{
			name:  "field with no numeric history",
			field: "memory",
			history: []tracker.Snapshot{
				{Timestamp: time.Now(), Values: map[string]interface{}{"cpu": "100m"}},
			},
			expectError: true,
			errorMsg:    "field memory has no numeric history",
		},
		{
			name:  "successful render with int values",
			field: "cpu",
			history: []tracker.Snapshot{
				{Timestamp: time.Now(), Values: map[string]interface{}{"cpu": 1}},
				{Timestamp: time.Now().Add(time.Minute), Values: map[string]interface{}{"cpu": 2}},
			},
			expectError: false,
		},
		{
			name:  "successful render with float64 values",
			field: "memory",
			history: []tracker.Snapshot{
				{Timestamp: time.Now(), Values: map[string]interface{}{"memory": 1.5}},
				{Timestamp: time.Now().Add(time.Minute), Values: map[string]interface{}{"memory": 2.5}},
			},
			expectError: false,
		},
		{
			name:  "successful render with quantity strings",
			field: "cpu",
			history: []tracker.Snapshot{
				{Timestamp: time.Now(), Values: map[string]interface{}{"cpu": "100m"}},
				{Timestamp: time.Now().Add(time.Minute), Values: map[string]interface{}{"cpu": "200m"}},
			},
			expectError: false,
		},
		{
			name:  "mixed data types with some non-numeric",
			field: "cpu",
			history: []tracker.Snapshot{
				{Timestamp: time.Now(), Values: map[string]interface{}{"cpu": 1}},
				{Timestamp: time.Now().Add(time.Minute), Values: map[string]interface{}{"cpu": "200m"}},
				{Timestamp: time.Now().Add(2 * time.Minute), Values: map[string]interface{}{"cpu": "not-a-number"}},
			},
			expectError: false,
		},
		{
			name:  "history with nil values map",
			field: "cpu",
			history: []tracker.Snapshot{
				{Timestamp: time.Now(), Values: map[string]interface{}{"cpu": 1}},
				{Timestamp: time.Now().Add(time.Minute), Values: nil},
			},
			expectError: false,
		},
		{
			name:  "history with zero timestamp",
			field: "cpu",
			history: []tracker.Snapshot{
				{Values: map[string]interface{}{"cpu": 1}},
				{Values: map[string]interface{}{"cpu": 2}},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Render(tt.field, tt.history)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				// We can't easily validate the content of the PNG,
				// but we can check that it's not empty.
				assert.True(t, len(result) > 0)
			}
		})
	}
}

func Test_valueToFloat(t *testing.T) {
	q, _ := resource.ParseQuantity("100m")

	tests := []struct {
		name          string
		value         interface{}
		expectedFloat float64
		expectedOK    bool
	}{
		{"float64", 1.23, 1.23, true},
		{"float32", float32(1.23), 1.23, true},
		{"int", 123, 123.0, true},
		{"int32", int32(123), 123.0, true},
		{"int64", int64(123), 123.0, true},
		{"quantity string", "100m", q.AsApproximateFloat64(), true},
		{"invalid quantity string", "abc", 0, false},
		{"unsupported type", []string{"a"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, ok := valueToFloat(tt.value)
			assert.Equal(t, tt.expectedOK, ok)
			if ok {
				// Allow for small floating point inaccuracies
				assert.InDelta(t, tt.expectedFloat, f, 0.001)
			}
		})
	}
}
