package tracker

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMetricStore_Observe(t *testing.T) {
	store := NewMetricStore(time.Hour)
	resourceKey := "pod/default/my-pod"
	field := "cpu"

	delta1 := store.Observe(resourceKey, field, 100.0)
	assert.False(t, delta1.HasPrevious, "first observation should not have a previous value")

	time.Sleep(10 * time.Millisecond)
	delta2 := store.Observe(resourceKey, field, 150.0)
	assert.True(t, delta2.HasPrevious, "second observation should have a previous value")
	assert.InDelta(t, 50.0, delta2.Delta, 0.001, "delta should be the difference between values")
	assert.Greater(t, delta2.RatePerSec, 0.0, "rate per second should be positive")

	// Observation with no field
	delta3 := store.Observe(resourceKey, "", 200.0)
	assert.False(t, delta3.HasPrevious, "observation with no field should be a no-op")
	assert.Zero(t, delta3.Delta)
	assert.Zero(t, delta3.RatePerSec)
}

func TestMetricStore_evictExpired(t *testing.T) {
	retention := 50 * time.Millisecond
	store := NewMetricStore(retention)
	key1 := "pod/default/p1|cpu"
	key2 := "pod/default/p2|memory"

	store.mu.Lock()
	store.values[key1] = MetricPoint{Value: 1.0, Timestamp: time.Now().Add(-2 * retention)}
	store.values[key2] = MetricPoint{Value: 2.0, Timestamp: time.Now()}
	store.mu.Unlock()

	store.evictExpired()

	store.mu.RLock()
	defer store.mu.RUnlock()

	_, ok1 := store.values[key1]
	assert.False(t, ok1, "expired metric should be evicted")

	_, ok2 := store.values[key2]
	assert.True(t, ok2, "non-expired metric should be retained")
}

func TestMetricStore_CleanupLoop(t *testing.T) {
	retention := 50 * time.Millisecond
	interval := 10 * time.Millisecond
	store := NewMetricStore(retention)
	key := "pod/default/p1|cpu"

	store.mu.Lock()
	store.values[key] = MetricPoint{Value: 1.0, Timestamp: time.Now()}
	store.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go store.CleanupLoop(ctx, interval)

	time.Sleep(retention + 2*interval)

	store.mu.RLock()
	defer store.mu.RUnlock()

	_, ok := store.values[key]
	assert.False(t, ok, "metric should be evicted by the cleanup loop")
}
