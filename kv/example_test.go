package kv_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/erlorenz/go-toolbox/kv"
)

// User represents a user in your application
type User struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserCache is an application-specific adapter for the kv store
type UserCache struct {
	store kv.Store
}

func NewUserCache(store kv.Store) *UserCache {
	return &UserCache{store: store}
}

func (c *UserCache) Get(ctx context.Context, userID string) (*User, error) {
	key := fmt.Sprintf("user:%s", userID)
	data, err := c.store.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var user User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}

func (c *UserCache) Set(ctx context.Context, user *User, ttl time.Duration) error {
	key := fmt.Sprintf("user:%s", user.ID)
	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	return c.store.Set(ctx, key, data, ttl)
}

func (c *UserCache) Delete(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user:%s", userID)
	return c.store.Delete(ctx, key)
}

func Example_userCache() {
	ctx := context.Background()

	// Create an in-memory store
	store := kv.NewMemoryStore()
	defer store.Close()

	// Create application-specific cache
	cache := NewUserCache(store)

	// Store a user with 5 minute TTL
	user := &User{
		ID:    "123",
		Name:  "Alice",
		Email: "alice@example.com",
	}
	cache.Set(ctx, user, 5*time.Minute)

	// Retrieve the user
	retrieved, err := cache.Get(ctx, "123")
	if err != nil {
		panic(err)
	}

	fmt.Printf("User: %s (%s)\n", retrieved.Name, retrieved.Email)
	// Output: User: Alice (alice@example.com)
}
