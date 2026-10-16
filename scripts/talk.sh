#!/usr/bin/env bash
# One command before you walk on stage: a tmux session with the slides in window 1
# and the agent command typed but NOT started in window 2.
#   prefix-2 (ctrl-b 2) → press Enter → agent starts → prefix-1 → present.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
S=gca26
tmux kill-session -t "$S" 2>/dev/null || true
tmux new-session -d -s "$S" -n slides -c "$ROOT" "$ROOT/scripts/present.sh"
tmux new-window -t "$S" -n agent -c "$ROOT"
tmux send-keys -t "$S:agent" "$ROOT/scripts/live-agent.sh"
tmux select-window -t "$S:slides"
exec tmux attach -t "$S"
