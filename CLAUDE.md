# CLAUDE.md

This file provides guidance for working with the KeyHarbour CLI codebase.

## Project Overview

`kh` is the official CLI for [KeyHarbour](https://keyharbour.ca), a self-hosted Terraform state backend. It enables migrating Terraform state from any backend (local, S3, Terraform Cloud, HTTP) to KeyHarbour, and provides day-to-day state management operations.

## Build & Development Commands

```bash
make build          # Compile binary for current platform → ./bin/kh
make test           # Run unit tests
make test-coverage  # Unit tests with HTML coverage report
make vet            # Static analysis
make fmt            # Format check
make tidy           # go mod tidy
make clean          # Remove build artifacts and coverage files
make build-cross    # Cross-compile for darwin/linux/windows (arm64/amd64)
make install        # Install to /usr/local/bin
make run ARGS="..." # Build and run with arguments
```

Integration test modes (require a live KeyHarbour backend):
```bash
make snapshot    # Record baseline state before a backend deployment
make regression  # Compare live system against saved snapshot
make diagnostics # Non-destructive health checks (safe to run anytime)
```

## Architecture

```
cmd/kh/             # Entry point (calls cli.Execute())
internal/
  cli/              # All Cobra command definitions and handlers
  backend/          # Pluggable Reader/Writer interfaces for state backends
  khclient/         # HTTP client for the KeyHarbour API
  config/           # Configuration loading (~/.kh/config)
  output/           # Table/JSON output formatting
  state/            # Terraform state parsing and validation
  exitcodes/        # Standardized exit codes
  workerpool/       # Generic concurrent worker pool
  logging/          # Debug logging (stderr, toggled by --debug)
pkg/                # Public packages (version info)
integration-tests/  # End-to-end tests that invoke the compiled binary
testdata/           # Snapshots and test fixtures
```

## Key Patterns

### Commands (Cobra)
Every command is a `newXxxCmd()` function returning `*cobra.Command`. Subcommands are added via `cmd.AddCommand(...)`. All commands are wired together in `internal/cli/root.go`.

### Error handling with exit codes
Use `exitcodes.With(code, err)` to attach a specific exit code to an error. The root command maps errors to `os.Exit()` calls.

```go
return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("workspace name required"))
```

Standard exit codes: `0` OK, `1` Unknown, `2` Partial, `3` Validation, `4` Auth, `5` BackendIO, `6` Lock.

### Backend abstraction
All state transfer is done through two interfaces in `internal/backend/`:
```go
type Reader interface {
    List(ctx context.Context) ([]Object, error)
    Get(ctx context.Context, key string) ([]byte, Object, error)
}
type Writer interface {
    Put(ctx context.Context, key string, data []byte, overwrite bool) (Object, error)
}
```
Implementations: `LocalReader/Writer`, `HTTPReader/Writer`, `TFCReader/Writer`, `KeyHarbourReader/Writer`.

### Configuration loading
```go
cfg, err := config.LoadWithEnv() // merges file < env vars < flags
```
Config stored at `~/.kh/config`. Environment variables follow the `KH_` prefix convention (e.g., `KH_TOKEN`, `KH_ENDPOINT`, `KH_PROJECT`).

### Concurrent operations
Use `workerpool.Run(items, concurrency, func(item T) error {...})` for parallel state transfers. Default concurrency is 4 (configurable via `KH_CONCURRENCY` or `--concurrency` flag).

### Output formatting
```go
printer := output.Printer{Format: cfg.OutputFormat, W: cmd.OutOrStdout()}
printer.Print(data)
```
Always use `cmd.OutOrStdout()` / `cmd.ErrOrStderr()` — never `os.Stdout` directly — so commands are testable.

### HTTP client retries
The `khclient` package retries on 5xx, 429, and network errors. Default: 2 retries, 200ms wait, 30s timeout.

## Testing

### Unit tests
Run with `make test`. Use table-driven tests. HTTP interactions use `httptest.NewServer`. Test helpers shared within packages via `testutil_test.go`.

### Integration tests
- Build tag `//go:build integration` prevents accidental runs
- Tests invoke the pre-built `bin/kh` binary via `exec.Command`
- Require `KH_ENDPOINT`, `KH_TOKEN`, `KH_PROJECT` env vars
- Three modes: snapshot (pre-deploy), regression (post-deploy), diagnostics (health checks)

## Dependencies

Intentionally minimal:
- `github.com/spf13/cobra` — CLI framework (only direct dependency)
- Standard library only for everything else (`net/http`, `crypto/sha256`, `encoding/json`, `context`, `sync`)

## Conventions

- Workspace names must be alphanumeric only; the CLI auto-sanitizes and warns users
- All file transfers include SHA256 checksum verification
- Commands that mutate state support `--dry-run` to preview without changes
- Debug output goes to stderr via `logging.Debugf(...)`, enabled with `--debug` or `KH_DEBUG=1`
- JSON output mode (`--output json` or `KH_OUTPUT=json`) is preferred for CI/CD usage

## Git Conventions

All commits must use **Conventional Commits** format:

```
type(scope): short description
```

**Types:**

| Type | When to use |
| --- | --- |
| `feat` | New feature or command |
| `fix` | Bug fix |
| `test` | Adding or fixing tests |
| `docs` | Documentation only |
| `refactor` | Code restructuring without behavior change |
| `chore` | Build scripts, CI, dependency updates |

**Scopes** (optional but encouraged): `cli`, `client`, `backend`, `config`, `output`, `test`, `build`

**Examples:**

```text
feat(cli): add --dry-run flag to sync command
fix(test): restore global debug state after each test
test(integration): add snapshot and regression test suite
docs: add CLAUDE.md with project structure and conventions
chore(build): fix SNAPSHOT_DIR to use absolute path in Makefile
```
