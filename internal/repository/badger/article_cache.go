package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
	"github.com/dgraph-io/badger/v4"
)

// ArticleCache provides local caching and indexing for articles
// Articles are stored on IPFS, this is just a cache for fast access
type ArticleCache struct {
	db *DB
}

// NewArticleCache creates a new article cache
func NewArticleCache(db *DB) *ArticleCache {
	return &ArticleCache{db: db}
}

// CacheArticle stores an article in the local cache
func (c *ArticleCache) CacheArticle(ctx context.Context, article *domain.Article) error {
	return c.db.Update(func(txn *badger.Txn) error {
		// Store by CID
		cidKey := []byte(fmt.Sprintf("article:cid:%s", article.CID))
		data, err := json.Marshal(article)
		if err != nil {
			return err
		}

		entry := badger.NewEntry(cidKey, data).WithTTL(24 * time.Hour)
		if err := txn.SetEntry(entry); err != nil {
			return err
		}

		// Store by ID
		idKey := []byte(fmt.Sprintf("article:id:%s", article.ID))
		if err := txn.SetEntry(badger.NewEntry(idKey, []byte(article.CID))); err != nil {
			return err
		}

		// Index by author
		authorKey := []byte(fmt.Sprintf("index:author:%s:%s", article.Author, article.CID))
		if err := txn.SetEntry(badger.NewEntry(authorKey, []byte(article.Timestamp.Format(time.RFC3339)))); err != nil {
			return err
		}

		// Index by timestamp
		timeKey := []byte(fmt.Sprintf("index:time:%s:%s", article.Timestamp.Format(time.RFC3339), article.CID))
		if err := txn.SetEntry(badger.NewEntry(timeKey, []byte(article.ID))); err != nil {
			return err
		}

		return nil
	})
}

// GetByCID retrieves a cached article by CID
func (c *ArticleCache) GetByCID(ctx context.Context, cid string) (*domain.Article, error) {
	var article domain.Article

	err := c.db.View(func(txn *badger.Txn) error {
		key := []byte(fmt.Sprintf("article:cid:%s", cid))
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &article)
		})
	})

	if err == badger.ErrKeyNotFound {
		return nil, domain.ErrArticleNotFound
	}
	if err != nil {
		return nil, err
	}

	return &article, nil
}

// ListRecent retrieves recent articles from cache
func (c *ArticleCache) ListRecent(ctx context.Context, limit int) ([]*domain.Article, error) {
	var articles []*domain.Article

	err := c.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		opts.PrefetchSize = limit

		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("index:time:")
		count := 0

		for it.Seek(append(prefix, []byte("9")...)); it.ValidForPrefix(prefix) && count < limit; it.Next() {
			item := it.Item()

			// Extract CID from key
			key := string(item.Key())
			// Key format: index:time:<timestamp>:<cid>
			parts := []byte(key)[len("index:time:"):]
			cidStart := -1
			for i := len(parts) - 1; i >= 0; i-- {
				if parts[i] == ':' {
					cidStart = i + 1
					break
				}
			}
			if cidStart == -1 {
				continue
			}
			cid := string(parts[cidStart:])

			// Get article
			article, err := c.GetByCID(ctx, cid)
			if err == nil {
				articles = append(articles, article)
				count++
			}
		}

		return nil
	})

	return articles, err
}

// Invalidate removes an article from cache
func (c *ArticleCache) Invalidate(ctx context.Context, cid string) error {
	return c.db.Update(func(txn *badger.Txn) error {
		key := []byte(fmt.Sprintf("article:cid:%s", cid))
		return txn.Delete(key)
	})
}
