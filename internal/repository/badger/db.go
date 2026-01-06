package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

// DB wraps BadgerDB for local caching and indexing
type DB struct {
	*badger.DB
}

// New creates a new BadgerDB instance
func New(dbPath string) (*DB, error) {
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable badger's logger

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	return &DB{DB: db}, nil
}

// Close closes the database
func (db *DB) Close() error {
	return db.DB.Close()
}

// HealthCheck checks if the database is healthy
func (db *DB) HealthCheck() error {
	return db.View(func(txn *badger.Txn) error {
		return nil
	})
}
