# KeyHarbour CLI - TODO

**Last Updated:** 2025-11-17

## Current Status

Version: v0.5.0 (MVP complete)

CLI is feature-complete for MVP but needs integration with live KeyHarbour backend API.

## High Priority

### Backend API Integration
- [x] Connect to live KeyHarbour backend endpoints
  - [x] Replace stubbed API calls in `internal/khclient/`
  - [x] Implement proper HTTP client with retry logic
  - [x] Add request/response validation
  - [x] Handle API errors gracefully
- [ ] Test all commands against live backend
  - [ ] `kh state ls` - list states from backend
  - [ ] `kh state show` - fetch state details
  - [ ] `kh import tfstate` - upload states to backend
  - [ ] `kh export tfstate` - download states from backend
  - [ ] `kh lock/unlock` - test locking mechanism
- [ ] Update authentication flow
  - [ ] Validate token format and expiration
  - [ ] Implement token refresh if needed
  - [ ] Handle 401/403 responses properly

### Testing & Quality
- [ ] Increase test coverage to >80%
  - [ ] Add tests for `internal/backend/` detection logic
  - [ ] Add tests for `internal/state/` validation
  - [ ] Add tests for `internal/cli/migrate.go` 
  - [ ] Add integration tests with mock backend
- [ ] Add end-to-end tests
  - [ ] Test full migration flow (local → KeyHarbour)
  - [ ] Test batch workspace migration
  - [ ] Test rollback functionality
- [ ] Error handling improvements
  - [ ] Better error messages for common failures
  - [ ] Distinguish network vs. validation vs. backend errors
  - [ ] Add helpful suggestions in error output

## Medium Priority

### Enhanced Backend Support
- [ ] Direct import from S3 backend
  - [ ] Parse S3 backend config from terraform files
  - [ ] Use AWS SDK to fetch state directly
  - [ ] Handle S3 authentication (IAM roles, profiles)
- [ ] Direct import from Azure Blob backend
  - [ ] Parse azurerm backend config
  - [ ] Use Azure SDK to fetch state
  - [ ] Handle Azure authentication
- [ ] Direct import from GCS backend
  - [ ] Parse gcs backend config
  - [ ] Use Google Cloud SDK to fetch state
  - [ ] Handle GCP authentication

### User Experience
- [ ] Progress indicators for long operations
  - [ ] Show progress during batch migrations
  - [ ] Display upload/download progress
  - [ ] Add spinner for API calls
- [ ] Interactive mode improvements
  - [ ] Add confirmation prompts for destructive operations
  - [ ] Allow workspace selection from list
  - [ ] Interactive project/module selection
- [ ] Better output formatting
  - [ ] Colorized output for errors/warnings/success
  - [ ] Table formatting improvements
  - [ ] JSON schema for programmatic use

### Documentation
- [ ] User guides
  - [ ] Getting started guide
  - [ ] Migration guide per backend type
  - [ ] Troubleshooting guide
  - [ ] FAQ
- [ ] Examples
  - [ ] Common migration scenarios
  - [ ] CI/CD integration examples
  - [ ] Terraform backend configuration examples
- [ ] API documentation
  - [ ] Document JSON output schemas
  - [ ] Document exit codes
  - [ ] Document environment variables

## Low Priority

### Features
- [ ] State diff command
  - [ ] Compare two state versions
  - [ ] Show resource changes between versions
- [ ] State history command
  - [ ] List all versions of a state
  - [ ] Show metadata for each version
- [ ] Workspace commands
  - [ ] `kh workspace create`
  - [ ] `kh workspace delete`
  - [ ] `kh workspace list` (enhanced)
- [ ] Project management commands
  - [ ] `kh project create`
  - [ ] `kh project list`
  - [ ] `kh project delete`

### Performance
- [ ] Optimize concurrent operations
  - [ ] Tune worker pool size
  - [ ] Add rate limiting for API calls
  - [ ] Implement connection pooling
- [ ] Caching
  - [ ] Cache project/workspace lists
  - [ ] Cache authentication tokens
  - [ ] Add cache invalidation logic

### Developer Experience
- [ ] Improve debugging
  - [ ] Add trace-level logging
  - [ ] Request/response logging in debug mode
  - [ ] Add profiling flags
- [ ] Development tools
  - [ ] Add mock backend server for testing
  - [ ] Add CLI test harness
  - [ ] Improve Makefile targets

## Future Considerations

### Advanced Features (Post-MVP)
- [ ] State encryption/decryption locally
- [ ] State compression for large files
- [ ] Multi-backend sync/replication
- [ ] Automated state cleanup/archival
- [ ] State analytics and reporting
- [ ] Integration with Terraform workspaces
- [ ] Support for Terragrunt projects
- [ ] Plugin system for custom backends

### Distribution
- [ ] Homebrew formula
- [ ] Chocolatey package (Windows)
- [ ] APT repository (Debian/Ubuntu)
- [ ] YUM repository (RHEL/CentOS)
- [ ] Snap package
- [ ] Docker image
- [ ] GitHub Actions integration

### Security
- [ ] Code signing for releases
- [ ] SBOM generation
- [ ] Vulnerability scanning
- [ ] Secrets management improvements
- [ ] MFA support for authentication

## Completed (v0.5.0)

- ✅ Basic CLI structure with Cobra
- ✅ Authentication commands (login, whoami)
- ✅ Configuration management
- ✅ State listing and display
- ✅ Import from local/HTTP/TFC
- ✅ Export to file/HTTP/TFC
- ✅ Automated migration with backend detection
- ✅ Batch workspace migration
- ✅ State validation
- ✅ Migration reports (JSON)
- ✅ Rollback support
- ✅ Project scaffolding (init project)
- ✅ State locking/unlocking commands
- ✅ Upload verification with checksums
- ✅ CI/CD pipeline (Bitbucket)
- ✅ Multi-OS releases with GoReleaser
- ✅ Basic test coverage

## Notes

### Dependencies on Other Components

**Blocked by `app/` (Rails backend):**
- Full API integration requires completed backend endpoints
- State CRUD operations need working API
- Lock/unlock functionality needs backend implementation
- User authentication and token validation

**Nice to have from `website/`:**
- CLI installation instructions
- Migration tutorials
- Troubleshooting guides

### Development Workflow

1. **For API integration work:**
   - Start with `internal/khclient/client.go`
   - Add real HTTP calls to replace stubs
   - Add integration tests with test backend
   - Update commands to use real client

2. **For backend support:**
   - Add backend detection in `internal/backend/detector.go`
   - Implement backend reader in `internal/backend/`
   - Add backend-specific config parsing
   - Update `migrate auto` command

3. **For testing:**
   - Run `make test` frequently
   - Use `make test-coverage` to check coverage
   - Add tests before implementing features (TDD)

### Quick Commands

```bash
# Development
make tidy          # Update dependencies
make build         # Build binary
make test          # Run tests
make test-coverage # Generate coverage report

# Installation
sudo make install  # Install to /usr/local/bin

# Testing
./bin/kh --debug migrate auto --project=test --dry-run
```

## Questions / Decisions Needed

- [ ] Should we support Terraform 0.x state format? (Currently v4 only)
- [ ] What's the max state file size we should support?
- [ ] Should we add a `kh doctor` command for environment diagnostics?
- [ ] Rate limiting strategy for API calls?
- [ ] Should CLI cache API responses? For how long?
