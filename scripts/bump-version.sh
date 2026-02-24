#!/usr/bin/env bash
# bump-version.sh — Auto-increments VERSION and updates CHANGELOG.md based on
# conventional commit messages since the last version tag.
#
# Bump rules (SemVer):
#   BREAKING CHANGE or type!:  → major
#   feat:                      → minor
#   anything else              → patch
#
# Skips (exits 0) if there are no new commits since the last tag.

set -euo pipefail

VERSION_FILE="VERSION"
CHANGELOG_FILE="CHANGELOG.md"

# Fetch all tags so git log range works on a fresh CI checkout
git fetch --tags --force 2>/dev/null || true

# Use the latest git tag as the source of truth for the current version.
# This is more reliable than the VERSION file because the tag always exists
# in git history, whereas VERSION might be ahead/behind the actual tags.
last_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

if [ -n "$last_tag" ]; then
  # Strip leading 'v' to get a plain semver string (e.g. v0.7.1 → 0.7.1)
  current_version="${last_tag#v}"
  range="${last_tag}..HEAD"
else
  # No tags yet — fall back to VERSION file; scan all commits
  current_version=$(cat "$VERSION_FILE" 2>/dev/null || echo "0.1.0")
  range="HEAD"
fi

IFS='.' read -r major minor patch <<< "$current_version"

echo "Last tag   : ${last_tag:-"(none)"}"
echo "Current    : v${current_version}"
echo "Scanning   : ${range}"

mapfile -t raw_commits < <(git log "$range" --pretty=format:"%s (%h)" 2>/dev/null || true)

if [ ${#raw_commits[@]} -eq 0 ]; then
  echo "✨ No new commits since v${current_version} — nothing to bump."
  exit 0
fi

# ── Determine bump type ───────────────────────────────────────────────────────
bump_type="patch"
for commit in "${raw_commits[@]}"; do
  if echo "$commit" | grep -qE "(BREAKING CHANGE|^[a-z]+(\(.*\))?!:)"; then
    bump_type="major"
    break
  elif echo "$commit" | grep -qE "^feat(\(.*\))?:"; then
    bump_type="minor"
  fi
done

# ── Calculate new version ─────────────────────────────────────────────────────
case "$bump_type" in
  major) major=$((major + 1)); minor=0; patch=0 ;;
  minor) minor=$((minor + 1)); patch=0 ;;
  patch) patch=$((patch + 1)) ;;
esac
new_version="${major}.${minor}.${patch}"

# ── Sort commits into groups ──────────────────────────────────────────────────
features=()
fixes=()
maintenance=()
docs=()

for commit in "${raw_commits[@]}"; do
  if echo "$commit" | grep -qE "^feat(\(.*\))?:"; then
    features+=("$commit")
  elif echo "$commit" | grep -qE "^fix(\(.*\))?:"; then
    fixes+=("$commit")
  elif echo "$commit" | grep -qE "^(chore|refactor|ci|build)(\(.*\))?:"; then
    maintenance+=("$commit")
  elif echo "$commit" | grep -qE "^docs(\(.*\))?:"; then
    docs+=("$commit")
  fi
done

# ── Build Markdown entry ──────────────────────────────────────────────────────
date_str=$(date '+%Y-%m-%d')
new_content="## v${new_version} (${date_str})\n\n"

append_group() {
  local title="$1"; shift
  local items=("$@")
  if [ ${#items[@]} -gt 0 ]; then
    new_content+="### ${title}\n"
    for item in "${items[@]}"; do
      # Strip conventional commit prefix (e.g. "feat(cli): " or "fix: ")
      clean=$(echo "$item" | sed -E 's/^[a-z]+(\([^)]*\))?!?: //')
      new_content+="* ${clean}\n"
    done
    new_content+="\n"
  fi
}

# Only expand arrays when they have elements (avoids unbound variable errors)
append_group "Features"      ${features[@]+"${features[@]}"}
append_group "Bug Fixes"     ${fixes[@]+"${fixes[@]}"}
append_group "Maintenance"   ${maintenance[@]+"${maintenance[@]}"}
append_group "Documentation" ${docs[@]+"${docs[@]}"}

# ── Write files ───────────────────────────────────────────────────────────────
echo "$new_version" > "$VERSION_FILE"

existing_content=$(cat "$CHANGELOG_FILE" 2>/dev/null || echo "")
printf "%b---\n\n%s" "$new_content" "$existing_content" > "$CHANGELOG_FILE"

echo "⬆️  Version: ${current_version} → ${new_version} (${bump_type^^})"
echo "✅ Changelog written to ${CHANGELOG_FILE}"
