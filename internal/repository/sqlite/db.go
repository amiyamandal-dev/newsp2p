package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// DB wraps the SQL database connection
type DB struct {
	*sql.DB
}

// New creates a new database connection and runs migrations
func New(dbPath string, maxOpenConns, maxIdleConns int) (*DB, error) {
	// Ensure the directory exists
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(30 * time.Minute)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{DB: sqlDB}

	// Run migrations
	if err := db.runMigrations(); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// runMigrations applies database migrations
func (db *DB) runMigrations() error {
	// List of migration files in order
	migrations := []string{
		"migrations/001_initial_schema.sql",
		"migrations/002_add_composite_indexes.sql",
	}

	for _, migrationFile := range migrations {
		migrationSQL, err := os.ReadFile(migrationFile)
		if err != nil {
			// Skip missing migration files (allows for optional migrations)
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("failed to read migration file %s: %w", migrationFile, err)
		}

		if _, err := db.Exec(string(migrationSQL)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", migrationFile, err)
		}
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// HealthCheck checks if the database is healthy
func (db *DB) HealthCheck() error {
	return db.Ping()
}
