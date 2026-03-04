package actions

import (
	"fmt"
	"sync"
	"time"

	"kube-watcher/watcher/internal/config"
)

type bucket struct {
	minuteStart time.Time
	minuteCount int
	hourStart   time.Time
	hourCount   int
}

type Throttler struct {
	mu      sync.Mutex
	buckets map[string]bucket
}

func NewThrottler() *Throttler {
	return &Throttler{
		buckets: make(map[string]bucket),
	}
}

func (t *Throttler) key(ruleName, resourceKey string) string {
	if resourceKey == "" {
		return ruleName
	}
	return fmt.Sprintf("%s:%s", ruleName, resourceKey)
}

func (t *Throttler) Allow(ruleName, resourceKey string, throttle config.Throttle) bool {
	if throttle.MaxPerMinute == 0 && throttle.MaxPerHour == 0 {
		return true
	}

	key := t.key(ruleName, resourceKey)
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	b, ok := t.buckets[key]
	if !ok {
		b = bucket{}
	}

	// Reset minute counter if the minute has passed
	if now.Truncate(time.Minute) != b.minuteStart.Truncate(time.Minute) {
		b.minuteStart = now
		b.minuteCount = 0
	}

	// Reset hour counter if the hour has passed
	if now.Truncate(time.Hour) != b.hourStart.Truncate(time.Hour) {
		b.hourStart = now
		b.hourCount = 0
	}

	if throttle.MaxPerMinute > 0 && b.minuteCount >= throttle.MaxPerMinute {
		t.buckets[key] = b
		return false
	}
	if throttle.MaxPerHour > 0 && b.hourCount >= throttle.MaxPerHour {
		t.buckets[key] = b
		return false
	}

	if throttle.MaxPerMinute > 0 {
		b.minuteCount++
	}
	if throttle.MaxPerHour > 0 {
		b.hourCount++
	}

	t.buckets[key] = b
	return true
}
