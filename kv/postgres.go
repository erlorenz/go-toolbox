package kv

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore is a PostgreSQL implementation of Store.
// It uses FNV-1a hashing for fast lookups with a BIGINT primary key,
// storing the actual key for collision detection.
// Values can be stored as JSONB (default) or BYTEA (for encryption or binary data).
type PostgresStore struct {
	pool         *pgxpool.Pool
	tableName    string
	schema       string
	format       string // "JSONB" or "BYTEA"
	unlogged     bool
	keyIndex     bool
	encryptor    Encryptor
	cleanupDone  chan struct{}
	cleanupClose chan struct{}
}

// PostgresOption configures a PostgresStore.
type PostgresOption func(*PostgresStore)

// WithTableName sets the table name for the store.
// Overrides the automatic table naming based on encryption and unlogged settings.
// Default: auto-generated based on configuration
func WithTableName(name string) PostgresOption {
	return func(s *PostgresStore) {
		s.tableName = name
	}
}

// WithSchema sets the PostgreSQL schema for the table.
// Default: "public"
func WithSchema(schema string) PostgresOption {
	return func(s *PostgresStore) {
		s.schema = schema
	}
}

// WithFormat sets the storage format for values.
// Options: "JSONB" (default) or "BYTEA"
// JSONB validates JSON and allows PostgreSQL JSON queries.
// BYTEA is for binary data or non-JSON serialization (protobuf, msgpack, etc.)
// Default: "JSONB" unless WithEncryption is used (then "BYTEA")
func WithFormat(format string) PostgresOption {
	return func(s *PostgresStore) {
		s.format = format
	}
}

// WithEncryption enables encryption for all values using the provided Encryptor.
// Automatically sets format to BYTEA unless explicitly overridden with WithFormat.
// Default: no encryption
func WithEncryption(encryptor Encryptor) PostgresOption {
	return func(s *PostgresStore) {
		s.encryptor = encryptor
		// Default to BYTEA for encrypted data (can be overridden)
		if s.format == "" {
			s.format = "BYTEA"
		}
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

// WithKeyIndex creates an index on the key column for fast prefix searches.
// Useful if you frequently use Keys() with prefixes on large datasets.
// Adds storage overhead and slows writes slightly.
// Default: false
func WithKeyIndex(enabled bool) PostgresOption {
	return func(s *PostgresStore) {
		s.keyIndex = enabled
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
//
// Default configuration:
//   - Schema: "public"
//   - Format: "JSONB" (or "BYTEA" if encryption is enabled)
//   - Table: Auto-generated based on settings
//   - Unlogged: false
//   - KeyIndex: false
//   - Cleanup: manual
func NewPostgresStore(pool *pgxpool.Pool, opts ...PostgresOption) *PostgresStore {
	s := &PostgresStore{
		pool:         pool,
		schema:       "public",
		format:       "JSONB",
		unlogged:     false,
		keyIndex:     false,
		cleanupClose: make(chan struct{}),
		cleanupDone:  make(chan struct{}),
	}

	// By default, no cleanup loop (cleanupDone is already closed conceptually)
	close(s.cleanupDone)

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Set default table name if not explicitly set
	if s.tableName == "" {
		s.tableName = s.defaultTableName()
	}

	return s
}

// defaultTableName generates a table name based on configuration.
// Examples:
//   - kv_store (JSONB, logged)
//   - kv_store_unlogged (JSONB, unlogged)
//   - kv_store_encrypted (BYTEA with encryption, logged)
//   - kv_store_encrypted_unlogged (BYTEA with encryption, unlogged)
func (s *PostgresStore) defaultTableName() string {
	base := "kv_store"

	if s.encryptor != nil {
		base = "kv_store_encrypted"
	}

	if s.unlogged {
		base += "_unlogged"
	}

	return base
}

// CreateTable creates the key-value table with TTL support.
// Uses key_hash (BIGINT) as primary key for fast lookups regardless of key length.
// Creates the table in the configured schema with the appropriate value column type (JSONB or BYTEA).
func (s *PostgresStore) CreateTable(ctx context.Context) error {
	unloggedClause := ""
	if s.unlogged {
		unloggedClause = "UNLOGGED"
	}

	// Determine value column type
	valueType := s.format
	if valueType == "" {
		valueType = "JSONB"
	}

	// Full table identifier (schema.table)
	fullTableName := pgx.Identifier{s.schema, s.tableName}.Sanitize()

	query := fmt.Sprintf(`
		CREATE %s TABLE IF NOT EXISTS %s (
			key_hash BIGINT PRIMARY KEY,
			key TEXT NOT NULL,
			value %s NOT NULL,
			expires_at TIMESTAMPTZ,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, unloggedClause, fullTableName, valueType)

	_, err := s.pool.Exec(ctx, query)
	if err != nil {
		return err
	}

	// Create index on expires_at for cleanup queries
	expiresIdxName := s.tableName + "_expires_idx"
	expiresIdxQuery := fmt.Sprintf(`
		CREATE INDEX IF NOT EXISTS %s ON %s (expires_at)
		WHERE expires_at IS NOT NULL
	`, pgx.Identifier{expiresIdxName}.Sanitize(), fullTableName)

	_, err = s.pool.Exec(ctx, expiresIdxQuery)
	if err != nil {
		return err
	}

	// Create index on key column if requested (for fast prefix searches)
	if s.keyIndex {
		keyIdxName := s.tableName + "_key_idx"
		keyIdxQuery := fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s (key text_pattern_ops)
		`, pgx.Identifier{keyIdxName}.Sanitize(), fullTableName)

		_, err = s.pool.Exec(ctx, keyIdxQuery)
		if err != nil {
			return err
		}
	}

	return nil
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
// Decrypts the value if encryption is enabled.
func (s *PostgresStore) Get(ctx context.Context, key string) ([]byte, error) {
	keyHash := hashKey(key)
	fullTableName := pgx.Identifier{s.schema, s.tableName}.Sanitize()

	query := fmt.Sprintf(`
		SELECT value FROM %s
		WHERE key_hash = $1
		AND key = $2
		AND (expires_at IS NULL OR expires_at > NOW())
	`, fullTableName)

	var data []byte
	err := s.pool.QueryRow(ctx, query, keyHash, key).Scan(&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Decrypt if encryptor is configured
	if s.encryptor != nil {
		return s.encryptor.Decrypt(ctx, data)
	}

	return data, nil
}

// Set stores a value with the given key.
// If ttl is 0, the value never expires.
// Updates updated_at timestamp on every write.
// Encrypts the value if encryption is enabled.
// Validates JSON if format is JSONB.
func (s *PostgresStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	keyHash := hashKey(key)
	fullTableName := pgx.Identifier{s.schema, s.tableName}.Sanitize()

	// Encrypt if encryptor is configured
	dataToStore := value
	if s.encryptor != nil {
		encrypted, err := s.encryptor.Encrypt(ctx, value)
		if err != nil {
			return fmt.Errorf("encryption failed: %w", err)
		}
		dataToStore = encrypted
	}

	var expiresAt any
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (key_hash, key, value, expires_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (key_hash)
		DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at, updated_at = NOW()
	`, fullTableName)

	_, err := s.pool.Exec(ctx, query, keyHash, key, dataToStore, expiresAt)
	if err != nil {
		return err
	}

	return nil
}

// SetMany stores multiple key-value pairs with the same TTL in a single round trip.
// This is more efficient than calling Set multiple times.
// If ttl is 0, the values never expire.
// Encrypts all values if encryption is enabled.
func (s *PostgresStore) SetMany(ctx context.Context, items map[string][]byte, ttl time.Duration) error {
	if len(items) == 0 {
		return nil
	}

	fullTableName := pgx.Identifier{s.schema, s.tableName}.Sanitize()

	var expiresAt any
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}

	// Build multi-row INSERT: INSERT INTO table (key_hash, key, value, expires_at, updated_at)
	// VALUES ($1, $2, $3, $4, NOW()), ($5, $6, $7, $8, NOW()), ...
	// ON CONFLICT (key_hash) DO UPDATE SET ...

	args := make([]any, 0, len(items)*4)
	valueStrings := make([]string, 0, len(items))
	paramIdx := 1

	for key, value := range items {
		// Encrypt if encryptor is configured
		dataToStore := value
		if s.encryptor != nil {
			encrypted, err := s.encryptor.Encrypt(ctx, value)
			if err != nil {
				return fmt.Errorf("encryption failed for key %s: %w", key, err)
			}
			dataToStore = encrypted
		}

		valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, NOW())",
			paramIdx, paramIdx+1, paramIdx+2, paramIdx+3))
		args = append(args, hashKey(key), key, dataToStore, expiresAt)
		paramIdx += 4
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (key_hash, key, value, expires_at, updated_at)
		VALUES %s
		ON CONFLICT (key_hash)
		DO UPDATE SET value = EXCLUDED.value, expires_at = EXCLUDED.expires_at, updated_at = NOW()
	`, fullTableName, strings.Join(valueStrings, ", "))

	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

// Update atomically reads, modifies, and writes a value using a transaction.
// The function receives the current value (or nil if key doesn't exist/expired).
// If the function returns an error, the transaction is rolled back.
// Uses SELECT FOR UPDATE to lock the row and prevent concurrent modifications.
// Handles encryption/decryption if enabled.
func (s *PostgresStore) Update(ctx context.Context, key string, ttl time.Duration, fn func(current []byte) ([]byte, error)) error {
	keyHash := hashKey(key)
	fullTableName := pgx.Identifier{s.schema, s.tableName}.Sanitize()

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
	`, fullTableName)

	var storedValue []byte
	err = tx.QueryRow(ctx, selectQuery, keyHash, key).Scan(&storedValue)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	// If ErrNoRows, storedValue remains nil (key doesn't exist)

	// Decrypt current value if encryptor is configured
	var current []byte
	if storedValue != nil && s.encryptor != nil {
		current, err = s.encryptor.Decrypt(ctx, storedValue)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
	} else {
		current = storedValue
	}

	// Call user function
	newValue, err := fn(current)
	if err != nil {
		return err
	}

	// Encrypt new value if encryptor is configured
	dataToStore := newValue
	if s.encryptor != nil {
		encrypted, err := s.encryptor.Encrypt(ctx, newValue)
		if err != nil {
			return fmt.Errorf("encryption failed: %w", err)
		}
		dataToStore = encrypted
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
	`, fullTableName)

	_, err = tx.Exec(ctx, upsertQuery, keyHash, key, dataToStore, expiresAt)
	if err != nil {
		return err
	}

	// Commit transaction
	return tx.Commit(ctx)
}

// Delete removes a value by key. Returns nil if the key doesn't exist.
func (s *PostgresStore) Delete(ctx context.Context, key string) error {
	keyHash := hashKey(key)
	fullTableName := pgx.Identifier{s.schema, s.tableName}.Sanitize()

	query := fmt.Sprintf(`
		DELETE FROM %s WHERE key_hash = $1 AND key = $2
	`, fullTableName)

	_, err := s.pool.Exec(ctx, query, keyHash, key)
	return err
}

// Keys returns all keys matching the given prefix.
// If prefix is empty, returns all keys (excluding expired entries).
func (s *PostgresStore) Keys(ctx context.Context, prefix string) ([]string, error) {
	fullTableName := pgx.Identifier{s.schema, s.tableName}.Sanitize()
	var query string
	var args []any

	if prefix == "" {
		query = fmt.Sprintf(`
			SELECT key FROM %s
			WHERE expires_at IS NULL OR expires_at > NOW()
			ORDER BY key
		`, fullTableName)
	} else {
		query = fmt.Sprintf(`
			SELECT key FROM %s
			WHERE key LIKE $1 || '%%'
			AND (expires_at IS NULL OR expires_at > NOW())
			ORDER BY key
		`, fullTableName)
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
	fullTableName := pgx.Identifier{s.schema, s.tableName}.Sanitize()
	query := fmt.Sprintf(`
		DELETE FROM %s
		WHERE expires_at IS NOT NULL AND expires_at <= NOW()
	`, fullTableName)

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
