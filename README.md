# KeyHarbour CLI (`kh`)

The official command-line interface for KeyHarbour, a secure, self-hosted Terraform backend and state management platform.

The `kh` tool simplifies migrating to a remote backend, managing Terraform state versions, and handling day-to-day operations like state locking and workspace management.

## Getting Started

### Installation

#### From Binary (Recommended)
Download the latest release for your platform from the [Releases page](#).

#### Using Go
```zsh
go install github.com/key-harbour/kh/cmd/kh@latest
```

#### Build from Source
```zsh
git clone https://github.com/key-harbour/workspace.git
cd workspace/cli
make build
# Binary is available at ./bin/kh
```

### Authentication

Authenticate with your KeyHarbour instance:

```zsh
# Save token and endpoint to ~/.kh/config
kh login --token <your-api-token> --endpoint https://app.keyharbour.ca/api/v2

# Or use environment variables (recommended for CI)
export KH_TOKEN="your-api-token"
export KH_ENDPOINT="https://app.keyharbour.ca/api/v2"
```

Verify your session:
```zsh
kh whoami
```

---

## Quick Start: New Project

Scaffold a new Terraform project configured for KeyHarbour:

```zsh
mkdir my-infrastructure && cd my-infrastructure

kh init project \
  --name "demo-infra" \
  --env "production" \
  --module "aws-vpc" \
  --backend http
```

This generates `backend.tf` and `backend.hcl` pre-configured for your project.

---

## Command Reference

### Syncing State (`sync`)

The unified `sync` command allows bidirectional state transfer between any backends.

**Supported Sources** (`--from`): `local`, `http`, `tfc`, `keyharbour`
**Supported Destinations** (`--to`): `keyharbour`, `file`, `http`, `tfc` (default: `keyharbour`)

#### Common Use Cases

#### 1. Import to KeyHarbour

```zsh
# From local file
kh sync --from=local --path ./terraform.tfstate --project <uuid> --workspace <name>

# From HTTP backend
kh sync --from=http --url https://old-backend.com/state --project <uuid> --workspace <name>

# From Terraform Cloud (auto-create workspace if needed)
kh sync --from=tfc --tfc-org <org> --tfc-workspace <ws> --project <uuid> --create-workspace
```

#### 2. Export from KeyHarbour

```zsh
# To local file
kh sync --from=keyharbour --src-project <uuid> --src-workspace <name> \
  --to=file --out ./backup.tfstate

# To Terraform Cloud
kh sync --from=keyharbour --src-project <uuid> --src-workspace <name> \
  --to=tfc --dest-tfc-org <org> --dest-tfc-workspace <ws>

# To HTTP backend
kh sync --from=keyharbour --src-project <uuid> --src-workspace <name> \
  --to=http --dest-url https://other-backend.com/state
```

#### 3. Copy Between KeyHarbour Workspaces

```zsh
kh sync --from=keyharbour --src-project <proj1> --src-workspace <ws1> \
  --to=keyharbour --project <proj2> --workspace <ws2> --create-workspace
```

#### Advanced Options

```zsh
# Dry-run mode (preview what will be synced)
kh sync --from=tfc --tfc-org <org> --tfc-workspace <ws> --dry-run

# Verify checksums during sync
kh sync --from=local --path ./state.tfstate --verify-checksum

# Control parallelism
kh sync --from=keyharbour --src-workspace <ws> --to=file --out ./backup.tfstate --concurrency 8

# Lock state during export (KeyHarbour sources only)
kh sync --from=keyharbour --src-workspace <ws> --to=file --out ./backup.tfstate --lock

# Filter statefiles by environment (KeyHarbour sources only)
kh sync --from=keyharbour --src-project <uuid> --src-workspace <ws> \
  --env <env-name> --to=file --out ./backup.tfstate

# Workspace pattern filtering (local sources only)
kh sync --from=local --path ./terraform.tfstate.d --workspace-pattern "prod.*"
```

**Notes:**

- Workspace names must be alphanumeric only. Names with hyphens (e.g., `my-prod-app`) will be automatically sanitized to `myprodapp` with a warning.
- Use `{workspace}` and `{key}` placeholders in `--out` paths for `--to=file`.

### Version Control (`statefiles`)

Manage statefile versions for a workspace.

Commands that act on the workspace collection (`ls`, `last`, `push`, `rm-all`) require `--project` and `--workspace`.
Commands that act on a specific version (`get`, `rm`) only need the statefile UUID — no `--project` or `--workspace` required.

```zsh
# List all statefile versions for a workspace
kh statefiles ls --project <uuid> --workspace <uuid>

# Show the latest statefile (raw Terraform JSON)
kh statefiles last --project <uuid> --workspace <uuid> --raw

# Get a specific version by UUID
kh statefiles get <statefile-uuid>
kh statefiles get <statefile-uuid> --raw

# Upload a new version
kh statefiles push --project <uuid> --workspace <uuid> --file ./terraform.tfstate
terraform state pull | kh statefiles push --project <uuid> --workspace prod --stdin

# Delete a specific version by UUID
kh statefiles rm <statefile-uuid>

# Delete all versions for a workspace (irreversible)
kh statefiles rm-all --project <uuid> --workspace <uuid> --force
```

### Locking (`lock` / `unlock`)

Manage Terraform state locks manually (useful for clearing stuck locks).

```zsh
kh lock <state-id>
kh unlock <state-id> --force
```

### Workspace Management

```zsh
# List all workspaces in a project
kh workspaces ls --project <uuid>

# Show workspace details
kh workspaces show <name-or-uuid> --project <uuid>
```

### Key/Value Management (`kv`)

Store and retrieve configuration values scoped to a workspace.

Commands that act on the workspace collection (`ls`, `set`) require `--project` and `--workspace`.
Commands that act on a specific key (`get`, `update`, `delete`) only need the key name — no `--project` or `--workspace` required.

```zsh
# List all key/value pairs in a workspace
kh kv ls --project <uuid> --workspace <uuid>

# Get a specific key (--reveal to show private values in plain text)
kh kv get MY_KEY
kh kv get MY_API_TOKEN --reveal

# Create a new key/value
kh kv set MY_KEY my-value --project <uuid> --workspace <uuid>
kh kv set MY_SECRET s3cr3t --project <uuid> --workspace <uuid> --private
kh kv set MY_TEMP value --project <uuid> --workspace <uuid> --expires-at 2026-12-31T00:00:00Z

# Update an existing key
kh kv update MY_KEY --value new-value
kh kv update MY_KEY --value new-value --private false

# Delete a key (--force required to confirm)
kh kv delete MY_KEY --force
```

**Notes:**
- `--project` and `--workspace` accept a UUID or name; `KH_PROJECT` and `KH_WORKSPACE` env vars are also respected.
- Private values are masked as `***` in `ls` and `get` output unless `--reveal` is passed.
- All commands support `-o json` for machine-readable output.

### Integrity (`verify`)

Run deep integrity checks on stored state files.

```zsh
kh verify <state-id> --full
```

---

## Configuration

### Environment Variables

| Variable | Description |
|----------|-------------|
| `KH_TOKEN` | API token for authentication |
| `KH_ENDPOINT` | KeyHarbour API base URL including version path (e.g., `https://app.keyharbour.ca/api/v2`) |
| `KH_PROJECT` | Default project UUID |
| `KH_ORG` | Default organization slug |
| `KH_WORKSPACE` | Default workspace UUID or name |
| `KH_CONCURRENCY` | Default concurrency for parallel operations (default: 4) |
| `KH_DEBUG` | Set to `1` for verbose debug logs |

**Note:** All commands support environment variable defaults. For example, if `KH_PROJECT` is set, you don't need to specify `--project` on every command.

### Config File
The CLI stores configuration in `~/.kh/config.json`. You can manage it via:

```zsh
kh config set endpoint https://app.keyharbour.ca/api/v2
kh config set project <uuid>
kh config get project
```

---

## CI/CD Integration

The CLI is designed to run in CI environments.

```yaml
image: deniscdevops/keyharbour-cli:latest

pipelines:
  branches:
    master:
      - step:
          script:
            - export KH_TOKEN=$KH_API_TOKEN
            - export KH_ENDPOINT=https://app.keyharbour.ca/api/v2
            - export KH_PROJECT=<project-uuid>
            - kh statefiles push --workspace prod --file ./terraform.tfstate
```

### Exit Codes
- `0`: Success
- `2`: Partial success
- `3`: Validation error
- `4`: Authentication error
- `5`: Backend I/O error
- `6`: Lock error

---

## Troubleshooting

### Workspace Name Validation Errors

**Problem:** Getting 422 errors when creating workspaces.

**Solution:** Workspace names must be alphanumeric only (no hyphens, underscores, or special characters). The CLI automatically sanitizes names during `sync`. Ensure names contain only letters and numbers.

### Token Expiration

**Problem:** Getting 401 "Invalid or outdated token" errors.

**Solution:** Generate a new token and save it:
```zsh
kh login --token <new-token> --endpoint https://app.keyharbour.ca/api/v2
```

### Debug Mode

For detailed logging of API calls and troubleshooting:
```zsh
kh <command> --debug
# or
export KH_DEBUG=1
```

---

## License

Copyright (c) 2024 KeyHarbour. All rights reserved.
