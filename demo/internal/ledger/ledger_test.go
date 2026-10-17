package ledger

import (
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func amt(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// Invariant L1: every batch balances to zero across all accounts.
func TestBatchBalancesToZero(t *testing.T) {
	b := New()
	_, err := b.Post(
		Entry{Debit: "sec:a:AAPL", Credit: "cash:a:EUR", Amount: amt("150.25"), Currency: "EUR"},
		Entry{Debit: "fees:house:EUR", Credit: "cash:a:EUR", Amount: amt("0.15"), Currency: "EUR"},
	)
	if err != nil {
		t.Fatal(err)
	}
	sum := decimal.Zero
	for _, acct := range []AccountID{"sec:a:AAPL", "cash:a:EUR", "fees:house:EUR"} {
		sum = sum.Add(b.Balance(acct, "EUR"))
	}
	if !sum.IsZero() {
		t.Fatalf("book does not balance: %s", sum)
	}
}

// Invariant L2: Post is atomic. A bad entry anywhere in the batch writes nothing.
func TestPostIsAtomic(t *testing.T) {
	b := New()
	_, err := b.Post(
		Entry{Debit: "sec:a:AAPL", Credit: "cash:a:EUR", Amount: amt("10"), Currency: "EUR"},
		Entry{Debit: "cash:a:EUR", Credit: "cash:a:EUR", Amount: amt("1"), Currency: "EUR"},
	)
	if !errors.Is(err, ErrSameAccount) {
		t.Fatalf("want ErrSameAccount, got %v", err)
	}
	if n := len(b.Entries()); n != 0 {
		t.Fatalf("partial write: %d entries", n)
	}
}

// Invariant L3: amounts are exact decimals. 0.1 + 0.2 is 0.3, not 0.30000000000000004.
func TestDecimalIsExact(t *testing.T) {
	b := New()
	b.Post(Entry{Debit: "cash:a:EUR", Credit: "x", Amount: amt("0.1"), Currency: "EUR"})
	b.Post(Entry{Debit: "cash:a:EUR", Credit: "x", Amount: amt("0.2"), Currency: "EUR"})
	if got := b.Balance("cash:a:EUR", "EUR"); !got.Equal(amt("0.3")) {
		t.Fatalf("got %s", got)
	}
}

// Invariant L5: primanota IDs are monotonic and unique.
func TestPrimanotaIsMonotonic(t *testing.T) {
	b := New()
	var last PrimanotaID
	for i := 0; i < 3; i++ {
		id, _ := b.Post(Entry{Debit: "x", Credit: "y", Amount: amt("1"), Currency: "EUR"})
		if id <= last {
			t.Fatalf("not monotonic: %s after %s", id, last)
		}
		last = id
	}
}

// Invariant L4: the book is append-only. Reverse undoes a batch with a new
// batch that swaps debit and credit and names the original primanota. The
// original entries are untouched and the book still balances to zero.
func TestReversalUndoesBatch(t *testing.T) {
	b := New()
	orig, err := b.Post(
		Entry{Debit: "sec:a:AAPL", Credit: "cash:a:EUR", Amount: amt("150.25"), Currency: "EUR", Memo: "buy"},
		Entry{Debit: "fees:house:EUR", Credit: "cash:a:EUR", Amount: amt("0.15"), Currency: "EUR", Memo: "fee"},
	)
	if err != nil {
		t.Fatal(err)
	}
	before := b.Entries()

	rev, err := b.Reverse(orig)
	if err != nil {
		t.Fatal(err)
	}
	if rev <= orig {
		t.Fatalf("reversal %s must get a new, later primanota than %s", rev, orig)
	}

	after := b.Entries()
	if len(after) != 2*len(before) {
		t.Fatalf("want %d entries after reversal, got %d", 2*len(before), len(after))
	}
	for i, e := range before {
		got := after[i]
		if got.Primanota != e.Primanota || got.Debit != e.Debit || got.Credit != e.Credit ||
			!got.Amount.Equal(e.Amount) || got.Currency != e.Currency || got.Memo != e.Memo {
			t.Fatalf("original entry %d was mutated: %+v -> %+v", i, e, got)
		}
	}

	revEntries, err := b.Batch(rev)
	if err != nil {
		t.Fatal(err)
	}
	for i, e := range revEntries {
		if e.Debit != before[i].Credit || e.Credit != before[i].Debit || !e.Amount.Equal(before[i].Amount) {
			t.Fatalf("reversal entry %d is not the swap of the original: %+v", i, e)
		}
		if !strings.Contains(e.Memo, string(orig)) {
			t.Fatalf("reversal memo %q does not name original primanota %s", e.Memo, orig)
		}
	}

	for _, acct := range []AccountID{"sec:a:AAPL", "cash:a:EUR", "fees:house:EUR"} {
		if got := b.Balance(acct, "EUR"); !got.IsZero() {
			t.Fatalf("%s = %s after reversal, want 0", acct, got)
		}
	}

	if _, err := b.Reverse("PN-999999"); !errors.Is(err, ErrUnknownBatch) {
		t.Fatalf("want ErrUnknownBatch for unknown primanota, got %v", err)
	}
}
