# internal/booking — invariants

Consumes `trades` events and posts batches to the ledger. This package is where agents most often "fix" something into a money bug. Read all of it.

## Invariants

- **B1 One batch per trade event.** A BUY debits `sec:` and `fees:`, credits `cash:`. A SELL is the mirror for principal; the fee always leaves cash. Test: `TestBuyBooksPrincipalAndFee`.
- **B2 Idempotent on `Event.ID`.** Retries, rebalances and replays redeliver events. The same event must never book twice. Test: `TestReplayIsIdempotent`.
- **B3 Corrections book again.** A trade correction re-emits the **same `TradeID` with a new `Event.ID`**. It is a new event and must produce a new batch. Dedup on `TradeID` is therefore a bug, not a safety net. Test: add `TestCorrectionBooksAgain` when you touch dedup.
- **B4 Mark after post.** The `seen` map is written only after `book.Post` returns nil. A failure must leave the event unseen so the bus retries it. Test: `TestBadSideIsRejected`.
- **B5 Unknown event types are skipped, not failed.** A new upstream type must not stall the consumer.

## Adding an event type

1. Add a `Type...` constant and a payload struct with `json` tags.
2. Add a `case` in `Handle` and a `handleX` method that follows B2 and B4.
3. If it changes the book, express it as a new batch (see `internal/ledger/AGENTS.md`, L4). Never mutate past entries.
4. Add the invariant line here and the named test.

## Gotchas

- `Processed(key)` is keyed the same way as `seen`. If you change the key, change both and the tests.
- Account naming: `cash:<account>:<ccy>`, `sec:<account>:<instrument>`, `fees:house:<ccy>`.
- `cmd/booking-service` publishes a correction for `T-1` (`evt-003`). If it does not show up in the printed book, B3 is broken.
