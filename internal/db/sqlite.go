package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const driverName = "sqlite3"

const dsnTemplate = "file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on&_synchronous=NORMAL&_txlock=immediate"

// DB wraps a sql.DB with agentcom-specific operations.
type DB struct {
	*sql.DB
}

// Open creates a new SQLite connection with WAL mode, busy timeout, and foreign keys enabled.
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf(dsnTemplate, path)

	sqlDB, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("db.Open: ping: %w", err)
	}

	return &DB{sqlDB}, nil
}

// OpenMemory opens an in-memory SQLite database for testing.
func OpenMemory() (*DB, error) {
	sqlDB, err := sql.Open(driverName, "file::memory:?cache=shared&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("db.OpenMemory: %w", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return &DB{sqlDB}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.DB.Close()
}
