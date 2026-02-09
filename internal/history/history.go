package history

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// IssueKind categorizes stored incidents for trend analysis.
type IssueKind string

// Predefined incident categories.
const (
	IncidentTypeNode  IssueKind = "node_anomaly"
	IncidentTypePod   IssueKind = "pod_anomaly"
	IncidentTypeEvent IssueKind = "event_spike"
)

// Incident captures a single recorded issue.
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

// Query constrains history retrieval.
type Query struct {
	Since    time.Duration
	Kind     IssueKind
	Severity string
	Limit    int
}

// FrequencyComparison highlights trend deltas.
type FrequencyComparison struct {
	Kind             IssueKind `json:"kind"`
	RecentCount      int       `json:"recent_count"`
	PreviousCount    int       `json:"previous_count"`
	PercentChange    float64   `json:"percent_change"`
	WindowHours      float64   `json:"window_hours"`
	PreviousWindowHr float64   `json:"previous_window_hours"`
}

// Store provides incident persistence with simple JSONL storage.
type Store struct {
	path      string
	mu        sync.RWMutex
	incidents []Incident
}

// NewStore initializes a Store backed by the provided file path.
func NewStore(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("history path is required")
	}

	resolved, err := resolvePath(path)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	s := &Store{path: resolved}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("failed to load history: %w", err)
	}
	return s, nil
}

func (s *Store) load() error {
	file, err := os.OpenFile(s.path, os.O_RDONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Using a decoder is more efficient and cleaner for JSONL
	dec := json.NewDecoder(bufio.NewReader(file))
	for {
		var incident Incident
		if err := dec.Decode(&incident); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			continue // Skip malformed lines
		}
		s.incidents = append(s.incidents, incident)
	}
	return nil
}

// Record stores a new incident on disk and memory.
func (s *Store) Record(_ context.Context, incident Incident) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	// NewEncoder adds the necessary newline for JSONL automatically
	if err := json.NewEncoder(file).Encode(incident); err != nil {
		return err
	}

	s.incidents = append(s.incidents, incident)
	return nil
}

// List returns incidents matching the provided query.
func (s *Store) List(_ context.Context, q Query) []Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []Incident
	var cutoff time.Time
	if q.Since > 0 {
		cutoff = time.Now().Add(-q.Since)
	}

	// Traverse backward to get latest incidents first
	for i := len(s.incidents) - 1; i >= 0; i-- {
		inc := s.incidents[i]

		if !cutoff.IsZero() && inc.Timestamp.Before(cutoff) {
			break // Optimization: assumes incidents are recorded chronologically
		}
		if q.Kind != "" && inc.Kind != q.Kind {
			continue
		}
		if q.Severity != "" && inc.Severity != q.Severity {
			continue
		}

		results = append(results, inc)
		if q.Limit > 0 && len(results) >= q.Limit {
			break
		}
	}
	return results
}

// CompareFrequency computes change between two distinct time windows.
func (s *Store) CompareFrequency(_ context.Context, kind IssueKind, recent, previous time.Duration) FrequencyComparison {
	if recent <= 0 {
		recent = 24 * time.Hour
	}
	if previous <= 0 {
		previous = recent
	}

	now := time.Now()
	rStart := now.Add(-recent)
	pStart := rStart.Add(-previous)

	s.mu.RLock()
	defer s.mu.RUnlock()

	var rCount, pCount int
	for _, inc := range s.incidents {
		if kind != "" && inc.Kind != kind {
			continue
		}

		if inc.Timestamp.After(rStart) {
			rCount++
		} else if inc.Timestamp.After(pStart) {
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
	}
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
	if prev == 0 {
		if current > 0 {
			return 100.0
		}
		return 0.0
	}
	return (float64(current-prev) / float64(prev)) * 100.0
}
