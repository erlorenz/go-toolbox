package pubsub_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/erlorenz/go-toolbox/pubsub"
)

// testBroker runs a common test suite against any broker implementation.
func testBroker(t *testing.T, createBroker func() pubsub.Broker, cleanup func()) {
	t.Helper()

	tests := []struct {
		name string
		test func(t *testing.T, broker pubsub.Broker)
	}{
		{"PublishWithNoSubscribers", testPublishWithNoSubscribers},
		{"SingleSubscriber", testSingleSubscriber},
		{"MultipleSubscribers", testMultipleSubscribers},
		{"MultipleTopics", testMultipleTopics},
		{"SubscriberContextCancellation", testSubscriberContextCancellation},
		{"PublisherContextCancellation", testPublisherContextCancellation},
		{"CloseBroker", testCloseBroker},
		{"PayloadIsolation", testPayloadIsolation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			broker := createBroker()
			defer broker.Close()
			if cleanup != nil {
				defer cleanup()
			}
			tt.test(t, broker)
		})
	}
}

func testPublishWithNoSubscribers(t *testing.T, broker pubsub.Broker) {
	ctx := context.Background()

	// Should not error even with no subscribers (fire-and-forget)
	err := broker.Publish(ctx, "test-topic", []byte("hello"))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
}

func testSingleSubscriber(t *testing.T, broker pubsub.Broker) {
	ctx := context.Background()
	received := make(chan []byte, 1)

	// Subscribe
	err := broker.Subscribe(ctx, "test-topic", func(payload []byte) {
		received <- payload
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Give subscriber time to set up (especially for Postgres)
	time.Sleep(50 * time.Millisecond)

	// Publish
	err = broker.Publish(ctx, "test-topic", []byte("hello"))
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	// Wait for message
	select {
	case msg := <-received:
		if string(msg) != "hello" {
			t.Errorf("Expected 'hello', got %q", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for message")
	}
}

func testMultipleSubscribers(t *testing.T, broker pubsub.Broker) {
	ctx := context.Background()
	received1 := make(chan []byte, 1)
	received2 := make(chan []byte, 1)
	received3 := make(chan []byte, 1)

	// Subscribe 3 handlers to same topic
	broker.Subscribe(ctx, "test-topic", func(payload []byte) {
		received1 <- payload
	})
	broker.Subscribe(ctx, "test-topic", func(payload []byte) {
		received2 <- payload
	})
	broker.Subscribe(ctx, "test-topic", func(payload []byte) {
		received3 <- payload
	})

	time.Sleep(50 * time.Millisecond)

	// Publish once
	broker.Publish(ctx, "test-topic", []byte("broadcast"))

	// All 3 should receive
	timeout := time.After(1 * time.Second)
	for i, ch := range []chan []byte{received1, received2, received3} {
		select {
		case msg := <-ch:
			if string(msg) != "broadcast" {
				t.Errorf("Subscriber %d: expected 'broadcast', got %q", i+1, msg)
			}
		case <-timeout:
			t.Fatalf("Subscriber %d: timeout waiting for message", i+1)
		}
	}
}

func testMultipleTopics(t *testing.T, broker pubsub.Broker) {
	ctx := context.Background()
	receivedA := make(chan []byte, 1)
	receivedB := make(chan []byte, 1)

	// Subscribe to different topics
	broker.Subscribe(ctx, "topic-a", func(payload []byte) {
		receivedA <- payload
	})
	broker.Subscribe(ctx, "topic-b", func(payload []byte) {
		receivedB <- payload
	})

	time.Sleep(50 * time.Millisecond)

	// Publish to topic-a
	broker.Publish(ctx, "topic-a", []byte("message-a"))

	// Only topic-a should receive
	select {
	case msg := <-receivedA:
		if string(msg) != "message-a" {
			t.Errorf("Expected 'message-a', got %q", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for topic-a message")
	}

	// topic-b should not receive anything
	select {
	case msg := <-receivedB:
		t.Errorf("topic-b should not receive message, got %q", msg)
	case <-time.After(100 * time.Millisecond):
		// Expected - no message
	}

	// Publish to topic-b
	broker.Publish(ctx, "topic-b", []byte("message-b"))

	select {
	case msg := <-receivedB:
		if string(msg) != "message-b" {
			t.Errorf("Expected 'message-b', got %q", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for topic-b message")
	}
}

func testSubscriberContextCancellation(t *testing.T, broker pubsub.Broker) {
	ctx, cancel := context.WithCancel(context.Background())
	received := make(chan []byte, 10)

	// Subscribe with cancellable context
	broker.Subscribe(ctx, "test-topic", func(payload []byte) {
		received <- payload
	})

	time.Sleep(50 * time.Millisecond)

	// Publish first message
	broker.Publish(context.Background(), "test-topic", []byte("message-1"))

	select {
	case msg := <-received:
		if string(msg) != "message-1" {
			t.Errorf("Expected 'message-1', got %q", msg)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for message-1")
	}

	// Cancel subscriber context
	cancel()
	time.Sleep(100 * time.Millisecond)

	// Publish second message
	broker.Publish(context.Background(), "test-topic", []byte("message-2"))

	// Should NOT receive message-2
	select {
	case msg := <-received:
		t.Errorf("Should not receive after cancel, got %q", msg)
	case <-time.After(200 * time.Millisecond):
		// Expected - no message
	}
}

func testPublisherContextCancellation(t *testing.T, broker pubsub.Broker) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Should fail or handle gracefully
	err := broker.Publish(ctx, "test-topic", []byte("hello"))
	if err == nil {
		t.Log("Publish with canceled context succeeded (implementation-specific)")
	} else if err != context.Canceled {
		t.Logf("Publish returned error (expected): %v", err)
	}
}

func testCloseBroker(t *testing.T, broker pubsub.Broker) {
	ctx := context.Background()

	// Subscribe
	broker.Subscribe(ctx, "test-topic", func(payload []byte) {})

	// Close broker
	err := broker.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Operations after close should fail
	err = broker.Publish(ctx, "test-topic", []byte("hello"))
	if err != pubsub.ErrClosed {
		t.Errorf("Expected ErrClosed after Close, got %v", err)
	}

	err = broker.Subscribe(ctx, "test-topic", func(payload []byte) {})
	if err != pubsub.ErrClosed {
		t.Errorf("Expected ErrClosed after Close, got %v", err)
	}

	// Double close should not panic
	err = broker.Close()
	if err != pubsub.ErrClosed {
		t.Errorf("Expected ErrClosed on double close, got %v", err)
	}
}

func testPayloadIsolation(t *testing.T, broker pubsub.Broker) {
	ctx := context.Background()
	var mu sync.Mutex
	var received []byte

	// Subscribe with a handler that modifies the payload
	broker.Subscribe(ctx, "test-topic", func(payload []byte) {
		mu.Lock()
		received = payload
		// Try to modify it
		if len(payload) > 0 {
			payload[0] = 'X'
		}
		mu.Unlock()
	})

	time.Sleep(50 * time.Millisecond)

	// Publish
	original := []byte("hello")
	err := broker.Publish(ctx, "test-topic", original)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Original should not be modified
	if string(original) != "hello" {
		t.Errorf("Original payload was modified: %q", original)
	}

	mu.Lock()
	if string(received) != "hello" && string(received) != "Xello" {
		t.Errorf("Unexpected received payload: %q", received)
	}
	mu.Unlock()
}

// Benchmark publishing with varying numbers of subscribers
func benchmarkPublish(b *testing.B, broker pubsub.Broker, numSubscribers int) {
	ctx := context.Background()

	// Add subscribers
	for i := 0; i < numSubscribers; i++ {
		broker.Subscribe(ctx, "bench-topic", func(payload []byte) {
			// No-op handler
		})
	}

	time.Sleep(50 * time.Millisecond)

	payload := []byte("benchmark message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broker.Publish(ctx, "bench-topic", payload)
	}
}
