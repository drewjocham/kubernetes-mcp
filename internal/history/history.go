package history

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type IssueKind string

const (
	IncidentTypeNode  IssueKind = "node_anomaly"
	IncidentTypePod   IssueKind = "pod_anomaly"
	IncidentTypeEvent IssueKind = "event_spike"
)

type Incident struct {
	ID          string         `json:"id"`
	Timestamp   time.Time      `json:"timestamp"`
	Kind        IssueKind      `json:"kind"`
	Severity    string         `json:"severity"`
	Namespace   string         `json:"namespace,omitempty"`
	Name        string         `json:"name,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Message     string         `json:"message,omitempty"`
	Occurrences int            `json:"occurrences"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type FrequencyComparison struct {
	Kind             IssueKind `json:"kind"`
	RecentCount      int       `json:"recent_count"`
	PreviousCount    int       `json:"previous_count"`
	PercentChange    float64   `json:"percent_change"`
	WindowHours      float64   `json:"window_hours"`
	PreviousWindowHr float64   `json:"previous_window_hours"`
}

type Store struct {
	db *badger.DB
}

func NewStore(path string) (*Store, error) {
	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return nil, err
	}

	opts := badger.DefaultOptions(resolved).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Record(ctx context.Context, inc Incident) error {
	return s.db.Update(func(txn *badger.Txn) error {
		ts := uint64(inc.Timestamp.UnixNano())
		key := make([]byte, len(inc.Kind)+1+8+len(inc.ID))
		offset := copy(key, inc.Kind)
		key[offset] = ':'
		offset++
		binary.BigEndian.PutUint64(key[offset:], ts)
		offset += 8
		copy(key[offset:], inc.ID)

		val, err := json.Marshal(inc)
		if err != nil {
			return err
		}
		return txn.Set(key, val)
	})
}

func (s *Store) List(ctx context.Context, kind IssueKind, since time.Duration) ([]Incident, error) {
	var incidents []Incident
	cutoff := time.Now().Add(-since).UnixNano()

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(string(kind) + ":")
		seekKey := make([]byte, len(prefix)+8)
		copy(seekKey, prefix)
		binary.BigEndian.PutUint64(seekKey[len(prefix):], uint64(cutoff))

		for it.Seek(seekKey); it.ValidForPrefix(prefix); it.Next() {
			err := it.Item().Value(func(v []byte) error {
				var inc Incident
				if err := json.Unmarshal(v, &inc); err != nil {
					return err
				}
				incidents = append(incidents, inc)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return incidents, err
}

func (s *Store) CompareFrequency(ctx context.Context, kind IssueKind, recent, previous time.Duration) (FrequencyComparison, error) {
	data, err := s.List(ctx, kind, recent+previous)
	if err != nil {
		return FrequencyComparison{}, err
	}

	rStart := time.Now().Add(-recent)
	var rCount, pCount int

	for _, inc := range data {
		if inc.Timestamp.After(rStart) {
			rCount++
		} else {
			pCount++
		}
	}

	return FrequencyComparison{
		Kind:             kind,
		RecentCount:      rCount,
		PreviousCount:    pCount,
		PercentChange:    calculateChange(pCount, rCount),
		WindowHours:      recent.Hours(),
		PreviousWindowHr: previous.Hours(),
	}, nil
}

func resolvePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[1:]), nil
}

func calculateChange(prev, current int) float64 {
	if prev <= 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}
	return (float64(current-prev) / float64(prev)) * 100.0
}
