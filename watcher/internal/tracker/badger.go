package tracker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) (*BadgerStore, error) {
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
	return &BadgerStore{db: db}, nil
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

func (b *BadgerStore) Close() error {
	return b.db.Close()
}
