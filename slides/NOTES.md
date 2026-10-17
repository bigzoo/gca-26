# Speaker notes and timing (10:00)

Keep the agent window ready before you start. Press Enter at 0:00, switch to slides.

| Time | Slide | Say |
|------|-------|-----|
| 0:00 | 1 Title | "Hi, I'm Chris. Go Nairobi, more recently Go Berlin. I picked up Go at GitHub on a Ruby + Go team and never looked back. Since then, Go microservices at Kappa Pay and Upvest." Then: "An agent started working on a repo 20 seconds ago. We'll look at what it did at the end." |
| 1:00 | 2 Setup | Brokerage infra: many small Go services, Kafka between them, double-entry ledger. Agents arrived, everyone tried Claude Code and Codex. Scaffolding and renames went fast. |
| 2:00 | 3 What went wrong | The code on screen compiles and passes tests. Dedup on TradeID drops corrections. Real class of bug: idempotency, ordering, ack timing. "The fix is not a better prompt." |
| 3:15 | 4 Root dump | What we did first: one CLAUDE.md that points at 16 KB of docs. Agents read context then grep. Every task pays for the other four services. The one line that mattered was 12 words. |
| 4:30 | 5 Proposal | Root file: 20 lines. Rules next to the code. Nested CLAUDE.md loads only when the agent touches that directory. Each invariant names its test. |
| 5:30 | 6 Artifact | Pause here. "This is the slide to photograph." Point at B3 and B4. Invariants, not architecture. |
| 6:30 | 7 Proof | ctrl+e on both blocks. Tests green. Run output: evt-003 is missing. "That is the bug the agent got at 0:00." |
| 7:30 | 8 Three rules | Read them. One sentence each. On rule 1 add: "and write them in Simplified Technical English. Agents love to overspecify. Mine did, until the root file told it not to." |
| 8:30 | 9 The PR | ctrl+e: PR title, URL, diff stat. Check three things out loud: Event.ID dedup, TestCorrectionBooksAgain, reversal not delete. "I read the invariants list in the PR body, not the diff." |
| 9:30 | 10 Close | Generation is cheap; written-down correctness is scarce. Repo link. Asante sana. |

## If the agent is not done at 8:30

- Say so. "It is still running, which is also a data point." Show the branch: `git log --oneline main..` in the agent window.
- Fallback: the rehearsal PR from the night before. Keep it open on a second tab in the browser.

## If the wifi dies

- The agent needs the API. Tether to your phone before you start; `preflight.sh` checks `gh auth`, not connectivity.
- Slides 1–8 need nothing. Slide 9 needs the network for `gh`; the diff stat block works offline against the local branch.

## Rehearse tonight

1. `./scripts/preflight.sh`
2. `./scripts/talk.sh`, do the whole thing with a timer.
3. Leave the resulting PR open as the fallback. Then `./scripts/reset.sh` **only on the morning after you have a second one**, or keep that PR and start the live one on a new branch (the script always creates a new branch, so both can exist).

## Verified on 16 Oct

- Three dry runs with `claude -p` (Sonnet 5.5) in a throwaway worktree. The third one, with the allowlist and repo trust in place, finished in 103 s: Event.ID dedup, `TestCorrectionBooksAgain`, `Book.Reverse` with reversal batches, AGENTS.md updated, tests green.
- First two runs stalled on permission prompts. That is why `demo/.claude/settings.json` has an allowlist and why `preflight.sh` checks that the repo is trusted.
- Codex is not installed on this machine. `AGENT=codex ./scripts/live-agent.sh` is wired but untested. Rehearse with Claude unless you install and test Codex first.
