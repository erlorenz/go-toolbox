// Package pubsub provides a simple publish-subscribe messaging system.
//
// The package defines low-level interfaces for publishing and subscribing to
// topics with []byte payloads. It's designed to be a dumb transport layer -
// users should build their own adapters for type safety, filtering, and
// business logic.
//
// Two implementations are provided:
//   - InMemory: Channel-based, single-process pub/sub
//   - Postgres: LISTEN/NOTIFY-based, multi-process pub/sub
package pubsub

import (
	"context"
	"errors"
)

// Common errors.
var (
	// ErrClosed is returned when operations are attempted on a closed broker.
	ErrClosed = errors.New("pubsub: broker is closed")
)

// Publisher publishes messages to topics.
type Publisher interface {
	// Publish sends a message to the specified topic.
	// The payload is delivered to all active subscribers.
	// Publishing is fire-and-forget - if no subscribers exist, the message is dropped.
	Publish(ctx context.Context, topic string, payload []byte) error

	// Close releases any resources held by the publisher.
	Close() error
}

// Subscriber subscribes to topics and receives messages via handlers.
type Subscriber interface {
	// Subscribe registers a handler for the specified topic.
	// The handler is called asynchronously for each message published to the topic.
	// Multiple subscribers to the same topic each receive a copy of every message.
	//
	// The subscription remains active until the context is canceled or Close is called.
	// Handlers should be fast and non-blocking. For slow operations, handlers should
	// spawn goroutines or use channels to bridge to synchronous code.
	Subscribe(ctx context.Context, topic string, handler func([]byte)) error

	// Close releases any resources held by the subscriber and stops all handlers.
	Close() error
}

// Broker combines Publisher and Subscriber interfaces.
// Most implementations provide both capabilities.
type Broker interface {
	Publisher
	Subscriber
}
