# Invariants Are the New Interface

Lightning talk, GopherCon Africa 2026, Nairobi, 17 October, 13:20, 10 minutes.
Everything here is open source: the slides, the demo service, and the scripts that run the live part.

| Folder     | What it is                                                                 |
|------------|----------------------------------------------------------------------------|
| `slides/`  | Terminal slide deck for [slides](https://github.com/maaslalani/slides) (Go). Markdown in, TUI out. |
| `demo/`    | `ledger-lite`: a small Go booking service with per-package `AGENTS.md` invariant files. |
| `scripts/` | `talk.sh` (tmux: slides + agent), `live-agent.sh`, `preflight.sh`, `reset.sh`. |

## Run the talk

```
brew install slides tmux        # or: go install github.com/maaslalani/slides@latest
./scripts/preflight.sh          # morning of: everything must say ok
./scripts/talk.sh               # tmux session: window 1 slides, window 2 agent (Enter to start)
```

In `slides`: `→`/`space` next, `←` back, `ctrl+e` runs the code block on the current slide, `q` quits.
The code blocks on slides 7 and 9 run `go test`, `go run`, and `gh pr list` against `demo/`.

## The live part

At 0:00, press Enter in the agent window. It creates a `live/…` branch, reads `demo/TASK.md`
(one planted bug, one feature), reads the `AGENTS.md` next to each package it touches,
commits, pushes, and opens a PR. At 8:30 you switch to it and show the PR.

After each rehearsal: `./scripts/reset.sh` closes the PR, deletes the branch, and restores the planted bug.
