# KeyHarbour CLI (`kh`)

The official command-line interface for KeyHarbour, a secure, self-hosted Terraform backend and state management platform.

## Command Structure

```text
kh auth        Authenticate and manage identity
  login          Save token and endpoint to ~/.kh/config
  whoami         Show current authenticated identity

kh tf          Terraform state management
  state          Inspect and manage states
    ls             List all states
    show           Show a state's JSON
    lock           Acquire an advisory lock
    unlock         Release an advisory lock
    verify         Validate a state's integrity
  versions       Manage statefile versions for a workspace
    ls             List all versions
    last           Show the latest version
    get            Download a specific version by UUID
    push           Upload a new version
    rm             Delete a specific version
    rm-all         Delete all versions for a workspace
  sync           Migrate state between backends
  init           Scaffold a Terraform project for KeyHarbour

kh project     Inspect Key-Harbour projects
kh workspace   Inspect and manage project workspaces
kh kv          Manage key/value pairs in a workspace
kh config      Manage CLI configuration
kh license     Manage software license records
kh completion  Generate shell completion scripts
```

---

## Installation

### From Binary (Recommended)

Download the latest release for your platform from the Releases page.

### Using Go

```zsh
go install github.com/key-harbour/kh/cmd/kh@latest
```

### Build from Source

```zsh
git clone https://github.com/key-harbour/workspace.git
cd workspace/cli
make build
# Binary is available at ./bin/kh
```

---

## Authentication

```zsh
# Save token and endpoint to ~/.kh/config
kh auth login --token <your-api-token> --endpoint https://app.keyharbour.ca/api/v2

# Or use environment variables (recommended for CI)
export KH_TOKEN="your-api-token"
export KH_ENDPOINT="https://app.keyharbour.ca/api/v2"

# Verify your session
kh auth whoami
```

---

## Quick Start: New Project

Scaffold a new Terraform project configured for KeyHarbour:

```zsh
mkdir my-infrastructure && cd my-infrastructure

kh tf init \
  --name "demo-infra" \
  --env "production" \
  --module "aws-vpc" \
  --backend http
```

This generates `backend.tf` and `backend.hcl` pre-configured for your project.

---

## Command Reference

### Syncing State (`kh tf sync`)

Bidirectional state transfer between any supported backends.

**Sources** (`--from`): `local`, `http`, `tfc`, `keyharbour`
**Destinations** (`--to`): `keyharbour`, `file`, `http`, `tfc` (default: `keyharbour`)

```zsh
# From local file to KeyHarbour
kh tf sync --from=local --path ./terraform.tfstate --project <uuid> --workspace <workspace-uuid>

# From HTTP backend to KeyHarbour
kh tf sync --from=http --url https://old-backend.com/state --project <uuid> --workspace <workspace-uuid>

# From Terraform Cloud to KeyHarbour (auto-create workspace if needed)
kh tf sync --from=tfc --tfc-org <org> --tfc-workspace <ws> --project <uuid> --create-workspace

# From KeyHarbour to local file
kh tf sync --from=keyharbour --src-project <uuid> --src-workspace <workspace-uuid> \
  --to=file --out ./backup.tfstate

# From KeyHarbour to Terraform Cloud
kh tf sync --from=keyharbour --src-project <uuid> --src-workspace <workspace-uuid> \
  --to=tfc --dest-tfc-org <org> --dest-tfc-workspace <ws>

# Between two KeyHarbour workspaces
kh tf sync --from=keyharbour --src-project <proj1> --src-workspace <ws1-uuid> \
  --to=keyharbour --project <proj2> --workspace <ws2-uuid> --create-workspace

# Dry-run: preview what will be synced without writing
kh tf sync --from=tfc --tfc-org <org> --tfc-workspace <ws> --dry-run

# Generate backend.hcl after a successful sync
kh tf sync --from=local --path ./terraform.tfstate --project <uuid> --workspace <workspace-uuid> \
  --gen-backend
```

**Notes:**

- Use `{workspace}` and `{key}` placeholders in `--out` for batch `--to=file` exports.

---

### State Operations (`kh tf state`)

```zsh
# List all states
kh tf state ls
kh tf state ls --project <uuid> --workspace <workspace-uuid>

# Show a state's full Terraform JSON
kh tf state show <state-id>
kh tf state show <state-id> --raw

# Acquire / release an advisory lock
kh tf state lock <state-id>
kh tf state unlock <state-id> --force

# Validate a state's integrity
kh tf state verify <state-id> --full
```

---

### Statefile Versions (`kh tf version`)

Commands acting on the workspace collection (`ls`, `last`, `push`, `rm-all`) require `--project` and `--workspace`.
Commands acting on a specific version (`get`, `rm`) only need the statefile UUID.

```zsh
# List all versions for a workspace
kh tf version ls --project <uuid> --workspace <uuid>

# Show the latest version
kh tf version last --project <uuid> --workspace <uuid> --raw

# Download a specific version
kh tf version get <statefile-uuid>
kh tf version get <statefile-uuid> --raw

# Upload a new version
kh tf version push --project <uuid> --workspace <uuid> --file ./terraform.tfstate
terraform state pull | kh tf version push --project <uuid> --workspace <workspace-uuid> --stdin

# Delete a specific version
kh tf version rm <statefile-uuid>

# Delete all versions for a workspace (irreversible)
kh tf version rm-all --project <uuid> --workspace <uuid> --force
```

---

### Workspace Management (`kh workspace`)

```zsh
kh workspace ls --project <uuid>
kh workspace show <workspace-uuid> --project <uuid>
kh workspace create <name> --project <uuid>
kh workspace update <workspace-uuid> --project <uuid> --name <new-name>
kh workspace delete <workspace-uuid> --project <uuid> --force
```

---

### Key/Value Management (`kh kv`)

Commands acting on the workspace collection (`ls`, `set`) require `--workspace` (workspace UUID).
Commands acting on a specific key (`get`, `update`, `delete`) only need the key name.

```zsh
# List all key/value pairs
kh kv ls --workspace <uuid>

# Get a key (--reveal to show private values)
kh kv get MY_KEY
kh kv get MY_API_TOKEN --reveal

# Create a key
kh kv set MY_KEY my-value --workspace <uuid>
kh kv set MY_SECRET s3cr3t --workspace <uuid> --private
kh kv set MY_TEMP value --workspace <uuid> --expires-at 2026-12-31T00:00:00Z
kh kv set CERT --value-file ./cert.pem --workspace <uuid>

# Update a key
kh kv update MY_KEY --value new-value
kh kv update MY_KEY --value-file ./cert.pem

# Delete a key
kh kv delete MY_KEY --force

# Inject all workspace KVs into the current shell
eval $(kh kv env --workspace <workspace-uuid>)

# Inject only keys prefixed with KH_ENV_, stripping the prefix
# KH_ENV_DATABASE_URL → DATABASE_URL, KH_ENV_API_KEY → API_KEY
eval $(kh kv env --workspace <workspace-uuid> --prefix KH_ENV_)

# Write a .env file
kh kv env --workspace <uuid> --format dotenv > .env
kh kv env --workspace <uuid> --prefix KH_ENV_ --format dotenv > .env

# Run a command with workspace KVs in its environment (safer than eval)
kh kv run --workspace <workspace-uuid> -- terraform apply
kh kv run --workspace <workspace-uuid> --prefix KH_ENV_ -- terraform apply
kh kv run --workspace <uuid> --environment staging -- ./deploy.sh
```

#### Client-Side Encryption

Values can be encrypted with AES-256-GCM before being sent to the server. The key never leaves the client.

```zsh
# Generate a key
openssl rand -hex 32 > ~/.kh/enc.key && chmod 600 ~/.kh/enc.key

# Store encrypted / retrieve decrypted
kh kv set DB_PASSWORD s3cr3t --workspace <uuid> --encryption-key-file ~/.kh/enc.key
kh kv get DB_PASSWORD --encryption-key-file ~/.kh/enc.key

# Or via env var (recommended for CI)
export KH_ENCRYPTION_KEY_FILE=~/.kh/enc.key
```

---

### License Management (`kh license`)

Manage software applications, their instances, licensees, and team members.

#### Applications

```zsh
kh license ls
kh license show <uuid>
kh license create "Terraform Cloud" --short-name tfc --owner ops --vendor HashiCorp \
  --tier Plus --seats 50 --renewal-date 2027-01-01 --unit-cost 4.99
kh license update <uuid> --status disabled --unit-cost 5.99
kh license delete <uuid> --force
```

#### Instances (deployments of an application)

```zsh
kh license instance ls <app-uuid>
kh license instance show <instance-uuid>
kh license instance create <app-uuid> "Production" --short-name prod --owner ops \
  --renewal-date 2027-01-01 --seats 25 --unit-cost 4.99
kh license instance update <instance-uuid> --status disabled --seats 50
kh license instance delete <instance-uuid> --force
```

#### Licensees (users assigned to an instance)

```zsh
kh license licensee ls <instance-uuid>
kh license licensee show <licensee-uuid>
kh license licensee add <instance-uuid> <member-uuid>
kh license licensee update <licensee-uuid> --status inactive
kh license licensee delete <licensee-uuid> --force
```

#### Team Members (organisation-wide member registry)

```zsh
kh license team-member ls
kh license team-member show <uuid>
kh license team-member add <uuid>
kh license team-member update <uuid> --manager-uuid <manager-uuid>
kh license team-member delete <uuid> --force
```

#### Bulk Import from CSV

```zsh
# Import applications — required columns: name, short_name, owner, vendor
# Optional columns: renewal_date, tier, seats, unit_cost
kh license apps import applications.csv

# Import team members — required column: uuid
# Optional column: manager_uuid
kh license users import members.csv
```

---

## Configuration

### Environment Variables

| Variable | Description |
| -------- | ----------- |
| `KH_ENDPOINT` | API base URL (e.g. `https://app.keyharbour.ca/api/v2`) |
| `KH_TOKEN` | API token for authentication |
| `KH_PROJECT` | Default project UUID |
| `KH_WORKSPACE` | Default workspace UUID or name |
| `KH_ORG` | Default organization slug |
| `KH_CONCURRENCY` | Parallelism for sync operations (default: 4) |
| `KH_OUTPUT` | Default output format: `table` or `json` |
| `KH_DEBUG` | Set to `1` for verbose debug logs |
| `KH_INSECURE` | Set to `1` to skip TLS certificate verification (dev/test only) |
| `KH_ENCRYPTION_KEY_FILE` | Path to hex-encoded 256-bit AES key file for client-side KV encryption |

Priority order: config file < environment variable < CLI flag.

### Config File

```zsh
kh config set endpoint https://app.keyharbour.ca/api/v2
kh config set project <uuid>
kh config get project
```

Configuration is stored at `~/.config/kh/config`.

---

## CI/CD Integration

```yaml
# Example: Bitbucket Pipelines
image: deniscdevops/keyharbour-cli:latest

pipelines:
  branches:
    master:
      - step:
          script:
            - export KH_TOKEN=$KH_API_TOKEN
            - export KH_ENDPOINT=https://app.keyharbour.ca/api/v2
            - export KH_PROJECT=<project-uuid>
            - kh tf version push --workspace prod --file ./terraform.tfstate
```

### Exit Codes

| Code | Meaning |
| ---- | ------- |
| `0` | Success |
| `2` | Partial success |
| `3` | Validation error |
| `4` | Authentication error |
| `5` | Backend I/O error |
| `6` | Lock error |

---

## Troubleshooting

### Workspace Name Validation Errors

Workspace names must be alphanumeric only (no hyphens or underscores). The CLI auto-sanitizes names during `sync` with a warning.

### Token Expiration

```zsh
kh auth login --token <new-token> --endpoint https://app.keyharbour.ca/api/v2
```

### Debug Mode

```zsh
kh tf sync --from=local --path ./state.tfstate --debug
# or
export KH_DEBUG=1
```

---

## License

Copyright (c) 2024 KeyHarbour. All rights reserved.
