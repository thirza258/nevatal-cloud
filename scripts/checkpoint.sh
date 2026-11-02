#!/usr/bin/env bash
# Checkpoint all uncommitted work to origin/main.
#
# Used when the Claude usage limit is hit, so in-progress work is never lost
# (user-authorized policy, see CONTEXT.md section 0). Safe to run by hand:
#
#   scripts/checkpoint.sh "domains: edge tests in progress"
#   NO_CHECKS=1 scripts/checkpoint.sh "note"     # fastest: skip build checks
#
# What it does:
#   1. Stages everything (git add -A; .gitignore keeps .env, dist/, bins out).
#   2. Refuses to continue if anything secret-looking is staged.
#   3. Runs quick checks (go build, tsc) unless NO_CHECKS=1 and records the
#      result in the commit message. It never blocks on failures: the goal is
#      to save work, so the commit may be WIP.
#   4. Commits on main and pushes to origin/main. Never force-pushes. If the
#      push is rejected (main moved on the remote), it pushes the same commit
#      to a new branch checkpoint/<timestamp> instead and tells you.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NOTE="${1:-work in progress}"

die() { printf '[checkpoint] error: %s\n' "$*" >&2; exit 1; }
log() { printf '[checkpoint] %s\n' "$*"; }

branch="$(git rev-parse --abbrev-ref HEAD)"
[ "$branch" = "main" ] || die "current branch is '$branch'; this script only checkpoints main"

git add -A

# Never publish secrets, even if a .gitignore rule is missing.
leaks="$(git diff --cached --name-only | grep -E '(^|/)\.env($|\.)|\.env$|secrets\.env$|tunnel\.env$|\.(pem|key|crt|p12|pfx)$|id_(rsa|ed25519)$' | grep -v '\.example$' || true)"
if [ -n "$leaks" ]; then
  git reset -q
  die "refusing to commit secret-looking files (unstaged everything):
$leaks"
fi

checks="checks skipped (NO_CHECKS=1)"
if [ "${NO_CHECKS:-0}" != "1" ]; then
  backend="ok"
  frontend="ok"
  (cd backend && go build ./... >/dev/null 2>&1) || backend="FAILING"
  if [ -d frontend/node_modules ]; then
    (cd frontend && npx tsc -p tsconfig.json --noEmit >/dev/null 2>&1) || frontend="FAILING"
  else
    frontend="not checked (no node_modules)"
  fi
  checks="backend build: $backend; frontend typecheck: $frontend"
fi
log "$checks"

if git diff --cached --quiet; then
  log "nothing new to commit"
else
  git commit -q -F - <<EOF
chore(checkpoint): $NOTE

Automatic checkpoint (usage limit). May contain work in progress.
$checks. See CONTEXT.md for current state and next steps.

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
EOF
  log "committed $(git rev-parse --short HEAD)"
fi

if git push origin main; then
  log "pushed to origin/main"
else
  fallback="checkpoint/$(date +%Y%m%d-%H%M%S)"
  log "push to main rejected (remote moved); pushing to $fallback instead"
  git push origin "HEAD:refs/heads/$fallback"
  log "pushed to origin/$fallback - merge it into main later"
fi
