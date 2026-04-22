package source

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kube-watcher/watcher/internal/events"
)

// Ensure events package is used (for linting)
var _ events.ResourceEvent

func TestParsePubSubMessageData_Valid(t *testing.T) {
	// Create a valid pubsub message payload
	anomalies := []Anomaly{
		{
			ID:        "test-id-1",
			Title:     "Test Anomaly 1",
			Severity:  "critical",
			Message:   "Test message 1",
			Timestamp: time.Now().Unix(),
		},
		{
			ID:        "test-id-2",
			Title:     "Test Anomaly 2",
			Severity:  "warning",
			Message:   "Test message 2",
			Timestamp: time.Now().Unix() - 100,
		},
	}
	payload := map[string]interface{}{
		"anomalies": anomalies,
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	// Parse the data
	evts, err := parsePubSubMessageData(data)
	require.NoError(t, err)
	assert.Len(t, evts, 2)

	// Check first event
	assert.Equal(t, "Anomaly", evts[0].Kind)
	assert.Equal(t, "anomstack", evts[0].Namespace)
	assert.Equal(t, "test-id-1", evts[0].Name)
	assert.Equal(t, "critical", evts[0].Object["severity"])
	assert.Equal(t, "Test Anomaly 1", evts[0].Object["title"])
	assert.Equal(t, "Test message 1", evts[0].Object["message"])
	assert.Equal(t, anomalies[0].Timestamp, evts[0].Object["timestamp"])

	// Check second event
	assert.Equal(t, "test-id-2", evts[1].Name)
	assert.Equal(t, "warning", evts[1].Object["severity"])
}

func TestParsePubSubMessageData_InvalidJSON(t *testing.T) {
	// Invalid JSON
	data := []byte("invalid json")
	_, err := parsePubSubMessageData(data)
	assert.Error(t, err)
}

func TestParsePubSubMessageData_EmptyAnomalies(t *testing.T) {
	// JSON with empty anomalies array
	payload := map[string]interface{}{
		"anomalies": []Anomaly{},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	evts, err := parsePubSubMessageData(data)
	require.NoError(t, err)
	assert.Empty(t, evts)
}

func TestParsePubSubMessageData_MissingAnomaliesField(t *testing.T) {
	// JSON without anomalies field
	payload := map[string]interface{}{
		"other": "field",
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	evts, err := parsePubSubMessageData(data)
	require.NoError(t, err)
	// Should return empty slice, not error
	assert.Empty(t, evts)
}

func TestParsePubSubMessageData_NullAnomalies(t *testing.T) {
	// JSON with null anomalies
	payload := map[string]interface{}{
		"anomalies": nil,
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	evts, err := parsePubSubMessageData(data)
	require.NoError(t, err)
	assert.Empty(t, evts)
}

func TestParsePubSubMessageData_WrongAnomaliesType(t *testing.T) {
	// JSON with wrong type for anomalies
	payload := map[string]interface{}{
		"anomalies": "not an array",
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	_, err = parsePubSubMessageData(data)
	assert.Error(t, err)
}

func TestParsePubSubMessageData_PartialAnomaly(t *testing.T) {
	// Test with partial anomaly data (missing some fields)
	// The struct tags should handle missing fields
	payload := map[string]interface{}{
		"anomalies": []map[string]interface{}{
			{
				"id":    "test-id",
				"title": "Test",
				// missing severity, message, timestamp
			},
		},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	evts, err := parsePubSubMessageData(data)
	// JSON unmarshal should succeed but fields will be zero values
	require.NoError(t, err)
	require.Len(t, evts, 1)
	assert.Equal(t, "test-id", evts[0].Name)
	assert.Equal(t, "", evts[0].Object["severity"]) // zero value
	assert.Equal(t, "Test", evts[0].Object["title"])
	assert.Equal(t, "", evts[0].Object["message"])         // zero value
	assert.Equal(t, int64(0), evts[0].Object["timestamp"]) // zero value
}
