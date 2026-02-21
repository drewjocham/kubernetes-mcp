package tracker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
)

type BadgerStore struct {
	db *badger.DB
}

func NewBadgerStore(path string) (*BadgerStore, error) {
	if path == "" {
		return nil, fmt.Errorf("tracker: path required")
	}
	if path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, path[1:])
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, err
	}
	opts := badger.DefaultOptions(path).WithLogger(nil)
	db, err := badger.Open(opts)
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
	if err != nil {
		return Snapshot{}, false
	}
	return snap, true
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
