// Package events is an in-memory stand-in for Kafka with at-least-once delivery.
//
// Read AGENTS.md in this directory before changing anything here.
package events

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Event is one record on a topic. ID is unique per record; Key groups records
// whose relative order is guaranteed (like a Kafka partition key).
type Event struct {
	ID      string
	Type    string
	Key     string
	Payload []byte
	Offset  int64
}

// Handler processes one event. Return an error to get the event redelivered.
type Handler func(ctx context.Context, evt Event) error

// ErrGiveUp is returned by Consume when a handler keeps failing.
var ErrGiveUp = errors.New("events: handler failed after max attempts")

// Bus holds topics in memory. Delivery is at-least-once: Consume retries on
// error, and Replay redelivers everything, which is what a consumer-group
// rebalance looks like from the handler's point of view.
type Bus struct {
	mu         sync.Mutex
	topics     map[string][]Event
	MaxRetries int
}

// New returns an empty bus that retries a failing handler up to 3 times.
func New() *Bus { return &Bus{topics: map[string][]Event{}, MaxRetries: 3} }

// Publish appends an event to a topic and assigns its offset.
func (b *Bus) Publish(topic string, evt Event) Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	evt.Offset = int64(len(b.topics[topic]))
	b.topics[topic] = append(b.topics[topic], evt)
	return evt
}

// Consume delivers every event from offset `from` in order. A handler error
// triggers a retry of the same event; after MaxRetries it returns ErrGiveUp.
func (b *Bus) Consume(ctx context.Context, topic string, from int64, h Handler) error {
	for _, evt := range b.snapshot(topic) {
		if evt.Offset < from {
			continue
		}
		if err := b.deliver(ctx, evt, h); err != nil {
			return err
		}
	}
	return nil
}

// Replay delivers the whole topic again from offset 0.
func (b *Bus) Replay(ctx context.Context, topic string, h Handler) error {
	return b.Consume(ctx, topic, 0, h)
}

// Len reports how many events a topic holds.
func (b *Bus) Len(topic string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.topics[topic])
}

func (b *Bus) deliver(ctx context.Context, evt Event, h Handler) error {
	var err error
	for attempt := 0; attempt <= b.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err = h(ctx, evt); err == nil {
			return nil
		}
	}
	return fmt.Errorf("%w: %s (%s): %v", ErrGiveUp, evt.ID, evt.Type, err)
}

func (b *Bus) snapshot(topic string) []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Event, len(b.topics[topic]))
	copy(out, b.topics[topic])
	return out
}
