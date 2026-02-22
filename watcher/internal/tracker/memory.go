package tracker

import (
	"sync"
)

type Snapshot struct {
	ResourceVersion string
	Values          map[string]interface{}
}

type Store interface {
	Get(key string) (Snapshot, bool)
	Set(key string, snap Snapshot)
	Close() error
}

type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]Snapshot
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]Snapshot),
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

func (m *MemoryStore) Close() error {
	return nil
}
