Two things, in one PR. You are already on a fresh branch; do not create another.

1. Bug: trade corrections are dropped. `go run ./cmd/booking-service` publishes
   evt-003, a correction for trade T-1, and it never appears in the book.
   Reproduce it with a test, then fix it.

2. Feature: support a `TradeCancelled` event (`{"trade_id": "..."}`). Cancelling
   a trade must undo its booking in the ledger. Add the event type, handle it,
   test it, and make `cmd/booking-service` publish one cancellation so the
   output shows it.

Follow the AGENTS.md next to each package you change. When done, run the full
definition of done from the root AGENTS.md, commit, push, and open a PR with
`gh pr create`. The PR body must list the invariants you protected and the
tests that prove them.
