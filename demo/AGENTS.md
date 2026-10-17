# ledger-lite

A tiny double-entry booking service: trade events in, ledger entries out.
Go 1.26, no external services. `go test ./...` runs everything in seconds.

## Where the rules live

This file is deliberately short. Each package owns its invariants:

| Package              | Read before touching it         |
|----------------------|---------------------------------|
| `internal/ledger`    | `internal/ledger/AGENTS.md`     |
| `internal/events`    | `internal/events/AGENTS.md`     |
| `internal/booking`   | `internal/booking/AGENTS.md`    |

Open the AGENTS.md next to the code you change. Do not copy its content here.

## How to write a rule

Use Simplified Technical English. One rule per sentence. One meaning per word. Under 20 words.
Say what must never happen and which test proves it. Do not explain the architecture; the code does that.

## Definition of done

1. `go build ./... && go vet ./... && go test ./...` pass.
2. Every invariant you touched has a named test in its package (`// Invariant Xn:` comment).
3. `go run ./cmd/booking-service` output still balances.
4. Commit on a branch, open a PR with: what changed, which invariants it protects, which tests prove it.
