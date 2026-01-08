package pubsub

import (
	"context"
	"sync"
)

// InMemory is a simple in-memory broker using Go channels.
// It's suitable for single-process applications, testing, and development.
// Messages are not persisted and are lost if no subscribers are active.
type InMemory struct {
	mu       sync.RWMutex
	subs     map[string][]subscription
	closed   bool
	closedCh chan struct{}
}

// subscription represents a single subscriber's handler and context.
type subscription struct {
	ctx     context.Context
	handler func([]byte)
	cancel  context.CancelFunc
}

// NewInMemory creates a new in-memory broker.
func NewInMemory() *InMemory {
	return &InMemory{
		subs:     make(map[string][]subscription),
		closedCh: make(chan struct{}),
	}
}

// Publish sends a message to all subscribers of the topic.
// If no subscribers exist, the message is dropped (fire-and-forget).
// Each subscriber's handler is called in its own goroutine.
func (m *InMemory) Publish(ctx context.Context, topic string, payload []byte) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return ErrClosed
	}

	// Check context first
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Get subscribers for this topic
	subs := m.subs[topic]
	if len(subs) == 0 {
		return nil // No subscribers, fire-and-forget
	}

	// Copy payload so handlers can't mutate it
	payloadCopy := make([]byte, len(payload))
	copy(payloadCopy, payload)

	// Broadcast to all subscribers
	for _, sub := range subs {
		// Skip if subscriber's context is done
		if sub.ctx.Err() != nil {
			continue
		}

		// Run handler in goroutine so slow handlers don't block publisher
		go sub.handler(payloadCopy)
	}

	return nil
}

// Subscribe registers a handler for the specified topic.
// The handler is called in a new goroutine for each message.
// The subscription remains active until ctx is canceled or Close is called.
func (m *InMemory) Subscribe(ctx context.Context, topic string, handler func([]byte)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrClosed
	}

	// Create a cancellable context for this subscription
	subCtx, cancel := context.WithCancel(ctx)

	sub := subscription{
		ctx:     subCtx,
		handler: handler,
		cancel:  cancel,
	}

	m.subs[topic] = append(m.subs[topic], sub)

	// Watch for context cancellation and clean up
	go m.watchSubscription(topic, sub)

	return nil
}

// watchSubscription monitors a subscription's context and removes it when done.
func (m *InMemory) watchSubscription(topic string, sub subscription) {
	select {
	case <-sub.ctx.Done():
		m.removeSubscription(topic, sub)
	case <-m.closedCh:
		sub.cancel()
	}
}

// removeSubscription removes a specific subscription from a topic.
func (m *InMemory) removeSubscription(topic string, target subscription) {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs := m.subs[topic]
	for i, sub := range subs {
		// Compare by context (unique per subscription)
		if sub.ctx == target.ctx {
			// Remove this subscription
			m.subs[topic] = append(subs[:i], subs[i+1:]...)
			sub.cancel()
			break
		}
	}

	// Clean up empty topic
	if len(m.subs[topic]) == 0 {
		delete(m.subs, topic)
	}
}

// Close stops all subscriptions and prevents new ones.
func (m *InMemory) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrClosed
	}

	m.closed = true
	close(m.closedCh)

	// Cancel all subscriptions
	for _, subs := range m.subs {
		for _, sub := range subs {
			sub.cancel()
		}
	}

	m.subs = make(map[string][]subscription)

	return nil
}
