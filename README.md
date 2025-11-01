# kh (Key-Harbour CLI)

CLI for moving Terraform state between common backends and Key-Harbour.

Status: MVP scaffold (commands parse flags, dry-run supported; some KH APIs not wired yet).

## Getting Started

Prerequisites: macOS, zsh, Go 1.22+ (for local build) or a prebuilt binary.

Install one of the following ways:

1) System-wide (recommended; installs to /usr/local/bin)

```zsh
cd keyharbour-cli
make tidy
make build
sudo make install   # installs /usr/local/bin/kh
kh --help
```

2) User install (no sudo; installs to ~/.local/bin)

```zsh
cd keyharbour-cli
make tidy
make build
make install DESTDIR="$HOME/.local" PREFIX=""
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc
exec zsh -l
kh --help
```

3) Go install into $GOBIN/$GOPATH/bin

```zsh
cd keyharbour-cli
go mod tidy
go install ./cmd/kh
echo 'export PATH="$(go env GOPATH)/bin:$PATH"' >> ~/.zshrc
exec zsh -l
kh --help
```

4) Symlink the built binary

```zsh
cd keyharbour-cli
make tidy && make build
sudo ln -sf "$PWD/bin/kh" /usr/local/bin/kh
kh --help
```

Quick start:

```zsh
kh --debug --help
kh login --token <PAT>
kh state ls -o json
kh init project -n demo -e dev -m app --dir ./infra
```

## Auth & config

- `kh login --token <PAT>` or `kh login --device` (device flow is a stub)
- `kh whoami` shows masked token and org
- `kh config get|set <key> [value]` for endpoint, org, project, token, concurrency
- Config path: `${XDG_CONFIG_HOME}/kh/config` (JSON)
- Env overrides: `KH_ENDPOINT`, `KH_TOKEN`, `KH_ORG`, `KH_PROJECT`, `KH_CONCURRENCY`, `KH_DEBUG`

## Command reference

Global flags:

```text
-o, --output table|json   (default: table)
--debug                   or KH_DEBUG=1 for verbose logs
```

### login
Authenticate with a personal access token (PAT) or stub device flow.

Usage:

```zsh
kh login --token <PAT>
kh login --device
```

Flags:

```text
--token string     PAT value
--device           start device flow (stub)
```

### whoami
Show current auth context.

Usage: `kh whoami [-o table|json]`

### config
Get or set configuration values.

Usage:

```zsh
kh config get <key>
kh config set <key> <value>
```

Keys: `endpoint`, `token`, `org`, `project`, `concurrency`

### state
List or show Terraform states in Key-Harbour.

Usage:

```zsh
kh state ls [--project ...] [--module ...] [--workspace ...] [-o table|json]
kh state show <state-id> [--raw] [-o table|json]
```

Flags (ls):

```text
--project string     filter by project
--module string      filter by module
--workspace string   filter by workspace
```

Flags (show):

```text
--raw    output raw v4 state JSON
```

### import tfstate
Import Terraform state objects from a source backend. Ingest into KH is pending; currently reads/validates and reports.

Usage:

```zsh
kh import tfstate --from=http|local [--path <dir|file> | --url <src>] [--project ... --module ... --env ...] [--workspace-pattern '.*'] [--verify-checksum] [--concurrency N] [--report out.json] [--dry-run]
```

Flags:

```text
--from string              http or local
--path string              local file/dir for --from=local
--url string               HTTP source for --from=http
--workspace-pattern regex  infer workspace from filenames (default: .*)
--project string           annotate target project
--module string            annotate module (e.g., repo/path)
--env string               annotate environment
--verify-checksum          compute SHA256 and fail on mismatch
--concurrency int          parallel I/O (defaults from KH_CONCURRENCY or config)
--report path              write JSON report
--dry-run                  preview without ingest
```

### export tfstate
Export Terraform state from KH to a destination backend.

Usage:

```zsh
kh export tfstate --to=file|http [--out /path/{module}-{workspace}.tfstate | --url <dest>] [--verify-checksum] [--overwrite] [--idempotency-key <key>] [--concurrency N] [--dry-run] [--format v4] [--state-id ... | filters] [--lock]
```

Flags:

```text
--to string                 file or http
--out path                  file path template when --to=file (supports {module}, {workspace})
--url string                destination URL when --to=http (supports {module}, {workspace})
--verify-checksum           verify KH checksum pre-write and destination checksum post-write
--overwrite                 allow overwriting existing files
--idempotency-key string    set Idempotency-Key header for HTTP writes
--concurrency int           parallel exports (defaults from KH_CONCURRENCY or config)
--format string             state format (default v4)
--state-id string           export a specific state
--project, --module, --workspace   filters for selection
--dry-run                   preview without writing
--lock                      acquire advisory lock per state during export
```

### migrate backend

Plan migrations between backends (scaffolding).

Usage: `kh migrate backend --from <src> --to <dest> [--dry-run]`

Flags:

```text
--from string    source backend
--to string      destination backend
--dry-run        preview without changes
```

### verify
Integrity checks for a given state.

Usage: `kh verify <state-id> [--full]`

Flags:

```text
--full           deep verification
```

### lock / unlock
Advisory locks on KH states.

Usage:

```zsh
kh lock <state-id>
kh unlock <state-id> [--force]
```

Flags (unlock):

```text
--force          force unlock
```

### init project
Scaffold a minimal Terraform project that uses the KeyHarbour HTTP backend (backend.hcl for addresses, backend.tf for partial configuration).

Usage:

```zsh
kh init project -n <project-name> -e <environment> [--module <name>] [--dir <path>] [--endpoint <url>] [--org <org>] [--kh-project <kh_project>] [--force]
```

Examples:

```zsh
# Scaffold into ./tmp/app/dev with default endpoint https://api.keyharbour.ca
kh init project -n sample -e dev -m app --dir ./tmp

# Use values from config/env when flags are omitted
KH_ENDPOINT=https://api.keyharbour.ca kh init project -n sample -e staging
```

Generated layout (dir/module/env):

```text
<dir>/
	<module>/
		<env>/
			backend.tf        # terraform { backend "http" {} }
			backend.hcl       # address/lock/unlock URLs and retry settings
			versions.tf       # TF/core and providers constraints
			providers.tf      # minimal provider set (hashicorp/null)
			variables.tf      # project/environment/module variables
			outputs.tf        # basic outputs
			main.tf           # placeholder resource + locals
			.gitignore        # standard Terraform ignores
			README.md         # quick usage notes
```

Backend configuration (backend.hcl):

```hcl
address        = "https://api.keyharbour.ca/api/v1/states/<id>"
lock_address   = "https://api.keyharbour.ca/api/v1/states/<id>/lock"
unlock_address = "https://api.keyharbour.ca/api/v1/states/<id>/unlock"
lock_method    = "POST"
unlock_method  = "POST"
retry_max      = 2
```

Initialize and plan:

```zsh
cd <dir>/<module>/<env>
terraform init -backend-config=backend.hcl
terraform plan -var="project=<name>" -var="environment=<env>" -var="module=<module>"
```

## Output & exits

- `-o, --output table|json` everywhere; stable JSON for CI
- Exit codes: 0 ok, 2 partial, 3 validation error, 4 auth, 5 backend IO, 6 lock error

## Debugging

- `--debug` enables verbose debug logs to stderr
- Or set `KH_DEBUG=1` to turn on debug without adding the flag

## Completion

```zsh
kh completion zsh > /usr/local/share/zsh/site-functions/_kh
```

## Make targets

- `make tidy` — resolve/update modules
- `make build` — build binary at `bin/kh`
- `make run args="..."` — build and run with args
- `make test` — run tests
- `make test-coverage` — run tests with coverage and generate HTML/text reports in `coverage/`
- `make coverage-report` — generate coverage reports even if tests fail (CI-friendly)
- `make vet` — static analysis
- `make fmt` — format code
- `make clean` — remove build artifacts

## CI: Bitbucket Pipelines

This repo includes a Bitbucket Pipelines configuration to build, test, and publish coverage artifacts.

### Triggers

- Default: runs on all branches when you push
- Pull requests: runs when you open/update a PR
- Tags `v*`: builds a release binary with version info embedded

### Build environment

- Base image: `golang:1.22`
- Caches:
	- `go-mod` → `/go/pkg/mod` (Go modules)
	- `go-build` → `/root/.cache/go-build` (compiler cache)

### Artifacts

- `bin/kh` — compiled CLI
- `coverage/**` — coverage outputs:
	- `coverage/coverage.out` — raw profile
	- `coverage/coverage.html` — interactive HTML report
	- `coverage/coverage.txt` — text summary by function

### Manual run

In Bitbucket → Pipelines → “Run pipeline”, choose your branch and start a run. Ensure Pipelines is enabled in repository settings.

### Notes on Free plan

- Pipelines are available on the Free plan with limited monthly build minutes and single-step concurrency. If minutes are exhausted, runs queue/fail until the quota resets or the plan is upgraded.

### Troubleshooting

- Pipeline not showing up?
	- Verify `bitbucket-pipelines.yml` exists at the repository root
	- Ensure Pipelines is enabled (Repository settings → Pipelines)
	- Confirm you pushed to the branch you’re viewing in Pipelines
	- YAML must be valid; caches `go-mod` and `go-build` are defined under `definitions.caches`
