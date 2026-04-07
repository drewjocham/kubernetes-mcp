package widgets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	data "kube-watcher-app/internal/data"
)

// Store manages persistence of widgets.
type Store struct {
	mu      sync.RWMutex
	widgets []data.Widget
	path    string
}

// NewStore creates a new widget store.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("failed to load widgets: %w", err)
	}
	return s, nil
}

// DefaultStore returns a store using the default config location.
func DefaultStore() (*Store, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("cannot get user config dir: %w", err)
	}
	appDir := filepath.Join(configDir, "kube-watcher-app")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create app config dir: %w", err)
	}
	storePath := filepath.Join(appDir, "widgets.json")
	return NewStore(storePath)
}

// load reads widgets from the JSON file.
func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fileData, err := os.ReadFile(s.path)
	if err != nil {
		// If file doesn't exist, start with empty slice
		if os.IsNotExist(err) {
			s.widgets = []data.Widget{}
			return nil
		}
		return err
	}

	if err := json.Unmarshal(fileData, &s.widgets); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// save writes widgets to the JSON file.
func (s *Store) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jsonData, err := json.MarshalIndent(s.widgets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal widgets: %w", err)
	}

	// Write to a temporary file first, then rename for atomicity
	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.path)
}

// All returns all widgets sorted by position.
func (s *Store) All(ctx context.Context) ([]data.Widget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Copy to avoid mutation
	result := make([]data.Widget, len(s.widgets))
	copy(result, s.widgets)

	// Sort by position
	sort.Slice(result, func(i, j int) bool {
		return result[i].Position < result[j].Position
	})
	return result, nil
}

// Get returns a widget by ID.
func (s *Store) Get(ctx context.Context, id string) (*data.Widget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, w := range s.widgets {
		if w.ID == id {
			// Return a copy
			wCopy := w
			return &wCopy, nil
		}
	}
	return nil, fmt.Errorf("widget not found: %s", id)
}

// Save creates or updates a widget.
func (s *Store) Save(ctx context.Context, widget data.Widget) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure ID is set
	if widget.ID == "" {
		widget.ID = generateID()
	}
	// Ensure labels have IDs
	for i := range widget.Labels {
		if widget.Labels[i].ID == "" {
			widget.Labels[i].ID = generateID()
		}
	}

	// Find existing index
	foundIdx := -1
	for i, w := range s.widgets {
		if w.ID == widget.ID {
			foundIdx = i
			break
		}
	}

	if foundIdx >= 0 {
		// Update
		s.widgets[foundIdx] = widget
	} else {
		// Append
		s.widgets = append(s.widgets, widget)
	}

	return s.save()
}

// Delete removes a widget by ID.
func (s *Store) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	newWidgets := make([]data.Widget, 0, len(s.widgets))
	for _, w := range s.widgets {
		if w.ID != id {
			newWidgets = append(newWidgets, w)
		}
	}
	if len(newWidgets) == len(s.widgets) {
		return fmt.Errorf("widget not found: %s", id)
	}
	s.widgets = newWidgets
	return s.save()
}

// generateID creates a simple unique ID based on timestamp.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
