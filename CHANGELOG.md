## v1.4.0 (2026-04-04)

### Features
* add licence instances, licensees, and team-members management (31d2eed)

### Bug Fixes
* fix /license/ URL paths and add unit_cost field (31d2eed)

### Maintenance
* add unit tests for instances, licensees, team-members (6827fd8)
* update openapi.yaml with new licence endpoints and unit_cost (1d8f274)

---

## v1.3.0 (2026-04-03)

### Features
* upsert support in kv update command (4044b5e)
* add KH_INSECURE env var to skip TLS certificate verification (aa3b91e)

### Bug Fixes
* respect KH_WORKSPACE and KH_OUTPUT env vars consistently (bcaea59)
* always apply env var overrides even when config file fails to load (daccb5d)

### Maintenance
* use singular names for project, workspace, version commands (1ef27cb)
* move lock, unlock, verify under kh tf state (54e931f)
* reorganize top-level commands into auth and tf groups (0281bdf)

### Documentation
* update README for new command structure (f66bc8a)

---

## v1.2.0 (2026-04-02)

### Features
* add --value-file flag to kv set and update commands (cfd47bf)

---

## v1.1.0 (2026-04-02)

### Features
* add workspace create, update, and delete commands (9a4b01d)
* add client-side AES-256-GCM encryption for kv values via `--encryption-key-file` (c599f6f)
* add `kh license` command for software license management (c3f91cf)
* normalize `KH_ENDPOINT` — no need to append `/api/v2` manually (3904693)
* add `--gen-backend` flag to `kh sync` — generates `kh_backend.tf.sample` after migration (a62ed0f)

### Bug Fixes
* use human-readable table output by default; JSON only with `-o json` (173a972)
* replace `--encryption-key` flag with `--encryption-key-file` to avoid shell history exposure (0636b00)

### Maintenance
* add integration test workflow for pull requests (b42cc1a)
* remove export and import commands (147f0d5)

---

## v0.9.0 (2026-03-01)

### Features
* migrate to V2 API and remove migrate command (6325add)
* support KH_ENVIRONMENT env var for --env flag (405b38e)
* add kv commands to manage workspace key/value pairs (4f1327e)

### Documentation
* add KH_ENVIRONMENT to kv notes and env vars table (11ad189)
* add kv commands and KH_WORKSPACE to README (9349c11)

---

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