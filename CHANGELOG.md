## v0.8.8 (2026-02-24)

### Maintenance
* remove Homebrew formula update from private repo (d1660b8)

---

## v0.8.7 (2026-02-24)

### Maintenance
* update .gitignore to cover backup files and local binaries (f11e4a0)

---

## v0.8.6 (2026-02-24)

### Bug Fixes
* untrack .env.local (80fbaf0)

---

## v0.8.5 (2026-02-24)

---

## v0.8.4 (2026-02-24)

### Maintenance
* add Homebrew tap automation via GoReleaser (0f97d06)

---

## v0.8.3 (2026-02-24)

### Bug Fixes
* run release in same workflow run as bump_version (2e61dba)

---

## v0.8.2 (2026-02-24)

### Bug Fixes
* format migrate_test.go (6d77478)
* harden go-ci workflow for reliable bump-tag-release (18aa630)

---

## v0.8.1 (2026-02-24)

### Bug Fixes
* use GH_PAT in bump_version checkout to trigger release on tag push (af24e37)

---

## v0.8.0 (2026-02-24)

### Features
* add auto version bump and changelog generation on merge to main (dfa3e9d)

### Bug Fixes
* don't skip build on tag pushes, only on branch bump commits (a7ad7d1)
* derive version range from last git tag, not VERSION file (98aa892)
* skip commit and tag when nothing to bump (c3bb9a3)
* add GH_PAT token to checkout to allow workflow triggers (baab642)
* harden release pipeline security and fix goreleaser config (5ba5bfa)

### Maintenance
* bump version to 0.8.0 [skip ci] (54d04e4)
* fix GoReleaser v2 deprecation warnings (a4b07b0)
* add macOS code signing and notarization to release pipeline (4dc6841)

---

## v0.8.0 (2026-02-24)

### Features
* add auto version bump and changelog generation on merge to main (dfa3e9d)

### Bug Fixes
* derive version range from last git tag, not VERSION file (98aa892)
* skip commit and tag when nothing to bump (c3bb9a3)
* add GH_PAT token to checkout to allow workflow triggers (baab642)
* harden release pipeline security and fix goreleaser config (5ba5bfa)

### Maintenance
* fix GoReleaser v2 deprecation warnings (a4b07b0)
* add macOS code signing and notarization to release pipeline (4dc6841)

---

# Changelog

All notable changes to the `kh` CLI are documented here.
Versions follow [Semantic Versioning](https://semver.org) and commits follow [Conventional Commits](https://www.conventionalcommits.org).

---