package kv_test

import (
	"context"
	"testing"
	"time"

	"github.com/erlorenz/go-toolbox/kv"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	store := kv.NewMemoryStore()
	defer store.Close()

	t.Run("SetAndGet", func(t *testing.T) {
		key := "test:key"
		value := []byte("test value")

		err := store.Set(ctx, key, value, 0)
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		got, err := store.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}

		if string(got) != string(value) {
			t.Errorf("Get returned %q, want %q", got, value)
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		_, err := store.Get(ctx, "nonexistent")
		if err != kv.ErrNotFound {
			t.Errorf("Get returned %v, want ErrNotFound", err)
		}
	})

	t.Run("Delete", func(t *testing.T) {
		key := "test:delete"
		value := []byte("delete me")

		store.Set(ctx, key, value, 0)
		err := store.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		_, err = store.Get(ctx, key)
		if err != kv.ErrNotFound {
			t.Errorf("Get after Delete returned %v, want ErrNotFound", err)
		}
	})

	t.Run("TTLExpiration", func(t *testing.T) {
		key := "test:ttl"
		value := []byte("expires soon")

		err := store.Set(ctx, key, value, 100*time.Millisecond)
		if err != nil {
			t.Fatalf("Set with TTL failed: %v", err)
		}

		// Should exist immediately
		_, err = store.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get before expiration failed: %v", err)
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Should be expired
		_, err = store.Get(ctx, key)
		if err != kv.ErrNotFound {
			t.Errorf("Get after expiration returned %v, want ErrNotFound", err)
		}
	})

	t.Run("KeysWithPrefix", func(t *testing.T) {
		// Setup test data
		store.Set(ctx, "user:1", []byte("alice"), 0)
		store.Set(ctx, "user:2", []byte("bob"), 0)
		store.Set(ctx, "session:1", []byte("s1"), 0)

		keys, err := store.Keys(ctx, "user:")
		if err != nil {
			t.Fatalf("Keys failed: %v", err)
		}

		if len(keys) != 2 {
			t.Errorf("Keys returned %d keys, want 2", len(keys))
		}
	})

	t.Run("KeysAll", func(t *testing.T) {
		keys, err := store.Keys(ctx, "")
		if err != nil {
			t.Fatalf("Keys failed: %v", err)
		}

		// Should have at least the keys we added
		if len(keys) < 3 {
			t.Errorf("Keys returned %d keys, want at least 3", len(keys))
		}
	})
}
