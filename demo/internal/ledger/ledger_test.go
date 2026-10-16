package ledger

import (
	"errors"
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

// Invariant L4: primanota IDs are monotonic and unique.
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
