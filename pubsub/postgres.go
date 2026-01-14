package pubsub

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is a broker that uses PostgreSQL's LISTEN/NOTIFY for pub/sub.
// It's suitable for multi-process applications where events need to be
// shared across different instances or services connected to the same database.
//
// Unlike InMemory, Postgres can distribute messages across multiple processes,
// but it still provides no durability - messages are lost if no subscribers
// are listening.
type Postgres struct {
	pool      *pgxpool.Pool
	mu        sync.RWMutex
	listeners map[string]*topicListener
	closed    bool
}

// topicListener manages all subscriptions for a single topic.
type topicListener struct {
	topic    string
	conn     *pgx.Conn
	handlers []handler
	cancel   context.CancelFunc
	mu       sync.RWMutex
}

// handler represents a single subscriber's handler and context.
type handler struct {
	ctx    context.Context
	fn     func([]byte)
	cancel context.CancelFunc
}

// NewPostgres creates a new Postgres broker using the provided connection pool.
// The pool must remain open for the lifetime of the broker.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{
		pool:      pool,
		listeners: make(map[string]*topicListener),
	}
}

// Publish sends a message to all subscribers of the topic across all processes.
// It uses PostgreSQL's NOTIFY command. The payload is sent as the notification payload.
func (p *Postgres) Publish(ctx context.Context, topic string, payload []byte) error {
	p.mu.RLock()
	closed := p.closed
	p.mu.RUnlock()

	if closed {
		return ErrClosed
	}

	// Use NOTIFY with payload
	// Note: PostgreSQL NOTIFY payload is limited to 8000 bytes
	if len(payload) > 8000 {
		return errors.New("pubsub: payload exceeds PostgreSQL NOTIFY limit of 8000 bytes")
	}

	_, err := p.pool.Exec(ctx, "SELECT pg_notify($1, $2)", topic, string(payload))
	return err
}

// Subscribe registers a handler for the specified topic.
// It creates a dedicated PostgreSQL connection with LISTEN for this topic
// if one doesn't already exist. Multiple handlers for the same topic share
// a single LISTEN connection.
func (p *Postgres) Subscribe(ctx context.Context, topic string, fn func([]byte)) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrClosed
	}

	// Create handler with cancellable context
	handlerCtx, cancel := context.WithCancel(ctx)
	h := handler{
		ctx:    handlerCtx,
		fn:     fn,
		cancel: cancel,
	}

	// Get or create topic listener
	tl, exists := p.listeners[topic]
	if !exists {
		var err error
		tl, err = p.createTopicListener(ctx, topic)
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create listener for topic %q: %w", topic, err)
		}
		p.listeners[topic] = tl
	}

	// Add handler to topic listener
	tl.mu.Lock()
	tl.handlers = append(tl.handlers, h)
	tl.mu.Unlock()

	// Watch for context cancellation
	go p.watchHandler(topic, h)

	return nil
}

// createTopicListener creates a new listener for a topic with a dedicated connection.
func (p *Postgres) createTopicListener(ctx context.Context, topic string) (*topicListener, error) {
	// Acquire a connection from the pool for listening
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}

	// Create cancellable context for this listener
	listenerCtx, cancel := context.WithCancel(context.Background())

	tl := &topicListener{
		topic:    topic,
		conn:     conn.Conn(),
		handlers: []handler{},
		cancel:   cancel,
	}

	// Start LISTEN
	_, err = conn.Exec(listenerCtx, "LISTEN "+pgx.Identifier{topic}.Sanitize())
	if err != nil {
		conn.Release()
		cancel()
		return nil, err
	}

	// Start notification loop
	go tl.listen(listenerCtx, conn)

	return tl, nil
}

// listen waits for notifications and dispatches them to handlers.
func (tl *topicListener) listen(ctx context.Context, conn *pgxpool.Conn) {
	defer conn.Release()
	defer tl.cancel()

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			// Context canceled or connection error
			return
		}

		// Dispatch to all handlers
		tl.mu.RLock()
		handlers := make([]handler, len(tl.handlers))
		copy(handlers, tl.handlers)
		tl.mu.RUnlock()

		payload := []byte(notification.Payload)

		for _, h := range handlers {
			// Skip if handler's context is done
			if h.ctx.Err() != nil {
				continue
			}

			// Call handler in goroutine
			go h.fn(payload)
		}
	}
}

// watchHandler monitors a handler's context and removes it when done.
func (p *Postgres) watchHandler(topic string, h handler) {
	<-h.ctx.Done()
	p.removeHandler(topic, h)
}

// removeHandler removes a specific handler from a topic.
func (p *Postgres) removeHandler(topic string, target handler) {
	p.mu.Lock()
	defer p.mu.Unlock()

	tl, exists := p.listeners[topic]
	if !exists {
		return
	}

	tl.mu.Lock()
	defer tl.mu.Unlock()

	// Find and remove the handler
	for i, h := range tl.handlers {
		if h.ctx == target.ctx {
			tl.handlers = append(tl.handlers[:i], tl.handlers[i+1:]...)
			h.cancel()
			break
		}
	}

	// If no more handlers, stop the listener
	if len(tl.handlers) == 0 {
		tl.cancel()
		delete(p.listeners, topic)
	}
}

// Close stops all listeners and releases connections.
func (p *Postgres) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return ErrClosed
	}

	p.closed = true

	// Cancel all listeners
	for _, tl := range p.listeners {
		tl.cancel()
		tl.mu.Lock()
		for _, h := range tl.handlers {
			h.cancel()
		}
		tl.mu.Unlock()
	}

	p.listeners = make(map[string]*topicListener)

	return nil
}
