package chatbridge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIdempotencyStore_SeenOrAdd(t *testing.T) {
	now := time.Now()
	store := NewIdempotencyStore(2 * time.Minute)

	assert.False(t, store.SeenOrAdd("evt-1", now))
	assert.True(t, store.SeenOrAdd("evt-1", now.Add(30*time.Second)))
	assert.False(t, store.SeenOrAdd("evt-1", now.Add(3*time.Minute)))
}
