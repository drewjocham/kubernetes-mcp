package chatbridge

import (
	"sync"
	"time"
)

type IdempotencyStore struct {
	mu             sync.RWMutex
	cache          map[string]time.Time
	ttl            time.Duration
	cleanupCounter int // counter for periodic cleanup
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{
		cache: make(map[string]time.Time),
		ttl:   ttl,
	}
}

func (s *IdempotencyStore) SeenOrAdd(id string, now time.Time) bool {
	if id == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cleanup expired entries periodically (every 100 operations or when cache is large)
	s.cleanupCounter++
	if s.cleanupCounter >= 100 || len(s.cache) > 1000 {
		for k, v := range s.cache {
			if now.Sub(v) > s.ttl {
				delete(s.cache, k)
			}
		}
		s.cleanupCounter = 0
	}

	if t, ok := s.cache[id]; ok && now.Sub(t) < s.ttl {
		return true
	}

	s.cache[id] = now
	return false
}
