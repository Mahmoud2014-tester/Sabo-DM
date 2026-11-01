package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// queueStore persists the queue in SQLite. Each snapshot replacement is one
// transaction, so a process crash cannot leave a partially-written queue.
type queueStore struct {
	db *sql.DB
}

func openQueueStore(path string) (*queueStore, error) {
	if path == "" {
		return nil, fmt.Errorf("empty database path")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", abs)
	if err != nil {
		return nil, err
	}
	// One connection keeps connection-scoped PRAGMAs consistent and is
	// sufficient because queue writes are serialized on the UI thread.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	store := &queueStore{db: db}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pragmas := []string{
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = FULL",
		"PRAGMA temp_store = MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("configure SQLite (%s): %w", pragma, err)
		}
	}
	schemaStatements := []string{
		`CREATE TABLE IF NOT EXISTS queue_items (
			position INTEGER PRIMARY KEY,
			payload TEXT NOT NULL,
			updated_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS queue_metadata (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`INSERT INTO queue_metadata(key, value) VALUES('schema_version', '1')
		 ON CONFLICT(key) DO NOTHING`,
	}
	for _, statement := range schemaStatements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("initialize SQLite schema: %w", err)
		}
	}
	var check string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil || check != "ok" {
		_ = db.Close()
		if err != nil {
			return nil, fmt.Errorf("SQLite integrity check: %w", err)
		}
		return nil, fmt.Errorf("SQLite integrity check failed: %s", check)
	}
	return store, nil
}

func (s *queueStore) load() ([]savedItem, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("SQLite store is not initialized")
	}
	rows, err := s.db.Query(`SELECT payload FROM queue_items ORDER BY position ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []savedItem
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		var item savedItem
		if err := json.Unmarshal([]byte(payload), &item); err != nil {
			return nil, fmt.Errorf("decode saved queue item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *queueStore) replace(items []savedItem) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("SQLite store is not initialized")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		_ = tx.Rollback()
		return cause
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM queue_items`); err != nil {
		return rollback(err)
	}
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO queue_items(position, payload, updated_at) VALUES(?, ?, ?)`)
	if err != nil {
		return rollback(err)
	}
	defer stmt.Close()
	now := time.Now().Unix()
	for i, item := range items {
		payload, err := json.Marshal(item)
		if err != nil {
			return rollback(err)
		}
		if _, err := stmt.ExecContext(ctx, i, string(payload), now); err != nil {
			return rollback(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *queueStore) close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
