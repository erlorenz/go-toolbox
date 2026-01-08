# pubsub

A simple, low-level publish-subscribe messaging system for Go.

## Overview

The `pubsub` package provides a minimal interface for publishing and subscribing to topics with `[]byte` payloads. It's designed to be a **dumb transport layer** - users build their own adapters for type safety, filtering, and business logic.

## Features

- **Two implementations:**
  - `InMemory`: Channel-based, single-process pub/sub
  - `Postgres`: LISTEN/NOTIFY-based, multi-process pub/sub
- **Fire-and-forget**: Messages are dropped if no subscribers exist
- **No durability**: This is an event notification system, not a queue
- **Interface-based**: Easy to mock and test

## Installation

```bash
go get github.com/erlorenz/go-toolbox/pubsub
```

## Usage

### Basic Example

```go
package main

import (
    "context"
    "fmt"
    "github.com/erlorenz/go-toolbox/pubsub"
)

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

### Real-World Pattern: Adapter Layer

The recommended pattern is to create an adapter that wraps the broker with your application's types and logic:

```go
// Domain type
type JobCompleted struct {
    JobID   string
    BatchID string
    Status  string
}

// Adapter with type safety and filtering
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

See [examples/pubsub](../examples/pubsub) for a complete SSE example.

## Implementations

### InMemory

```go
broker := pubsub.NewInMemory()
```

- Best for: Single-process apps, testing, development
- Thread-safe
- Low latency
- Messages lost on restart

### Postgres

```go
pool, _ := pgxpool.New(ctx, connString)
broker := pubsub.NewPostgres(pool)
```

- Best for: Multi-process apps, distributed systems
- Uses PostgreSQL LISTEN/NOTIFY
- Messages shared across all connected processes
- 8000 byte payload limit
- Still not durable (messages lost if no listeners)

## Design Philosophy

This package intentionally does **not** provide:

- Generic type parameters
- Message serialization/deserialization
- Filtering or routing logic
- Pattern matching or wildcards (yet)
- Durability or persistence
- Message acknowledgment
- Retries or error handling

These concerns belong in your application's adapter layer, where you have full control over the implementation.

## Testing

```bash
# Run tests
go test ./pubsub

# Run benchmarks
go test -bench=. ./pubsub

# Run with race detection
go test -race ./pubsub
```

## License

MIT
