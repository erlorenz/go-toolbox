# go-toolbox

A collection of Go packages for building web servers and rapid prototyping.

**Note:** These packages are designed for quick prototyping and experimentation. For production use, consider copying and adapting the code to your specific needs.

## Installation

```bash
# Configuration management
go get github.com/erlorenz/go-toolbox/cfgx

# Pub/sub messaging
go get github.com/erlorenz/go-toolbox/pubsub
```

## Packages

### cfgx

The cfgx package provides a simple and flexible way to handle application configuration through environment variables and command-line flags. It uses struct tags to parse into a configuration struct. Heavily inspired by [github.com/ardanlabs/conf](https://pkg.go.dev/github.com/ardanlabs/conf/v3).

#### Key Features

- Unified handling of environment variables and command-line flags
- Automatic parsing into configuration structs based on naming convention
- Validation of required configuration values
- Clear error messages for missing or invalid configuration
- Support for default values

#### Usage

First, define your configuration structure with field tags:

```go
type Config struct {
    Version string // populated with BuildInfo.Main.Version
    Port     int    `env:"MY_PORT" short:"p" default:"8080"` // also reads flag -p
    DBString string `flag:"dsn" required:"true"` // errors if empty
    // Nested structs are prefixed
    Log struct {
        Level string // env=LOG_LEVEL flag=log-level
    }
}
```

Then load your configuration:

```go
import "github.com/erlorenz/go-toolbox/cfgx"

// Optional: Set via -ldflags at build time for production
var Version string

func main() {
    cfg := Config{
        Version: Version, // Highest priority - set via ldflags
    }

    if err := cfgx.Parse(&cfg, cfgx.Options{}); err != nil {
        log.Fatalf("Configuration error: %v", err)
    }

    // Configuration is now ready to use
    log.Printf("Version %s: Starting server on port %d", cfg.Version, cfg.Port)
}
```

#### Setting Version for Production

For production builds, inject the version using `-ldflags`:

```bash
# Local build
VERSION=$(git describe --tags --always --dirty)
go build -ldflags="-X main.Version=$VERSION" -o myapp

# Docker build
docker build --build-arg VERSION=$(git describe --tags --always) -t myapp .
```

Example Dockerfile:
```dockerfile
FROM golang:1.24 AS builder
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN go build -ldflags="-X main.Version=${VERSION}" -o /app

FROM gcr.io/distroless/base-debian12
COPY --from=builder /app /app
ENTRYPOINT ["/app"]
```

The `Version` field will automatically use build info from `go build` during local development (returns version tag or `"(devel)"`). For production, use `-ldflags` as shown above for reliable version tracking.

### pubsub

A simple, low-level publish-subscribe messaging system for Go. Designed as a **dumb transport layer** for prototyping event-driven applications.

#### Key Features

- Fire-and-forget event notifications (not a queue)
- Two implementations:
  - `InMemory`: Channel-based, single-process
  - `Postgres`: LISTEN/NOTIFY-based, multi-process
- Interface-based for easy mocking
- No durability, generics, or serialization - build your own adapters

#### Quick Start

```go
import "github.com/erlorenz/go-toolbox/pubsub"

func main() {
    // Create broker
    broker := pubsub.NewInMemory()
    defer broker.Close()

    ctx := context.Background()

    // Subscribe
    broker.Subscribe(ctx, "events", func(payload []byte) {
        fmt.Printf("Received: %s\n", payload)
    })

    // Publish
    broker.Publish(ctx, "events", []byte("hello world"))
}
```

#### Recommended Pattern: Adapter Layer

Build a typed adapter in your application for type safety and filtering:

```go
// Domain type
type JobCompleted struct {
    JobID   string `json:"job_id"`
    BatchID string `json:"batch_id"`
    Status  string `json:"status"`
}

// Adapter with type safety
type JobEventsAdapter struct {
    broker pubsub.Broker
}

func (a *JobEventsAdapter) PublishJobCompleted(ctx context.Context, event JobCompleted) error {
    data, _ := json.Marshal(event)
    return a.broker.Publish(ctx, "job.completed", data)
}

func (a *JobEventsAdapter) SubscribeToJobsInBatch(ctx context.Context, batchID string) <-chan JobCompleted {
    ch := make(chan JobCompleted, 10)

    a.broker.Subscribe(ctx, "job.completed", func(payload []byte) {
        var event JobCompleted
        json.Unmarshal(payload, &event)

        // Filter by batch ID
        if event.BatchID == batchID {
            ch <- event
        }
    })

    return ch
}
```

See [examples/pubsub](examples/pubsub) for a complete SSE example.

#### Postgres Implementation

```go
import "github.com/jackc/pgx/v5/pgxpool"

pool, _ := pgxpool.New(ctx, connString)
broker := pubsub.NewPostgres(pool)
defer broker.Close()

// Now publish/subscribe across multiple processes
```

**Limitations:**
- 8000 byte payload limit (PostgreSQL NOTIFY restriction)
- No durability (events lost if no listeners)
- Still fire-and-forget (not a message queue)

## Development

### Running Tests

```bash
# All tests
mise run test:all

# Specific package
mise run test:cfgx
go test -v ./pubsub
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

## License

This project is licensed under the MIT License - see the LICENSE file for details.
