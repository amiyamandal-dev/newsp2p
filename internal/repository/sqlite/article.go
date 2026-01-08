package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/amiyamandal-dev/newsp2p/internal/domain"
)

// articleColumns defines the standard SELECT columns for articles
const articleColumns = `id, cid, title, body, author, author_pubkey, signature, timestamp, tags, category, version, created_at, updated_at`

// scanner interface for scanning rows
type scanner interface {
	Scan(dest ...any) error
}

// scanArticle scans a single row into an Article struct
func scanArticle(row scanner) (*domain.Article, error) {
	var article domain.Article
	var tagsJSON string

	err := row.Scan(
		&article.ID,
		&article.CID,
		&article.Title,
		&article.Body,
		&article.Author,
		&article.AuthorPubKey,
		&article.Signature,
		&article.Timestamp,
		&tagsJSON,
		&article.Category,
		&article.Version,
		&article.CreatedAt,
		&article.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &article.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	return &article, nil
}

// scanArticles scans multiple rows into an Article slice
func scanArticles(rows *sql.Rows) ([]*domain.Article, error) {
	var articles []*domain.Article
	for rows.Next() {
		article, err := scanArticle(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan article: %w", err)
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating articles: %w", err)
	}
	return articles, nil
}

// ArticleRepo implements the ArticleRepository interface using SQLite
type ArticleRepo struct {
	db *DB
}

// NewArticleRepo creates a new article repository
func NewArticleRepo(db *DB) *ArticleRepo {
	return &ArticleRepo{db: db}
}

// Create creates a new article
func (r *ArticleRepo) Create(ctx context.Context, article *domain.Article) error {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
		INSERT INTO articles (id, cid, title, body, author, author_pubkey, signature, timestamp, tags, category, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err = r.db.ExecContext(ctx, query,
		article.ID,
		article.CID,
		article.Title,
		article.Body,
		article.Author,
		article.AuthorPubKey,
		article.Signature,
		article.Timestamp,
		string(tagsJSON),
		article.Category,
		article.Version,
		article.CreatedAt,
		article.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create article: %w", err)
	}

	return nil
}

// GetByID retrieves an article by ID
func (r *ArticleRepo) GetByID(ctx context.Context, id string) (*domain.Article, error) {
	query := fmt.Sprintf(`SELECT %s FROM articles WHERE id = ?`, articleColumns)

	article, err := scanArticle(r.db.QueryRowContext(ctx, query, id))
	if err == sql.ErrNoRows {
		return nil, domain.ErrArticleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get article: %w", err)
	}

	return article, nil
}

// GetByCID retrieves an article by CID
func (r *ArticleRepo) GetByCID(ctx context.Context, cid string) (*domain.Article, error) {
	query := fmt.Sprintf(`SELECT %s FROM articles WHERE cid = ?`, articleColumns)

	article, err := scanArticle(r.db.QueryRowContext(ctx, query, cid))
	if err == sql.ErrNoRows {
		return nil, domain.ErrArticleNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get article: %w", err)
	}

	return article, nil
}

// Update updates an existing article
func (r *ArticleRepo) Update(ctx context.Context, article *domain.Article) error {
	tagsJSON, err := json.Marshal(article.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `
		UPDATE articles
		SET title = ?, body = ?, tags = ?, category = ?, version = version + 1, updated_at = ?
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query,
		article.Title,
		article.Body,
		string(tagsJSON),
		article.Category,
		article.UpdatedAt,
		article.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update article: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrArticleNotFound
	}

	return nil
}

// Delete deletes an article by ID
func (r *ArticleRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM articles WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete article: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrArticleNotFound
	}

	return nil
}

// List retrieves articles with pagination and filtering
func (r *ArticleRepo) List(ctx context.Context, filter *domain.ArticleListFilter) ([]*domain.Article, int, error) {
	// Build WHERE clause
	var conditions []string
	var args []interface{}

	if filter.Author != "" {
		conditions = append(conditions, "author = ?")
		args = append(args, filter.Author)
	}

	if filter.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, filter.Category)
	}

	if !filter.FromDate.IsZero() {
		conditions = append(conditions, "timestamp >= ?")
		args = append(args, filter.FromDate)
	}

	if !filter.ToDate.IsZero() {
		conditions = append(conditions, "timestamp <= ?")
		args = append(args, filter.ToDate)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM articles %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count articles: %w", err)
	}

	// Get articles with pagination
	offset := (filter.Page - 1) * filter.Limit
	query := fmt.Sprintf(`SELECT %s FROM articles %s ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
		articleColumns, whereClause)

	args = append(args, filter.Limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query articles: %w", err)
	}
	defer rows.Close()

	articles, err := scanArticles(rows)
	if err != nil {
		return nil, 0, err
	}

	return articles, total, nil
}

// ListRecent retrieves recent articles for a feed
func (r *ArticleRepo) ListRecent(ctx context.Context, limit int) ([]*domain.Article, error) {
	query := fmt.Sprintf(`SELECT %s FROM articles ORDER BY created_at DESC LIMIT ?`, articleColumns)

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent articles: %w", err)
	}
	defer rows.Close()

	return scanArticles(rows)
}

// ListByAuthor retrieves articles by author with pagination
func (r *ArticleRepo) ListByAuthor(ctx context.Context, author string, page, limit int) ([]*domain.Article, int, error) {
	// Count total
	countQuery := "SELECT COUNT(*) FROM articles WHERE author = ?"
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, author).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count articles: %w", err)
	}

	// Get articles
	offset := (page - 1) * limit
	query := fmt.Sprintf(`SELECT %s FROM articles WHERE author = ? ORDER BY timestamp DESC LIMIT ? OFFSET ?`,
		articleColumns)

	rows, err := r.db.QueryContext(ctx, query, author, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query articles: %w", err)
	}
	defer rows.Close()

	articles, err := scanArticles(rows)
	if err != nil {
		return nil, 0, err
	}

	return articles, total, nil
}

// GetByIDs retrieves articles by a list of IDs (for search results)
func (r *ArticleRepo) GetByIDs(ctx context.Context, ids []string) ([]*domain.Article, error) {
	if len(ids) == 0 {
		return []*domain.Article{}, nil
	}

	// Build placeholders
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`SELECT %s FROM articles WHERE id IN (%s)`,
		articleColumns, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query articles by IDs: %w", err)
	}
	defer rows.Close()

	return scanArticles(rows)
}
