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
- Available commands: `login`, `whoami`, `config get|set`, `state ls|show`, `import tfstate`, `export tfstate`, `migrate backend`, `verify`, `lock|unlock`, and `completion`
- Supported backends: `local` and `http` for import/export
  - Import: `kh import tfstate --from=local --path=...` or `--from=http --url=...`
  - Export: `kh export tfstate --to=file --out=...` or `--to=http --url=...` (supports dry-run JSON plans)
- Authentication: PAT-first via `kh login --token ...` (device flow is stubbed)

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
