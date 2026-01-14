# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Structure

This is a **single-module Go toolbox** repository designed to house multiple independent utility packages. Users can install individual packages via:

```bash
go get github.com/erlorenz/go-toolbox/<package-name>
```

**Current packages:**
- `cfgx` - Configuration management from multiple sources (env vars, flags, docker secrets, defaults)
- `pubsub` - Simple publish-subscribe messaging (in-memory & PostgreSQL)
- `kv` - Key-value store with TTL support (in-memory & PostgreSQL)

## Documentation Structure

- **Main README** ([README.md](../README.md)) - Overview with table of contents linking to package-specific docs
- **Package READMEs** - Each package has its own detailed README:
  - [cfgx/README.md](../cfgx/README.md)
  - [pubsub/README.md](../pubsub/README.md)
  - [kv/README.md](../kv/README.md)

## Development Commands

**Run tests:**
```bash
# Run all tests
go test -v ./...

# Run tests with race detection
go test --race -v ./...

# Run specific package tests
go test -v ./cfgx
go test -v ./pubsub
go test -v ./kv

# Run specific test
go test -v ./cfgx -run TestParse
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
- `PostgresStore` - PostgreSQL-backed with opt-in cleanup

**Key Components:**
- `Store` interface ([kv.go](../kv/kv.go))
- FNV-1a hash for fast lookups (BIGINT primary key)
- TTL/expiration support
- Prefix-based key listing

**PostgreSQL Schema:**
```sql
CREATE UNLOGGED TABLE kv_store (
    key_hash BIGINT PRIMARY KEY,        -- FNV-1a hash
    key TEXT NOT NULL,                  -- Actual key for collision detection
    value BYTEA NOT NULL,               -- Raw bytes
    expires_at TIMESTAMPTZ,             -- Optional expiration
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Design decisions:**
- `key_hash` (BIGINT) for consistent fast lookups regardless of key length
- FNV-1a hash: fast, good distribution, non-cryptographic
- `updated_at` instead of `created_at`: more useful for cache use cases
- UNLOGGED table option: 2-3x faster, acceptable for cache (data lost on crash)

**Cleanup strategies:**
- `MemoryStore`: Always automatic (cheap, in-process)
- `PostgresStore`: Opt-in via `WithCleanup(interval)` or manual via `Cleanup(ctx)`

**Testing patterns:** Interface-based testing, black-box tests in `package kv_test`.

See [kv/README.md](../kv/README.md) for full documentation.

## Common Patterns

### Interface-Based Design
All packages provide simple interfaces that are easy to mock and test:
- `cfgx.Source` - Configuration sources
- `pubsub.Broker` - Pub/sub messaging
- `kv.Store` - Key-value storage

### Build Your Own Adapters
Packages provide primitives, not frameworks. Users should build application-specific adapters:
- Type-safe wrappers around `pubsub.Broker`
- Serialization adapters around `kv.Store`
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
