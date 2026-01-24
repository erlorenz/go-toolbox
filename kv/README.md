# kv

A simple, fast key-value store package for Go with support for multiple backends (in-memory, PostgreSQL).

## Features

- **Simple `[]byte` interface** - Handle your own serialization (JSON, protobuf, etc.)
- **Atomic updates** - Read-modify-write operations without race conditions
- **TTL support** - Automatic expiration of entries
- **Prefix-based key listing** - Find all keys matching a prefix
- **Encryption** - Optional transparent encryption with custom encryptors
- **JSONB support** - Store and query JSON data directly in PostgreSQL
- **Multiple backends**:
  - **MemoryStore** - In-memory with automatic cleanup
  - **PostgresStore** - PostgreSQL-backed with JSONB or BYTEA storage
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

// Atomic update - read, modify, write in one operation
err := store.Update(ctx, "counter:visits", 0, func(current []byte) ([]byte, error) {
    var count int
    if current != nil {
        count, _ = strconv.Atoi(string(current))
    }
    count++
    return []byte(strconv.Itoa(count)), nil
})

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

### PostgreSQL Configuration Options

```go
// Storage format
kv.WithFormat("JSONB")      // Default: Store as JSONB for JSON data
kv.WithFormat("BYTEA")      // Use BYTEA for binary data or non-JSON

// Encryption
encryptor := NewAESEncryptor(key)  // Your Encryptor implementation
kv.WithEncryption(encryptor)       // Automatically uses BYTEA format

// Table customization
kv.WithTableName("custom_kv")      // Override auto-generated table name
kv.WithSchema("myschema")          // Use specific schema (default: "public")

// Performance
kv.WithUnlogged(true)              // 2-3x faster, data lost on crash
kv.WithKeyIndex(true)              // Index for fast prefix searches (adds overhead)
kv.WithCleanup(5*time.Minute)      // Auto-cleanup expired entries

// Example: Encrypted cache with fast prefix searches
store := kv.NewPostgresStore(pool,
    kv.WithEncryption(encryptor),
    kv.WithUnlogged(true),
    kv.WithKeyIndex(true),
    kv.WithSchema("cache"),
)
// Auto-generates table name: "kv_store_encrypted_unlogged"
```

## Atomic Updates

The `Update` method provides atomic read-modify-write operations, preventing race conditions when multiple processes update the same key.

```go
// Increment a counter atomically
err := store.Update(ctx, "page:views", 0, func(current []byte) ([]byte, error) {
    var count int
    if current != nil {
        count, _ = strconv.Atoi(string(current))
    }
    count++
    return []byte(strconv.Itoa(count)), nil
})

// Append to a list atomically
err := store.Update(ctx, "recent:events", time.Hour, func(current []byte) ([]byte, error) {
    var events []Event
    if current != nil {
        json.Unmarshal(current, &events)
    }

    events = append(events, newEvent)

    // Keep only last 100
    if len(events) > 100 {
        events = events[len(events)-100:]
    }

    return json.Marshal(events)
})

// Conditional update with error rollback
err := store.Update(ctx, "balance", 0, func(current []byte) ([]byte, error) {
    balance, _ := strconv.Atoi(string(current))

    if balance < withdrawAmount {
        return nil, errors.New("insufficient funds") // No update happens
    }

    balance -= withdrawAmount
    return []byte(strconv.Itoa(balance)), nil
})
```

**Implementation details:**
- **MemoryStore**: Uses write lock for entire operation
- **PostgresStore**: Uses transaction with `SELECT FOR UPDATE` for row-level locking
- If the update function returns an error, no changes are made
- The function receives `nil` if the key doesn't exist or is expired

## Encryption

Implement the `Encryptor` interface to add transparent encryption:

```go
type Encryptor interface {
    Encrypt(ctx context.Context, plaintext []byte) ([]byte, error)
    Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error)
}
```

Example using AES-256-GCM:

```go
type AESEncryptor struct {
    gcm cipher.AEAD
}

func NewAESEncryptor(key []byte) (*AESEncryptor, error) {
    block, _ := aes.NewCipher(key) // 32-byte key for AES-256
    gcm, _ := cipher.NewGCM(block)
    return &AESEncryptor{gcm: gcm}, nil
}

func (e *AESEncryptor) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
    nonce := make([]byte, e.gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    return e.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (e *AESEncryptor) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
    nonceSize := e.gcm.NonceSize()
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    return e.gcm.Open(nil, nonce, ciphertext, nil)
}

// Use with store
encryptor, _ := NewAESEncryptor(key)
store := kv.NewPostgresStore(pool, kv.WithEncryption(encryptor))
```

**Encryption features:**
- Transparent - Get/Set/Update automatically encrypt/decrypt
- Context-aware - Encryptor receives context for cancellation
- Format override - Uses BYTEA storage by default
- Zero performance impact on MemoryStore (just []byte storage)

**Production recommendations:**
- Use proper key management (AWS KMS, HashiCorp Vault, etc.)
- Implement key rotation strategy
- Consider envelope encryption for large values
- Monitor decryption failures

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

### JSONB vs BYTEA

**JSONB (default):**
- Best for JSON-serialized data
- Validates JSON on write (fails fast on invalid JSON)
- Easier debugging - can query PostgreSQL directly
- Native PostgreSQL indexing and query support
- Slightly larger storage overhead

**BYTEA:**
- For binary formats (protobuf, msgpack, etc.)
- Required for encryption
- No validation overhead
- More compact for non-text data

**Recommendation:** Use JSONB unless you need encryption or non-JSON serialization.

### PostgreSQL Schema

```sql
CREATE UNLOGGED TABLE kv_store (
    key_hash BIGINT PRIMARY KEY,        -- FNV-1a hash for fast lookups
    key TEXT NOT NULL,                  -- Actual key for collision detection
    value JSONB NOT NULL,               -- JSONB or BYTEA depending on config
    expires_at TIMESTAMPTZ,             -- Optional expiration
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX kv_store_expires_idx ON kv_store (expires_at)
WHERE expires_at IS NOT NULL;

-- Optional: fast prefix searches
CREATE INDEX kv_store_key_idx ON kv_store (key text_pattern_ops);
```

**Why `key_hash`?**
- Consistent fast lookups regardless of key length (URLs, composite keys, etc.)
- BIGINT (8 bytes) vs potentially hundreds of bytes for long keys
- Better index performance on fixed-width integers

**Why FNV-1a?**
- Fast non-cryptographic hash
- Good distribution for cache keys
- Collision detection via storing actual key

**Table naming:**
- Auto-generated based on configuration for multiple use cases
- Examples: `kv_store`, `kv_store_unlogged`, `kv_store_encrypted`, etc.
- Override with `WithTableName()` if needed

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
