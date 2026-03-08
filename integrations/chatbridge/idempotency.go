package chatbridge

import (
	"sync"
	"time"
)

type IdempotencyStore struct {
	ttl   time.Duration
	mu    sync.Mutex
	items map[string]time.Time
}

func NewIdempotencyStore(ttl time.Duration) *IdempotencyStore {
	return &IdempotencyStore{
		ttl:   ttl,
		items: make(map[string]time.Time),
	}
}

func (s *IdempotencyStore) SeenOrAdd(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cleanupLocked(now)

	exp, ok := s.items[key]
	if ok && exp.After(now) {
		return true
	}
	s.items[key] = now.Add(s.ttl)
	return false
}

func (s *IdempotencyStore) cleanupLocked(now time.Time) {
	for key, exp := range s.items {
		if !exp.After(now) {
			delete(s.items, key)
		}
	}
}
