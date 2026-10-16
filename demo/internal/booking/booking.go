// Package booking consumes trade events and posts them to the ledger.
//
// Read AGENTS.md in this directory before changing anything here.
package booking

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/shopspring/decimal"

	"github.com/bigzoo/gca-26/demo/internal/events"
	"github.com/bigzoo/gca-26/demo/internal/ledger"
)

// Topic is the Kafka topic (here: bus topic) this service consumes.
const Topic = "trades"

// Event types this service understands.
const (
	TypeTradeExecuted  = "TradeExecuted"
	TypeTradeCancelled = "TradeCancelled"
)

// TradeCancelled is the payload of a TypeTradeCancelled event. It undoes every
// batch booked for TradeID, by posting reversals.
type TradeCancelled struct {
	TradeID string `json:"trade_id"`
}

// TradeExecuted is the payload of a TypeTradeExecuted event.
// A trade correction re-emits the same TradeID with a new event ID.
type TradeExecuted struct {
	TradeID    string          `json:"trade_id"`
	Account    string          `json:"account"`
	Instrument string          `json:"instrument"`
	Side       string          `json:"side"` // BUY or SELL
	Quantity   decimal.Decimal `json:"quantity"`
	Price      decimal.Decimal `json:"price"`
	Fee        decimal.Decimal `json:"fee"`
	Currency   string          `json:"currency"`
}

// Service turns trade events into ledger entries exactly once per event.
type Service struct {
	book *ledger.Book

	mu   sync.Mutex
	seen map[string]ledger.PrimanotaID // idempotency: processed event IDs -> their batch
	// trades maps a trade to the batches that booked it, in posting order.
	trades map[string][]ledger.PrimanotaID
	// reversed holds batches that already have a reversal, so a retried
	// cancellation never reverses twice.
	reversed map[ledger.PrimanotaID]bool
}

// New wires a service to a book.
func New(book *ledger.Book) *Service {
	return &Service{
		book:     book,
		seen:     map[string]ledger.PrimanotaID{},
		trades:   map[string][]ledger.PrimanotaID{},
		reversed: map[ledger.PrimanotaID]bool{},
	}
}

// Handle dispatches one event by type. Unknown types are skipped, not failed,
// so a new producer-side event type never stalls the consumer.
func (s *Service) Handle(ctx context.Context, evt events.Event) error {
	switch evt.Type {
	case TypeTradeExecuted:
		return s.handleTradeExecuted(ctx, evt)
	case TypeTradeCancelled:
		return s.handleTradeCancelled(ctx, evt)
	default:
		return nil
	}
}

func (s *Service) handleTradeExecuted(_ context.Context, evt events.Event) error {
	var t TradeExecuted
	if err := json.Unmarshal(evt.Payload, &t); err != nil {
		return fmt.Errorf("booking: decode %s: %w", evt.ID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, done := s.seen[evt.ID]; done {
		return nil
	}

	entries, err := entriesFor(t)
	if err != nil {
		return err
	}
	pn, err := s.book.Post(entries...)
	if err != nil {
		return fmt.Errorf("booking: post %s: %w", evt.ID, err)
	}
	s.seen[evt.ID] = pn
	s.trades[t.TradeID] = append(s.trades[t.TradeID], pn)
	return nil
}

// handleTradeCancelled reverses every live batch of the trade. Each reversal is
// a new batch; past entries are never changed. A cancel for an unknown trade
// books nothing.
func (s *Service) handleTradeCancelled(_ context.Context, evt events.Event) error {
	var c TradeCancelled
	if err := json.Unmarshal(evt.Payload, &c); err != nil {
		return fmt.Errorf("booking: decode %s: %w", evt.ID, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, done := s.seen[evt.ID]; done {
		return nil
	}

	var last ledger.PrimanotaID
	for _, orig := range s.trades[c.TradeID] {
		if s.reversed[orig] {
			continue
		}
		batch, err := s.book.Batch(orig)
		if err != nil {
			return fmt.Errorf("booking: cancel %s: %w", evt.ID, err)
		}
		rev := make([]ledger.Entry, len(batch))
		for i, e := range batch {
			rev[i] = ledger.Entry{
				Debit: e.Credit, Credit: e.Debit, Amount: e.Amount, Currency: e.Currency,
				Memo: "reversal of " + string(orig) + " (cancel " + c.TradeID + ")",
			}
		}
		pn, err := s.book.Post(rev...)
		if err != nil {
			return fmt.Errorf("booking: post %s: %w", evt.ID, err)
		}
		s.reversed[orig] = true
		last = pn
	}
	s.seen[evt.ID] = last
	return nil
}

// entriesFor maps a trade to its double-entry batch.
//
//	BUY : securities <- cash (principal), fees <- cash (fee)
//	SELL: cash <- securities (principal), fees <- cash (fee)
func entriesFor(t TradeExecuted) ([]ledger.Entry, error) {
	cash := ledger.AccountID("cash:" + t.Account + ":" + t.Currency)
	sec := ledger.AccountID("sec:" + t.Account + ":" + t.Instrument)
	fees := ledger.AccountID("fees:house:" + t.Currency)
	principal := t.Quantity.Mul(t.Price)

	var e []ledger.Entry
	switch t.Side {
	case "BUY":
		e = append(e, ledger.Entry{Debit: sec, Credit: cash, Amount: principal, Currency: t.Currency, Memo: "buy " + t.TradeID})
	case "SELL":
		e = append(e, ledger.Entry{Debit: cash, Credit: sec, Amount: principal, Currency: t.Currency, Memo: "sell " + t.TradeID})
	default:
		return nil, fmt.Errorf("booking: unknown side %q on %s", t.Side, t.TradeID)
	}
	if t.Fee.IsPositive() {
		e = append(e, ledger.Entry{Debit: fees, Credit: cash, Amount: t.Fee, Currency: t.Currency, Memo: "fee " + t.TradeID})
	}
	return e, nil
}

// Processed reports whether an event has been booked and under which batch.
func (s *Service) Processed(key string) (ledger.PrimanotaID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pn, ok := s.seen[key]
	return pn, ok
}
