# internal/ledger — invariants

The book is the source of truth for money. If it is wrong, every report is wrong.

## Invariants

- **L1 Balanced.** Every batch moves value from debit to credit, so the sum of all balances is always zero. Test: `TestBatchBalancesToZero`.
- **L2 Atomic.** `Post` validates the whole batch first. One bad entry means nothing is written. Test: `TestPostIsAtomic`.
- **L3 Exact.** Amounts are `decimal.Decimal`. Never `float64`, never `int` cents. Test: `TestDecimalIsExact`.
- **L4 Append-only.** There is no update or delete, on purpose. To undo a batch, post a reversal: a new batch under a new primanota, with the same entries but debit and credit swapped, with a `Memo` that names the original primanota. `Reverse` does this. Test: `TestReversalUndoesBatch`.
- **L5 Monotonic primanota.** IDs are `PN-000001`, `PN-000002`, ... in posting order. Test: `TestPrimanotaIsMonotonic`.

## Gotchas

- `Balance` is debits minus credits (the asset view): cash you hold is positive, cash paid out is negative. Looks inverted if you think in bank-statement terms.
- One currency per batch. FX is a separate concern and not in this demo.
- Account IDs are plain strings by convention: `cash:<account>:<ccy>`, `sec:<account>:<instrument>`, `fees:house:<ccy>`. The ledger does not validate them; `booking` builds them.

## Done means

A new invariant gets a numbered line here and a test with the matching `// Invariant Lx:` comment.
