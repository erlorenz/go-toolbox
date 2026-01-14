# go-toolbox

A collection of lightweight, focused Go packages for building web applications and rapid prototyping.

**Note:** These packages are designed for quick prototyping and experimentation. For production use, consider copying and adapting the code to your specific needs.

## Packages

| Package | Description | Documentation |
|---------|-------------|---------------|
| **[cfgx](cfgx/)** | Configuration management from env vars, flags, Docker secrets | [README](cfgx/README.md) |
| **[pubsub](pubsub/)** | Simple publish-subscribe messaging (in-memory & PostgreSQL) | [README](pubsub/README.md) |
| **[kv](kv/)** | Key-value store with TTL support (in-memory & PostgreSQL) | [README](kv/README.md) |

## Quick Start

Install individual packages:

```bash
# Configuration management
go get github.com/erlorenz/go-toolbox/cfgx

# Pub/sub messaging
go get github.com/erlorenz/go-toolbox/pubsub

# Key-value store
go get github.com/erlorenz/go-toolbox/kv
```

## Package Overview

### cfgx - Configuration Management

Parse configuration from multiple sources with priority-based overrides.

```go
type Config struct {
    Port int `env:"PORT" default:"8080"`
}

cfgx.Parse(&cfg, cfgx.Options{})
```

**Features:**
- Environment variables, command-line flags, Docker secrets, defaults
- Struct tags for declarative configuration
- Nested struct support
- Required field validation

[Full documentation →](cfgx/README.md)

### pubsub - Publish-Subscribe Messaging

Fire-and-forget event notifications for decoupling application components.

```go
broker := pubsub.NewInMemory()
broker.Subscribe(ctx, "events", func(payload []byte) {
    fmt.Println("Received:", string(payload))
})
broker.Publish(ctx, "events", []byte("hello"))
```

**Features:**
- In-memory (single process) and PostgreSQL (multi-process) backends
- Context-aware subscriptions
- Build your own typed adapters
- Perfect for SSE, cache invalidation, event notifications

[Full documentation →](pubsub/README.md)

### kv - Key-Value Store

Simple key-value store with TTL support and multiple backends.

```go
store := kv.NewMemoryStore()
store.Set(ctx, "user:123", []byte("alice"), 5*time.Minute)
data, _ := store.Get(ctx, "user:123")
```

**Features:**
- `[]byte` interface - handle your own serialization
- TTL/expiration support
- Prefix-based key listing
- In-memory (auto-cleanup) and PostgreSQL (opt-in cleanup) backends
- UNLOGGED table support for 2-3x faster Postgres performance

[Full documentation →](kv/README.md)

## Development

### Running Tests

```bash
# All tests
go test -v ./...

# Specific package
go test -v ./cfgx
go test -v ./pubsub
go test -v ./kv
```

Using mise:

```bash
# All tests
mise run test:all

# Specific package
mise run test:cfgx
```

### Creating Releases

Use the tagbump task to bump semantic versions:

```bash
# Bump patch version (0.4.0 → 0.4.1)
mise run tagbump patch

# Bump minor version (0.4.0 → 0.5.0)
mise run tagbump minor

# Bump major version (0.4.0 → 1.0.0)
mise run tagbump major
```

This creates a git tag locally. To push:

```bash
git push origin v0.5.0
```

## Design Philosophy

These packages follow a few core principles:

1. **Simple interfaces** - Easy to understand, test, and mock
2. **No magic** - Explicit behavior, clear error messages
3. **Build your own adapters** - Provide primitives, not frameworks
4. **Production-ready patterns** - Based on proven designs (Rails Solid Cache, etc.)
5. **Minimal dependencies** - Only what's necessary

## License

MIT License - see [LICENSE](LICENSE) for details.
