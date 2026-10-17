// Package ledger is a minimal double-entry book.
//
// Read AGENTS.md in this directory before changing anything here.
package ledger

import (
	"errors"
	"fmt"
	"sync"

	"github.com/shopspring/decimal"
)

// AccountID names an account, e.g. "cash:acc-1:EUR" or "sec:acc-1:AAPL".
type AccountID string

// PrimanotaID identifies one atomic batch of entries.
type PrimanotaID string

// Entry moves Amount of Currency from Debit to Credit.
type Entry struct {
	Primanota PrimanotaID
	Debit     AccountID
	Credit    AccountID
	Amount    decimal.Decimal
	Currency  string
	Memo      string
}

var (
	ErrEmptyBatch     = errors.New("ledger: batch has no entries")
	ErrNonPositive    = errors.New("ledger: amount must be > 0")
	ErrSameAccount    = errors.New("ledger: debit and credit must differ")
	ErrMixedCurrency  = errors.New("ledger: one currency per batch")
	ErrUnknownBatch   = errors.New("ledger: unknown primanota")
	ErrMissingAccount = errors.New("ledger: debit and credit are required")
)

// Book is an append-only list of entries grouped in primanota batches.
type Book struct {
	mu      sync.Mutex
	seq     int
	entries []Entry
}

// New returns an empty book.
func New() *Book { return &Book{} }

// Post validates the batch and appends it atomically under a new primanota.
// Either every entry is written or none is.
func (b *Book) Post(entries ...Entry) (PrimanotaID, error) {
	if len(entries) == 0 {
		return "", ErrEmptyBatch
	}
	ccy := entries[0].Currency
	for _, e := range entries {
		switch {
		case e.Debit == "" || e.Credit == "":
			return "", ErrMissingAccount
		case e.Debit == e.Credit:
			return "", fmt.Errorf("%w: %s", ErrSameAccount, e.Debit)
		case !e.Amount.IsPositive():
			return "", fmt.Errorf("%w: got %s", ErrNonPositive, e.Amount)
		case e.Currency != ccy:
			return "", fmt.Errorf("%w: %s vs %s", ErrMixedCurrency, ccy, e.Currency)
		}
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	b.seq++
	id := PrimanotaID(fmt.Sprintf("PN-%06d", b.seq))
	for _, e := range entries {
		e.Primanota = id
		b.entries = append(b.entries, e)
	}
	return id, nil
}

// Entries returns a copy of every entry in posting order.
func (b *Book) Entries() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Entry, len(b.entries))
	copy(out, b.entries)
	return out
}

// Batch returns the entries posted under one primanota.
func (b *Book) Batch(id PrimanotaID) ([]Entry, error) {
	var out []Entry
	for _, e := range b.Entries() {
		if e.Primanota == id {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnknownBatch, id)
	}
	return out, nil
}

// Reverse undoes the batch id by posting a new batch under a new primanota.
// The new batch has the same entries with debit and credit swapped, and every
// Memo names the original primanota. The original entries are never touched.
func (b *Book) Reverse(id PrimanotaID) (PrimanotaID, error) {
	orig, err := b.Batch(id)
	if err != nil {
		return "", err
	}
	rev := make([]Entry, 0, len(orig))
	for _, e := range orig {
		rev = append(rev, Entry{
			Debit:    e.Credit,
			Credit:   e.Debit,
			Amount:   e.Amount,
			Currency: e.Currency,
			Memo:     fmt.Sprintf("reverse %s: %s", id, e.Memo),
		})
	}
	return b.Post(rev...)
}

// Balance is debits minus credits for one account in one currency, i.e. the
// asset view: cash you hold is positive, cash you paid out is negative.
func (b *Book) Balance(acct AccountID, currency string) decimal.Decimal {
	total := decimal.Zero
	for _, e := range b.Entries() {
		if e.Currency != currency {
			continue
		}
		if e.Debit == acct {
			total = total.Add(e.Amount)
		}
		if e.Credit == acct {
			total = total.Sub(e.Amount)
		}
	}
	return total
}
