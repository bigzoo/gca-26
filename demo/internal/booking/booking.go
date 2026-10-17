// Package booking consumes trade events and posts them to the ledger.
//
// Read AGENTS.md in this directory before changing anything here.
package booking

import (
	"context"
	"encoding/json"
	"errors"
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

// ErrUnknownTrade is returned when a cancellation names a trade that was
// never booked. The bus retries it and then gives up; it is never skipped.
var ErrUnknownTrade = errors.New("booking: cancel for unknown trade")

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

// TradeCancelled is the payload of a TypeTradeCancelled event. It undoes
// every open batch of TradeID with one reversal batch each (ledger L4).
type TradeCancelled struct {
	TradeID string `json:"trade_id"`
}

// Service turns trade events into ledger entries exactly once per event.
type Service struct {
	book *ledger.Book

	mu   sync.Mutex
	seen map[string]ledger.PrimanotaID   // idempotency: Event.ID -> the last batch it posted
	open map[string][]ledger.PrimanotaID // TradeID -> batches not yet reversed, oldest first
}

// New wires a service to a book.
func New(book *ledger.Book) *Service {
	return &Service{
		book: book,
		seen: map[string]ledger.PrimanotaID{},
		open: map[string][]ledger.PrimanotaID{},
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
	// B2/B3: dedup on Event.ID, never on TradeID. A correction reuses the
	// TradeID under a new Event.ID and must book again.
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
	s.open[t.TradeID] = append(s.open[t.TradeID], pn)
	return nil
}

// handleTradeCancelled posts one reversal batch for every open batch of the
// trade, oldest first. A batch leaves the open list as soon as its reversal is
// posted, so a retry after a failed Reverse never reverses a batch twice (B4).
// A trade with nothing open (already cancelled) books nothing. A trade that
// was never booked is an error, so the bus retries and surfaces it.
func (s *Service) handleTradeCancelled(_ context.Context, evt events.Event) error {
	var c TradeCancelled
	if err := json.Unmarshal(evt.Payload, &c); err != nil {
		return fmt.Errorf("booking: decode %s: %w", evt.ID, err)
	}
	if c.TradeID == "" {
		return fmt.Errorf("booking: cancel %s: missing trade_id", evt.ID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, done := s.seen[evt.ID]; done {
		return nil
	}
	batches, known := s.open[c.TradeID]
	if !known {
		return fmt.Errorf("%w: %s (%s)", ErrUnknownTrade, c.TradeID, evt.ID)
	}

	var last ledger.PrimanotaID
	for len(batches) > 0 {
		pn, err := s.book.Reverse(batches[0])
		if err != nil {
			s.open[c.TradeID] = batches
			return fmt.Errorf("booking: cancel %s: %w", evt.ID, err)
		}
		batches = batches[1:]
		last = pn
	}
	s.open[c.TradeID] = nil // keep the key: the trade is known, nothing is open
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

// Processed reports whether an event (by Event.ID) has been handled and
// which batch it posted last. A cancel that found nothing open reports "".
func (s *Service) Processed(eventID string) (ledger.PrimanotaID, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pn, ok := s.seen[eventID]
	return pn, ok
}
