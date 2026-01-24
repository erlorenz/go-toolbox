package kv

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a PostgreSQL implementation of Store.
// It uses FNV-1a hashing for fast lookups with a BIGINT primary key,
// storing the actual key for collision detection.
// Values are stored as raw bytes in a BYTEA column.
type PostgresStore struct {
	pool         *pgxpool.Pool
	tableName    string
	unlogged     bool
	cleanupDone  chan struct{}
	cleanupClose chan struct{}
}

// PostgresOption configures a PostgresStore.
type PostgresOption func(*PostgresStore)

// WithTableName sets the table name for the store.
// Default: "kv_store"
func WithTableName(name string) PostgresOption {
	return func(s *PostgresStore) {
		s.tableName = name
	}
}

// WithUnlogged creates an UNLOGGED table for better performance.
// UNLOGGED tables are 2-3x faster but data is lost on crash.
// Perfect for caches and temporary state. Default: false
func WithUnlogged(unlogged bool) PostgresOption {
	return func(s *PostgresStore) {
		s.unlogged = unlogged
	}
}

// WithCleanup enables automatic cleanup of expired entries at the specified interval.
// If not set, users must call Cleanup() manually (e.g., via cron).
// Default: no automatic cleanup
func WithCleanup(interval time.Duration) PostgresOption {
	return func(s *PostgresStore) {
		if interval > 0 {
			go s.cleanupLoop(interval)
		}
	}
}

// NewPostgresStore creates a new PostgreSQL-backed store.
// The table must be created using CreateTable() before use.
func NewPostgresStore(pool *pgxpool.Pool, opts ...PostgresOption) *PostgresStore {
	s := &PostgresStore{
		pool:         pool,
		tableName:    "kv_store",
		unlogged:     false,
		cleanupClose: make(chan struct{}),
		cleanupDone:  make(chan struct{}),
	}

	// By default, no cleanup loop (cleanupDone is already closed conceptually)
	close(s.cleanupDone)

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// CreateTable creates the key-value table with TTL support.
// Uses key_hash (BIGINT) as primary key for fast lookups regardless of key length.
func (s *PostgresStore) CreateTable(ctx context.Context) error {
	unloggedClause := ""
	if s.unlogged {
		unloggedClause = "UNLOGGED"
	}

	query := fmt.Sprintf(`
		CREATE %s TABLE IF NOT EXISTS %s (
			key_hash BIGINT PRIMARY KEY,
			key TEXT NOT NULL,
			value BYTEA NOT NULL,
			expires_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, unloggedClause, pgx.Identifier{s.tableName}.Sanitize())

	_, err := s.pool.Exec(ctx, query)
	if err != nil {
		return err
	}

	// Create index on expires_at for cleanup queries
	indexQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s ON %s (expires_at)
		WHERE expires_at IS NOT NULL
	`, pgx.Identifier{s.tableName + "_expires_idx"}.Sanitize(), pgx.Identifier{s.tableName}.Sanitize())

	_, err = s.pool.Exec(ctx, indexQuery)
	return err
}

// hashKey creates a deterministic 64-bit hash from a key string using FNV-1a.
// FNV-1a is fast and has good distribution for cache keys.
func hashKey(key string) int64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return int64(h.Sum64())
}

// Get retrieves a value by key. Returns ErrNotFound if the key doesn't exist or has expired.
// Uses key_hash for fast lookup, then verifies actual key to handle collisions.
func (s *PostgresStore) Get(ctx context.Context, key string) ([]byte, error) {
	keyHash := hashKey(key)

	query := fmt.Sprintf(`
		SELECT value FROM %s
		WHERE key_hash = $1
		AND key = $2
		AND (expires_at IS NULL OR expires_at > NOW())
	`, pgx.Identifier{s.tableName}.Sanitize())

	var data []byte
	err := s.pool.QueryRow(ctx, query, keyHash, key).Scan(&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	return data, nil
}

// Set stores a value with the given key.
// If ttl is 0, the value never expires.
// Updates updated_at timestamp on every write.
func (s *PostgresStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	keyHash := hashKey(key)

	var expiresAt any
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (key_hash, key, value, expires_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (key_hash)
		DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at, updated_at = NOW()
	`, pgx.Identifier{s.tableName}.Sanitize())

	_, err := s.pool.Exec(ctx, query, keyHash, key, value, expiresAt)
	return err
}

// Update atomically reads, modifies, and writes a value using a transaction.
// The function receives the current value (or nil if key doesn't exist/expired).
// If the function returns an error, the transaction is rolled back.
// Uses SELECT FOR UPDATE to lock the row and prevent concurrent modifications.
func (s *PostgresStore) Update(ctx context.Context, key string, ttl time.Duration, fn func(current []byte) ([]byte, error)) error {
	keyHash := hashKey(key)

	// Start transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Lock the row and get current value (if exists and not expired)
	selectQuery := fmt.Sprintf(`
		SELECT value FROM %s
		WHERE key_hash = $1
		AND key = $2
		AND (expires_at IS NULL OR expires_at > NOW())
		FOR UPDATE
	`, pgx.Identifier{s.tableName}.Sanitize())

	var current []byte
	err = tx.QueryRow(ctx, selectQuery, keyHash, key).Scan(&current)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// If ErrNoRows, current remains nil (key doesn't exist)

	// Call user function
	newValue, err := fn(current)
	if err != nil {
		return err
	}

	// Store the new value
	var expiresAt any
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	upsertQuery := fmt.Sprintf(`
		INSERT INTO %s (key_hash, key, value, expires_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (key_hash)
		DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at, updated_at = NOW()
	`, pgx.Identifier{s.tableName}.Sanitize())

	_, err = tx.Exec(ctx, upsertQuery, keyHash, key, newValue, expiresAt)
	if err != nil {
		return err
	}

	// Commit transaction
	return tx.Commit(ctx)
}

// Delete removes a value by key. Returns nil if the key doesn't exist.
func (s *PostgresStore) Delete(ctx context.Context, key string) error {
	keyHash := hashKey(key)

	query := fmt.Sprintf(`
		DELETE FROM %s WHERE key_hash = $1
	`, pgx.Identifier{s.tableName}.Sanitize())

	_, err := s.pool.Exec(ctx, query, keyHash)
	return err
}

// Keys returns all keys matching the given prefix.
// If prefix is empty, returns all keys (excluding expired entries).
func (s *PostgresStore) Keys(ctx context.Context, prefix string) ([]string, error) {
	var query string
	var args []any

	if prefix == "" {
		query = fmt.Sprintf(`
			SELECT key FROM %s
			WHERE expires_at IS NULL OR expires_at > NOW()
			ORDER BY key
		`, pgx.Identifier{s.tableName}.Sanitize())
	} else {
		query = fmt.Sprintf(`
			SELECT key FROM %s
			WHERE key LIKE $1 || '%%'
			AND (expires_at IS NULL OR expires_at > NOW())
			ORDER BY key
		`, pgx.Identifier{s.tableName}.Sanitize())
		args = append(args, prefix)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// Cleanup removes expired entries from the store.
// Returns the number of entries deleted.
// Call this manually via cron/scheduler, or use WithCleanup() for automatic cleanup.
func (s *PostgresStore) Cleanup(ctx context.Context) (int64, error) {
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE expires_at IS NOT NULL AND expires_at <= NOW()
	`, pgx.Identifier{s.tableName}.Sanitize())

	result, err := s.pool.Exec(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected(), nil
}

// cleanupLoop runs cleanup at the specified interval.
func (s *PostgresStore) cleanupLoop(interval time.Duration) {
	// Reset cleanupDone since we're actually running cleanup
	s.cleanupDone = make(chan struct{})

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer close(s.cleanupDone)

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			s.Cleanup(ctx)
			cancel()
		case <-s.cleanupClose:
			return
		}
	}
}

// Close closes the store and stops any background cleanup goroutine.
// Note: it does NOT close the pool as it may be shared with other components.
func (s *PostgresStore) Close() error {
	close(s.cleanupClose)
	<-s.cleanupDone // Wait for cleanup goroutine to finish
	return nil
}
