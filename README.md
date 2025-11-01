# kh (Key-Harbour CLI)

CLI for moving Terraform state between common backends and Key-Harbour.

Status: MVP scaffold (commands parse flags and support dry-run; backends/API not wired yet).

## Install/build

Requires Go 1.22+.

```zsh
cd keyharbour-cli
# initialize deps and build
make tidy
make build

## Install/build
make run args="--help"
```

## Auth & config

- `kh login --token <PAT>` or `kh login --device` (stub device flow)
- `kh whoami` shows token mask and org
- `kh config get|set <key> [value]` for endpoint, org, project, token, concurrency
- Config stored at `${XDG_CONFIG_HOME}/kh/config` (JSON). Env overrides: `KH_ENDPOINT`, `KH_TOKEN`, `KH_ORG`, `KH_PROJECT`, `KH_CONCURRENCY`.

## Inspect

### Install the `kh` binary into your PATH

Pick one of the options below.

1) System-wide (requires sudo; installs to /usr/local/bin)

```zsh

- `kh state ls [--project ...] [--module ...] [--workspace ...] [-o table|json]`
- `kh state show <state-id> [--raw]`

## Import

2) User installation (no sudo; installs to ~/.local/bin)

```zsh

- `kh import tfstate --from=http|local [--path <dir|file> | --url <src>] [--dry-run] [--project ... --module ... --env ... --workspace-pattern '.*'] [--verify-checksum] [--concurrency N] [--report out.json]`
	- Sources prioritized for MVP: local filesystem and HTTP.
	- `--verify-checksum` computes SHA256 on each blob and fails on mismatch before ingest (ingest API pending).
	- `--concurrency` defaults from `KH_CONCURRENCY` or config value.

## Export


3) Using Go to install into $GOBIN (or $GOPATH/bin)

```zsh
- `kh export tfstate --to=file|http [--out /path/{module}-{workspace}.tfstate | --url <dest>] [--verify-checksum] [--overwrite] [--idempotency-key <key>] [--concurrency N] [--dry-run] [--format v4] [--state-id ... | filters]`
	- Targets prioritized for MVP: file and HTTP.
	- Placeholders supported in `--out` / `--url`: `{module}`, `{workspace}` (falls back to `default` when absent).
	- `--verify-checksum` verifies KH metadata checksum before write and destination checksum after write.
	- `--overwrite` allows writing over existing files.
	- `--idempotency-key` sets the `Idempotency-Key` header for HTTP writes.
	- `--concurrency` defaults from `KH_CONCURRENCY` or config value.


4) Symlink the built binary (quick, but depends on repo path not moving)

```zsh
## Migrate

- `kh migrate backend --from ... --to ... [--dry-run]`

## Integrity & locking

- `kh verify <state-id> [--full]`
- `kh lock <state-id>` / `kh unlock <state-id> [--force]`

## Output & exits

- `-o, --output table|json` everywhere; stable JSON for CI.
- Exit codes: 0 ok, 2 partial, 3 validation error, 4 auth, 5 backend IO, 6 lock error.

## Debugging

- `--debug` enables verbose debug logs to stderr.
- Or set `KH_DEBUG=1` to turn on debug without adding the flag.

## Completion

```zsh
kh completion zsh > /usr/local/share/zsh/site-functions/_kh
```

## Make targets

- `make tidy` — resolve/update modules
- `make build` — build binary at `bin/kh`
- `make run args="..."` — build and run with args
- `make test` — run tests
- `make vet` — static analysis
- `make fmt` — format code
- `make clean` — remove build artifacts
