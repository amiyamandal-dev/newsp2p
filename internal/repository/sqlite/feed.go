package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

// FeedRepo implements the FeedRepository interface using SQLite
type FeedRepo struct {
	db *DB
}

// NewFeedRepo creates a new feed repository
func NewFeedRepo(db *DB) *FeedRepo {
	return &FeedRepo{db: db}
}

// Create creates a new feed
func (r *FeedRepo) Create(ctx context.Context, feed *domain.Feed) error {
	query := `
		INSERT INTO feeds (id, name, ipns_key, ipns_address, last_cid, last_sync, sync_interval, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := r.db.ExecContext(ctx, query,
		feed.ID,
		feed.Name,
		feed.IPNSKey,
		feed.IPNSAddress,
		feed.LastCID,
		feed.LastSync,
		feed.SyncInterval,
		feed.CreatedAt,
		feed.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create feed: %w", err)
	}

	return nil
}

// GetByID retrieves a feed by ID
func (r *FeedRepo) GetByID(ctx context.Context, id string) (*domain.Feed, error) {
	query := `
		SELECT id, name, ipns_key, ipns_address, last_cid, last_sync, sync_interval, created_at, updated_at
		FROM feeds
		WHERE id = ?
	`

	var feed domain.Feed
	var lastSync sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&feed.ID,
		&feed.Name,
		&feed.IPNSKey,
		&feed.IPNSAddress,
		&feed.LastCID,
		&lastSync,
		&feed.SyncInterval,
		&feed.CreatedAt,
		&feed.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrFeedNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}

	if lastSync.Valid {
		feed.LastSync = lastSync.Time
	}

	return &feed, nil
}

// GetByName retrieves a feed by name
func (r *FeedRepo) GetByName(ctx context.Context, name string) (*domain.Feed, error) {
	query := `
		SELECT id, name, ipns_key, ipns_address, last_cid, last_sync, sync_interval, created_at, updated_at
		FROM feeds
		WHERE name = ?
	`

	var feed domain.Feed
	var lastSync sql.NullTime

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&feed.ID,
		&feed.Name,
		&feed.IPNSKey,
		&feed.IPNSAddress,
		&feed.LastCID,
		&lastSync,
		&feed.SyncInterval,
		&feed.CreatedAt,
		&feed.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, domain.ErrFeedNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feed: %w", err)
	}

	if lastSync.Valid {
		feed.LastSync = lastSync.Time
	}

	return &feed, nil
}

// Update updates an existing feed
func (r *FeedRepo) Update(ctx context.Context, feed *domain.Feed) error {
	query := `
		UPDATE feeds
		SET ipns_address = ?, last_cid = ?, last_sync = ?, sync_interval = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		feed.IPNSAddress,
		feed.LastCID,
		feed.LastSync,
		feed.SyncInterval,
		feed.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update feed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrFeedNotFound
	}

	return nil
}

// Delete deletes a feed by ID
func (r *FeedRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM feeds WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete feed: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrFeedNotFound
	}

	return nil
}

// List retrieves all feeds
func (r *FeedRepo) List(ctx context.Context) ([]*domain.Feed, error) {
	query := `
		SELECT id, name, ipns_key, ipns_address, last_cid, last_sync, sync_interval, created_at, updated_at
		FROM feeds
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list feeds: %w", err)
	}
	defer rows.Close()

	var feeds []*domain.Feed
	for rows.Next() {
		var feed domain.Feed
		var lastSync sql.NullTime

		err := rows.Scan(
			&feed.ID,
			&feed.Name,
			&feed.IPNSKey,
			&feed.IPNSAddress,
			&feed.LastCID,
			&lastSync,
			&feed.SyncInterval,
			&feed.CreatedAt,
			&feed.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan feed: %w", err)
		}

		if lastSync.Valid {
			feed.LastSync = lastSync.Time
		}

		feeds = append(feeds, &feed)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating feeds: %w", err)
	}

	return feeds, nil
}

// ListDueForSync retrieves feeds that are due for syncing
func (r *FeedRepo) ListDueForSync(ctx context.Context) ([]*domain.Feed, error) {
	query := `
		SELECT id, name, ipns_key, ipns_address, last_cid, last_sync, sync_interval, created_at, updated_at
		FROM feeds
		WHERE last_sync IS NULL
		   OR datetime(last_sync, '+' || sync_interval || ' minutes') <= datetime('now')
		ORDER BY last_sync ASC NULLS FIRST
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list feeds due for sync: %w", err)
	}
	defer rows.Close()

	var feeds []*domain.Feed
	for rows.Next() {
		var feed domain.Feed
		var lastSync sql.NullTime

		err := rows.Scan(
			&feed.ID,
			&feed.Name,
			&feed.IPNSKey,
			&feed.IPNSAddress,
			&feed.LastCID,
			&lastSync,
			&feed.SyncInterval,
			&feed.CreatedAt,
			&feed.UpdatedAt,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to scan feed: %w", err)
		}

		if lastSync.Valid {
			feed.LastSync = lastSync.Time
		}

		feeds = append(feeds, &feed)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating feeds: %w", err)
	}

	return feeds, nil
}
