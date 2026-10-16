# ledger-lite — demo for "Invariants Are the New Interface"

A deliberately small Go service used in a GopherCon Africa 2026 lightning talk.
Trade events go in, double-entry ledger entries come out.

```
go test ./...                 # all invariants, ~2s
go run ./cmd/booking-service  # prints the book and balances
```

## The point

The root `AGENTS.md` is 20 lines. The rules live next to the code:

```
AGENTS.md                      what the repo is, where the rules are, what "done" means
internal/ledger/AGENTS.md      L1–L5: balanced, atomic, exact, append-only, monotonic
internal/events/AGENTS.md      E1–E4: at-least-once, bounded retries, ordered, replayable
internal/booking/AGENTS.md     B1–B5: one batch per event, idempotent on Event.ID, ...
```

`CLAUDE.md` in each directory is a symlink to `AGENTS.md`, so Claude Code and
Codex read the same file. Claude Code loads a nested `CLAUDE.md` only when it
works on files under that directory, so the agent learns a package's rules on
demand instead of carrying every rule in every session.

Every invariant names the test that enforces it. The agent's job is to keep
that list true, and the tests are what the reviewer trusts.

## The live demo

`TASK.md` is the prompt given to the agent at the start of the talk. It
contains one planted bug (B3: dedup keyed on `TradeID`) and one feature
(`TradeCancelled`, which forces a reversal and therefore a reading of L4).

See `../scripts/` for the exact commands.
