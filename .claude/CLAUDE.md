# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Structure

This is a **single-module Go toolbox** repository designed to house multiple independent utility packages. Users can install individual packages via:

```bash
go get github.com/erlorenz/go-toolbox/<package-name>
```

**Current packages:**
- `casing` - String case conversion utilities (snake_case, camelCase, PascalCase, kebab-case, etc.)
- `cfgx` - Configuration management from multiple sources (env vars, flags, docker secrets, defaults)
- `pubsub` - Simple publish-subscribe messaging (in-memory & PostgreSQL)
- `kv` - Key-value store with TTL, encryption, and atomic updates (in-memory & PostgreSQL)
- `assetmgr` - Static asset manager with versioning, import maps, and immutable caching

## Documentation Structure

- **Main README** ([README.md](../README.md)) - Overview with table of contents linking to package-specific docs
- **Package READMEs** - Each package has its own detailed README:
  - [casing/README.md](../casing/README.md)
  - [cfgx/README.md](../cfgx/README.md)
  - [pubsub/README.md](../pubsub/README.md)
  - [kv/README.md](../kv/README.md)
  - [assetmgr/README.md](../assetmgr/README.md)

## Development Commands

**Run tests:**
```bash
# Run all tests
go test -v ./...

# Run tests with race detection
go test --race -v ./...

# Run specific package tests
go test -v ./casing
go test -v ./cfgx
go test -v ./pubsub
go test -v ./kv
go test -v ./assetmgr

# Run specific test
go test -v ./casing -run TestToSnake
```

**Lint:**
```bash
go vet ./...
```

**Run examples:**
```bash
go run ./examples/config
go run ./examples/pubsub
```

## Package Architectures

### casing - String Case Conversion

**Core Design:** Pure functions for converting between different string casing conventions.

**Functions:**
- `ToSnake(s string) string` - Converts to snake_case
- `ToScreamingSnake(s string) string` - Converts to SCREAMING_SNAKE_CASE
- `ToKebab(s string) string` - Converts to kebab-case
- `ToPascal(s string) string` - Converts to PascalCase
- `ToCamel(s string) string` - Converts to camelCase

**Key Features:**
- Handles acronyms correctly (e.g., "HTTPServer" → "http_server")
- Preserves word boundaries with dots (e.g., "User.Name" → "user_name")
- No dependencies, just standard library

**Use Cases:**
- API/database field name conversion
- Configuration key normalization
- Code generation

**Testing patterns:** Table-driven tests with comprehensive edge cases.

See [casing/README.md](../casing/README.md) for full documentation.

### cfgx - Configuration Management

**Core Design:** Source-based architecture where configuration values come from "sources" processed in priority order.

**Built-in source priorities:**
- Command-line flags: 100 (highest)
- Docker secrets: 75
- Environment variables: 50
- Default struct tags: 0 (lowest)

**Key Components:**
- `Source` interface ([cfgx.go](../cfgx/cfgx.go))
- `ConfigField` - Represents parsed struct field with metadata
- Built-in sources in [sources.go](../cfgx/sources.go)

**Supported types:** `string`, `int`, `bool`. Nested structs supported (dot notation).

**Testing patterns:** Table-driven subtests with black-box testing (`package cfgx_test`).

See [cfgx/README.md](../cfgx/README.md) for full documentation.

### pubsub - Publish-Subscribe Messaging

**Core Design:** Fire-and-forget event notifications with interface-based design.

**Implementations:**
- `InMemory` - Channel-based, single-process
- `Postgres` - PostgreSQL LISTEN/NOTIFY, multi-process

**Key Components:**
- `Broker` interface ([pubsub.go](../pubsub/pubsub.go))
- Context-aware subscriptions (auto-unsubscribe on cancel)
- No durability, generics, or serialization - users build typed adapters

**Important patterns:**
- Build application-specific adapters for type safety
- Handler errors are silently ignored (fire-and-forget)
- Postgres payload limit: 8000 bytes

**Testing:** Mock the `Broker` interface.

See [pubsub/README.md](../pubsub/README.md) for full documentation.

### kv - Key-Value Store

**Core Design:** Simple `[]byte` interface allowing users to handle their own serialization.

**Implementations:**
- `MemoryStore` - In-memory with automatic cleanup every 1 minute
- `PostgresStore` - PostgreSQL-backed with JSONB or BYTEA storage, optional cleanup

**Key Components:**
- `Store` interface ([kv.go](../kv/kv.go)) - Get, Set, Update, Delete, Keys, Close
- `Encryptor` interface - Encrypt/Decrypt for transparent encryption
- `AESEncryptor` - Built-in AES-256-GCM encryption
- FNV-1a hash for fast lookups (BIGINT primary key)
- TTL/expiration support
- Atomic updates with `Update()` method
- Prefix-based key listing

**Atomic Updates:**
- `Update(ctx, key, ttl, fn)` - Read-modify-write in one operation
- **MemoryStore**: Uses write lock for atomicity
- **PostgresStore**: Uses transaction with `SELECT FOR UPDATE` for row-level locking
- Function receives current value (or nil), returns new value or error
- On error, transaction rolls back - no changes made

**Encryption:**
- Built-in `AESEncryptor` using AES-256-GCM
- Custom encryptors via `Encryptor` interface
- Transparent encrypt/decrypt in Get/Set/Update
- Authenticated encryption (GCM prevents tampering)

**PostgreSQL Options:**
- `WithFormat("JSONB" | "BYTEA")` - Storage format (default: JSONB, or BYTEA if encrypted)
- `WithEncryption(Encryptor)` - Enable encryption
- `WithSchema(string)` - PostgreSQL schema (default: "public")
- `WithTableName(string)` - Override auto-generated table name
- `WithUnlogged(bool)` - UNLOGGED table for 2-3x faster performance
- `WithKeyIndex(bool)` - Index on key column for fast prefix searches
- `WithCleanup(duration)` - Auto-cleanup expired entries

**PostgreSQL Schema:**
```sql
CREATE UNLOGGED TABLE kv_store (
    key_hash BIGINT PRIMARY KEY,        -- FNV-1a hash
    key TEXT NOT NULL,                  -- Actual key for collision detection
    value JSONB NOT NULL,               -- JSONB (default) or BYTEA (encrypted)
    expires_at TIMESTAMPTZ,             -- Optional expiration
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX kv_store_expires_idx ON kv_store (expires_at)
WHERE expires_at IS NOT NULL;

-- Optional: for fast prefix searches
CREATE INDEX kv_store_key_idx ON kv_store (key text_pattern_ops);
```

**Table Naming:**
Auto-generated based on configuration:
- `kv_store` - JSONB, logged
- `kv_store_unlogged` - JSONB, unlogged
- `kv_store_encrypted` - BYTEA with encryption, logged
- `kv_store_encrypted_unlogged` - BYTEA with encryption, unlogged

**Design decisions:**
- `key_hash` (BIGINT) for consistent fast lookups regardless of key length
- FNV-1a hash: fast, good distribution, non-cryptographic
- `updated_at` instead of `created_at`: more useful for cache use cases
- JSONB default: validates JSON, enables PostgreSQL queries, easier debugging
- BYTEA for encryption: required for encrypted binary data
- UNLOGGED table option: 2-3x faster, acceptable for cache (data lost on crash)

**Cleanup strategies:**
- `MemoryStore`: Always automatic (cheap, in-process)
- `PostgresStore`: Opt-in via `WithCleanup(interval)` or manual via `Cleanup(ctx)`

**Testing patterns:** Interface-based testing, black-box tests in `package kv_test`.

**Important files:**
- [kv.go](../kv/kv.go) - Interfaces (Store, Encryptor)
- [memory.go](../kv/memory.go) - In-memory implementation
- [postgres.go](../kv/postgres.go) - PostgreSQL implementation
- [aes_encryptor.go](../kv/aes_encryptor.go) - Built-in AES-256-GCM encryptor

See [kv/README.md](../kv/README.md) for full documentation.

### assetmgr - Static Asset Manager

**Core Design:** Static asset serving with content-based versioning and import map support. Follows the "no build" philosophy - no bundling or transpiling.

**Key Components:**
- `Manager` - Main type that walks filesystems, hashes content, serves assets
- `Asset` struct - Represents an asset with path, hash, pre-rendered tags
- `ImportMap` - JavaScript import map with automatic path rewriting

**Options:**
- `WithFS(prefix, fs.FS)` - Add filesystem with URL prefix
- `WithImportMap(path)` - Load and rewrite import map
- `WithDevMode(bool)` - Enable dev mode (no caching, re-reads files)
- `WithEnvVar(name)` - Environment variable for dev mode detection

**Versioning Strategy:**
- Query string versioning: `/static/app.js?v=abc123`
- FNV-1a content hashing (same as kv package)
- No file renaming = no import rewriting needed

**HTTP Caching:**
- Versioned requests (`?v=`): `Cache-Control: public, max-age=31536000, immutable`
- Non-versioned: `Cache-Control: no-cache` with ETag

**Pre-rendered Tags:**
- `asset.ScriptTag` - `<script type="module" src="...?v=hash"></script>`
- `asset.LinkTag` - `<link rel="stylesheet" href="...?v=hash">`
- Computed at startup, zero runtime overhead

**Import Map Handling:**
- Parses JSON import map from assets
- Rewrites local paths to versioned paths
- Preserves remote URLs (CDN, etc.)
- `mgr.ImportMapTag()` returns ready-to-use `<script type="importmap">`

**Helper Methods:**
- `Get(path)` / `MustGet(path)` - Get single asset
- `All()` - All assets sorted by path
- `ByExtension(".js")` - Filter by extension
- `ByPrefix("/static/js/")` - Filter by path prefix
- `ScriptTags(prefix)` - All script tags for JS files under prefix
- `LinkTags(prefix)` - All link tags for CSS files under prefix

**Dev Mode:**
- Enabled when `APP_ENV != "production"` (or custom env var)
- Re-reads files on each request (for `os.DirFS`)
- No caching headers

**Use Cases:**
- htmx + Templ applications
- Deno-style ES modules with import maps
- Monorepo with assets in multiple locations

**Testing patterns:** Black-box tests with `testing/fstest.MapFS`.

**Important files:**
- [assetmgr.go](../assetmgr/assetmgr.go) - All types and implementation

See [assetmgr/README.md](../assetmgr/README.md) for full documentation.

## Common Patterns

### Interface-Based Design
All packages provide simple interfaces that are easy to mock and test:
- `cfgx.Source` - Configuration sources
- `pubsub.Broker` - Pub/sub messaging
- `kv.Store` - Key-value storage
- `kv.Encryptor` - Encryption/decryption

### Build Your Own Adapters
Packages provide primitives, not frameworks. Users should build application-specific adapters:
- Type-safe wrappers around `pubsub.Broker`
- Serialization adapters around `kv.Store`
- Custom encryptors for `kv.Encryptor` (KMS, Vault, etc.)
- Custom configuration sources for `cfgx`

### Testing
- Use table-driven subtests with `t.Run()`
- Black-box testing with `_test` package suffix
- Mock interfaces for unit tests
- Use options to skip sources/features during testing

## Design Philosophy

1. **Simple interfaces** - Easy to understand, test, and mock
2. **No magic** - Explicit behavior, clear error messages
3. **Build your own adapters** - Provide primitives, not frameworks
4. **Production-ready patterns** - Based on proven designs (Rails Solid Cache, etc.)
5. **Minimal dependencies** - Only what's necessary
