package tracker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestMemoryStore_GetSet(t *testing.T) {
	store := NewMemoryStore()
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

func TestMemoryStore_History(t *testing.T) {
	store := NewMemoryStore()
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

	history := store.History("nonexistent-key", 0)
	assert.Empty(t, history)
}

func TestMemoryStore_HistoryLimit(t *testing.T) {
	store := NewMemoryStore()
	store.historyLimit = 2
	key := "limit-test"

	store.RecordHistory(key, Snapshot{Values: map[string]interface{}{"val": 1}})
	store.RecordHistory(key, Snapshot{Values: map[string]interface{}{"val": 2}})
	store.RecordHistory(key, Snapshot{Values: map[string]interface{}{"val": 3}})

	history := store.History(key, 0)
	assert.Len(t, history, 2)
	assert.Equal(t, 2, history[0].Values["val"])
	assert.Equal(t, 3, history[1].Values["val"])
}

func TestMemoryStore_Close(t *testing.T) {
	store := NewMemoryStore()
	err := store.Close()
	assert.NoError(t, err)
}
