#!/usr/bin/env bash
# Run this the morning of the talk. Every line must say ok.
set -u
cd "$(dirname "$0")/.."
ok() { printf '  ok   %s\n' "$1"; }
bad() { printf '  FAIL %s\n' "$1"; FAILED=1; }
FAILED=0
command -v slides >/dev/null && ok "slides installed" || bad "slides missing: brew install slides"
command -v tmux   >/dev/null && ok "tmux installed"   || bad "tmux missing: brew install tmux"
command -v claude >/dev/null && ok "claude installed" || bad "claude missing"
gh auth status >/dev/null 2>&1 && ok "gh authenticated" || bad "gh not logged in: gh auth login"
ROOT="$(pwd)"; [ "$(jq -r --arg p "$ROOT" '.projects[$p].hasTrustDialogAccepted // false' ~/.claude.json)" = true ] && ok "repo trusted by Claude Code (allowlist active)" || bad "repo not trusted: run 'claude' in demo/ once and accept the trust dialog"
git -C demo remote get-url origin >/dev/null 2>&1 && ok "origin remote set" || bad "no origin remote: gh repo create"
[ "$(git branch --show-current)" = main ] && ok "on main" || bad "not on main"
git diff --quiet && git diff --cached --quiet && ok "working tree clean" || bad "uncommitted changes"
(cd demo && go test ./... >/dev/null 2>&1) && ok "go test ./... green" || bad "tests failing"
(cd demo && go run ./cmd/booking-service | grep -q 'buy T-2') && ok "demo runs" || bad "demo does not run"
grep -q 'TradeID\]; done' demo/internal/booking/booking.go && ok "planted bug still in place (B3)" || bad "planted bug missing: git checkout main -- demo/internal/booking/booking.go"
[ $FAILED -eq 0 ] && echo "preflight: all good" || { echo "preflight: fix the FAIL lines"; exit 1; }
