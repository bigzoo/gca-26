package booking

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/bigzoo/gca-26/demo/internal/events"
	"github.com/bigzoo/gca-26/demo/internal/ledger"
)

func tradeEvent(id, tradeID, side, qty, price, fee string) events.Event {
	p, _ := json.Marshal(TradeExecuted{
		TradeID: tradeID, Account: "acc-1", Instrument: "SAF", Side: side,
		Quantity: decimal.RequireFromString(qty), Price: decimal.RequireFromString(price),
		Fee: decimal.RequireFromString(fee), Currency: "KES",
	})
	return events.Event{ID: id, Type: TypeTradeExecuted, Key: "acc-1", Payload: p}
}

// Invariant B1: a BUY debits securities and fees, credits cash, in one batch.
func TestBuyBooksPrincipalAndFee(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "trade-1", "BUY", "10", "25.50", "2.00"))
	if err := bus.Consume(context.Background(), Topic, 0, svc.Handle); err != nil {
		t.Fatal(err)
	}
	if got := book.Balance("cash:acc-1:KES", "KES"); !got.Equal(decimal.RequireFromString("-257")) {
		t.Fatalf("cash = %s, want -257", got)
	}
	if got := book.Balance("sec:acc-1:SAF", "KES"); !got.Equal(decimal.RequireFromString("255")) {
		t.Fatalf("securities = %s, want 255", got)
	}
	if n := len(book.Entries()); n != 2 {
		t.Fatalf("want 2 entries in one batch, got %d", n)
	}
}

// Invariant B2: redelivery of the same event (retry, rebalance, replay)
// books nothing twice.
func TestReplayIsIdempotent(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "trade-1", "BUY", "1", "100", "0"))
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := bus.Replay(ctx, Topic, svc.Handle); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(book.Entries()); n != 1 {
		t.Fatalf("replayed event booked %d entries, want 1", n)
	}
}

// Invariant B3: a bad payload is retried, then surfaced; it never panics and
// never writes a partial batch.
func TestBadSideIsRejected(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "trade-1", "SHORT", "1", "100", "0"))
	err := bus.Consume(context.Background(), Topic, 0, svc.Handle)
	if err == nil {
		t.Fatal("want error for unknown side")
	}
	if n := len(book.Entries()); n != 0 {
		t.Fatalf("partial write: %d entries", n)
	}
}
