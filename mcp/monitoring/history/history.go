package history

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
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

func (i Incident) GetID() string               { return i.ID }
func (i Incident) GetTimestamp() time.Time     { return i.Timestamp }
func (i Incident) GetKind() IssueKind          { return i.Kind }
func (i Incident) GetSeverity() string         { return i.Severity }
func (i Incident) GetNamespace() string        { return i.Namespace }
func (i Incident) GetName() string             { return i.Name }
func (i Incident) GetReason() string           { return i.Reason }
func (i Incident) GetMessage() string          { return i.Message }
func (i Incident) GetOccurrences() int         { return i.Occurrences }
func (i Incident) GetMetadata() map[string]any { return i.Metadata }

type Recordable interface {
	GetID() string
	GetTimestamp() time.Time
	GetKind() IssueKind
	GetSeverity() string
	GetNamespace() string
	GetName() string
	GetReason() string
	GetMessage() string
	GetOccurrences() int
	GetMetadata() map[string]any
}

type FrequencyComparison struct {
	Kind             IssueKind `json:"kind"`
	RecentCount      int       `json:"recent_count"`
	PreviousCount    int       `json:"previous_count"`
	PercentChange    float64   `json:"percent_change"`
	WindowHours      float64   `json:"window_hours"`
	PreviousWindowHr float64   `json:"previous_window_hours"`
}

type Recorder interface {
	Record(ctx context.Context, entry Recordable) error
	List(ctx context.Context, kind IssueKind, since time.Duration) ([]Incident, error)
	CompareFrequency(ctx context.Context, kind IssueKind, recent, previous time.Duration) (FrequencyComparison, error)
	Close() error
}

type Store struct {
	db *badger.DB
}

func NewStore(path string) (*Store, error) {
	dir, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	opts := badger.DefaultOptions(dir).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("badger open: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Record(ctx context.Context, entry Recordable) error {
	if entry == nil {
		return errors.New("history: cannot record nil entry")
	}

	inc := toIncident(entry)
	key := s.buildKey(inc.Kind, inc.Timestamp, inc.ID)

	return s.db.Update(func(txn *badger.Txn) error {
		val, err := json.Marshal(inc)
		if err != nil {
			return err
		}
		return txn.Set(key, val)
	})
}

func (s *Store) List(ctx context.Context, kind IssueKind, since time.Duration) ([]Incident, error) {
	var incidents []Incident
	prefix := []byte(string(kind) + ":")
	start := time.Now().Add(-since)

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		seek := s.buildKey(kind, start, "")
		for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
			if err := it.Item().Value(func(v []byte) error {
				var inc Incident
				if err := json.Unmarshal(v, &inc); err != nil {
					return err
				}
				incidents = append(incidents, inc)
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})

	return incidents, err
}

func (s *Store) CompareFrequency(ctx context.Context, kind IssueKind, recent, previous time.Duration) (FrequencyComparison, error) {
	now := time.Now()

	rCount, err := s.countRange(kind, now.Add(-recent), now)
	if err != nil {
		return FrequencyComparison{}, err
	}

	pCount, err := s.countRange(kind, now.Add(-(recent + previous)), now.Add(-recent))
	if err != nil {
		return FrequencyComparison{}, err
	}

	return FrequencyComparison{
		Kind:             kind,
		RecentCount:      rCount,
		PreviousCount:    pCount,
		PercentChange:    calcChange(pCount, rCount),
		WindowHours:      recent.Hours(),
		PreviousWindowHr: previous.Hours(),
	}, nil
}

func (s *Store) countRange(kind IssueKind, start, end time.Time) (int, error) {
	var count int
	prefix := []byte(string(kind) + ":")
	endNs := uint64(end.UnixNano())

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		seek := s.buildKey(kind, start, "")
		for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			ts := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])
			if ts > endNs {
				break
			}
			count++
		}
		return nil
	})
	return count, err
}

func (s *Store) StartGC(ctx context.Context, retention, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.performCleanup(retention)
		}
	}
}

func (s *Store) performCleanup(retention time.Duration) {
	cutoff := uint64(time.Now().Add(-retention).UnixNano())
	kinds := []IssueKind{IncidentTypeNode, IncidentTypePod, IncidentTypeEvent}

	for _, kind := range kinds {
		prefix := []byte(string(kind) + ":")
		_ = s.db.Update(func(txn *badger.Txn) error {
			it := txn.NewIterator(badger.DefaultIteratorOptions)
			defer it.Close()

			for it.Rewind(); it.ValidForPrefix(prefix); it.Next() {
				key := it.Item().Key()
				ts := binary.BigEndian.Uint64(key[len(prefix) : len(prefix)+8])
				if ts > cutoff {
					break
				}
				_ = txn.Delete(key)
			}
			return nil
		})
	}
	_ = s.db.RunValueLogGC(0.5)
}

func (s *Store) buildKey(kind IssueKind, ts time.Time, id string) []byte {
	pre := []byte(string(kind) + ":")
	buf := make([]byte, len(pre)+8+len(id))
	copy(buf, pre)
	binary.BigEndian.PutUint64(buf[len(pre):], uint64(ts.UnixNano()))
	copy(buf[len(pre)+8:], id)
	return buf
}

func (s *Store) Close() error {
	return s.db.Close()
}

func resolvePath(path string) (string, error) {
	if !strings.HasPrefix(path, "~") {
		return filepath.Abs(path)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, path[1:]), nil
}

func calcChange(prev, curr int) float64 {
	if prev <= 0 {
		if curr > 0 {
			return 100.0
		}
		return 0.0
	}
	return (float64(curr-prev) / float64(prev)) * 100.0
}

func toIncident(e Recordable) Incident {
	if inc, ok := e.(Incident); ok {
		return inc
	}
	return Incident{
		ID:          e.GetID(),
		Timestamp:   e.GetTimestamp(),
		Kind:        e.GetKind(),
		Severity:    e.GetSeverity(),
		Namespace:   e.GetNamespace(),
		Name:        e.GetName(),
		Reason:      e.GetReason(),
		Message:     e.GetMessage(),
		Occurrences: e.GetOccurrences(),
		Metadata:    e.GetMetadata(),
	}
}
