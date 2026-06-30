#!/usr/bin/env bash
# sync-public.sh — Creates a filtered sync branch from cli/main to push to KeyHarbour/kh (public)
#                  and opens a PR automatically via the GitHub CLI (gh).
#
# Usage:
#   ./scripts/sync-public.sh              # branch name: sync/YYYY-MM-DD
#   ./scripts/sync-public.sh sync/v0.10.0 # custom branch name
#
# Prerequisites: gh CLI authenticated (gh auth login)

set -euo pipefail

BRANCH="${1:-sync/$(date +%Y-%m-%d)}"
PUBLIC_REMOTE="public"
PUBLIC_REPO="KeyHarbour/kh"

# Files and directories stripped from the public repo.
# Add paths here to keep them private in kh-cli.
PRIVATE_PATHS=(
  "API_IMPLEMENTATION_PROMPT.md"
  "CLAUDE.md"
  "RELEASING.md"
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

if ! command -v gh &>/dev/null; then
  echo "Error: 'gh' CLI not found. Install it with: brew install gh"
  exit 1
fi

if ! gh auth status &>/dev/null; then
  echo "Error: not authenticated with gh. Run: gh auth login"
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
  # Resolve remaining conflicts by taking the cli/main version (--theirs)
  git diff --name-only --diff-filter=U | while IFS= read -r path; do
    git checkout --theirs -- "$path" 2>/dev/null && git add "$path" || git rm -f "$path"
  done
}

# ── Stage everything, then strip private paths ────────────────────────────────

git add -A

# ── Re-apply any file that main has but the squash silently dropped ───────────
# git merge --squash can auto-resolve delete/modify conflicts by keeping the
# deletion without flagging the file as unmerged (status U).  Walk every file
# that exists in main, skip private paths, and restore it if it is absent from
# the working tree after the squash.
while IFS= read -r tracked_file; do
  # Skip if this file matches a private path prefix
  is_private=false
  for priv in "${PRIVATE_PATHS[@]}"; do
    if [[ "$tracked_file" == "$priv" || "$tracked_file" == "$priv/"* ]]; then
      is_private=true
      break
    fi
  done
  [[ "$is_private" == true ]] && continue

  # If the file is missing from the working tree, restore it from main
  if [[ ! -e "$tracked_file" ]]; then
    git checkout main -- "$tracked_file" 2>/dev/null && \
      echo "  restored (silently dropped by squash): $tracked_file"
  fi
done < <(git ls-tree -r --name-only main)

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

# ── Safety check: abort if the synced code does not compile cleanly ───────────
# Squash merges can produce duplicate imports when public/main and private/main
# have different gofmt-sorted import blocks. Catch this before committing.
if ! go vet ./... 2>/dev/null; then
  echo ""
  echo "Error: 'go vet' failed on the synced branch. Fix the issues above and re-run."
  git checkout main
  exit 1
fi

# ── Commit ────────────────────────────────────────────────────────────────────

VERSION=$(cat VERSION 2>/dev/null || echo "unknown")
git commit -m "chore: sync public release v${VERSION}"

# ── Push ──────────────────────────────────────────────────────────────────────

echo "Pushing '$BRANCH' to $PUBLIC_REMOTE..."
git push "$PUBLIC_REMOTE" "$BRANCH"

# ── Restore original branch ───────────────────────────────────────────────────

git checkout main

# ── Open PR on KeyHarbour/kh ─────────────────────────────────────────────────

VERSION=$(cat VERSION 2>/dev/null || echo "unknown")

echo ""
echo "Opening PR on ${PUBLIC_REPO}..."
gh pr create \
  --repo "$PUBLIC_REPO" \
  --head "$BRANCH" \
  --base main \
  --title "chore: sync public release v${VERSION}" \
  --body "Automated sync from private CLI repo.

**Version:** v${VERSION}
**Merge strategy:** Squash and merge
**Suggested commit message:** \`feat(cli): <short description of what's new>\`"

echo ""
echo "PR opened. Review it, update the squash commit message, then merge."
echo "After merge, the auto-tag workflow will create v${VERSION} and trigger GoReleaser."
