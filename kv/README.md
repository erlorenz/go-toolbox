# kv

A simple, fast key-value store package for Go with support for multiple backends (in-memory, PostgreSQL).

## Features

- **Simple `[]byte` interface** - Handle your own serialization (JSON, protobuf, etc.)
- **TTL support** - Automatic expiration of entries
- **Prefix-based key listing** - Find all keys matching a prefix
- **Multiple backends**:
  - **MemoryStore** - In-memory with automatic cleanup
  - **PostgresStore** - PostgreSQL-backed with optional cleanup
- **Production-ready** - Inspired by Rails Solid Cache design

## Installation

```bash
go get github.com/erlorenz/go-toolbox/kv
```

## Quick Start

### In-Memory Store

```go
import "github.com/erlorenz/go-toolbox/kv"

// Create store
store := kv.NewMemoryStore()
defer store.Close()

// Set value with 5 minute TTL
store.Set(ctx, "user:123", []byte("alice"), 5*time.Minute)

// Get value
data, err := store.Get(ctx, "user:123")
if err == kv.ErrNotFound {
    // Key doesn't exist or expired
}

// Delete
store.Delete(ctx, "user:123")

// List keys with prefix
keys, _ := store.Keys(ctx, "user:")
```

### PostgreSQL Store

```go
import (
    "github.com/erlorenz/go-toolbox/kv"
    "github.com/jackc/pgx/v5/pgxpool"
)

// Create connection pool
pool, _ := pgxpool.New(ctx, "postgres://...")
defer pool.Close()

// Create store with options
store := kv.NewPostgresStore(pool,
    kv.WithUnlogged(true),           // 2-3x faster, data lost on crash
    kv.WithCleanup(5*time.Minute),   // Auto-cleanup every 5 minutes
)
defer store.Close()

// Create table (run once)
store.CreateTable(ctx)

// Use same API as MemoryStore
store.Set(ctx, "key", []byte("value"), time.Hour)
```

## Application-Specific Adapters

Build type-safe adapters for your use case:

```go
type UserCache struct {
    store kv.Store
}

func (c *UserCache) Get(ctx context.Context, userID string) (*User, error) {
    key := fmt.Sprintf("user:%s", userID)
    data, err := c.store.Get(ctx, key)
    if err != nil {
        return nil, err
    }

    var user User
    json.Unmarshal(data, &user)
    return &user, nil
}

func (c *UserCache) Set(ctx context.Context, user *User, ttl time.Duration) error {
    key := fmt.Sprintf("user:%s", user.ID)
    data, _ := json.Marshal(user)
    return c.store.Set(ctx, key, data, ttl)
}
```

## Design Decisions

### Why `[]byte` instead of generics?

- Maximum flexibility for users to handle serialization
- No codec abstraction needed - users know their data best
- Simpler implementation, easier to reason about

### PostgreSQL Schema

```sql
CREATE UNLOGGED TABLE kv_store (
    key_hash BIGINT PRIMARY KEY,        -- FNV-1a hash for fast lookups
    key TEXT NOT NULL,                  -- Actual key for collision detection
    value BYTEA NOT NULL,               -- Raw bytes
    expires_at TIMESTAMPTZ,             -- Optional expiration
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX kv_store_expires_idx ON kv_store (expires_at)
WHERE expires_at IS NOT NULL;
```

**Why `key_hash`?**
- Consistent fast lookups regardless of key length (URLs, composite keys, etc.)
- BIGINT (8 bytes) vs potentially hundreds of bytes for long keys
- Better index performance on fixed-width integers

**Why FNV-1a?**
- Fast non-cryptographic hash
- Good distribution for cache keys
- Collision detection via storing actual key

## Cleanup Strategies

### MemoryStore
- **Always automatic** - Cleans up expired entries every 1 minute
- Cheap since it's in-process

### PostgresStore
- **Opt-in automatic** - Use `WithCleanup(interval)` option
- **Manual** - Call `Cleanup(ctx)` from your cron/scheduler
- Default is manual to avoid thundering herd in multi-instance deployments

## Performance

**MemoryStore:**
- Nanosecond-level operations
- Perfect for single-instance applications

**PostgresStore (UNLOGGED):**
- 2-3x faster than logged tables
- Data lost on crash (acceptable for cache)
- Great for temporary client state across multiple instances

## Use Cases

- **Session storage** - Temporary user session data
- **Client state** - React useState equivalent on server
- **Cache** - General-purpose caching
- **Rate limiting** - With TTL for automatic cleanup
- **Feature flags** - Fast in-memory or distributed via Postgres

## Comparison to Alternatives

| Feature | kv | Redis | BadgerDB |
|---------|----|----|---------|
| Setup | Minimal | Separate service | Embedded |
| Dependencies | Postgres (optional) | Redis server | File system |
| TTL | ✓ | ✓ | ✓ |
| Distributed | ✓ (Postgres) | ✓ | ✗ |
| In-memory | ✓ | ✓ | ✗ |
| Complexity | Low | Medium | Low |

## License

MIT
