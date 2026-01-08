-- Migration 002: Add composite indexes for common query patterns
-- These indexes optimize queries that filter by author/category and sort by timestamp

-- Composite index for author + timestamp queries (e.g., "list articles by author X, sorted by date")
CREATE INDEX IF NOT EXISTS idx_articles_author_timestamp ON articles(author, timestamp DESC);

-- Composite index for category + timestamp queries (e.g., "list articles in category Y, sorted by date")
CREATE INDEX IF NOT EXISTS idx_articles_category_timestamp ON articles(category, timestamp DESC);
