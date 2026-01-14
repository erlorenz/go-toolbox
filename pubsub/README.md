# pubsub

A simple, low-level publish-subscribe messaging system for Go. Designed as a **dumb transport layer** for prototyping event-driven applications.

## Features

- **Fire-and-forget**: Event notifications, not a queue (no guarantees, no retries)
- **Two implementations**:
  - `InMemory`: Channel-based, single-process
  - `Postgres`: LISTEN/NOTIFY-based, multi-process
- **Interface-based**: Easy to mock and test
- **No opinions**: No durability, generics, or serialization - build your own adapters
- **Context-aware**: Subscribers automatically unsubscribe when context is cancelled

## Installation

```bash
go get github.com/erlorenz/go-toolbox/pubsub
```

## Quick Start

### In-Memory (Single Process)

```go
import "github.com/erlorenz/go-toolbox/pubsub"

func main() {
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

### PostgreSQL (Multi-Process)

```go
import (
    "github.com/erlorenz/go-toolbox/pubsub"
    "github.com/jackc/pgx/v5/pgxpool"
)

pool, _ := pgxpool.New(ctx, "postgres://...")
defer pool.Close()

broker := pubsub.NewPostgres(pool)
defer broker.Close()

// Subscribe (in process A)
broker.Subscribe(ctx, "events", func(payload []byte) {
    fmt.Printf("Process A received: %s\n", payload)
})

// Publish (in process B)
broker.Publish(ctx, "events", []byte("hello from B"))
```

## Interface

```go
type Broker interface {
    Publish(ctx context.Context, topic string, payload []byte) error
    Subscribe(ctx context.Context, topic string, fn func([]byte)) error
    Close() error
}
```

## Use Cases

### ✅ Good For

- **Event notifications** - Notify other parts of your app when something happens
- **Cache invalidation** - Broadcast cache clear events
- **Live updates** - SSE/WebSocket fan-out (see examples)
- **Development** - Prototyping event-driven architectures
- **Decoupling** - Separate components that don't need guaranteed delivery

### ❌ Not Good For

- **Task queues** - No retry, no persistence (use pgboss, River, etc.)
- **Critical events** - No delivery guarantees
- **Ordering** - No guarantee of message order
- **Large payloads** - 8KB limit for Postgres

## Recommended Pattern: Typed Adapters

Build application-specific adapters for type safety and business logic:

```go
// Domain event
type JobCompleted struct {
    JobID   string `json:"job_id"`
    BatchID string `json:"batch_id"`
    Status  string `json:"status"`
}

// Typed adapter
type JobEventsAdapter struct {
    broker pubsub.Broker
}

func NewJobEventsAdapter(broker pubsub.Broker) *JobEventsAdapter {
    return &JobEventsAdapter{broker: broker}
}

// Type-safe publish
func (a *JobEventsAdapter) PublishJobCompleted(ctx context.Context, event JobCompleted) error {
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    return a.broker.Publish(ctx, "job.completed", data)
}

// Filtered subscription
func (a *JobEventsAdapter) SubscribeToJobsInBatch(ctx context.Context, batchID string) <-chan JobCompleted {
    ch := make(chan JobCompleted, 10)

    a.broker.Subscribe(ctx, "job.completed", func(payload []byte) {
        var event JobCompleted
        if err := json.Unmarshal(payload, &event); err != nil {
            return
        }

        // Application-level filtering
        if event.BatchID == batchID {
            select {
            case ch <- event:
            default:
                // Drop if channel is full
            }
        }
    })

    return ch
}
```

Usage:

```go
adapter := NewJobEventsAdapter(broker)

// Publish
adapter.PublishJobCompleted(ctx, JobCompleted{
    JobID:   "123",
    BatchID: "batch-1",
    Status:  "success",
})

// Subscribe with filtering
events := adapter.SubscribeToJobsInBatch(ctx, "batch-1")
for event := range events {
    fmt.Printf("Job %s completed\n", event.JobID)
}
```

## Context Handling

Subscribers automatically unsubscribe when their context is cancelled:

```go
ctx, cancel := context.WithCancel(context.Background())

broker.Subscribe(ctx, "events", func(payload []byte) {
    fmt.Println("Received:", string(payload))
})

// Later...
cancel()  // Subscriber is automatically removed
```

## Implementation Details

### InMemory

- Uses Go channels for message delivery
- Goroutines for each subscriber
- Handler errors are silently ignored (fire-and-forget)
- Perfect for single-process applications

### Postgres

- Uses PostgreSQL's `LISTEN`/`NOTIFY` commands
- One dedicated connection per topic
- Multiple subscribers on same topic share one `LISTEN` connection
- Automatic cleanup when all subscribers unsubscribe
- **Payload limit**: 8000 bytes (PostgreSQL restriction)
- **No durability**: Messages lost if no subscribers

## Error Handling

Both implementations follow fire-and-forget semantics:

- `Publish()` may return connection errors
- Handler panics are caught and ignored
- Slow handlers won't block publishers
- No retry logic

## Examples

### Server-Sent Events (SSE)

See [examples/pubsub](../examples/pubsub) for a complete SSE implementation showing:
- Broadcasting events to multiple clients
- Client disconnect handling
- Typed event adapters

### Cache Invalidation

```go
type CacheInvalidator struct {
    broker pubsub.Broker
}

func (c *CacheInvalidator) Invalidate(ctx context.Context, keys ...string) error {
    data, _ := json.Marshal(keys)
    return c.broker.Publish(ctx, "cache.invalidate", data)
}

func (c *CacheInvalidator) Watch(ctx context.Context, onInvalidate func([]string)) {
    c.broker.Subscribe(ctx, "cache.invalidate", func(payload []byte) {
        var keys []string
        json.Unmarshal(payload, &keys)
        onInvalidate(keys)
    })
}
```

## Testing

Mock the interface for testing:

```go
type MockBroker struct {
    published []struct {
        topic   string
        payload []byte
    }
}

func (m *MockBroker) Publish(ctx context.Context, topic string, payload []byte) error {
    m.published = append(m.published, struct{topic string; payload []byte}{topic, payload})
    return nil
}

func (m *MockBroker) Subscribe(ctx context.Context, topic string, fn func([]byte)) error {
    return nil
}

func (m *MockBroker) Close() error {
    return nil
}
```

## Comparison

| Feature | InMemory | Postgres |
|---------|----------|----------|
| Single process | ✓ | ✓ |
| Multi-process | ✗ | ✓ |
| Payload limit | None | 8KB |
| Setup | None | PostgreSQL |
| Performance | Very fast | Fast |
| Durability | None | None |

## When to Upgrade

Consider upgrading to a proper message queue when you need:

- **Guaranteed delivery** - Messages must not be lost
- **Retries** - Failed handlers should retry
- **Ordering** - Messages must be processed in order
- **Persistence** - Messages survive crashes
- **Backpressure** - Slow consumers shouldn't lose messages

Good alternatives:
- [River](https://riverqueue.com/) - Job queue for Postgres
- [pgboss](https://github.com/timgit/pg-boss) - Node.js equivalent
- NATS - Lightweight messaging system
- Redis Pub/Sub - Similar semantics, widely supported

## License

MIT
