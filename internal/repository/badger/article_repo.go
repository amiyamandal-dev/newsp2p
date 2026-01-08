package badger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

// ArticleRepo implements ArticleRepository using BadgerDB
type ArticleRepo struct {
	db *DB
}

// NewArticleRepo creates a new BadgerDB-based article repository
func NewArticleRepo(db *DB) *ArticleRepo {
	return &ArticleRepo{db: db}
}

// Create creates a new article
func (r *ArticleRepo) Create(ctx context.Context, article *domain.Article) error {
	return r.db.Update(func(txn *badger.Txn) error {
		// Save article data
		data, err := json.Marshal(article)
		if err != nil {
			return err
		}

		idKey := []byte(fmt.Sprintf("article:id:%s", article.ID))
		if err := txn.Set(idKey, data); err != nil {
			return err
		}

		// Indexes
		cidKey := []byte(fmt.Sprintf("article:cid:%s", article.CID))
		if err := txn.Set(cidKey, []byte(article.ID)); err != nil {
			return err
		}

		// Time index for sorting (descending scan needs careful key design, or use reverse iterator)
		// Format: article:time:<timestamp_unix_nano>:<id>
		timeKey := []byte(fmt.Sprintf("article:time:%d:%s", article.Timestamp.UnixNano(), article.ID))
		if err := txn.Set(timeKey, []byte(article.ID)); err != nil {
			return err
		}

		// Author index
		authorKey := []byte(fmt.Sprintf("article:author:%s:%d:%s", strings.ToLower(article.Author), article.Timestamp.UnixNano(), article.ID))
		if err := txn.Set(authorKey, []byte(article.ID)); err != nil {
			return err
		}

		return nil
	})
}

// GetByID retrieves an article by ID
func (r *ArticleRepo) GetByID(ctx context.Context, id string) (*domain.Article, error) {
	var article domain.Article
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("article:id:%s", id)))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return domain.ErrArticleNotFound
			}
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &article)
		})
	})
	if err != nil {
		return nil, err
	}
	return &article, nil
}

// GetByCID retrieves an article by CID
func (r *ArticleRepo) GetByCID(ctx context.Context, cid string) (*domain.Article, error) {
	var id []byte
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("article:cid:%s", cid)))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return domain.ErrArticleNotFound
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

// Update updates an existing article
func (r *ArticleRepo) Update(ctx context.Context, article *domain.Article) error {
	return r.db.Update(func(txn *badger.Txn) error {
		// In a real implementation, we should cleanup old indexes if sort keys (time/author) change.
		// Assuming immutable metadata for now except content body.
		data, err := json.Marshal(article)
		if err != nil {
			return err
		}
		return txn.Set([]byte(fmt.Sprintf("article:id:%s", article.ID)), data)
	})
}

// Delete deletes an article by ID
func (r *ArticleRepo) Delete(ctx context.Context, id string) error {
	return r.db.Update(func(txn *badger.Txn) error {
		// Load article to find index keys
		item, err := txn.Get([]byte(fmt.Sprintf("article:id:%s", id)))
		if err != nil {
			return err
		}
		var article domain.Article
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &article)
		}); err != nil {
			return err
		}

		// Delete indexes
		txn.Delete([]byte(fmt.Sprintf("article:cid:%s", article.CID)))
		txn.Delete([]byte(fmt.Sprintf("article:time:%d:%s", article.Timestamp.UnixNano(), article.ID)))
		txn.Delete([]byte(fmt.Sprintf("article:author:%s:%d:%s", strings.ToLower(article.Author), article.Timestamp.UnixNano(), article.ID)))

		// Delete data
		return txn.Delete([]byte(fmt.Sprintf("article:id:%s", id)))
	})
}

// List retrieves articles with pagination and filtering
// Note: This is an expensive implementation for BadgerDB (in-memory scan/filter)
// For production, use Bleve search for complex queries or proper index iteration.
func (r *ArticleRepo) List(ctx context.Context, filter *domain.ArticleListFilter) ([]*domain.Article, int, error) {
	var articles []*domain.Article
	
	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100
		opts.Reverse = true // Newest first
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("article:time:")
		
		for it.Seek(append(prefix, 0xFF)); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var id string
			err := item.Value(func(val []byte) error {
				id = string(val)
				return nil
			})
			if err != nil {
				continue
			}

			// Get full article
			artItem, err := txn.Get([]byte(fmt.Sprintf("article:id:%s", id)))
			if err != nil {
				continue
			}
			
			var art domain.Article
			if err := artItem.Value(func(val []byte) error {
				return json.Unmarshal(val, &art)
			}); err != nil {
				continue
			}

			// Apply filters
			if filter.Author != "" && !strings.EqualFold(art.Author, filter.Author) {
				continue
			}
			if filter.Category != "" && !strings.EqualFold(art.Category, filter.Category) {
				continue
			}
			if !filter.FromDate.IsZero() && art.Timestamp.Before(filter.FromDate) {
				continue
			}
			if !filter.ToDate.IsZero() && art.Timestamp.After(filter.ToDate) {
				continue
			}

			articles = append(articles, &art)
		}
		return nil
	})

	if err != nil {
		return nil, 0, err
	}

	total := len(articles)
	
	// Pagination
	start := (filter.Page - 1) * filter.Limit
	if start > total {
		start = total
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}

	return articles[start:end], total, nil
}

// ListRecent retrieves recent articles
func (r *ArticleRepo) ListRecent(ctx context.Context, limit int) ([]*domain.Article, error) {
	filter := &domain.ArticleListFilter{
		Page:  1,
		Limit: limit,
	}
	articles, _, err := r.List(ctx, filter)
	return articles, err
}

// ListByAuthor retrieves articles by author
func (r *ArticleRepo) ListByAuthor(ctx context.Context, author string, page, limit int) ([]*domain.Article, int, error) {
	filter := &domain.ArticleListFilter{
		Author: author,
		Page:   page,
		Limit:  limit,
	}
	return r.List(ctx, filter)
}

// GetByIDs retrieves articles by IDs
func (r *ArticleRepo) GetByIDs(ctx context.Context, ids []string) ([]*domain.Article, error) {
	var articles []*domain.Article
	for _, id := range ids {
		art, err := r.GetByID(ctx, id)
		if err == nil {
			articles = append(articles, art)
		}
	}
	return articles, nil
}
