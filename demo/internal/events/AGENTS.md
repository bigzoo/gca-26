# internal/events — invariants

An in-memory stand-in for Kafka. It is small so the delivery semantics are obvious; the semantics are the real ones.

## Invariants

- **E1 At-least-once.** A handler that returns an error sees the same event again. Test: `TestRetriesOnError`.
- **E2 Bounded retries.** After `MaxRetries` the bus returns `ErrGiveUp` and stops. No infinite loop on a poison pill. Test: `TestGivesUpAfterMaxRetries`.
- **E3 Ordered within a topic.** Delivery order equals publish order. Test: `TestOrderIsPreserved`.
- **E4 Replay is normal.** `Replay` delivers everything again from offset 0. That is what a consumer-group rebalance looks like to a handler. Handlers must survive it.

## What this means for a consumer

- Your handler must be idempotent on `Event.ID`. `Event.Key` is a partition key, not a uniqueness key.
- Return an error to get a retry. Do not swallow errors, do not panic, do not sleep.
- Never "ack" (mark as processed) before the side effect is durable. Here that means: mark after `Post` returns nil.

## Done means

Behaviour changes here need a test in this package and a note in the consumer that relies on it (`internal/booking/AGENTS.md`).
