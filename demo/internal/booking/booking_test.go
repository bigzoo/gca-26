package booking

import (
	"context"
	"encoding/json"
	"strings"
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

// Invariant B3: a correction re-emits the same TradeID with a new Event.ID and
// must book again.
func TestCorrectionBooksAgain(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "T-1", "BUY", "100", "10", "0"))
	bus.Publish(Topic, tradeEvent("evt-2", "T-1", "SELL", "30", "10", "0"))
	if err := bus.Consume(context.Background(), Topic, 0, svc.Handle); err != nil {
		t.Fatal(err)
	}
	if n := len(book.Entries()); n != 2 {
		t.Fatalf("want 2 entries (original + correction), got %d", n)
	}
	if _, ok := svc.Processed("evt-2"); !ok {
		t.Fatal("correction event not marked processed")
	}
}

func cancelEvent(id, tradeID string) events.Event {
	p, _ := json.Marshal(TradeCancelled{TradeID: tradeID})
	return events.Event{ID: id, Type: TypeTradeCancelled, Key: "acc-1", Payload: p}
}

// Invariants B6 and L4: a cancel posts reversals as new batches that name the
// original primanota; the original entries stay; all balances return to zero.
func TestCancelReversesBooking(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "T-1", "BUY", "10", "25.50", "2.00"))
	bus.Publish(Topic, cancelEvent("evt-2", "T-1"))
	if err := bus.Consume(context.Background(), Topic, 0, svc.Handle); err != nil {
		t.Fatal(err)
	}
	entries := book.Entries()
	if len(entries) != 4 {
		t.Fatalf("want 2 original + 2 reversal entries, got %d", len(entries))
	}
	for _, a := range []ledger.AccountID{"cash:acc-1:KES", "sec:acc-1:SAF", "fees:house:KES"} {
		if got := book.Balance(a, "KES"); !got.IsZero() {
			t.Fatalf("%s = %s, want 0", a, got)
		}
	}
	for _, e := range entries[2:] {
		if e.Primanota != "PN-000002" || !strings.Contains(e.Memo, "PN-000001") {
			t.Fatalf("reversal must be a new batch naming PN-000001: %+v", e)
		}
	}
}

// Invariants B2 and B6: a replayed or repeated cancel reverses once; a cancel
// also undoes a correction booked for the same trade.
func TestCancelIsIdempotentAndCoversCorrections(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "T-1", "BUY", "100", "10", "0"))
	bus.Publish(Topic, tradeEvent("evt-2", "T-1", "SELL", "30", "10", "0"))
	bus.Publish(Topic, cancelEvent("evt-3", "T-1"))
	ctx := context.Background()
	if err := bus.Consume(ctx, Topic, 0, svc.Handle); err != nil {
		t.Fatal(err)
	}
	if err := bus.Replay(ctx, Topic, svc.Handle); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, cancelEvent("evt-4", "T-1")); err != nil {
		t.Fatal(err)
	}
	if n := len(book.Entries()); n != 4 {
		t.Fatalf("want 4 entries, got %d", n)
	}
	for _, a := range []ledger.AccountID{"cash:acc-1:KES", "sec:acc-1:SAF"} {
		if got := book.Balance(a, "KES"); !got.IsZero() {
			t.Fatalf("%s = %s, want 0", a, got)
		}
	}
}

// Invariant B6: cancelling an unknown trade books nothing and does not fail.
func TestCancelUnknownTradeIsNoop(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	if err := svc.Handle(context.Background(), cancelEvent("evt-1", "T-404")); err != nil {
		t.Fatal(err)
	}
	if n := len(book.Entries()); n != 0 {
		t.Fatalf("want 0 entries, got %d", n)
	}
}

// Invariant B4: a bad payload is retried, then surfaced; it never panics and
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
