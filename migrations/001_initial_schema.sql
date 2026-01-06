-- Initial database schema for distributed news platform
-- SQLite 3 with WAL mode enabled

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    public_key TEXT NOT NULL,
    private_key TEXT NOT NULL,  -- Encrypted
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Articles table
CREATE TABLE IF NOT EXISTS articles (
    id TEXT PRIMARY KEY,
    cid TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    author TEXT NOT NULL,
    author_pubkey TEXT NOT NULL,
    signature TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    tags TEXT,  -- JSON array stored as text
    category TEXT,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (author) REFERENCES users(username) ON DELETE CASCADE
);

-- Feeds table
CREATE TABLE IF NOT EXISTS feeds (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    ipns_key TEXT NOT NULL,
    ipns_address TEXT,
    last_cid TEXT,
    last_sync TIMESTAMP,
    sync_interval INTEGER DEFAULT 15,  -- minutes
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Feed articles junction table (many-to-many relationship)
CREATE TABLE IF NOT EXISTS feed_articles (
    feed_id TEXT NOT NULL,
    article_id TEXT NOT NULL,
    position INTEGER,  -- Order in feed
    added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (feed_id, article_id),
    FOREIGN KEY (feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
    FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE
);

-- Indexes for performance

-- User indexes
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_active ON users(is_active);

-- Article indexes
CREATE INDEX IF NOT EXISTS idx_articles_author ON articles(author);
CREATE INDEX IF NOT EXISTS idx_articles_timestamp ON articles(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_articles_category ON articles(category);
CREATE INDEX IF NOT EXISTS idx_articles_cid ON articles(cid);
CREATE INDEX IF NOT EXISTS idx_articles_created_at ON articles(created_at DESC);

-- Feed indexes
CREATE INDEX IF NOT EXISTS idx_feeds_name ON feeds(name);
CREATE INDEX IF NOT EXISTS idx_feeds_last_sync ON feeds(last_sync);

-- Feed articles indexes
CREATE INDEX IF NOT EXISTS idx_feed_articles_feed_id ON feed_articles(feed_id);
CREATE INDEX IF NOT EXISTS idx_feed_articles_article_id ON feed_articles(article_id);
CREATE INDEX IF NOT EXISTS idx_feed_articles_position ON feed_articles(feed_id, position);
CREATE INDEX IF NOT EXISTS idx_feed_articles_added_at ON feed_articles(added_at DESC);

-- Triggers for updated_at

-- Update timestamp trigger for users
CREATE TRIGGER IF NOT EXISTS update_users_updated_at
AFTER UPDATE ON users
FOR EACH ROW
BEGIN
    UPDATE users SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Update timestamp trigger for articles
CREATE TRIGGER IF NOT EXISTS update_articles_updated_at
AFTER UPDATE ON articles
FOR EACH ROW
BEGIN
    UPDATE articles SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

-- Update timestamp trigger for feeds
CREATE TRIGGER IF NOT EXISTS update_feeds_updated_at
AFTER UPDATE ON feeds
FOR EACH ROW
BEGIN
    UPDATE feeds SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;
