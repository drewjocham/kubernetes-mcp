package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/dgraph-io/badger/v4"
	"kube-watcher/watcher/internal/tracker"
)

//go:embed ui/index.html
var indexHTML []byte

type ResourceRecord struct {
	Key       string            `json:"key"`
	Kind      string            `json:"kind"`
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Snapshot  tracker.Snapshot  `json:"snapshot"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type ResourceFilter struct {
	Search    string
	Kind      string
	Namespace string
	Limit     int
}

type Store struct {
	db           *badger.DB
	historyLimit int
	logger       *slog.Logger
	snapshotPath string
}

func NewStore(path string, historyLimit int, logger *slog.Logger) (*Store, error) {
	db, err := openReadOnlyWithRecovery(path, logger)
	if err != nil {
		if strings.Contains(err.Error(), "Cannot acquire directory lock") {
			snapshotPath := filepath.Join(os.TempDir(), fmt.Sprintf("watcher-explorer-snapshot-%d", time.Now().UnixNano()))
			logger.Warn("badger lock held by another process, using snapshot copy", "source", path, "snapshot", snapshotPath)
			if copyErr := copyDir(path, snapshotPath); copyErr != nil {
				return nil, fmt.Errorf("badger open: lock held and snapshot copy failed: %w", copyErr)
			}
			db, err = openReadOnlyWithRecovery(snapshotPath, logger)
			if err != nil {
				_ = os.RemoveAll(snapshotPath)
				return nil, fmt.Errorf("badger open snapshot: %w", err)
			}
			return &Store{db: db, historyLimit: historyLimit, logger: logger, snapshotPath: snapshotPath}, nil
		}
		return nil, fmt.Errorf("badger open: %w", err)
	}
	return &Store{db: db, historyLimit: historyLimit, logger: logger}, nil
}

func openReadOnlyWithRecovery(path string, logger *slog.Logger) (*badger.DB, error) {
	opts := badger.DefaultOptions(path).WithReadOnly(true).WithLogger(nil)
	db, err := badger.Open(opts)
	if err != nil && strings.Contains(err.Error(), "Log truncate required") {
		logger.Warn("badger requires log truncate, attempting one-time recovery", "path", path)
		recoverOpts := badger.DefaultOptions(path).WithLogger(nil)
		recovered, recoverErr := badger.Open(recoverOpts)
		if recoverErr != nil {
			return nil, fmt.Errorf("badger recovery open: %w", recoverErr)
		}
		if closeErr := recovered.Close(); closeErr != nil {
			return nil, fmt.Errorf("badger recovery close: %w", closeErr)
		}
		db, err = badger.Open(opts)
	}
	return db, err
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func (s *Store) Close() error {
	var firstErr error
	if err := s.db.Close(); err != nil {
		firstErr = err
	}
	if s.snapshotPath != "" {
		if err := os.RemoveAll(s.snapshotPath); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Store) List(ctx context.Context, f ResourceFilter) ([]ResourceRecord, error) {
	var records []ResourceRecord
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			item := it.Item()
			key := string(item.Key())

			if strings.Contains(key, ":") || !s.matches(key, f) {
				continue
			}

			var snap tracker.Snapshot
			if err := item.Value(func(v []byte) error { return json.Unmarshal(v, &snap) }); err != nil {
				continue
			}

			kind, ns, name := parseKey(key)
			records = append(records, ResourceRecord{
				Key: key, Kind: kind, Namespace: ns, Name: name, Snapshot: snap,
			})

			if f.Limit > 0 && len(records) >= f.Limit {
				break
			}
		}
		return nil
	})

	slices.SortFunc(records, func(a, b ResourceRecord) int {
		return b.Snapshot.Timestamp.Compare(a.Snapshot.Timestamp)
	})
	return records, err
}

func (s *Store) History(ctx context.Context, key string, limit int) ([]tracker.Snapshot, error) {
	if limit <= 0 || limit > s.historyLimit {
		limit = s.historyLimit
	}

	prefix := []byte(key + ":")
	seek := append(prefix, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)

	var history []tracker.Snapshot
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(seek); it.ValidForPrefix(prefix); it.Next() {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			var snap tracker.Snapshot
			if err := it.Item().Value(func(v []byte) error { return json.Unmarshal(v, &snap) }); err != nil {
				continue
			}
			history = append(history, snap)
			if len(history) >= limit {
				break
			}
		}
		return nil
	})
	return history, err
}

func (s *Store) matches(key string, f ResourceFilter) bool {
	kind, ns, _ := parseKey(key)
	match := func(target, filter string) bool {
		return filter == "" || strings.EqualFold(target, filter)
	}
	if !match(kind, f.Kind) || !match(ns, f.Namespace) {
		return false
	}
	return f.Search == "" || strings.Contains(strings.ToLower(key), strings.ToLower(f.Search))
}

type Handler struct {
	store  *Store
	logger *slog.Logger
	tmpl   *template.Template
}

func NewHandler(s *Store, l *slog.Logger) *Handler {
	t := template.Must(template.New("index").Parse(string(indexHTML)))
	return &Handler{store: s, logger: l, tmpl: t}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.handleIndex)
	mux.HandleFunc("GET /healthz", h.handleHealth)
	mux.HandleFunc("GET /api/resources", h.handleResources)
	mux.HandleFunc("GET /api/history", h.handleHistory)
	return h.withMiddleware(mux)
}

func (h *Handler) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				h.logger.Error("panic recovered", "error", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	f := ResourceFilter{
		Search:    r.URL.Query().Get("search"),
		Kind:      r.URL.Query().Get("kind"),
		Namespace: r.URL.Query().Get("namespace"),
		Limit:     50,
	}
	records, _ := h.store.List(r.Context(), f)
	_ = h.tmpl.Execute(w, struct {
		Title          string
		Filter         ResourceFilter
		InitialRecords []ResourceRecord
	}{
		Title:          "Watcher Badger Explorer",
		Filter:         f,
		InitialRecords: records,
	})
}

func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleResources(w http.ResponseWriter, r *http.Request) {
	f := ResourceFilter{
		Search:    r.URL.Query().Get("search"),
		Kind:      r.URL.Query().Get("kind"),
		Namespace: r.URL.Query().Get("namespace"),
		Limit:     parseInt(r.URL.Query().Get("limit"), 50),
	}
	data, err := h.store.List(r.Context(), f)
	if err != nil {
		h.sendError(w, err, http.StatusInternalServerError)
		return
	}
	h.sendJSON(w, data)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		h.sendError(w, fmt.Errorf("key query parameter required"), http.StatusBadRequest)
		return
	}
	limit := parseInt(r.URL.Query().Get("limit"), h.store.historyLimit)
	data, err := h.store.History(r.Context(), key, limit)
	if err != nil {
		h.sendError(w, err, http.StatusInternalServerError)
		return
	}
	h.sendJSON(w, data)
}

func (h *Handler) sendJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) sendError(w http.ResponseWriter, err error, code int) {
	h.logger.Error("api error", "error", err, "code", code)
	http.Error(w, err.Error(), code)
}

func parseKey(key string) (string, string, string) {
	p := strings.Split(key, "/")
	if len(p) < 3 {
		return key, "", ""
	}
	return p[0], p[1], p[2]
}

func parseInt(s string, fallback int) int {
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil || v <= 0 {
		return fallback
	}
	return v
}

func main() {
	var (
		dbPath  string
		addr    string
		histLim int
	)

	flag.Usage = func() {
		out := flag.CommandLine.Output()
		_, _ = fmt.Fprintln(out, "kube-watcher explorer")
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "Browse watcher Badger snapshots/history with a local web UI.")
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "Usage:")
		_, _ = fmt.Fprintf(out, "  %s [flags]\n", os.Args[0])
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "Flags:")
		flag.PrintDefaults()
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "Quick start:")
		_, _ = fmt.Fprintln(out, "  1) Run watcher engine to populate the store:")
		_, _ = fmt.Fprintln(out, "     go run ./watcher/cmd/engine --config watcher/internal/config/config.yaml")
		_, _ = fmt.Fprintln(out, "  2) Run explorer:")
		_, _ = fmt.Fprintln(out, "     go run ./watcher/cmd/explorer --db event-engine-badger --listen :4101")
		_, _ = fmt.Fprintln(out, "  3) Open:")
		_, _ = fmt.Fprintln(out, "     http://localhost:4101")
		_, _ = fmt.Fprintln(out, "")
		_, _ = fmt.Fprintln(out, "Explorer hotkeys:")
		_, _ = fmt.Fprintln(out, "  /        focus search")
		_, _ = fmt.Fprintln(out, "  g        focus kind filter")
		_, _ = fmt.Fprintln(out, "  n        focus namespace filter")
		_, _ = fmt.Fprintln(out, "  r        refresh resources")
		_, _ = fmt.Fprintln(out, "  j or ↓   next row")
		_, _ = fmt.Fprintln(out, "  k or ↑   previous row")
		_, _ = fmt.Fprintln(out, "  Enter    load history for selected row")
		_, _ = fmt.Fprintln(out, "  ?        toggle hotkey help")
	}
	flag.StringVar(&dbPath, "db", "event-engine-badger", "badger directory")
	flag.StringVar(&addr, "listen", ":4101", "listen address")
	flag.IntVar(&histLim, "history-limit", 120, "max history items")
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	store, err := NewStore(dbPath, histLim, logger)
	if err != nil {
		logger.Error("failed to open store", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      NewHandler(store, logger).Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("badger explorer starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = srv.Shutdown(ctx)
	_ = store.Close()
	logger.Info("shutdown complete")
}
