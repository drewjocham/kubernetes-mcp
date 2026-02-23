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

type Recordable interface {
	GetID() string
	GetTimestamp() time.Time
	GetKind() IssueKind
	GetSeverity() string
	GetNamespace() string
	GetName() string
	GetReason() string
	GetMessage() string
	GetMetadata() map[string]any
}

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
func (i Incident) GetMetadata() map[string]any { return i.Metadata }

type Recorder interface {
	Record(ctx context.Context, entry Recordable) error
	List(ctx context.Context, kind IssueKind, since time.Duration) ([]Incident, error)
	CompareFrequency(ctx context.Context, kind IssueKind, recent, previous time.Duration) (FrequencyComparison, error)
	Close() error
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
	db, err := badger.Open(badger.DefaultOptions(resolved).WithLogger(nil))
	if err != nil {
		return nil, fmt.Errorf("failed to open badger: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Record(ctx context.Context, entry Recordable) error {
	if entry == nil {
		return fmt.Errorf("history: recordable entry is nil")
	}
	inc := incidentFromRecordable(entry)
	return s.db.Update(func(txn *badger.Txn) error {
		val, err := json.Marshal(inc)
		if err != nil {
			return err
		}
		return txn.Set(s.buildKey(inc.Kind, inc.Timestamp, inc.ID), val)
	})
}

func (s *Store) List(ctx context.Context, kind IssueKind, since time.Duration) ([]Incident, error) {
	var incidents []Incident
	prefix := []byte(string(kind) + ":")
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		seek := s.buildKey(kind, time.Now().Add(-since), "")
		for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
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
	now := time.Now()
	rCount, err := s.count(kind, now.Add(-recent), now)
	if err != nil {
		return FrequencyComparison{}, err
	}
	pCount, err := s.count(kind, now.Add(-(recent + previous)), now.Add(-recent))
	if err != nil {
		return FrequencyComparison{}, err
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

func (s *Store) count(kind IssueKind, start, end time.Time) (int, error) {
	var count int
	prefix := []byte(string(kind) + ":")
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		seek := s.buildKey(kind, start, "")
		endTs := uint64(end.UnixNano())
		for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().Key()
			if binary.BigEndian.Uint64(key[len(prefix):len(prefix)+8]) > endTs {
				break
			}
			count++
		}
		return nil
	})
	return count, err
}

func (s *Store) buildKey(kind IssueKind, ts time.Time, id string) []byte {
	k := []byte(string(kind) + ":")
	buf := make([]byte, len(k)+8+len(id))
	copy(buf, k)
	binary.BigEndian.PutUint64(buf[len(k):], uint64(ts.UnixNano()))
	copy(buf[len(k)+8:], id)
	return buf
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

func incidentFromRecordable(entry Recordable) Incident {
	switch v := entry.(type) {
	case Incident:
		return v
	case *Incident:
		if v != nil {
			return *v
		}
	}
	return Incident{
		ID: entry.GetID(), Timestamp: entry.GetTimestamp(), Kind: entry.GetKind(),
		Severity: entry.GetSeverity(), Namespace: entry.GetNamespace(), Name: entry.GetName(),
		Reason: entry.GetReason(), Message: entry.GetMessage(), Metadata: entry.GetMetadata(),
	}
}

var _ Recorder = (*Store)(nil)
