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
# Using interactive login
kh login --endpoint https://api.keyharbour.ca

# Or using an environment variable (great for CI)
export KH_TOKEN="your-api-token"
export KH_ENDPOINT="https://api.keyharbour.ca"
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

# Initialize a new local project
kh init project \
  --name "demo-infra" \
  --environment "production" \
  --module "aws-vpc" \
  --backend http
```

This generates `backend.tf` and `backend.hcl` pre-configured for your project.

---

## Migration Guide

KeyHarbour specializes in seamless migration from other backends (Local, S3, Terraform Cloud, etc.).

### Automatic Migration (`migrate auto`)

The `migrate auto` command is the easiest way to move your state. It detects your current configuration, uploads state to KeyHarbour, and updates your backend files.

```zsh
# Migrate current directory (auto-detects backend)
kh migrate auto --project <project-uuid>

# Preview changes without applying
kh migrate auto --project <project-uuid> --dry-run
```

**What it does:**
1.  **Detects** current backend (local file, S3, TFC, etc.).
2.  **Fetches** the latest state.
3.  **Backs up** your existing `backend.tf`.
4.  **Uploads** state to KeyHarbour securely.
5.  **Generates** new `backend.tf` for KeyHarbour.

### Batch Migration

Migrate all workspaces in a directory (common with `terraform workspace` usage):

```zsh
kh migrate auto --project <project-uuid> --batch
```

**Note:** Workspace names are automatically sanitized to be alphanumeric-only during migration. Names with hyphens will be converted (e.g., `prod-app` → `prodapp`).

### Importing from Terraform Cloud

Migrate an entire organization from Terraform Cloud:

```zsh
# 1. List available workspaces
kh tfc list-workspaces --tfc-org MyOrg

# 2. Migrate all workspaces (creating them in KeyHarbour as needed)
kh migrate auto --all --tfc-org MyOrg --create-workspace --project <project-uuid>
```

---

## Command Reference

### State Management (`import` / `export`)

Manually move state in and out of KeyHarbour.

**Import State:**
```zsh
# Import a local file
kh import tfstate --from=local --path ./terraform.tfstate --project <uuid> --workspace <name>

# Import from Terraform Cloud (Read-Only fetch)
kh import tfstate --from=tfc --tfc-org <org> --tfc-workspace <ws> --out state.json
```

**Export State:**
```zsh
# Export state to a local file
kh export tfstate --to=file --out ./backup.tfstate --project <uuid> --workspace <name>

# Push state to another HTTP backend
kh export tfstate --to=http --url https://other-backend.com/state
```

### Syncing State (`sync`)

Sync allows you to upload state from a backend (Local, HTTP, Terraform Cloud) directly to KeyHarbour without modifying local files. This is useful for CI/CD pipelines pushing state to KeyHarbour.

**Features:**
- **Auto-sanitizes workspace names** - Removes hyphens and special characters (only alphanumeric allowed)
- **Auto-detects environment** - Uses the project's first available environment if not specified
- **Better error messages** - Provides helpful validation hints for common issues

```zsh
# Sync a local file to a specific workspace
kh sync --from=local --path ./terraform.tfstate --project <uuid> --workspace <name>

# Sync from HTTP backend
kh sync --from=http --url https://old-backend.com/state --project <uuid> --workspace <name>

# Sync from Terraform Cloud (auto-create workspace)
kh sync --from=tfc --tfc-org <org> --tfc-workspace <ws> --project <uuid> --create-workspace

# Specify environment explicitly (otherwise uses project's first environment)
kh sync --from=tfc --tfc-org <org> --tfc-workspace <ws> --project <uuid> --env <env-name> --create-workspace
```

**Note:** Workspace names must be alphanumeric only. Names with hyphens (e.g., `my-prod-app`) will be automatically sanitized to `myprodapp` with a warning.

### Version Control (`statefiles`)

Manage state file versions specifically for a workspace.

```zsh
# List all state versions for a workspace
kh statefiles ls --project <uuid> --workspace <uuid>

# Get the latest state content
kh statefiles last --project <uuid> --workspace <uuid>

# Delete a specific version
kh statefiles rm --project <uuid> --workspace <uuid> <version-uuid>
```

### Locking (`lock` / `unlock`)

Manage Terraform state locks manually (useful for clearing stuck locks).

```zsh
# Lock a state
kh lock <state-id>

# Unlock a state (force if necessary)
kh unlock <state-id> --force
```

### Workspace Management

```zsh
# List all workspaces
kh workspaces ls --project <uuid>

# Create a new workspace
kh workspaces create --project <uuid> --name "production-db"
```

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
| `KH_TOKEN` | API Token for authentication |
| `KH_ENDPOINT` | KeyHarbour API URL (e.g., `https://api.keyharbour.ca`) |
| `KH_PROJECT` | Default Project UUID |
| `KH_ORG` | Default Organization Slug |
| `KH_DEBUG` | Set to `1` for verbose debug logs |

### Config File
The CLI stores configuration in `~/.kh/config.json`. You can manage it via:

```zsh
kh config set project <uuid>
kh config get project
```

---

## CI/CD Integration

### Bitbucket Pipelines

The CLI is designed to run in CI environments.

```yaml
image: deniscdevops/keyharbour-cli:latest

pipelines:
  branches:
    master:
      - step:
          script:
            - export KH_TOKEN=$KH_API_TOKEN
            - kh init project ...
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

**Solution:** Workspace names must be alphanumeric only (no hyphens, underscores, or special characters). The CLI automatically sanitizes names, but if you see validation errors, check that:
- Names don't start with numbers
- Names contain only letters and numbers
- Environment names match those configured in your project

### Environment Validation Errors

**Problem:** State upload fails with 422 error about environment.

**Solution:** Ensure the environment name matches one of your project's environments. You can:
```zsh
# Check available environments
kh projects show <project-uuid>

# Specify environment explicitly
kh sync --from=tfc --env=<environment-name> ...
```

The CLI auto-detects the first available environment if `--env` is not specified.

### Token Expiration

**Problem:** Getting 401 "Invalid or outdated token" errors.

**Solution:** Generate a new token:
```zsh
kh login --endpoint https://app.keyharbour.ca
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
