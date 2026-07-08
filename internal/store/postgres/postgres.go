// Package postgres implements store.Store against PostgreSQL 16.
package postgres

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// DB wraps *sql.DB and implements store.Store.
// Each domain sub-store is embedded to keep files manageable.
type DB struct {
	db *sql.DB
}

// Open connects to dsn and verifies the connection.
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres: open: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("postgres: ping: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return &DB{db: db}, nil
}

// Close releases all connections.
func (d *DB) Close() error {
	return d.db.Close()
}

// DB exposes the underlying *sql.DB for use by the migration runner.
func (d *DB) DB() *sql.DB {
	return d.db
}
