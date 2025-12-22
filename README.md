# kh (Key-Harbour CLI)

CLI for moving Terraform state between common backends and Key-Harbour.

Status: MVP scaffold (commands parse flags, dry-run supported; some KH APIs not wired yet). Now includes:
- Automated project migration (`kh migrate auto`) with backend detection and state upload
- Terraform Cloud scaffolding (`init --backend cloud`) and read-only import
- Support for local, HTTP, and Terraform Cloud backends

## Getting Started

Prerequisites: macOS, zsh, Go 1.22+ (for local build) or a prebuilt binary.

Install one of the following ways:

1) System-wide (recommended; installs to /usr/local/bin)

```zsh
cd cli
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

# Scaffold a new Terraform project
kh init project -n demo -e dev -m app --dir ./infra --backend http
kh init project -n demo -e dev -m app --dir ./infra --backend cloud --tfc-org MyOrg --tfc-workspace demo-dev

# Migrate an existing Terraform project to KeyHarbour
cd /path/to/terraform/project
kh migrate auto --project=myapp --dry-run  # preview
kh migrate auto --project=myapp            # execute migration

# Bulk migrate all TFC workspaces to KeyHarbour
kh tfc list-workspaces --tfc-org MyOrg     # discover workspaces
kh migrate auto --all --tfc-org MyOrg --create-workspace --dry-run  # preview
kh migrate auto --all --tfc-org MyOrg --create-workspace  # migrate all

# Show version
kh --version
kh --version -o json
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

### projects
List or inspect Key-Harbour projects.

Usage:

```zsh
kh projects show <name-or-uuid> [-o table|json]
```

Note:
- Listing projects (`kh projects ls`) is temporarily unsupported by the server API and hidden in the CLI. Use:
	- `kh projects show <uuid>` to fetch a specific project
	- `kh workspaces ls --project <uuid>` to explore its workspaces

### workspaces
List or inspect workspaces within a project.

Usage:

```zsh
kh workspaces ls --project <name-or-uuid> [-o table|json]
kh workspaces show <name-or-uuid> --project <name-or-uuid> [-o table|json]
```

Flags:

```text
--project string   Project UUID (or KH_PROJECT)
```

### state
List or show Terraform states in Key-Harbour.

Usage:

```zsh
kh state ls [--project ...] [--module ...] [--workspace ...] [-o table|json]
kh state show <state-id> [--raw] [-o table|json]
```

Flags (ls):

```text
--project string     filter by project UUID
--module string      filter by module
--workspace string   filter by workspace
```

Flags (show):

```text
--raw    output raw v4 state JSON
```

### statefiles
Manage historical statefiles scoped to a project/workspace pair.

Usage:

```zsh
kh statefiles ls --project <name-or-uuid> --workspace <name-or-uuid> [--environment <env>] [-o table|json]
kh statefiles last --project <name-or-uuid> --workspace <name-or-uuid> [--environment <env>] [--raw]
kh statefiles get <uuid> --project <name-or-uuid> --workspace <name-or-uuid> [--raw]
kh statefiles push --project <name-or-uuid> --workspace <name-or-uuid> (--file path | --stdin) [--environment <env>]
kh statefiles rm <uuid> --project <name-or-uuid> --workspace <name-or-uuid>
kh statefiles rm-all --project <name-or-uuid> --workspace <name-or-uuid> --force
```

Flags:

```text
--project string      Project UUID (defaults to KH_PROJECT)
--workspace string    Workspace name or UUID (required)
--environment string  Filter or tag a specific environment
--file string         Path to tfstate file for push
--stdin               Read push payload from stdin
--raw                 Print raw content for get/last
--force               Required for rm-all
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

### tfc list-workspaces

List all workspaces in a Terraform Cloud organization. Useful for discovering workspaces before bulk migration.

Usage:

```zsh
kh tfc list-workspaces --tfc-org MyOrg
```

Flags:

```text
--tfc-org string         Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)
--tfc-host string        Terraform Cloud hostname (default: https://app.terraform.io)
--tfc-token string       API token; can be provided via TF_API_TOKEN/TFC_TOKEN
```

Example output:

```json
{
  "organization": "MyOrg",
  "count": 4,
  "workspaces": [
    {"id": "ws-abc123", "name": "app-staging"},
    {"id": "ws-def456", "name": "app-production"},
    {"id": "ws-ghi789", "name": "infra-shared"},
    {"id": "ws-jkl012", "name": "cli-migration-test"}
  ]
}
```

### migrate

Automate migration of Terraform projects from any backend to KeyHarbour. The `migrate` command simplifies the process by:
1. Detecting your current backend configuration (local, http, tfc, s3, etc.)
2. Retrieving the current state
3. Backing up your current backend config
4. Uploading the state to KeyHarbour
5. Generating new backend.tf and backend.hcl for KeyHarbour

#### migrate auto

Automatically migrate the current Terraform project to KeyHarbour.

> **Prerequisites**
> - The KeyHarbour project must already exist and be accessible via the API.
> - A workspace with the same name (or UUID when using `--workspace`) must exist under that project. The CLI now uploads state via `/v1/projects/{project_uuid}/workspaces/{workspace_uuid}/statefiles`, so migration will fail if the workspace cannot be resolved.
> - `KH_TOKEN` (or `kh config set token`) must grant access to the target project/workspace.

Usage:

```zsh
# Migrate current directory (auto-detect backend)
kh migrate auto --project=myapp

# Migrate with explicit workspace and module naming
kh migrate auto --project=myapp --module=infra --workspace=prod

# Preview what will happen without making changes
kh migrate auto --project=myapp --dry-run

# Migrate all workspaces in terraform.tfstate.d/
kh migrate auto --project=myapp --batch

# Migrate with state validation before and after
kh migrate auto --project=myapp --validate

# Generate detailed migration report
kh migrate auto --project=myapp --report=migration-report.json

# Migrate with validation and report
kh migrate auto --project=myapp --batch --validate --report=migration-report.json

# Rollback a migration from backup
kh migrate auto --rollback

# Rollback from custom backup directory
kh migrate auto --rollback --rollback-from=/path/to/backup

# Migrate with custom KeyHarbour endpoint
kh migrate auto --project=myapp --endpoint=https://kh.example.com

# Skip backup (not recommended)
kh migrate auto --project=myapp --skip-backup

# Force overwrite existing backend.tf files
kh migrate auto --project=myapp --force
```

Flags:

```text
-d, --dir string           Terraform project directory (default: ".")
--project string           KeyHarbour project UUID (required, or set KH_PROJECT)
-m, --module string        Module name (auto-detected or defaults to 'infra')
-w, --workspace string     Workspace name (auto-detected or defaults to 'default')
--environment string       KeyHarbour environment tag (defaults to workspace or KH_ENVIRONMENT)
--dry-run                  Preview actions without making changes
--batch                    Migrate all workspaces (discovers from terraform.tfstate.d/)
--validate                 Validate state before and after migration
--report string            Write detailed migration report to file (JSON)
--rollback                 Rollback migration from backup
--rollback-from string     Backup directory to rollback from (defaults to .kh-migrate-backup)
--backup-dir string        Backup directory (defaults to .kh-migrate-backup)
-f, --force                Overwrite existing files
--skip-backup              Skip backing up current backend config (not recommended)
--endpoint string          KeyHarbour API endpoint (or use KH_ENDPOINT)
--org string               KeyHarbour organization (or use KH_ORG)
--kh-project string        Alternative to --project flag
--all                      Migrate all workspaces from TFC organization
--tfc-org string           Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)
--tfc-workspace string     Terraform Cloud workspace name (or TF_WORKSPACE)
--create-workspace         Auto-create workspace in KeyHarbour if it doesn't exist
```

Environment variables:

```text
KH_PROJECT       KeyHarbour project UUID (alternative to --project)
KH_ENDPOINT      KeyHarbour API endpoint
KH_ORG           KeyHarbour organization
KH_TOKEN         KeyHarbour authentication token (required)
KH_WORKSPACE     Optional default workspace name/UUID (used when --workspace is omitted)
KH_ENVIRONMENT   Optional default environment tag (used when --environment is omitted)
```

Supported backend types:
- **local**: Reads from terraform.tfstate or terraform.tfstate.d/{workspace}/
- **http**: Detects address from backend.tf configuration
- **tfc/cloud**: Reads from Terraform Cloud (requires TF_API_TOKEN or similar)
- **s3/azurerm/gcs**: Detection supported; manual export required (see note below)

For backends not yet directly supported (s3, azurerm, gcs), you can manually export state first:

```zsh
# Export state manually
terraform state pull > state.tfstate

# Then import to KeyHarbour
kh import tfstate --from=local --path=state.tfstate --project=myapp --module=infra
```

**Batch Migration:**

Use `--batch` to automatically discover and migrate all workspaces in `terraform.tfstate.d/`:

```zsh
kh migrate auto --project=myapp --batch
```

This will:
- Discover all workspace directories in `terraform.tfstate.d/`
- Migrate each workspace independently
- Continue processing remaining workspaces even if one fails
- Report overall success/failure counts

**Bulk Migration from Terraform Cloud:**

Use `--all` with `--tfc-org` to migrate all workspaces from a Terraform Cloud organization to KeyHarbour:

```zsh
# List all TFC workspaces first (for discovery)
kh tfc list-workspaces --tfc-org MyOrg

# Preview bulk migration (dry-run)
kh migrate auto --all --tfc-org MyOrg --create-workspace --dry-run

# Migrate all TFC workspaces to KeyHarbour (creates workspaces automatically)
kh migrate auto --all --tfc-org MyOrg --create-workspace

# Migrate with custom KeyHarbour project
kh migrate auto --all --tfc-org MyOrg --create-workspace --project=myproject
```

TFC-specific flags:

```text
--all                      Migrate all workspaces from TFC organization
--tfc-org string           Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)
--tfc-workspace string     Specific TFC workspace to migrate (when not using --all)
--create-workspace         Auto-create workspaces in KeyHarbour if they don't exist
```

> **Note:** TFC workspace names with dashes/underscores are automatically sanitized for KeyHarbour compatibility:
> - `app-staging` → `appstaging`
> - `infra-shared` → `infrashared`
> - `my_workspace` → `myworkspace`

**Single Workspace Migration from TFC:**

To migrate a specific TFC workspace to a specific KeyHarbour workspace:

```zsh
# Migrate from TFC workspace to existing KeyHarbour workspace
kh migrate auto --tfc-org MyOrg --tfc-workspace app-prod --workspace Wks001

# Or create the workspace if it doesn't exist
kh migrate auto --tfc-org MyOrg --tfc-workspace app-prod --create-workspace
```

**State Validation:**

Use `--validate` to perform integrity checks before and after migration:

```zsh
kh migrate auto --project=myapp --validate
```

Validation checks include:
- JSON format validity
- Lineage presence and format
- Serial number validity
- State version compatibility
- Terraform version tracking
- State size limits

**Migration Reports:**

Use `--report` to generate a detailed JSON report of the migration:

```zsh
kh migrate auto --project=myapp --report=migration-report.json
```

The report includes:
- Migration metadata (start time, duration, status)
- Per-workspace migration details
- Pre/post validation results
- State IDs, lineages, and serials
- Backup file paths
- Any errors or warnings encountered

Example report structure:

```json
{
  "started_at": "2025-11-09T10:30:00Z",
  "completed_at": "2025-11-09T10:30:15Z",
  "duration_seconds": 15.2,
  "total_workspaces": 3,
  "successful": 3,
  "failed": 0,
  "workspaces": [
    {
      "workspace": "default",
      "status": "success",
      "state_id": "myapp/infra/default",
      "lineage": "abc-123-def",
      "serial": 42,
      "backup_path": ".kh-migrate-backup/backend.tf.1731150600.bak",
      "pre_validation": { ... },
      "post_validation": { ... }
    }
  ]
}
```

**Rollback Support:**

If a migration fails or you need to revert, use `--rollback`:

```zsh
# Rollback using default backup directory
kh migrate auto --rollback

# Rollback from custom backup directory
kh migrate auto --rollback --rollback-from=/path/to/backup
```

Rollback will restore:
- Original `backend.tf` configuration
- Per-workspace state files from backups
- Original backend configuration

After rollback, reinitialize Terraform:

```zsh
terraform init -reconfigure
```

After migration:

```zsh
# Reinitialize Terraform with the new backend
terraform init -reconfigure -backend-config=backend.hcl

# Verify the migration
terraform plan
```

Your original backend configuration is backed up to `.kh-migrate-backup/` (by default).

#### migrate backend (legacy)

Legacy command for manual backend migrations (use `migrate auto` instead).

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
# Scaffold HTTP backend into ./tmp/app/dev with default endpoint https://api.keyharbour.test
kh init project -n sample -e dev -m app --dir ./tmp --backend http

# Use values from config/env when flags are omitted
KH_ENDPOINT=https://api.keyharbour.test kh init project -n sample -e staging

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
address        = "https://api.keyharbour.test/api/v1/states/<id>"
lock_address   = "https://api.keyharbour.test/api/v1/states/<id>/lock"
unlock_address = "https://api.keyharbour.test/api/v1/states/<id>/unlock"
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

### Upload verification (X-Checksum-Sha256)

Key-Harbour includes an optional upload verification mechanism used for HTTP/TFC uploads to protect against corrupted or truncated state files during transfer.

- What it is:
	- KH computes a SHA-256 checksum (hex) of the state file and, when verification is enabled, sends it in the `X-Checksum-Sha256` HTTP header with the PUT/POST upload request.
	- If the receiving server validates the checksum and echoes the same `X-Checksum-Sha256` value in its response headers, KH treats the upload as server-validated and skips a separate GET/read-back step.
	- If the server does not echo the header, or returns a different value, KH performs a GET/read-back and compares checksums. A mismatch causes the upload to fail.

- Why it matters:
	- Avoids accidental corruption of Terraform state during network transfers. Trusting a server-echoed checksum saves an extra round-trip when the server proves it validated the payload.

- Client flags and defaults:
	- `--verify-after-upload` enables this behavior. It is enabled by default for `kh http upload-state` and for HTTP export paths in `kh export tfstate`.
	- To disable verification for a single run: add `--verify-after-upload=false` to the command.

- Server expectations (recommended):
	- Validate the received payload's SHA-256 against the supplied `X-Checksum-Sha256` header before persisting.
	- If validation succeeds, include the same header in the response: `X-Checksum-Sha256: <hex>` and return 2xx.
	- If validation fails, return a non-2xx (for example 409 or 400) and do not persist the invalid payload.

- CLI JSON output:
	- When an upload succeeds and the server echoed/validated the checksum, KH includes `"server_validated": true` in the command's JSON result. If KH had to perform read-back verification (or the server did not echo the header), `server_validated` will be `false`.

- Example (using the bundled receiver):

	1) Start the example receiver (it validates and echoes checksums):

		 go build ./examples/http-receiver
		 ./http-receiver

	2) Upload with KH (default verifies after upload):

		 kh http upload-state \
			 --file ./tmp/app/dev/terraform.tfstate \
			 --url 'http://localhost:8080/states/app/dev.tfstate' \
			 -o json

	3) The receiver will validate `X-Checksum-Sha256`, persist the file, and echo the header back. KH will show `server_validated: true` in its JSON output and skip the read-back.

If you run your own service, implement the above header validation/echo to get the optimized flow.

Running the integration test locally

You can run the small integration test that validates the server-echo -> skip-read-back behavior. It uses an httptest server and asserts that when the server validates and echoes `X-Checksum-Sha256`, the writer returns the server checksum.

From the repository root (zsh):

```zsh
# run only the integration test in the backend package
go test ./internal/backend -run TestHTTPWriter_ServerEcho -v

# or run the full test suite
gofmt -w . && go test ./...
```

The focused `go test` invocation is handy when you're iterating on the example receiver or the HTTP writer behavior.

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

### Automated release (goreleaser) in Bitbucket Pipelines

We provide a `tags: 'v*'` pipeline that runs `goreleaser` to produce multi-OS artifacts (Linux/macOS/Windows for amd64 and arm64), generates checksums and release notes, and uploads the produced artifacts to Bitbucket Downloads.

Required repository variables (Repository settings → Pipelines → Repository variables):

- `BITBUCKET_USERNAME` — a user (or service account) that can upload Downloads (use an app password user).
- `BITBUCKET_APP_PASSWORD` — the app password for the user above with at least `Repository: Write` permission.

What the pipeline does on tag builds:

- Runs `make test`, then installs `goreleaser`.
- Calls `goreleaser release --rm-dist` which produces a `dist/` directory with archives and checksums.
- Creates a simple `dist/release-notes-<tag>.txt` from recent git commits and ensures a checksums file exists.
- Uploads every file in `dist/` to Bitbucket Downloads via the API.

Quick local test:

```zsh
# produce artifacts locally without publishing
goreleaser release --snapshot --rm-dist

# verify artifacts
ls -lha dist/
```

Notes:

- The pipeline uploads to Bitbucket Downloads. If you prefer publishing to GitHub Releases, you can configure goreleaser's `release` section with a GitHub token and endpoint instead.
- Keep `BITBUCKET_APP_PASSWORD` secret and masked in repository variables; do not commit secrets to the repo.
