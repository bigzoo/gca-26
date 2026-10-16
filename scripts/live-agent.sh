#!/usr/bin/env bash
# Start the live agent at 0:00 of the talk. Run this in a second terminal/tmux window.
# It creates a fresh branch and hands TASK.md to the agent. The agent commits, pushes and opens the PR.
set -euo pipefail
cd "$(dirname "$0")/../demo"
AGENT="${AGENT:-claude}"            # AGENT=codex ./scripts/live-agent.sh
BRANCH="live/gca-26-$(date +%H%M%S)"
git switch -q main
git switch -q -c "$BRANCH"
echo "branch: $BRANCH"
case "$AGENT" in
  claude) exec claude --permission-mode acceptEdits "$(cat TASK.md)" ;;
  codex)  exec codex --full-auto "$(cat TASK.md)" ;;
  *) echo "unknown AGENT=$AGENT" >&2; exit 1 ;;
esac
