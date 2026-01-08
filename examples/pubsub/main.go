// Example demonstrating pubsub usage with SSE (Server-Sent Events)
//
// This example shows:
// 1. Creating an adapter layer with type safety and filtering
// 2. Publishing job completion events
// 3. Subscribing to filtered events via SSE
//
// Run with: go run ./examples/pubsub
// Then visit: http://localhost:8080/events?batch_id=batch-123
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/erlorenz/go-toolbox/pubsub"
)

// --- Application Domain Types ---

// JobCompleted represents a job completion event.
type JobCompleted struct {
	JobID       string    `json:"job_id"`
	BatchID     string    `json:"batch_id"`
	Status      string    `json:"status"` // "completed", "failed", "canceled"
	CompletedAt time.Time `json:"completed_at"`
}

// --- Application Adapter Layer ---

// JobEventsAdapter wraps the low-level pubsub broker with type safety and filtering.
type JobEventsAdapter struct {
	broker pubsub.Broker
}

func NewJobEventsAdapter(broker pubsub.Broker) *JobEventsAdapter {
	return &JobEventsAdapter{broker: broker}
}

// PublishJobCompleted publishes a job completion event.
func (a *JobEventsAdapter) PublishJobCompleted(ctx context.Context, event JobCompleted) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return a.broker.Publish(ctx, "job.completed", data)
}

// SubscribeToJobsInBatch subscribes to job completion events for a specific batch.
// Returns a channel that receives only events for the specified batchID.
func (a *JobEventsAdapter) SubscribeToJobsInBatch(ctx context.Context, batchID string) <-chan JobCompleted {
	ch := make(chan JobCompleted, 10)

	// Subscribe to all job.completed events
	a.broker.Subscribe(ctx, "job.completed", func(payload []byte) {
		var event JobCompleted
		if err := json.Unmarshal(payload, &event); err != nil {
			log.Printf("Failed to unmarshal event: %v", err)
			return
		}

		// FILTER: Only send events for this batch
		if event.BatchID == batchID {
			select {
			case ch <- event:
			case <-ctx.Done():
			default:
				// Drop if channel full
				log.Printf("Warning: dropped event for batch %s (channel full)", batchID)
			}
		}
	})

	// Close channel when context is done
	go func() {
		<-ctx.Done()
		close(ch)
	}()

	return ch
}

// --- HTTP Handlers ---

// SSEHandler streams job completion events to clients via Server-Sent Events.
type SSEHandler struct {
	adapter *JobEventsAdapter
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	batchID := r.URL.Query().Get("batch_id")
	if batchID == "" {
		http.Error(w, "batch_id query parameter required", http.StatusBadRequest)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get filtered event stream from adapter
	events := h.adapter.SubscribeToJobsInBatch(r.Context(), batchID)

	log.Printf("Client connected for batch: %s", batchID)
	defer log.Printf("Client disconnected for batch: %s", batchID)

	// Stream events to client
	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				// Channel closed, exit
				return
			}

			// Send as SSE
			jsonData, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", jsonData)

			// Flush to client immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
	}
}

// --- Background Job Simulator ---

func simulateJobs(adapter *JobEventsAdapter) {
	ctx := context.Background()
	jobNum := 0

	batches := []string{"batch-123", "batch-456", "batch-789"}
	statuses := []string{"completed", "failed", "completed", "completed"}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		batchID := batches[jobNum%len(batches)]
		status := statuses[jobNum%len(statuses)]

		event := JobCompleted{
			JobID:       fmt.Sprintf("job-%d", jobNum),
			BatchID:     batchID,
			Status:      status,
			CompletedAt: time.Now(),
		}

		log.Printf("Publishing: %s in %s - %s", event.JobID, event.BatchID, event.Status)
		if err := adapter.PublishJobCompleted(ctx, event); err != nil {
			log.Printf("Error publishing: %v", err)
		}

		jobNum++
	}
}

// --- Main ---

func main() {
	// Create broker
	broker := pubsub.NewInMemory()
	defer broker.Close()

	// Create adapter
	adapter := NewJobEventsAdapter(broker)

	// Set up HTTP handler
	http.Handle("/events", &SSEHandler{adapter: adapter})

	// Start background job simulator
	go simulateJobs(adapter)

	// Start server
	addr := ":8080"
	fmt.Printf("Server running on http://localhost%s\n", addr)
	fmt.Printf("Try: curl http://localhost%s/events?batch_id=batch-123\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
