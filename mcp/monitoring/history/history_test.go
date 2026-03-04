package history

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalcChange(t *testing.T) {
	tests := []struct {
		name     string
		prev     int
		curr     int
		expected float64
	}{
		{"Increase", 10, 15, 50.0},
		{"Decrease", 10, 5, -50.0},
		{"NoChange", 10, 10, 0.0},
		{"FromZero", 0, 5, 100.0},
		{"BothZero", 0, 0, 0.0},
		{"LargeIncrease", 1, 10, 900.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calcChange(tt.prev, tt.curr)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestToIncident(t *testing.T) {

	tests := []struct {
		name  string
		input Recordable
		check func(*testing.T, Incident)
	}{
		{
			name: "DirectPassThrough",
			input: Incident{
				ID:          "test-1",
				Occurrences: 5,
				Kind:        IncidentTypePod,
			},
			check: func(t *testing.T, res Incident) {
				assert.Equal(t, "test-1", res.ID)
				assert.Equal(t, 5, res.Occurrences)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := toIncident(tt.input)
			tt.check(t, res)
		})
	}
}

func TestStore_Integration(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, store.Close())
	}()

	now := time.Now()

	// Seed data
	incidents := []Incident{
		{ID: "old", Timestamp: now.Add(-10 * time.Hour), Kind: IncidentTypePod},
		{ID: "recent-1", Timestamp: now.Add(-1 * time.Hour), Kind: IncidentTypePod},
		{ID: "recent-2", Timestamp: now.Add(-30 * time.Minute), Kind: IncidentTypePod},
	}

	for _, inc := range incidents {
		err := store.Record(ctx, inc)
		require.NoError(t, err)
	}

	t.Run("ListRecentOnly", func(t *testing.T) {
		res, err := store.List(ctx, IncidentTypePod, 2*time.Hour)
		assert.NoError(t, err)
		assert.Len(t, res, 2, "Should only return incidents within the 2h window")
	})

	t.Run("CompareFrequency", func(t *testing.T) {
		// Recent window (last 5h): 2 incidents
		// Previous window (5h to 15h ago): 1 incident
		comp, err := store.CompareFrequency(ctx, IncidentTypePod, 5*time.Hour, 10*time.Hour)
		assert.NoError(t, err)
		assert.Equal(t, 2, comp.RecentCount)
		assert.Equal(t, 1, comp.PreviousCount)
		assert.Equal(t, 100.0, comp.PercentChange)
	})

	t.Run("CleanupRetention", func(t *testing.T) {
		store.performCleanup(context.TODO(), 5*time.Hour)

		res, err := store.List(ctx, IncidentTypePod, 24*time.Hour)
		assert.NoError(t, err)
		assert.Len(t, res, 2, "Only 'recent' incidents should remain")
	})
}
