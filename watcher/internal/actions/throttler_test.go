package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"kube-watcher/watcher/internal/config"
)

func TestThrottler_Allow(t *testing.T) {
	testCases := []struct {
		name        string
		throttle    config.Throttle
		setup       func(throttler *Throttler, key string)
		iterations  int
		wantAllowed []bool
	}{
		{
			name:        "no throttle",
			throttle:    config.Throttle{},
			iterations:  5,
			wantAllowed: []bool{true, true, true, true, true},
		},
		{
			name:        "max per minute",
			throttle:    config.Throttle{MaxPerMinute: 2},
			iterations:  3,
			wantAllowed: []bool{true, true, false},
		},
		{
			name:        "max per hour",
			throttle:    config.Throttle{MaxPerHour: 1},
			iterations:  2,
			wantAllowed: []bool{true, false},
		},
		{
			name:        "minute limit resets",
			throttle:    config.Throttle{MaxPerMinute: 1},
			iterations:  2,
			wantAllowed: []bool{true, false},
			setup: func(throttler *Throttler, key string) {
				// Manually set the bucket start time to simulate a past minute
				throttler.mu.Lock()
				throttler.buckets[key] = bucket{
					minuteStart: time.Now().Add(-2 * time.Minute),
					minuteCount: 1,
				}
				throttler.mu.Unlock()
			},
		},
		{
			name:        "hour limit resets",
			throttle:    config.Throttle{MaxPerHour: 1},
			iterations:  2,
			wantAllowed: []bool{true, false},
			setup: func(throttler *Throttler, key string) {
				throttler.mu.Lock()
				throttler.buckets[key] = bucket{
					hourStart: time.Now().Add(-2 * time.Hour),
					hourCount: 1,
				}
				throttler.mu.Unlock()
			},
		},
		{
			name:        "combined minute and hour limits",
			throttle:    config.Throttle{MaxPerMinute: 2, MaxPerHour: 3},
			iterations:  4,
			wantAllowed: []bool{true, true, false, false},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			throttler := NewThrottler()
			ruleName := "test-rule"
			resourceKey := "pod/default/my-pod"

			if tc.setup != nil {
				tc.setup(throttler, throttler.key(ruleName, resourceKey))
			}

			for i := 0; i < tc.iterations; i++ {
				allowed := throttler.Allow(ruleName, resourceKey, tc.throttle)
				if i < len(tc.wantAllowed) {
					assert.Equal(t, tc.wantAllowed[i], allowed, "iteration %d", i)
				}
			}
		})
	}
}

func TestThrottler_key(t *testing.T) {
	throttler := NewThrottler()
	testCases := []struct {
		name        string
		ruleName    string
		resourceKey string
		wantKey     string
	}{
		{
			name:        "with resource key",
			ruleName:    "rule1",
			resourceKey: "pod/default/p1",
			wantKey:     "rule1:pod/default/p1",
		},
		{
			name:        "without resource key",
			ruleName:    "rule2",
			resourceKey: "",
			wantKey:     "rule2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := throttler.key(tc.ruleName, tc.resourceKey)
			assert.Equal(t, tc.wantKey, key)
		})
	}
}
