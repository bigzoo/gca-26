#!/usr/bin/env bash
# After a rehearsal: back to main, delete local live/* branches, close their PRs. The planted bug comes back.
set -euo pipefail
cd "$(dirname "$0")/.."
git switch -q main
for b in $(git branch --list 'live/*' --format='%(refname:short)'); do
  gh pr close "$b" --delete-branch >/dev/null 2>&1 || true
  git branch -D "$b" >/dev/null 2>&1 || true
  echo "removed $b"
done
git status --short
