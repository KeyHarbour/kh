# Copilot Instructions for KeyHarbour CLI

This project is the KeyHarbour CLI - a Go-based command-line tool for managing Terraform state operations.

## What's here
- KeyHarbour CLI (`kh`) — Go CLI for Terraform state management. See `ADR.md` for language decision rationale.

## Architecture and design
- CLI manages Terraform state operations (list/verify/migrate/lock/unlock). Per `ADR.md`, uses Go with cobra/viper and goreleaser for distribution.
- Clean service API boundaries designed for future SDKs (OpenAPI/Connect/gRPC).
- Static binary distribution for easy deployment on CI agents and servers.

## Development workflows
- Build: `go build -o bin/kh ./cmd/kh` or use `make build`
- Test: `go test ./...` to run all tests
- Run: `./bin/kh [command]` or `go run ./cmd/kh [command]`
- CLI structure: 
  - Entrypoint: `cmd/kh/main.go`
  - Commands: `internal/cli/`
  - Backends: `internal/backend/`
  - Configuration: `internal/config/`
- Config path: `${XDG_CONFIG_HOME}/kh/config` (JSON)
- Environment overrides: `KH_ENDPOINT`, `KH_TOKEN`, `KH_ORG`, `KH_PROJECT`, `KH_CONCURRENCY`
- Available commands: `login`, `whoami`, `config get|set`, `state ls|show`, `import tfstate`, `export tfstate`, `migrate backend`, `verify`, `lock|unlock`, `completion`, `init project`, `tfc upload-state`, and `http upload-state`
- Supported backends: `local`, `http`, and `tfc` (Terraform Cloud)
  - Import: `kh import tfstate --from=local --path=...`, `--from=http --url=...`, or `--from=tfc --tfc-org ... --tfc-workspace ...` (TFC import is read-only; supports `--out` to save files)
  - Export: `kh export tfstate --to=file --out=...`, `--to=http --url=...`, or `--to=tfc --tfc-org ... --tfc-workspace ...` (supports dry-run JSON plans)
  - Helpers: `kh http upload-state --file ... --url ...` and `kh tfc upload-state --file ... --tfc-org ... --tfc-workspace ...` (+ `--adopt-lineage`)
  - Authentication: PAT-first via `kh login --token ...` for KH; Terraform Cloud via `TF_API_TOKEN`/`TFC_TOKEN`/`TF_TOKEN_app_terraform_io`

## Project conventions
- Follow ADR-001 choices: Go, cobra/viper, retryablehttp
- Prefer static binaries for cross-platform distribution
- Keep API boundaries clean for future SDK development
- Use standard Go project layout:
  - `cmd/` for application entry points
  - `internal/` for private application code
  - `pkg/` for public library code (if any)
- Error handling: Use Go standard error patterns with proper wrapping
- Logging: Use structured logging via the `internal/logging` package
- Testing: Include unit tests alongside code, integration tests where appropriate
- Configuration: JSON-based config with environment variable overrides

## Documentation hygiene
- Each time you add or modify a function, command, flag, or user-visible behavior, verify that `README.md` remains accurate. If there is any drift, update the relevant sections of `README.md` in the same change/PR. Prefer adding concise examples that match the CLI help output.

## Notes on Terraform Cloud integration
- Reader uses `GET /api/v2/workspaces/{id}/current-state-version` and includes `Authorization: Bearer <token>` for protected download URLs.
- Writer uses `POST /api/v2/workspaces/{id}/state-versions` with base64 state and MD5; includes optional `serial`, `lineage`, and `terraform-version` when present.
- Env fallbacks: `TF_CLOUD_ORGANIZATION` → `--tfc-org`, `TF_WORKSPACE` → `--tfc-workspace`, and token via `TF_API_TOKEN`/`TFC_TOKEN`/`TF_TOKEN_app_terraform_io`.

## Example scaffolding
- `kh init project --backend http|cloud` will generate either HTTP backend files (`backend.tf` + `backend.hcl`) or a Terraform Cloud `cloud.tf` file. It respects env defaults where flags are omitted.

## Integration points
- HTTP backend integration for remote Terraform state operations
- Future API contracts should be defined via OpenAPI/Connect/gRPC (see ADR scope)
- Designed for integration with CI/CD pipelines and automation tools

## Code patterns to follow
- Command structure: Follow cobra patterns in `internal/cli/` commands
- Error handling: Use wrapped errors with context (see existing commands)
- Configuration: Follow patterns in `internal/config/config.go`
- Backend abstraction: Implement interfaces defined in `internal/backend/types.go`
- Client integration: Use patterns from `internal/khclient/` for API calls

## When in doubt
- Reference existing command implementations in `internal/cli/` for patterns
- Follow Go community standards and best practices
- Keep implementation aligned with ADR-001 decisions
- Maintain backward compatibility for configuration and command interfaces
