## v1.11.1 (2026-04-14)

### Bug Fixes
* detect final commit in bump script (823c144)
* rename multipart field from value-file to value_file (e96511b)

---

## v1.11.0 (2026-04-14)

### Features
* improve license show/licensee UX and consolidate team-member (b1c5372)
* add missing license management functionality (5c4df22)

### Bug Fixes
* rename multipart field from valuefile to value-file (2c575d8)
* resolve lint violations (errcheck, unparam, unused, ineffassign) (c9e0249)
* support value vs valuefile kv payloads (905ca70)

### Maintenance
* upgrade golangci-lint-action to v6 with goinstall mode (6b8c88b)
* upgrade govulncheck and gosec for Go 1.26 compatibility (1d66940)
* use Go 1.24 for lint job to match golangci-lint binary (17a4dbd)
* use golangci-lint action v4 for better Go 1.26 compatibility (638b30d)
* rebuild golangci-lint to match Go 1.26 version (f74cf13)
* upgrade Go from 1.22 to 1.26 to fix stdlib vulnerabilities (8df5ae6)
* fix golangci config and security workflow compatibility (cdc630a)
* add coverage summary, lint, and security jobs to Go CI (4887ec2)

### Documentation
* update v1.10.2 changelog with full release summary (7c2c955)

---

## Unreleased

### Improvements

* `license show`, `license instance show`, `license licensee show`, and
  `license team-member show` now accept `-o table|json`, consistent with all
  other show commands in the CLI.
* `license users` subcommand removed; functionality merged into
  `license team-member` (which now includes `import`).
* `license licensee ls` table expanded with NAME and EMAIL columns (dash when
  the API does not return them). `Licensee` type gains `name` and `email`
  fields.

---

## v1.10.2 (2026-04-14)

### Summary
This release consolidates all CLI changes shipped after v1.9.1, including
error handling improvements, command UX cleanup, and release pipeline hardening.

### Highlights
* Added structured CLI error taxonomy (`kherrors`) with stable error codes and
  clearer remediation hints.
* Improved command help consistency across the CLI (short/long descriptions,
  naming alignment, and environment variable guidance).
* Hardened public sync and release flow to reduce squash-merge drift risks:
  restore silently dropped files during sync and fail early with `go vet` when
  merge artifacts introduce invalid code.
* Updated CI formatting hygiene and GitHub Actions versions for newer runtime
  compatibility.

### Internal Changes
* Reverted the experimental `--explain` scaffolding and related tests.
* Applied repository-wide gofmt consistency fixes.

### Upgrade Notes
* No breaking CLI command changes are introduced in this release.
* If you maintain release automation, continue to merge the generated
  `sync/vX.Y.Z` PR into `kh/main` to trigger tagging and GoReleaser.

---

## v1.10.1 (2026-04-12)

### Maintenance
* enforce repo-wide gofmt and upgrade Node24-compatible actions (e1dc06f)
* apply gofmt on errors map alignment (e2d6b53)

---

## v1.10.0 (2026-04-12)

### Features
* add explain infrastructure for pre-flight command descriptions (d96af98)

### Documentation
* add IDP/license sync PRD, DX prompts, and release exclusions (a1e0898)
* improve help text consistency across all commands (4fa621e)

---

## v1.9.1 (2026-04-10)

### Bug Fixes
* Updating openapi spec (0b87990)

---

## v1.9.0 (2026-04-09)

### Features
* New kh binary (af14744)

---

## v1.8.6 (2026-04-09)

### Maintenance
* chore(doc):adr-0005 (a8b807e)

---

## v1.8.5 (2026-04-09)

### Bug Fixes
* detect duplicate key on set and surface actionable error (05d7096)

---

## v1.8.4 (2026-04-08)

### Features
* move license apps import to kh license import; add full test coverage (be51949)
* split get/show — get prints raw value, show prints full object (a309d30)
* require workspace UUID; drop workspace name resolution (6c9abd4)

### Bug Fixes
* correct action versions in go-ci.yml (checkout@v4, setup-go@v5) (a69de71)
* migrate goreleaser brews to homebrew_casks (8adbaa4)

### Maintenance
* bump version to v1.8.3 (10a2745)
* bump version to v1.8.2 (f58310e)
* bump version to v1.8.1 (6c9a6fe)
* bump version to v1.8.1 (8b9e206)
* bump version to v1.8.0 (daf386b)
* bump version to v1.8.0 (8be2dd0)
* default all integration workflow environments to prod (0868c5e)
* default workflow_dispatch environment to prod (2492758)
* default diagnostics environment to prod (d2806dc)
* upgrade GitHub Actions to Node.js 24-native versions (be6fe45)
* fix integration workflow triggers and malformed YAML (86df554)
* automate release flow (make release, gh pr create) (1f6fe2d)
* bump version to v1.7.0 (414a14e)
* opt into Node.js 24 for all GitHub Actions (0c461d6)
* pin actions to Node.js 24-compatible versions (734d0d3)
* add trailing newline to openapi.yaml (347dc03)

### Documentation
* remove kh license apps import from bulk import section (bbba8e8)
* use bump-version.sh in RELEASING.md (8eecf4f)
* add RELEASING.md (private, stripped from kh sync) (4be145c)
* update README with license apps/users subcommands and CSV import (81b8223)

---

## v1.8.3 (2026-04-08)

### Features
* move license apps import to kh license import; add full test coverage (be51949)
* split get/show — get prints raw value, show prints full object (a309d30)
* require workspace UUID; drop workspace name resolution (6c9abd4)

### Maintenance
* bump version to v1.8.2 (f58310e)
* bump version to v1.8.1 (6c9a6fe)
* bump version to v1.8.1 (8b9e206)
* bump version to v1.8.0 (daf386b)
* bump version to v1.8.0 (8be2dd0)
* default all integration workflow environments to prod (0868c5e)
* default workflow_dispatch environment to prod (2492758)
* default diagnostics environment to prod (d2806dc)
* upgrade GitHub Actions to Node.js 24-native versions (be6fe45)
* fix integration workflow triggers and malformed YAML (86df554)
* automate release flow (make release, gh pr create) (1f6fe2d)
* bump version to v1.7.0 (414a14e)
* opt into Node.js 24 for all GitHub Actions (0c461d6)
* pin actions to Node.js 24-compatible versions (734d0d3)
* add trailing newline to openapi.yaml (347dc03)

### Documentation
* remove kh license apps import from bulk import section (bbba8e8)
* use bump-version.sh in RELEASING.md (8eecf4f)
* add RELEASING.md (private, stripped from kh sync) (4be145c)
* update README with license apps/users subcommands and CSV import (81b8223)

---

## v1.8.2 (2026-04-08)

### Features
* move license apps import to kh license import; add full test coverage (be51949)
* split get/show — get prints raw value, show prints full object (a309d30)
* require workspace UUID; drop workspace name resolution (6c9abd4)

### Maintenance
* bump version to v1.8.1 (6c9a6fe)
* bump version to v1.8.1 (8b9e206)
* bump version to v1.8.0 (daf386b)
* bump version to v1.8.0 (8be2dd0)
* default all integration workflow environments to prod (0868c5e)
* default workflow_dispatch environment to prod (2492758)
* default diagnostics environment to prod (d2806dc)
* upgrade GitHub Actions to Node.js 24-native versions (be6fe45)
* fix integration workflow triggers and malformed YAML (86df554)
* automate release flow (make release, gh pr create) (1f6fe2d)
* bump version to v1.7.0 (414a14e)
* opt into Node.js 24 for all GitHub Actions (0c461d6)
* pin actions to Node.js 24-compatible versions (734d0d3)
* add trailing newline to openapi.yaml (347dc03)

### Documentation
* remove kh license apps import from bulk import section (bbba8e8)
* use bump-version.sh in RELEASING.md (8eecf4f)
* add RELEASING.md (private, stripped from kh sync) (4be145c)
* update README with license apps/users subcommands and CSV import (81b8223)

---

## v1.6.0 (2026-04-08)

### Features
* move license apps import to kh license import; add full test coverage (be51949)
* split get/show — get prints raw value, show prints full object (a309d30)
* require workspace UUID; drop workspace name resolution (6c9abd4)

### Maintenance
* bump version to v1.8.0 (daf386b)
* bump version to v1.8.0 (8be2dd0)
* default all integration workflow environments to prod (0868c5e)
* default workflow_dispatch environment to prod (2492758)
* default diagnostics environment to prod (d2806dc)
* upgrade GitHub Actions to Node.js 24-native versions (be6fe45)
* fix integration workflow triggers and malformed YAML (86df554)
* automate release flow (make release, gh pr create) (1f6fe2d)
* bump version to v1.7.0 (414a14e)
* opt into Node.js 24 for all GitHub Actions (0c461d6)
* pin actions to Node.js 24-compatible versions (734d0d3)
* add trailing newline to openapi.yaml (347dc03)

### Documentation
* remove kh license apps import from bulk import section (bbba8e8)
* use bump-version.sh in RELEASING.md (8eecf4f)
* add RELEASING.md (private, stripped from kh sync) (4be145c)
* update README with license apps/users subcommands and CSV import (81b8223)

---

## v1.8.0 (2026-04-08)

### Features
* move license apps import to kh license import; add full test coverage (be51949)
* split get/show — get prints raw value, show prints full object (a309d30)
* require workspace UUID; drop workspace name resolution (6c9abd4)

### Maintenance
* bump version to v1.8.0 (8be2dd0)
* default all integration workflow environments to prod (0868c5e)
* default workflow_dispatch environment to prod (2492758)
* default diagnostics environment to prod (d2806dc)
* upgrade GitHub Actions to Node.js 24-native versions (be6fe45)
* fix integration workflow triggers and malformed YAML (86df554)
* automate release flow (make release, gh pr create) (1f6fe2d)
* bump version to v1.7.0 (414a14e)
* opt into Node.js 24 for all GitHub Actions (0c461d6)
* pin actions to Node.js 24-compatible versions (734d0d3)
* add trailing newline to openapi.yaml (347dc03)

### Documentation
* remove kh license apps import from bulk import section (bbba8e8)
* use bump-version.sh in RELEASING.md (8eecf4f)
* add RELEASING.md (private, stripped from kh sync) (4be145c)
* update README with license apps/users subcommands and CSV import (81b8223)

---

## v1.8.0 (2026-04-08)

### Features
* move license apps import to kh license import; add full test coverage (be51949)
* split get/show — get prints raw value, show prints full object (a309d30)
* require workspace UUID; drop workspace name resolution (6c9abd4)

### Maintenance
* default all integration workflow environments to prod (0868c5e)
* default workflow_dispatch environment to prod (2492758)
* default diagnostics environment to prod (d2806dc)
* upgrade GitHub Actions to Node.js 24-native versions (be6fe45)
* fix integration workflow triggers and malformed YAML (86df554)
* automate release flow (make release, gh pr create) (1f6fe2d)
* bump version to v1.7.0 (414a14e)
* opt into Node.js 24 for all GitHub Actions (0c461d6)
* pin actions to Node.js 24-compatible versions (734d0d3)
* add trailing newline to openapi.yaml (347dc03)

### Documentation
* remove kh license apps import from bulk import section (bbba8e8)
* use bump-version.sh in RELEASING.md (8eecf4f)
* add RELEASING.md (private, stripped from kh sync) (4be145c)
* update README with license apps/users subcommands and CSV import (81b8223)

---

## v1.6.0 (2026-04-08)

### Features
* move license apps import to kh license import; add full test coverage (be51949)
* split get/show — get prints raw value, show prints full object (a309d30)
* require workspace UUID; drop workspace name resolution (6c9abd4)

### Maintenance
* default all integration workflow environments to prod (0868c5e)
* default workflow_dispatch environment to prod (2492758)
* default diagnostics environment to prod (d2806dc)
* upgrade GitHub Actions to Node.js 24-native versions (be6fe45)
* fix integration workflow triggers and malformed YAML (86df554)
* automate release flow (make release, gh pr create) (1f6fe2d)
* bump version to v1.7.0 (414a14e)
* opt into Node.js 24 for all GitHub Actions (0c461d6)
* pin actions to Node.js 24-compatible versions (734d0d3)
* add trailing newline to openapi.yaml (347dc03)

### Documentation
* remove kh license apps import from bulk import section (bbba8e8)
* use bump-version.sh in RELEASING.md (8eecf4f)
* add RELEASING.md (private, stripped from kh sync) (4be145c)
* update README with license apps/users subcommands and CSV import (81b8223)

---

## v1.7.0 (2026-04-07)

### Features
* split get/show — get prints raw value, show prints full object (a309d30)
* require workspace UUID; drop workspace name resolution (6c9abd4)

### Maintenance
* opt into Node.js 24 for all GitHub Actions (0c461d6)
* pin actions to Node.js 24-compatible versions (734d0d3)
* add trailing newline to openapi.yaml (347dc03)

### Documentation
* use bump-version.sh in RELEASING.md (8eecf4f)
* add RELEASING.md (private, stripped from kh sync) (4be145c)
* update README with license apps/users subcommands and CSV import (81b8223)

---

## v1.6.0 (2026-04-07)

### Maintenance
* sync public release v1.5.0 (#11) (e652c8d)

---

## v1.5.0 (2026-04-04)

### Features
* add `--prefix` flag to `kh kv env` and `kh kv run` — strip a key prefix before injecting variables (0f2eb42)

---

## v1.4.0 (2026-04-04)

### Features
* add `kh kv env` — print workspace key/values as shell exports or dotenv lines (868ba92)
* add `kh kv run` — exec a command with workspace key/values injected into its environment (868ba92)
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