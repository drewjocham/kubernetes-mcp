package tracker

import (
	"sync"
	"time"
)

type Snapshot struct {
	ResourceVersion string                 `json:"resourceVersion"`
	Values          map[string]interface{} `json:"values"`
	Timestamp       time.Time              `json:"timestamp"`
}

type Store interface {
	Get(key string) (Snapshot, bool)
	Set(key string, snap Snapshot)
	RecordHistory(key string, snap Snapshot)
	History(key string, limit int) []Snapshot
	Close() error
}

type MemoryStore struct {
	mu           sync.RWMutex
	data         map[string]Snapshot
	history      map[string][]Snapshot
	historyLimit int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data:         make(map[string]Snapshot),
		history:      make(map[string][]Snapshot),
		historyLimit: 128,
	}
}

func (m *MemoryStore) Get(key string) (Snapshot, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.data[key]
	return val, ok
}

func (m *MemoryStore) Set(key string, snap Snapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = snap
}

func (m *MemoryStore) RecordHistory(key string, snap Snapshot) {
	if snap.Timestamp.IsZero() {
		snap.Timestamp = time.Now()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	h := append(m.history[key], snap)
	if len(h) > m.historyLimit {
		h = h[len(h)-m.historyLimit:]
	}
	m.history[key] = h
}

func (m *MemoryStore) History(key string, limit int) []Snapshot {
	if limit <= 0 {
		limit = 20
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	h := m.history[key]
	if len(h) == 0 {
		return nil
	}
	if len(h) > limit {
		h = h[len(h)-limit:]
	}
	out := make([]Snapshot, len(h))
	copy(out, h)
	return out
}

func (m *MemoryStore) Close() error {
	return nil
}
