# 🚀 KeyHarbour CLI Demo Workflows

A collection of GitHub Actions workflows that demonstrate all CLI functionalities with proper error handling and automatic cleanup.

## Overview

These workflows provide hands-on demonstrations of the KeyHarbour CLI (`kh`) interacting with the KeyHarbour backend. Each workflow is self-contained, manually triggered, and includes cleanup steps where applicable.

## Prerequisites

All workflows require these GitHub repository environment variables to be configured:

| Variable | Description | Required |
|----------|-------------|----------|
| `KH_ENDPOINT` | KeyHarbour API endpoint (e.g., `https://app.keyharbour.ca/api/v2`) | ✅ |
| `KH_PROJECT` | Default project ID for demos | ✅ |
| `KH_ENVIRONMENT` | Default workspace/environment ID | ✅ |

And these secrets:

| Secret | Description | Required |
|--------|-------------|----------|
| `KH_TOKEN` | API authentication token | ✅ |

**Setup**: Go to Repository Settings → Environments or configure as repository secrets.

---

## Demo Workflows

### 1. 🔐 Authentication & Identity (`demo-auth.yml`)

**Purpose**: Verify CLI authentication and configuration

**Demonstrates**:
- Token validation
- Endpoint connectivity
- Identity verification
- Configuration loading

**Manual Inputs**: None

**Expected Output**: Confirmation of authentication success

**Cleanup**: Automatic (read-only operations)

---

### 2. 📦 Key-Value Store Operations (`demo-kv.yml`)

**Purpose**: Demonstrate CRUD operations on workspace key-value pairs

**Demonstrates**:
- Create key-value pairs (`kh kv set`)
- Read values (`kh kv get`)
- Update values (`kh kv update`)
- Delete pairs (`kh kv delete`)

**Manual Inputs**:
- `test_key` (default: `demo-key-ghactions`)
- `test_value` (default: `Demo from GitHub Actions`)

**Operations Sequence**:
1. Create test key with initial value
2. Retrieve and verify creation
3. Update value
4. Verify update
5. Delete key (cleanup)

**Cleanup**: Automatic deletion of test key

---

### 3. 🏗️ Terraform State Management (`demo-terraform-state.yml`)

**Purpose**: Explore and manage Terraform state files

**Demonstrates**:
- List workspace states (`kh tf state ls`)
- Show state details (`kh tf state show`)
- List state versions (`kh tf version ls`)
- Get latest version (`kh tf version last`)

**Manual Inputs**:
- `show_state_details` (default: `false`) - Display full state content

**Cleanup**: Automatic (read-only operations)

---

### 4. 🗂️ Project & Workspace Discovery (`demo-workspace.yml`)

**Purpose**: Explore account structure and resources

**Demonstrates**:
- List projects (`kh project list`)
- Describe project details (`kh project describe`)
- List workspaces (`kh workspace list`)
- Describe workspace (`kh workspace describe`)
- List environments (`kh workspace list-environments`)

**Manual Inputs**:
- `list_environments` (default: `true`) - Also list workspace environments

**Cleanup**: Automatic (read-only operations)

---

### 5. 📜 License Management (`demo-license.yml`)

**Purpose**: Demonstrate license CRUD operations

**Demonstrates**:
- List licenses (`kh license list`)
- Create licenses (`kh license create`)
- Get license details (`kh license get`)
- Update licenses (`kh license update`)
- Delete licenses (`kh license delete`)

**Manual Inputs**:
- `license_name` (default: `GitHub-Actions-Demo-License`)
- `license_type` (choices: `SaaS`, `Perpetual`, `Subscription`, `Trial`)
- `initial_seats` (default: `5`)

**Operations Sequence**:
1. List existing licenses
2. Create new test license
3. Retrieve license details
4. Update seat count
5. Verify update
6. Delete test license (cleanup)

**Cleanup**: Automatic deletion of test license

---

### 6. 🎯 Terraform Project Scaffolding (`demo-terraform-init.yml`)

**Purpose**: Demonstrate Terraform project initialization

**Demonstrates**:
- Scaffold new Terraform projects (`kh tf init`)
- Generate backend configuration
- Create backend.tf and backend.hcl files

**Manual Inputs**:
- `project_name` (default: `demo-infrastructure`)
- `environment` (choices: `dev`, `staging`, `production`)
- `module` (choices: `aws-vpc`, `aws-rds`, `aws-s3`, `gcp-compute`, `azure-storage`, `generic`)

**Cleanup**: Automatic deletion of temporary directory

---

### 7. 🚀 Full Integration Test (`demo-integration.yml`)

**Purpose**: Run all demo scenarios in sequence

**Demonstrates**: Complete end-to-end workflow

**Scenario Coverage**:
1. 🔐 **Authentication** - Verify identity
2. 📁 **Project Discovery** - List all projects
3. 🏢 **Workspace Overview** - Explore workspace structure
4. 📦 **Key-Value Store** - Create, read, update, delete KV pair
5. 🏗️ **Terraform State** - List and inspect states
6. 📜 **License Management** - View existing licenses
7. 📊 **System Health** - Verify connectivity and configuration

**Expected Duration**: 2-3 minutes

**Cleanup**: Automatic cleanup of test resources

---

## Usage Guide

### Running a Single Demo

1. Go to your GitHub repository
2. Click **Actions** tab
3. Select the demo workflow you want to run
4. Click **Run workflow** button
5. (Optional) Configure input parameters
6. Click **Run workflow** confirmation
7. Watch the live execution logs
8. Check the results

### Example: Run KV Store Demo

```bash
# From GitHub UI:
1. Actions → "Demo - Key-Value Store Operations"
2. Run workflow
3. Inputs:
   - test_key: "my-app-version"
   - test_value: "2.5.0"
4. Run workflow
5. Monitor execution
```

### Batch Running Demos

Recommended sequence:
1. `demo-auth.yml` (verify setup)
2. `demo-workspace.yml` (explore structure)
3. `demo-kv.yml` (test write operations)
4. `demo-terraform-state.yml` (view states)
5. `demo-integration.yml` (full test)

---

## Cleanup & Safety

All workflows include automatic cleanup:

| Workflow | Creates | Cleans Up |
|----------|---------|-----------|
| `demo-auth.yml` | Nothing | N/A (read-only) |
| `demo-kv.yml` | Test KV pair | ✅ Deletes test key |
| `demo-terraform-state.yml` | Nothing | N/A (read-only) |
| `demo-workspace.yml` | Nothing | N/A (read-only) |
| `demo-license.yml` | Test license | ✅ Deletes test license |
| `demo-terraform-init.yml` | Temp files | ✅ Deletes temp directory |
| `demo-integration.yml` | Test KV pair | ✅ Deletes test key |

**Safety**: All cleanup is `if: always()` - runs even if previous steps fail.

---

## Troubleshooting

### "Token authentication failed"
- Verify `KH_TOKEN` secret is configured
- Verify token is still valid (not expired)
- Check token has correct permissions

### "Project not found"
- Verify `KH_PROJECT` environment variable is set correctly
- Check project ID format
- Verify project exists in your account

### "Workspace not found"
- Verify `KH_ENVIRONMENT` is set correctly
- Check workspace/environment ID
- Verify workspace exists in the specified project

### "API connection failed"
- Verify `KH_ENDPOINT` is correct
- Check network connectivity to the endpoint
- Verify endpoint is accessible from GitHub Actions

---

## Performance & Costs

| Workflow | Duration | GitHub Actions Cost |
|----------|----------|-------------------|
| `demo-auth.yml` | ~30 seconds | Free |
| `demo-kv.yml` | ~1 minute | Free |
| `demo-terraform-state.yml` | ~1 minute | Free |
| `demo-workspace.yml` | ~45 seconds | Free |
| `demo-license.yml` | ~2 minutes | Free |
| `demo-terraform-init.yml` | ~1 minute | Free |
| `demo-integration.yml` | ~3 minutes | Free |

---

## Contributing

To improve demo workflows, create a pull request with:
- Clear description of improvements
- Testing confirmation
- Updated documentation if needed

---

## Related Documentation

- [KeyHarbour CLI README](../README.md)
- [KeyHarbour API Docs](https://app.keyharbour.ca/docs)
