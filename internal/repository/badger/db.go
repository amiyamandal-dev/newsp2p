package badger

import (
	"context"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// DB wraps BadgerDB for local caching and indexing
type DB struct {
	*badger.DB
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new BadgerDB instance and starts GC
func New(dbPath string) (*DB, error) {
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable badger's logger

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	wrapper := &DB{
		DB:     db,
		ctx:    ctx,
		cancel: cancel,
	}

	// Start GC in background
	go wrapper.runGC()

	return wrapper, nil
}

// Close closes the database
func (db *DB) Close() error {
	db.cancel() // Stop GC
	return db.DB.Close()
}

// HealthCheck checks if the database is healthy
func (db *DB) HealthCheck() error {
	return db.View(func(txn *badger.Txn) error {
		return nil
	})
}

// runGC runs value log garbage collection periodically
func (db *DB) runGC() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-db.ctx.Done():
			return
		case <-ticker.C:
			// Run GC until it returns an error (no more garbage to collect)
			// Limit to 10 iterations to prevent CPU spinning
			for i := 0; i < 10; i++ {
				err := db.DB.RunValueLogGC(0.7)
				if err != nil {
					break
				}
				// Small pause between GC runs to prevent CPU spinning
				time.Sleep(100 * time.Millisecond)
			}
		}
	}
}
