// Package kv provides a simple key-value store interface with
// support for multiple backends (in-memory, PostgreSQL, etc.).
//
// The store works with raw []byte values, allowing users to handle
// their own serialization (JSON, protobuf, etc.) in application-specific adapters.
package kv

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNotFound is returned when a key is not found in the store.
	ErrNotFound = errors.New("key not found")
)

// Encryptor provides encryption and decryption for values.
// Implementations should be safe for concurrent use.
type Encryptor interface {
	// Encrypt encrypts plaintext and returns ciphertext.
	Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)

	// Decrypt decrypts ciphertext and returns plaintext.
	Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}

// Store is a key-value store interface that works with raw bytes.
// Users should build their own adapters for type-safe operations and serialization.
type Store interface {
	// Get retrieves a value by key. Returns ErrNotFound if the key doesn't exist or has expired.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value with the given key.
	// If ttl is 0, the value never expires (if backend supports expiration).
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Update atomically reads, modifies, and writes a value.
	// The function receives the current value (or nil if key doesn't exist/expired).
	// If the function returns an error, the update is aborted and no changes are made.
	// If successful, the new value is stored with the given TTL (0 = no expiration).
	// This operation is atomic - no other operations can modify the key during the update.
	Update(ctx context.Context, key string, ttl time.Duration, fn func(current []byte) ([]byte, error)) error

	// Delete removes a value by key. Returns nil if the key doesn't exist.
	Delete(ctx context.Context, key string) error

	// Keys returns all keys matching the given prefix.
	// If prefix is empty, returns all keys (excluding expired entries).
	Keys(ctx context.Context, prefix string) ([]string, error)

	// Close closes the store and releases any resources.
	Close() error
}
