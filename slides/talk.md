---
theme: ./theme.json
author: Chris Nyaga · @bigzoo
date: GopherCon Africa 2026 · Nairobi · 17 Oct
paging: "%d / %d"
---

### NAIROBI · 15–17 OCTOBER 2026 · LIGHTNING TALK

# Invariants Are the New Interface

Agents make code cheap.
The scarce asset is a team that has written down what *correct* means.

```
Chris Nyaga  ·  Go Nairobi  →  Go Berlin
Picked up Go at GitHub (a Ruby + Go team). Never looked back.
Since then: Go microservices at Kappa Pay and Upvest.
```

> 0:00 — the agent is already working. We'll check on it at the end.

---

### 1 · THE SETUP

# A fleet of Go services on Kafka

```
 trades ──▶ [booking-service] ──▶ LedgerEntryCreated ──▶ [balance-service]
               │  double-entry                            │  aggregations
               ▼                                          ▼
            PostgreSQL                                 PostgreSQL
               ▲
 [adjustments-service] ◀── rounding events       [reconciliation-classifier]
```

- Brokerage infrastructure. Money. Double-entry bookkeeping.
- Then the agents arrived. Everyone tried them, in Claude Code and Codex.
- Scaffolding, schema plumbing, cross-service renames: **fast**.

---

### 2 · WHAT WENT WRONG

# The agent is confident. The system has rules.

```go
// agent-written consumer. compiles. tests pass. looks right.
if _, done := s.seen[t.TradeID]; done {   // dedup on the *trade*
    return nil
}
pn, err := s.book.Post(entries...)
s.seen[t.TradeID] = pn
```

- A trade **correction** re-emits the same `TradeID` with a new event ID.
- This "safety net" silently drops it. Books are wrong. Nobody notices for days.
- Every service has five of these: idempotency, ordering, retries, ack timing, what "done" means.
- The fix is not a better prompt.

---

### 3 · WHAT WE DID FIRST (AND WHAT EVERYONE DOES)

# Stuff it all into one root file

```
ledger/CLAUDE.md                     1.3 KB   "see docs/claude/*"
ledger/docs/claude/architecture.md  10.5 KB   every service, every RPC, every handler
ledger/docs/claude/patterns.md       0.7 KB   "consumers must handle duplicates"
                                    ──────
                                    ~16 KB loaded, every session, every task
```

- Agents do not traverse a repo like we do. They read what is in context, then grep.
- A root dump costs tokens on every turn **and** pollutes the task with the other 4 services.
- The one line that mattered ("handle duplicates") was 12 words in 16 KB.

---

### 4 · THE PROPOSAL

# Distribute the docs. Load on demand.

```
AGENTS.md                     20 lines: what, where the rules are, what "done" means
internal/ledger/AGENTS.md     L1–L5  balanced · atomic · exact · append-only · monotonic
internal/events/AGENTS.md     E1–E4  at-least-once · bounded retries · ordered · replayable
internal/booking/AGENTS.md    B1–B5  one batch per event · idempotent on Event.ID · ...
```

- `CLAUDE.md -> AGENTS.md` symlink in each dir. One file, both tools.
- Claude Code loads a nested `CLAUDE.md` **only** when it touches files under it.
- The agent learns a package's rules when it enters the package, like a new hire would.
- Each invariant **names the test that enforces it**. That is the contract.

---

### 5 · THE ARTIFACT

# What a per-package invariants file looks like

```markdown
# internal/booking — invariants

- B2 Idempotent on Event.ID. Retries, rebalances and replays redeliver
  events. The same event must never book twice.   Test: TestReplayIsIdempotent
- B3 Corrections book again. A correction re-emits the SAME TradeID with a
  NEW Event.ID. Dedup on TradeID is a bug, not a safety net.
                                                   Test: add TestCorrectionBooksAgain
- B4 Mark after post. `seen` is written only after book.Post returns nil.
                                                   Test: TestBadSideIsRejected

## Done means
A new invariant gets a numbered line here and a test with a matching comment.
```

Invariants, not architecture. Failure contracts, not file trees.
Written in **Simplified Technical English**: one rule per sentence, one meaning per word, under 20 words.

---

### 6 · PROOF, NOT PROSE

# The tests are the part a reviewer trusts

```bash
cd ../demo && go test ./... 2>&1 | tail -4
```

```bash
cd ../demo && go run ./cmd/booking-service | sed -n '3,9p'
```

> evt-003 is a correction for T-1. It is missing from the book.
> That is the planted bug the agent was asked to fix at 0:00.

---

### 7 · THREE RULES

# If you only remember three things

1. **Write down invariants, not architecture. Write them in STE.**
   The code already says what it is. Only you know what it must never do.
   Simplified Technical English keeps a rule to one short sentence. Agents that write these files overspecify; a style rule stops them.

2. **Put the verification where the agent will run it.**
   Every invariant names a test. `go test ./...` is the review.

3. **Keep the guide per package, not per repo.**
   Nested `CLAUDE.md` / `AGENTS.md` load on demand. Less context, less pollution, less back-and-forth.

---

### 8 · THE PR

# What the agent did while I was talking

```bash
cd ../demo && gh pr list --state all --limit 1 --json title,url,additions,deletions \
  --jq '.[] | "\(.title)\n\(.url)\n+\(.additions) -\(.deletions)"'
```

```bash
cd ../demo && git fetch -q origin && git diff --stat main...origin/$(gh pr list --limit 1 --json headRefName --jq '.[0].headRefName') | tail -8
```

- Did it dedup on `Event.ID`?  Did it add `TestCorrectionBooksAgain`?
- Did it implement cancellation as a **reversal batch**, not a delete? (L4)
- I did not read the diff yet. I read the invariants list in the PR body.

---

### 9 · CLOSE

# Generation is cheap now

The scarce thing is a team that has written down what *correct* means,
next to the code, with a test for each line.

```
Slides + demo (open source):  github.com/bigzoo/gca-26
Template:                      demo/internal/booking/AGENTS.md
```

**Chris Nyaga** · Go Nairobi · Go Berlin · @bigzoo

Asante sana. Questions?
