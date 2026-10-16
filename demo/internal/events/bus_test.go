package events

import (
	"context"
	"errors"
	"testing"
)

// Invariant E1: a failing handler sees the same event again (at-least-once).
func TestRetriesOnError(t *testing.T) {
	b := New()
	b.Publish("t", Event{ID: "e1", Type: "X"})
	calls := 0
	err := b.Consume(context.Background(), "t", 0, func(ctx context.Context, e Event) error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

// Invariant E2: a handler that never succeeds does not block the world forever.
func TestGivesUpAfterMaxRetries(t *testing.T) {
	b := New()
	b.Publish("t", Event{ID: "e1"})
	err := b.Consume(context.Background(), "t", 0, func(context.Context, Event) error {
		return errors.New("poison pill")
	})
	if !errors.Is(err, ErrGiveUp) {
		t.Fatalf("want ErrGiveUp, got %v", err)
	}
}

// Invariant E3: delivery order equals publish order within a topic.
func TestOrderIsPreserved(t *testing.T) {
	b := New()
	for _, id := range []string{"a", "b", "c"} {
		b.Publish("t", Event{ID: id})
	}
	var got []string
	b.Consume(context.Background(), "t", 0, func(_ context.Context, e Event) error {
		got = append(got, e.ID)
		return nil
	})
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Fatalf("order broken: %v", got)
	}
}
