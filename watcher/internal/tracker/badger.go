package tracker

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db         *badger.DB
	historyTTL time.Duration
}

func NewBadgerStore(path string, historyTTL time.Duration) (*BadgerStore, error) {
	if path == "" {
		return nil, fmt.Errorf("tracker: path required")
	}

	resolved := path
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		resolved = filepath.Join(home, path[1:])
	}

	if err := os.MkdirAll(resolved, 0o755); err != nil {
		return nil, err
	}

	db, err := badger.Open(badger.DefaultOptions(resolved).WithLogger(nil))
	if err != nil {
		return nil, err
	}
	if historyTTL <= 0 {
		historyTTL = time.Hour
	}
	return &BadgerStore{db: db, historyTTL: historyTTL}, nil
}

func (b *BadgerStore) Get(key string) (Snapshot, bool) {
	var snap Snapshot
	err := b.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &snap)
		})
	})
	return snap, err == nil
}

func (b *BadgerStore) Set(key string, snap Snapshot) {
	_ = b.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		return txn.Set([]byte(key), data)
	})
}

func (b *BadgerStore) SetWithTTL(key string, snap Snapshot, ttl time.Duration) {
	_ = b.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		e := badger.NewEntry([]byte(key), data).WithTTL(ttl)
		return txn.SetEntry(e)
	})
}

func (b *BadgerStore) Close() error {
	return b.db.Close()
}

func (b *BadgerStore) RecordHistory(key string, snap Snapshot) {
	if snap.Timestamp.IsZero() {
		snap.Timestamp = time.Now()
	}
	_ = b.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		entryKey := make([]byte, len(key)+1+8)
		copy(entryKey, key)
		entryKey[len(key)] = ':'
		binary.BigEndian.PutUint64(entryKey[len(key)+1:], uint64(snap.Timestamp.UnixNano()))
		e := badger.NewEntry(entryKey, data).WithTTL(b.historyTTL)
		return txn.SetEntry(e)
	})
}

func (b *BadgerStore) History(key string, limit int) []Snapshot {
	if limit <= 0 {
		limit = 20
	}
	var history []Snapshot
	_ = b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := append([]byte(key), ':')
		seekKey := append(append([]byte{}, prefix...), []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}...)

		for it.Seek(seekKey); it.Valid(); it.Next() {
			item := it.Item()
			if !strings.HasPrefix(string(item.Key()), key+":") {
				break
			}
			var snap Snapshot
			if err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &snap)
			}); err != nil {
				continue
			}
			history = append(history, snap)
			if len(history) >= limit {
				break
			}
		}
		return nil
	})

	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}
	return history
}
