package tracker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*BadgerStore, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "badger-test")
	require.NoError(t, err)

	store, err := NewBadgerStore(dir, time.Hour)
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, store.Close())
		require.NoError(t, os.RemoveAll(dir))
	}
	return store, cleanup
}

func TestNewBadgerStore(t *testing.T) {
	t.Run("valid path", func(t *testing.T) {
		dir := t.TempDir()
		store, err := NewBadgerStore(dir, time.Hour)
		require.NoError(t, err)
		assert.NotNil(t, store)
		require.NoError(t, store.Close())
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := NewBadgerStore("", time.Hour)
		assert.Error(t, err)
	})

	t.Run("home dir expansion", func(t *testing.T) {
		home, err := os.UserHomeDir()
		require.NoError(t, err)
		path := filepath.Join("~", "badger-test-home")
		defer func() {
			require.NoError(t, os.RemoveAll(filepath.Join(home, "badger-test-home")))
		}()

		store, err := NewBadgerStore(path, time.Hour)
		require.NoError(t, err)
		assert.NotNil(t, store)
		require.NoError(t, store.Close())
	})
}

func TestBadgerStore_GetSet(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	key := "test-key"
	snap := Snapshot{ResourceVersion: "1", Values: map[string]interface{}{"cpu": "100m"}}

	store.Set(key, snap)

	gotSnap, ok := store.Get(key)
	assert.True(t, ok)
	assert.Equal(t, snap.ResourceVersion, gotSnap.ResourceVersion)
	assert.Equal(t, snap.Values["cpu"], gotSnap.Values["cpu"])

	_, ok = store.Get("nonexistent-key")
	assert.False(t, ok)
}

func TestBadgerStore_SetWithTTL(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	key := "ttl-key"
	snap := Snapshot{ResourceVersion: "2"}
	ttl := 50 * time.Millisecond

	store.SetWithTTL(key, snap, ttl)

	_, ok := store.Get(key)
	assert.True(t, ok, "should get key before TTL expires")

	time.Sleep(ttl + 10*time.Millisecond)

	_, ok = store.Get(key)
	assert.False(t, ok, "should not get key after TTL expires")
}

func TestBadgerStore_History(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	key := "history-key"
	now := time.Now()

	snaps := []Snapshot{
		{Timestamp: now.Add(-3 * time.Second), Values: map[string]interface{}{"val": 1}},
		{Timestamp: now.Add(-2 * time.Second), Values: map[string]interface{}{"val": 2}},
		{Timestamp: now.Add(-1 * time.Second), Values: map[string]interface{}{"val": 3}},
	}

	for _, s := range snaps {
		store.RecordHistory(key, s)
	}

	testCases := []struct {
		name      string
		limit     int
		wantCount int
		wantFirst Snapshot
		wantLast  Snapshot
	}{
		{
			name:      "get all history",
			limit:     0, // default limit
			wantCount: 3,
			wantFirst: snaps[0],
			wantLast:  snaps[2],
		},
		{
			name:      "limit history",
			limit:     2,
			wantCount: 2,
			wantFirst: snaps[1],
			wantLast:  snaps[2],
		},
		{
			name:      "limit greater than history",
			limit:     5,
			wantCount: 3,
			wantFirst: snaps[0],
			wantLast:  snaps[2],
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			history := store.History(key, tc.limit)
			assert.Len(t, history, tc.wantCount)
			if tc.wantCount > 0 {
				assert.Equal(t, tc.wantFirst.Values, history[0].Values)
				assert.Equal(t, tc.wantLast.Values, history[len(history)-1].Values)
			}
		})
	}

	// Test empty history
	history := store.History("nonexistent-key", 0)
	assert.Empty(t, history)
}
