package tracker

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type MetricPoint struct {
	Value     float64
	Timestamp time.Time
}

type MetricDelta struct {
	Delta       float64
	RatePerSec  float64
	HasPrevious bool
}

type MetricStore struct {
	mu        sync.RWMutex
	values    map[string]MetricPoint
	retention time.Duration
}

func NewMetricStore(retention time.Duration) *MetricStore {
	if retention <= 0 {
		retention = time.Hour
	}
	return &MetricStore{
		values:    make(map[string]MetricPoint),
		retention: retention,
	}
}

// Observe records a numeric value for the given resource key and field.
// It returns the delta and per-second rate relative to the previous value.
func (m *MetricStore) Observe(resourceKey, field string, value float64) MetricDelta {
	if field == "" {
		return MetricDelta{}
	}
	key := fmt.Sprintf("%s|%s", resourceKey, field)
	now := time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	prev, ok := m.values[key]
	m.values[key] = MetricPoint{Value: value, Timestamp: now}

	if !ok {
		return MetricDelta{}
	}

	delta := value - prev.Value
	var rate float64
	if elapsed := now.Sub(prev.Timestamp).Seconds(); elapsed > 0 {
		rate = delta / elapsed
	}
	return MetricDelta{
		Delta:       delta,
		RatePerSec:  rate,
		HasPrevious: true,
	}
}

// CleanupLoop periodically removes stale metric entries based on retention.
func (m *MetricStore) CleanupLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.evictExpired()
		}
	}
}

func (m *MetricStore) evictExpired() {
	cutoff := time.Now().Add(-m.retention)

	m.mu.Lock()
	defer m.mu.Unlock()

	for key, point := range m.values {
		if point.Timestamp.Before(cutoff) {
			delete(m.values, key)
		}
	}
}
