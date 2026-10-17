package booking

import (
	"context"
	"encoding/json"
	"errors"
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

func cancelEvent(id, tradeID string) events.Event {
	p, _ := json.Marshal(TradeCancelled{TradeID: tradeID})
	return events.Event{ID: id, Type: TypeTradeCancelled, Key: "acc-1", Payload: p}
}

func sumBalances(book *ledger.Book) decimal.Decimal {
	total := decimal.Zero
	for _, a := range []ledger.AccountID{"cash:acc-1:KES", "sec:acc-1:SAF", "fees:house:KES"} {
		total = total.Add(book.Balance(a, "KES"))
	}
	return total
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

// Invariant B4: a bad payload is retried, then surfaced; it never panics,
// never writes a partial batch, and leaves the event unseen.
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
	if _, ok := svc.Processed("evt-1"); ok {
		t.Fatal("failed event was marked as seen")
	}
}

// Invariant B3: a correction re-emits the same TradeID under a new Event.ID.
// It is a new event and books a new batch. Dedup on TradeID drops it.
func TestCorrectionBooksAgain(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "trade-1", "BUY", "10", "25.50", "2.00"))
	bus.Publish(Topic, tradeEvent("evt-2", "trade-1", "SELL", "4", "26.00", "1.00")) // correction
	ctx := context.Background()
	if err := bus.Consume(ctx, Topic, 0, svc.Handle); err != nil {
		t.Fatal(err)
	}
	if n := len(book.Entries()); n != 4 {
		t.Fatalf("correction dropped: %d entries, want 4", n)
	}
	pn1, ok1 := svc.Processed("evt-1")
	pn2, ok2 := svc.Processed("evt-2")
	if !ok1 || !ok2 || pn1 == pn2 {
		t.Fatalf("want two distinct batches, got %q (%v) and %q (%v)", pn1, ok1, pn2, ok2)
	}
	// -257 from the BUY, +104 -1 from the correction.
	if got := book.Balance("cash:acc-1:KES", "KES"); !got.Equal(decimal.RequireFromString("-154")) {
		t.Fatalf("cash = %s, want -154", got)
	}
	// Still idempotent on Event.ID (B2) after the fix.
	if err := bus.Replay(ctx, Topic, svc.Handle); err != nil {
		t.Fatal(err)
	}
	if n := len(book.Entries()); n != 4 {
		t.Fatalf("replay booked again: %d entries, want 4", n)
	}
}

// Invariant B6: a cancellation reverses every open batch of the trade, the
// original and its corrections, with one reversal batch each. Nothing is
// deleted; the book grows and every balance of the trade returns to zero.
func TestCancelReversesTrade(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "trade-1", "BUY", "10", "25.50", "2.00"))
	bus.Publish(Topic, tradeEvent("evt-2", "trade-1", "SELL", "4", "26.00", "1.00")) // correction
	bus.Publish(Topic, cancelEvent("evt-3", "trade-1"))
	if err := bus.Consume(context.Background(), Topic, 0, svc.Handle); err != nil {
		t.Fatal(err)
	}

	entries := book.Entries()
	if n := len(entries); n != 8 {
		t.Fatalf("want 2+2 booked and 2+2 reversed = 8 entries, got %d", n)
	}
	for _, a := range []ledger.AccountID{"cash:acc-1:KES", "sec:acc-1:SAF", "fees:house:KES"} {
		if got := book.Balance(a, "KES"); !got.IsZero() {
			t.Fatalf("%s = %s after cancel, want 0", a, got)
		}
	}

	pn1, _ := svc.Processed("evt-1")
	pn2, _ := svc.Processed("evt-2")
	var named1, named2 bool
	for _, e := range entries[4:] {
		named1 = named1 || strings.Contains(e.Memo, string(pn1))
		named2 = named2 || strings.Contains(e.Memo, string(pn2))
	}
	if !named1 || !named2 {
		t.Fatalf("reversal memos must name both original batches %s and %s", pn1, pn2)
	}
	if pn, ok := svc.Processed("evt-3"); !ok || pn == "" {
		t.Fatalf("cancel not marked as processed: %q %v", pn, ok)
	}
}

// Invariant B2 and B6: a cancel is idempotent on Event.ID across replays, and
// a second cancel of the same trade under a new Event.ID books nothing. It
// must never re-book the trade by reversing the reversal.
func TestCancelIsIdempotent(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, tradeEvent("evt-1", "trade-1", "BUY", "10", "25.50", "2.00"))
	bus.Publish(Topic, cancelEvent("evt-2", "trade-1"))
	bus.Publish(Topic, cancelEvent("evt-3", "trade-1")) // duplicate cancel, new event
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := bus.Replay(ctx, Topic, svc.Handle); err != nil {
			t.Fatal(err)
		}
	}
	if n := len(book.Entries()); n != 4 {
		t.Fatalf("want 2 booked + 2 reversed = 4 entries, got %d", n)
	}
	if got := sumBalances(book); !got.IsZero() {
		t.Fatalf("book does not balance: %s", got)
	}
	if got := book.Balance("sec:acc-1:SAF", "KES"); !got.IsZero() {
		t.Fatalf("securities = %s after cancel, want 0", got)
	}
	if _, ok := svc.Processed("evt-3"); !ok {
		t.Fatal("duplicate cancel must be marked as seen so replays skip it")
	}
}

// Invariant B7: a cancel for a trade that was never booked is an error. It is
// retried, then surfaced, and the event stays unseen. It is never skipped.
func TestCancelUnknownTradeIsRejected(t *testing.T) {
	book := ledger.New()
	svc := New(book)
	bus := events.New()
	bus.Publish(Topic, cancelEvent("evt-1", "trade-9"))
	err := bus.Consume(context.Background(), Topic, 0, svc.Handle)
	if !errors.Is(err, events.ErrGiveUp) {
		t.Fatalf("want ErrGiveUp, got %v", err)
	}
	if err := svc.Handle(context.Background(), cancelEvent("evt-1", "trade-9")); !errors.Is(err, ErrUnknownTrade) {
		t.Fatalf("want ErrUnknownTrade, got %v", err)
	}
	if n := len(book.Entries()); n != 0 {
		t.Fatalf("unknown cancel wrote %d entries", n)
	}
	if _, ok := svc.Processed("evt-1"); ok {
		t.Fatal("failed cancel was marked as seen")
	}
}
