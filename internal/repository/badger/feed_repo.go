package badger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

// FeedRepo implements FeedRepository using BadgerDB
type FeedRepo struct {
	db *DB
}

// NewFeedRepo creates a new BadgerDB-based feed repository
func NewFeedRepo(db *DB) *FeedRepo {
	return &FeedRepo{db: db}
}

// Create creates a new feed
func (r *FeedRepo) Create(ctx context.Context, feed *domain.Feed) error {
	return r.db.Update(func(txn *badger.Txn) error {
		// Check name uniqueness
		nameKey := []byte(fmt.Sprintf("feed:name:%s", strings.ToLower(feed.Name)))
		if _, err := txn.Get(nameKey); err == nil {
			return domain.ErrFeedAlreadyExists
		}

		// Save feed
		data, err := json.Marshal(feed)
		if err != nil {
			return err
		}

		idKey := []byte(fmt.Sprintf("feed:id:%s", feed.ID))
		if err := txn.Set(idKey, data); err != nil {
			return err
		}

		// Save index
		if err := txn.Set(nameKey, []byte(feed.ID)); err != nil {
			return err
		}

		return nil
	})
}

// GetByID retrieves a feed by ID
func (r *FeedRepo) GetByID(ctx context.Context, id string) (*domain.Feed, error) {
	var feed domain.Feed
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("feed:id:%s", id)))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return domain.ErrFeedNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &feed)
		})
	})
	if err != nil {
		return nil, err
	}
	return &feed, nil
}

// GetByName retrieves a feed by name
func (r *FeedRepo) GetByName(ctx context.Context, name string) (*domain.Feed, error) {
	var id []byte
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("feed:name:%s", strings.ToLower(name))))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return domain.ErrFeedNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			id = append([]byte{}, val...)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return r.GetByID(ctx, string(id))
}

// Update updates an existing feed
func (r *FeedRepo) Update(ctx context.Context, feed *domain.Feed) error {
	return r.db.Update(func(txn *badger.Txn) error {
		data, err := json.Marshal(feed)
		if err != nil {
			return err
		}
		return txn.Set([]byte(fmt.Sprintf("feed:id:%s", feed.ID)), data)
	})
}

// Delete deletes a feed by ID
func (r *FeedRepo) Delete(ctx context.Context, id string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("feed:id:%s", id)))
		if err != nil {
			return err
		}
		var feed domain.Feed
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &feed)
		}); err != nil {
			return err
		}

		txn.Delete([]byte(fmt.Sprintf("feed:name:%s", strings.ToLower(feed.Name))))
		return txn.Delete([]byte(fmt.Sprintf("feed:id:%s", id)))
	})
}

// List retrieves all feeds
func (r *FeedRepo) List(ctx context.Context) ([]*domain.Feed, error) {
	var feeds []*domain.Feed
	err := r.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		
		prefix := []byte("feed:id:")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			var feed domain.Feed
			err := it.Item().Value(func(val []byte) error {
				return json.Unmarshal(val, &feed)
			})
			if err == nil {
				feeds = append(feeds, &feed)
			}
		}
		return nil
	})
	return feeds, err
}

// ListDueForSync retrieves feeds that are due for syncing
func (r *FeedRepo) ListDueForSync(ctx context.Context) ([]*domain.Feed, error) {
	allFeeds, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var due []*domain.Feed
	now := time.Now()
	for _, feed := range allFeeds {
		nextSync := feed.LastSync.Add(time.Duration(feed.SyncInterval) * time.Minute)
		if now.After(nextSync) {
			due = append(due, feed)
		}
	}
	return due, nil
}
