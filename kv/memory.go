package kv

import (
	"context"
	"strings"
	"sync"
	"time"
)

// item represents a value in the memory store with optional expiration.
type item struct {
	value     []byte
	expiresAt time.Time
}

// isExpired returns true if the item has an expiration time and it has passed.
func (i *item) isExpired() bool {
	return !i.expiresAt.IsZero() && time.Now().After(i.expiresAt)
}

// MemoryStore is an in-memory implementation of Store with TTL support.
// It is safe for concurrent use and automatically cleans up expired items every minute.
type MemoryStore struct {
	mu    sync.RWMutex
	data  map[string]*item
	close chan struct{}
}

// NewMemoryStore creates a new in-memory store.
// It starts a background goroutine to clean up expired items every minute.
func NewMemoryStore() *MemoryStore {
	s := &MemoryStore{
		data:  make(map[string]*item),
		close: make(chan struct{}),
	}

	// Start cleanup goroutine
	go s.cleanup()

	return s
}

// Get retrieves a value by key. Returns ErrNotFound if the key doesn't exist or has expired.
func (s *MemoryStore) Get(ctx context.Context, key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.data[key]
	if !ok || item.isExpired() {
		return nil, ErrNotFound
	}

	return item.value, nil
}

// Set stores a value with the given key.
// If ttl is 0, the value never expires.
func (s *MemoryStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item := &item{
		value: value,
	}

	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}

	s.data[key] = item
	return nil
}

// Delete removes a value by key. Returns nil if the key doesn't exist.
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// Keys returns all keys matching the given prefix.
// If prefix is empty, returns all keys (excluding expired entries).
func (s *MemoryStore) Keys(ctx context.Context, prefix string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0)
	for key, item := range s.data {
		if item.isExpired() {
			continue
		}

		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Close stops the cleanup goroutine and releases resources.
func (s *MemoryStore) Close() error {
	close(s.close)
	return nil
}

// cleanup runs in the background and removes expired items every minute.
func (s *MemoryStore) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.removeExpired()
		case <-s.close:
			return
		}
	}
}

// removeExpired removes all expired items from the store.
func (s *MemoryStore) removeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, item := range s.data {
		if item.isExpired() {
			delete(s.data, key)
		}
	}
}
