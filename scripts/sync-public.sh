#!/usr/bin/env bash
# sync-public.sh — Creates a filtered sync branch from cli/main to push to KeyHarbour/kh (public).
#
# Usage:
#   ./scripts/sync-public.sh              # branch name: sync/YYYY-MM-DD
#   ./scripts/sync-public.sh sync/v0.10.0 # custom branch name
#
# After running, open a PR on KeyHarbour/kh and squash merge it.

set -euo pipefail

BRANCH="${1:-sync/$(date +%Y-%m-%d)}"
PUBLIC_REMOTE="public"

# Files and directories stripped from the public repo.
# Add paths here to keep them private in kh-cli.
PRIVATE_PATHS=(
  "API_IMPLEMENTATION_PROMPT.md"
  "CLAUDE.md"
  "TODO.md"
  "ADR.md"
  "adr"
  "prd"
  "backend.hcl"
  "backend.tf"
  "data"
  "http-receiver"
  ".claude"
  ".env"
  ".env.local"
  ".env.sync"
  ".kh-migrate-backup"
  ".github/workflows/integration-pr.yml"
  ".github/workflows/integration-snapshot.yml"
  ".github/workflows/integration-regression.yml"
  ".github/workflows/integration-diagnostics.yml"
)

# ── Sanity checks ─────────────────────────────────────────────────────────────

if ! git remote get-url "$PUBLIC_REMOTE" &>/dev/null; then
  echo "Error: remote '$PUBLIC_REMOTE' not found."
  echo "Add it with: git remote add public https://github.com/KeyHarbour/kh.git"
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Error: working tree is not clean. Commit or stash your changes first."
  exit 1
fi

echo "Fetching $PUBLIC_REMOTE..."
git fetch "$PUBLIC_REMOTE" --quiet

# ── Create the sync branch from public/main ───────────────────────────────────

if git show-ref --quiet "refs/heads/$BRANCH"; then
  echo "Error: branch '$BRANCH' already exists. Delete it or choose a different name."
  exit 1
fi

echo "Creating branch '$BRANCH' from $PUBLIC_REMOTE/main..."
git checkout -b "$BRANCH" "$PUBLIC_REMOTE/main"

# ── Squash all new commits from cli/main onto the branch ─────────────────────

echo "Squashing changes from main..."
# Use -Xours on private paths to avoid conflicts on files we'll strip anyway.
# If merge --squash still exits with conflicts, resolve them by taking HEAD
# (i.e. the public side) for every private path, then continue.
git merge --squash main || {
  for path in "${PRIVATE_PATHS[@]}"; do
    git checkout HEAD -- "$path" 2>/dev/null || git rm -rf "$path" 2>/dev/null || true
  done
  # Resolve any remaining unmerged paths by removing them
  git diff --name-only --diff-filter=U | xargs -r git rm -f
}

# ── Stage everything, then strip private paths ────────────────────────────────

git add -A

stripped=()
for path in "${PRIVATE_PATHS[@]}"; do
  if git ls-files --cached --error-unmatch "$path" &>/dev/null 2>&1; then
    git rm -rf --cached "$path" &>/dev/null
    stripped+=("$path")
  fi
done

if [[ ${#stripped[@]} -gt 0 ]]; then
  echo "Stripped private paths:"
  for p in "${stripped[@]}"; do echo "  - $p"; done
fi

# ── Commit ────────────────────────────────────────────────────────────────────

VERSION=$(cat VERSION 2>/dev/null || echo "unknown")
git commit -m "chore: sync public release v${VERSION}"

# ── Push ──────────────────────────────────────────────────────────────────────

echo "Pushing '$BRANCH' to $PUBLIC_REMOTE..."
git push "$PUBLIC_REMOTE" "$BRANCH"

# ── Restore original branch ───────────────────────────────────────────────────

git checkout main

echo ""
echo "Done. Open a PR at:"
echo "  https://github.com/KeyHarbour/kh/compare/$BRANCH"
echo ""
echo "When merging, choose 'Squash and merge' and set the commit message to a"
echo "clean conventional commit (e.g. 'feat(cli): ...')."
