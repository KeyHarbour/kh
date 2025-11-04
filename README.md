# kh (Key-Harbour CLI)

CLI for moving Terraform state between common backends and Key-Harbour.

Status: MVP scaffold (commands parse flags, dry-run supported; some KH APIs not wired yet). Now includes Terraform Cloud scaffolding (init --backend cloud) and read-only import from Terraform Cloud.

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
kh init project -n demo -e dev -m app --dir ./infra --backend http
kh init project -n demo -e dev -m app --dir ./infra --backend cloud --tfc-org MyOrg --tfc-workspace demo-dev
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
Import Terraform state objects from a source backend. Ingest into KH is pending; currently reads/validates and reports. Supports local files, generic HTTP, and Terraform Cloud (read-only).

Usage:

```zsh
# Local or generic HTTP
kh import tfstate --from=http|local [--path <dir|file> | --url <src>]
	[--project ... --module ... --env ...]
	[--workspace-pattern '.*'] [--verify-checksum]
	[--concurrency N] [--report out.json] [--out pattern] [--overwrite] [--dry-run]

# Terraform Cloud (read-only)
kh import tfstate --from=tfc \
	--tfc-org <org> \
	--tfc-workspace <workspace> \
	[--tfc-host app.terraform.io] \
	[--out 'out/{workspace}.tfstate'] [--overwrite] [-o json]
```

Flags (common):

```text
--from string              http | local | tfc
--workspace-pattern regex  infer workspace from filenames (default: .*)
--project string           annotate target project
--module string            annotate module (e.g., repo/path)
--env string               annotate environment
--verify-checksum          compute SHA256 and fail on mismatch
--concurrency int          parallel I/O (defaults from KH_CONCURRENCY or config)
--report path              write JSON report
--out path                 optional: save fetched state(s) to files; supports {module}, {workspace}
--overwrite                allow overwriting existing files when using --out
--dry-run                  preview without ingest
```

Additional flags (Terraform Cloud):

```text
--tfc-org string           Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)
--tfc-workspace string     Terraform Cloud workspace name (or TF_WORKSPACE)
--tfc-host string          Terraform Cloud hostname (default: app.terraform.io)
--tfc-token string         API token; can be provided via env instead
```

Terraform Cloud auth environment variables (any of the following):

```text
TF_API_TOKEN                 standard Terraform Cloud token env
TFC_TOKEN                    alias token env
TF_TOKEN_app_terraform_io    host-scoped token env
```

Examples:

```zsh
# Save current state from TFC into out/test-workspace.tfstate
export TF_API_TOKEN=xxxx
kh import tfstate \
	--from=tfc \
	--tfc-org KeyHarbour \
	--tfc-workspace test-workspace \
	--out 'out/{workspace}.tfstate' \
	--overwrite \
	-o json

# Import from a directory of .tfstate files and write a report
kh import tfstate --from=local --path ./states --report ./import-report.json -o json
```

### export tfstate
Export Terraform state from KH to a destination backend.

Usage:

```zsh
# File or generic HTTP targets
kh export tfstate --to=file|http \
	[--out /path/{module}-{workspace}.tfstate | --url <dest>] \
	[--verify-checksum] [--overwrite] [--idempotency-key <key>] \
	[--concurrency N] [--dry-run] [--format v4] [--state-id ... | filters] [--lock]

# Terraform Cloud target
kh export tfstate --to=tfc \
	--tfc-org <org> \
	--tfc-workspace <workspace> \
	[--tfc-host app.terraform.io] \
	[--verify-checksum] [--concurrency N] [--state-id ... | filters]
```

Flags:

```text
--to string                 file | http | tfc
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
--tfc-org string            (tfc) Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)
--tfc-workspace string      (tfc) Terraform Cloud workspace name (or TF_WORKSPACE)
--tfc-host string           (tfc) Terraform Cloud hostname (default: app.terraform.io)
--tfc-token string          (tfc) API token; can be provided via TF_API_TOKEN/TFC_TOKEN
```

### http upload-state

Upload a local `.tfstate` file directly to an HTTP endpoint (useful with the included example receiver or your own service).

Usage:

```zsh
kh http upload-state \
	--file ./tmp/app/dev/terraform.tfstate \
	--url 'http://localhost:8080/states/app/dev.tfstate' \
	[--idempotency-key <key>] \
	[--content-type 'application/vnd.terraform.state+json;version=4'] \
	-o json
```

Example flow with the bundled receiver:

```zsh
# 1) Start the receiver in another terminal (listens on :8080)
go build ./examples/http-receiver
./http-receiver

# 2) Upload your local state
kh http upload-state \
	--file ./tmp/app/dev/terraform.tfstate \
	--url 'http://localhost:8080/states/app/dev.tfstate' \
	-o json

# 3) Verify
curl -sS http://localhost:8080/states/app/dev.tfstate | jq .
```

Examples:

```zsh
# Export one state to Terraform Cloud workspace
export TF_API_TOKEN=xxxx
kh export tfstate \
	--to=tfc \
	--tfc-org KeyHarbour \
	--tfc-workspace mvp-workspace \
	--state-id <state-id> \
	-o json

# Export multiple states to files with placeholders
kh export tfstate --to=file \
	--out 'out/{module}-{workspace}.tfstate' \
	--project myproj -o json
```

### tfc upload-state

Upload a local `.tfstate` file directly to a Terraform Cloud workspace (creates a new state version).

Usage:

```zsh
kh tfc upload-state \
	--file out/mvp-workspace.tfstate \
	--tfc-org KeyHarbour \
	--tfc-workspace export-workspace \
	[--tfc-host app.terraform.io]
```

Flags:

```text
--file string            Path to local .tfstate file
--tfc-org string         Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)
--tfc-workspace string   Terraform Cloud workspace name (or TF_WORKSPACE)
--tfc-host string        Terraform Cloud hostname (default: app.terraform.io)
--tfc-token string       API token; can be provided via TF_API_TOKEN/TFC_TOKEN
```

Example:

```zsh
export TF_API_TOKEN=xxxx
kh tfc upload-state \
	--file out/mvp-workspace.tfstate \
	--tfc-org KeyHarbour \
	--tfc-workspace export-workspace \
	-o json
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
Scaffold a minimal Terraform project that uses either the KeyHarbour HTTP backend or Terraform Cloud backend.

Usage:

```zsh
kh init project \
	-n <project-name> \
	-e <environment> \
	[--module <name>] [--dir <path>] \
	[--backend http|cloud] \
	[--endpoint <url>] [--org <org>] [--kh-project <kh_project>] \
	[--tfc-org <org>] [--tfc-workspace <ws>] \
	[--force]
```

Examples:

```zsh
# Scaffold HTTP backend into ./tmp/app/dev with default endpoint https://api.keyharbour.ca
kh init project -n sample -e dev -m app --dir ./tmp --backend http

# Use values from config/env when flags are omitted
KH_ENDPOINT=https://api.keyharbour.ca kh init project -n sample -e staging

# Scaffold Terraform Cloud backend (generates cloud.tf)
kh init project -n sample -e dev -m app --dir ./tmp \
	--backend cloud --tfc-org KeyHarbour --tfc-workspace sample-dev
```

Generated layout (dir/module/env):

```text
<dir>/
	<module>/
		<env>/
			backend.tf        # terraform { backend "http" {} } when --backend=http
			backend.hcl       # address/lock/unlock URLs and retry settings (HTTP)
			cloud.tf          # terraform { cloud { ... } } when --backend=cloud
			versions.tf       # TF/core and providers constraints
			providers.tf      # minimal provider set (hashicorp/null)
			variables.tf      # project/environment/module variables
			outputs.tf        # basic outputs
			main.tf           # placeholder resource + locals
			.gitignore        # standard Terraform ignores
			README.md         # quick usage notes
```

Backend configuration (backend.hcl for HTTP backend):

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
terraform init -backend-config=backend.hcl   # for HTTP backend
terraform plan -var="project=<name>" -var="environment=<env>" -var="module=<module>"

# For Terraform Cloud backend, terraform init uses the generated cloud.tf automatically
terraform init
terraform plan -var="project=<name>" -var="environment=<env>" -var="module=<module>"

Env fallbacks:

```text
# For init --backend cloud
TF_CLOUD_ORGANIZATION  → default for --tfc-org
TF_WORKSPACE           → default for --tfc-workspace

# For init --backend http
KH_ENDPOINT            → default for --endpoint
KH_ORG, KH_PROJECT     → used in generated defaults
```

Troubleshooting:

- Unknown flag errors (e.g., --out): ensure you're running the locally built binary `./bin/kh` or reinstall to PATH.
- Terraform Cloud 401 Unauthorized during import: make sure a token env is exported (TF_API_TOKEN, TFC_TOKEN, or TF_TOKEN_app_terraform_io).
- Terraform Cloud 404 on state version: workspace must exist and have at least one state; we query the current-state-version endpoint.
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
- Terraform Cloud 409 Conflict (lineage mismatch) on upload:
	- Why it happens: Your local .tfstate has a different `lineage` than the current workspace in Terraform Cloud, so TFC rejects the new state version.
	- Quick fix: use the helper to adopt the current workspace lineage into your local state before upload:

		```zsh
		export TF_API_TOKEN=xxxx
		kh tfc upload-state \
			--file out/<workspace>.tfstate \
			--tfc-org <org> \
			--tfc-workspace <workspace> \
			--adopt-lineage \
			-o json
		```

		This fetches the current workspace lineage/serial and rewrites your local state accordingly, then uploads a new version.
	- Alternative: if you're moving a workspace or intentionally changing backends, run Terraform with migration flags from the project directory:

		```zsh
		terraform init -migrate-state
		```

		After migration, retry the upload without adopt-lineage.
	- Confirm you pushed to the branch you’re viewing in Pipelines
	- YAML must be valid; caches `go-mod` and `go-build` are defined under `definitions.caches`
