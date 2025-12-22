package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"kh/internal/backend"
	"kh/internal/config"
	"kh/internal/exitcodes"
	"kh/internal/khclient"
	"kh/internal/logging"
	"kh/internal/output"
	"kh/internal/state"

	"github.com/spf13/cobra"
)

// BackendConfig represents a detected Terraform backend configuration
type BackendConfig struct {
	Type       string            // local, http, s3, azurerm, gcs, tfc, etc.
	Config     map[string]string // backend-specific settings
	FilePath   string            // path to the .tf file containing backend block
	BackupPath string            // where we backed up the original
}

// MigrationReport captures detailed information about a migration operation
type MigrationReport struct {
	StartTime      time.Time            `json:"start_time"`
	EndTime        time.Time            `json:"end_time"`
	Duration       string               `json:"duration"`
	Success        bool                 `json:"success"`
	Project        string               `json:"project"`
	Module         string               `json:"module"`
	BackendType    string               `json:"backend_type"`
	Workspaces     []WorkspaceMigration `json:"workspaces"`
	BackupLocation string               `json:"backup_location"`
	Errors         []string             `json:"errors,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
}

// WorkspaceMigration represents migration details for a single workspace
type WorkspaceMigration struct {
	Workspace      string           `json:"workspace"`
	WorkspaceUUID  string           `json:"workspace_uuid"`
	StateID        string           `json:"state_id"`
	SourceSize     int64            `json:"source_size"`
	SourceChecksum string           `json:"source_checksum"`
	TargetChecksum string           `json:"target_checksum"`
	Lineage        string           `json:"lineage"`
	Serial         int              `json:"serial"`
	Success        bool             `json:"success"`
	Error          string           `json:"error,omitempty"`
	ValidationPre  ValidationResult `json:"validation_pre"`
	ValidationPost ValidationResult `json:"validation_post"`
	BackupPath     string           `json:"backup_path"`
}

// ValidationResult captures state validation checks
type ValidationResult struct {
	Valid            bool     `json:"valid"`
	Checks           []string `json:"checks"`
	Errors           []string `json:"errors,omitempty"`
	Warnings         []string `json:"warnings,omitempty"`
	TerraformVersion string   `json:"terraform_version"`
	StateVersion     int      `json:"state_version"`
}

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate Terraform state from any backend to KeyHarbour",
		Long: `Migrate simplifies importing existing Terraform projects into KeyHarbour.

It will:
1. Detect your current backend configuration
2. Retrieve the current state
3. Backup the current backend config
4. Upload the state to KeyHarbour
5. Generate new backend.tf and backend.hcl for KeyHarbour

Usage:
  kh migrate [flags]

Examples:
  # Migrate current directory (auto-detect backend)
  kh migrate

  # Migrate with explicit project/workspace naming
  kh migrate --project=myapp --module=infra --workspace=prod

  # Dry-run to preview actions
  kh migrate --dry-run
`,
	}
	cmd.AddCommand(newMigrateBackendCmd())
	cmd.AddCommand(newMigrateAutoCmd())
	return cmd
}

func newMigrateAutoCmd() *cobra.Command {
	var (
		dir          string
		project      string
		module       string
		workspace    string
		envName      string
		dryRun       bool
		backupDir    string
		force        bool
		skipBackup   bool
		khEndpoint   string
		khOrg        string
		khProject    string
		batchMode    bool
		validate     bool
		reportPath   string
		rollback     bool
		rollbackFrom string
		// TFC source options (when migrating from Terraform Cloud)
		tfcOrg       string
		tfcWorkspace string
		// Bulk migration options
		migrateAll      bool
		createWorkspace bool
	)

	cmd := &cobra.Command{
		Use:     "auto",
		Short:   "Automatically migrate current Terraform project to KeyHarbour",
		Aliases: []string{"project"},
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			cfg, _ := config.Load()
			client := khclient.New(cfg)

			// Handle rollback mode
			if rollback {
				return performRollback(rollbackFrom, dir, printer)
			}

			// Initialize migration report
			report := &MigrationReport{
				StartTime:  time.Now(),
				Project:    project,
				Module:     module,
				Workspaces: []WorkspaceMigration{},
			}
			var projectUUID string

			// Resolve config from flags/env/config
			if khEndpoint == "" {
				khEndpoint = config.FromEnvOr(cfg, "KH_ENDPOINT", "https://api.keyharbour.test")
			}
			if khOrg == "" {
				khOrg = config.FromEnvOr(cfg, "KH_ORG", "")
			}
			if khProject == "" {
				khProject = config.FromEnvOr(cfg, "KH_PROJECT", "")
			}
			client.Endpoint = khEndpoint
			client.Org = khOrg
			client.Token = config.FromEnvOr(cfg, "KH_TOKEN", cfg.Token)
			projectRef := project
			if projectRef == "" {
				projectRef = khProject
			}
			if projectRef == "" {
				return exitcodes.With(exitcodes.ValidationError, errors.New("--project is required (or set KH_PROJECT)"))
			}

			resolver := clientReferenceResolver{client: client}
			ctxResolveProj, cancelProj := context.WithTimeout(cmd.Context(), 30*time.Second)
			proj, err := resolver.ResolveProject(ctxResolveProj, projectRef)
			cancelProj()
			if err != nil {
				return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("failed to resolve KeyHarbour project %q: %w", projectRef, err))
			}
			projectUUID = proj.UUID
			projectName := proj.Name
			if projectName == "" {
				projectName = projectRef
			}
			project = projectName
			report.Project = projectName

			// Step 1: Detect current backend
			logging.Debugf("Detecting backend in %s", dir)
			backendCfg, err := detectBackend(dir)
			if err != nil {
				return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("failed to detect backend: %w", err))
			}

			logging.Debugf("Detected backend: type=%s file=%s", backendCfg.Type, backendCfg.FilePath)
			report.BackendType = backendCfg.Type

			// Step 2: Discover workspaces (batch mode or single)
			var workspaces []string
			if !batchMode {
				if workspace == "" {
					if v := os.Getenv("KH_WORKSPACE"); v != "" {
						workspace = v
					}
				}
			}
			if envName == "" {
				if v := os.Getenv("KH_ENVIRONMENT"); v != "" {
					envName = v
				}
			}
			if batchMode {
				workspaces, err = discoverWorkspaces(dir, backendCfg)
				if err != nil {
					return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("failed to discover workspaces: %w", err))
				}
				logging.Debugf("Discovered %d workspaces for batch migration", len(workspaces))
			} else {
				workspaces = []string{workspace}
				if workspace == "" {
					workspaces = []string{"default"}
				}
			}

			// Setup backup directory
			if backupDir == "" {
				backupDir = filepath.Join(dir, ".kh-migrate-backup")
			}
			report.BackupLocation = backupDir

			// For TFC backends, allow overriding org/workspace from flags
			if backendCfg.Type == "tfc" || backendCfg.Type == "cloud" {
				if tfcOrg == "" {
					// Try environment variable
					if v := os.Getenv("TF_CLOUD_ORGANIZATION"); v != "" {
						tfcOrg = v
					}
				}
				if tfcOrg != "" {
					backendCfg.Config["organization"] = tfcOrg
				}
				if tfcWorkspace == "" {
					// Try environment variable
					if v := os.Getenv("TF_WORKSPACE"); v != "" {
						tfcWorkspace = v
					}
				}
			}

			// If --tfc-org is provided, override backend to TFC for source state retrieval
			// (even if local backend was detected)
			if tfcOrg != "" && backendCfg.Type != "tfc" && backendCfg.Type != "cloud" {
				backendCfg.Type = "tfc"
				backendCfg.Config["organization"] = tfcOrg
			}

			// Handle --all flag: list all TFC workspaces and migrate them
			// This works even if local backend is detected - we override to pull from TFC
			if migrateAll {
				if tfcOrg == "" {
					// Try environment variable
					if v := os.Getenv("TF_CLOUD_ORGANIZATION"); v != "" {
						tfcOrg = v
					}
				}
				if tfcOrg == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--tfc-org is required for --all"))
				}

				tfcToken := os.Getenv("TF_API_TOKEN")
				if tfcToken == "" {
					tfcToken = os.Getenv("TFC_TOKEN")
				}
				if tfcToken == "" {
					tfcToken = os.Getenv("TF_TOKEN_app_terraform_io")
				}
				if tfcToken == "" {
					return exitcodes.With(exitcodes.AuthError, errors.New("TF_API_TOKEN required for --all"))
				}

				logging.Debugf("Listing all workspaces in TFC org %s", tfcOrg)
				tfcReader := backend.NewTFCReader("https://app.terraform.io", tfcOrg, "", tfcToken)
				tfcWorkspaces, err := tfcReader.ListAllWorkspaces(cmd.Context())
				if err != nil {
					return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to list TFC workspaces: %w", err))
				}
				logging.Debugf("Found %d TFC workspaces", len(tfcWorkspaces))

				// Use TFC workspace names as the workspaces to migrate
				workspaces = make([]string, len(tfcWorkspaces))
				for i, w := range tfcWorkspaces {
					workspaces[i] = w.Name
				}
				batchMode = true // Enable batch mode for multiple workspaces

				// Override backend to TFC for state retrieval
				backendCfg.Type = "tfc"
				backendCfg.Config["organization"] = tfcOrg
			}

			// Step 3: Migrate each workspace
			for _, ws := range workspaces {
				wsMigration := WorkspaceMigration{
					Workspace: ws,
				}

				// Determine source workspace for state retrieval
				// For TFC, use --tfc-workspace if provided, otherwise fall back to target workspace name
				sourceWorkspace := ws
				if (backendCfg.Type == "tfc" || backendCfg.Type == "cloud") && tfcWorkspace != "" {
					sourceWorkspace = tfcWorkspace
				}

				logging.Debugf("Migrating workspace: %s (source: %s)", ws, sourceWorkspace)

				// Retrieve state from source
				stateData, stateMeta, err := retrieveState(cmd.Context(), backendCfg, sourceWorkspace)
				if err != nil {
					wsMigration.Success = false
					wsMigration.Error = fmt.Sprintf("failed to retrieve state: %v", err)
					report.Workspaces = append(report.Workspaces, wsMigration)
					report.Errors = append(report.Errors, wsMigration.Error)
					if !batchMode {
						return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to retrieve state: %w", err))
					}
					logging.Debugf("Error retrieving workspace %s: %v", ws, err)
					continue
				}

				if len(stateData) == 0 {
					wsMigration.Success = false
					wsMigration.Error = "no state data found"
					report.Workspaces = append(report.Workspaces, wsMigration)
					report.Warnings = append(report.Warnings, fmt.Sprintf("workspace %s: no state data", ws))
					continue
				}

				ctxResolveWs, cancelWs := context.WithTimeout(cmd.Context(), 30*time.Second)
				var workspaceEntity khclient.Workspace
				if createWorkspace {
					// Auto-create workspace if it doesn't exist
					// Sanitize the workspace name for KeyHarbour (letters and numbers only)
					targetWsName := sanitizeWorkspaceName(ws)
					if targetWsName != ws {
						logging.Debugf("Sanitized workspace name: %s -> %s", ws, targetWsName)
					}
					workspaceEntity, _, err = client.GetOrCreateWorkspace(ctxResolveWs, projectUUID, targetWsName)
					cancelWs()
					if err != nil {
						wsMigration.Success = false
						wsMigration.Error = fmt.Sprintf("failed to get/create KeyHarbour workspace %q: %v", targetWsName, err)
						report.Workspaces = append(report.Workspaces, wsMigration)
						report.Errors = append(report.Errors, wsMigration.Error)
						if !batchMode {
							return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("workspace get/create failed: %w", err))
						}
						continue
					}
				} else {
					// Resolve existing workspace only
					workspaceEntity, err = resolver.ResolveWorkspace(ctxResolveWs, projectUUID, ws)
					cancelWs()
					if err != nil {
						wsMigration.Success = false
						wsMigration.Error = fmt.Sprintf("failed to resolve KeyHarbour workspace %q: %v", ws, err)
						report.Workspaces = append(report.Workspaces, wsMigration)
						report.Errors = append(report.Errors, wsMigration.Error)
						if !batchMode {
							return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("workspace resolution failed: %w", err))
						}
						continue
					}
				}
				wsMigration.WorkspaceUUID = workspaceEntity.UUID

				wsMigration.SourceSize = int64(len(stateData))
				wsMigration.SourceChecksum = state.SHA256Hex(stateData)

				// Parse state metadata
				var tfState struct {
					Lineage          string `json:"lineage"`
					Serial           int    `json:"serial"`
					TerraformVersion string `json:"terraform_version"`
					Version          int    `json:"version"`
				}
				_ = json.Unmarshal(stateData, &tfState)
				wsMigration.Lineage = tfState.Lineage
				wsMigration.Serial = tfState.Serial

				// Infer module from state or use flag
				wsModule := module
				if wsModule == "" {
					wsModule = stateMeta.Module
					if wsModule == "" {
						wsModule = "infra"
					}
				}
				if report.Module == "" {
					report.Module = wsModule
				}

				// Derive KeyHarbour state ID
				stateID := fmt.Sprintf("%s-%s-%s", sanitizeID(project), sanitizeID(wsModule), sanitizeID(ws))
				wsMigration.StateID = stateID

				// Step 4: Pre-migration validation
				if validate {
					logging.Debugf("Validating state for workspace %s", ws)
					wsMigration.ValidationPre = validateState(stateData)
					if !wsMigration.ValidationPre.Valid {
						wsMigration.Success = false
						wsMigration.Error = fmt.Sprintf("pre-migration validation failed: %v", wsMigration.ValidationPre.Errors)
						report.Workspaces = append(report.Workspaces, wsMigration)
						report.Errors = append(report.Errors, wsMigration.Error)
						if !batchMode {
							return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("validation failed: %v", wsMigration.ValidationPre.Errors))
						}
						continue
					}
				}

				if dryRun {
					wsMigration.Success = true
					report.Workspaces = append(report.Workspaces, wsMigration)
					continue
				}

				// Step 5: Backup state file
				if !skipBackup {
					stateBackupPath := filepath.Join(backupDir, fmt.Sprintf("%s-%s.tfstate.bak", ws, time.Now().Format("20060102-150405")))
					if err := os.MkdirAll(backupDir, 0o755); err != nil {
						wsMigration.Success = false
						wsMigration.Error = fmt.Sprintf("failed to create backup dir: %v", err)
						report.Workspaces = append(report.Workspaces, wsMigration)
						report.Errors = append(report.Errors, wsMigration.Error)
						if !batchMode {
							return exitcodes.With(exitcodes.BackendIOError, err)
						}
						continue
					}
					if err := os.WriteFile(stateBackupPath, stateData, 0o644); err != nil {
						wsMigration.Success = false
						wsMigration.Error = fmt.Sprintf("failed to backup state: %v", err)
						report.Workspaces = append(report.Workspaces, wsMigration)
						report.Errors = append(report.Errors, wsMigration.Error)
						if !batchMode {
							return exitcodes.With(exitcodes.BackendIOError, err)
						}
						continue
					}
					wsMigration.BackupPath = stateBackupPath
					logging.Debugf("Backed up state to %s", stateBackupPath)
				}

				// Step 6: Upload state to KeyHarbour statefile API
				logging.Debugf("Uploading statefile to project=%s workspace=%s", projectUUID, workspaceEntity.UUID)
				envTag := envName
				if envTag == "" {
					envTag = ws
				}
				ctxUpload, cancelUpload := context.WithTimeout(cmd.Context(), 30*time.Second)
				_, err = client.CreateStatefile(ctxUpload, projectUUID, workspaceEntity.UUID, envTag, khclient.CreateStatefileRequest{Content: string(stateData)})
				cancelUpload()

				if err != nil {
					wsMigration.Success = false
					wsMigration.Error = fmt.Sprintf("failed to upload statefile: %v", err)
					report.Workspaces = append(report.Workspaces, wsMigration)
					report.Errors = append(report.Errors, wsMigration.Error)
					if !batchMode {
						return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to upload statefile: %w", err))
					}
					continue
				}

				wsMigration.TargetChecksum = wsMigration.SourceChecksum
				logging.Debugf("Uploaded statefile for workspace %s", workspaceEntity.Name)

				// Step 7: Post-migration validation
				if validate {
					logging.Debugf("Post-migration validation for workspace %s", ws)
					// Retrieve uploaded statefile and validate
					ctxFetch, cancelFetch := context.WithTimeout(cmd.Context(), 10*time.Second)
					uploadedState, err := client.GetLastStatefile(ctxFetch, projectUUID, workspaceEntity.UUID, envTag)
					cancelFetch()

					if err != nil {
						wsMigration.ValidationPost.Valid = false
						wsMigration.ValidationPost.Errors = []string{fmt.Sprintf("failed to retrieve uploaded statefile: %v", err)}
						report.Warnings = append(report.Warnings, fmt.Sprintf("workspace %s: post-validation skipped", ws))
					} else {
						uploadedData := []byte(uploadedState.Content)
						wsMigration.TargetChecksum = state.SHA256Hex(uploadedData)
						wsMigration.ValidationPost = validateState(uploadedData)
						if !wsMigration.ValidationPost.Valid {
							wsMigration.Success = false
							wsMigration.Error = fmt.Sprintf("post-migration validation failed: %v", wsMigration.ValidationPost.Errors)
							report.Workspaces = append(report.Workspaces, wsMigration)
							report.Errors = append(report.Errors, wsMigration.Error)
							// Attempt rollback
							if wsMigration.BackupPath != "" {
								report.Warnings = append(report.Warnings, fmt.Sprintf("workspace %s: use --rollback to restore", ws))
							}
							if !batchMode {
								return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("post-validation failed: %v", wsMigration.ValidationPost.Errors))
							}
							continue
						}
					}
				}

				wsMigration.Success = true
				report.Workspaces = append(report.Workspaces, wsMigration)
				logging.Debugf("Successfully migrated workspace %s", ws)
			}

			// Step 8: Backup backend config (once, not per workspace)
			if !skipBackup && !dryRun {
				backupPath, err := backupBackendConfig(backendCfg, backupDir)
				if err != nil {
					report.Warnings = append(report.Warnings, fmt.Sprintf("failed to backup backend config: %v", err))
				} else {
					logging.Debugf("Backed up backend config to %s", backupPath)
				}
			}

			// Step 9: Generate new KeyHarbour backend configuration (only if at least one succeeded)
			successCount := 0
			for _, wm := range report.Workspaces {
				if wm.Success {
					successCount++
				}
			}

			if successCount > 0 && !dryRun {
				logging.Debugf("Generating KeyHarbour backend configuration")
				newBackendPath := filepath.Join(dir, "backend.tf")
				newBackendHCLPath := filepath.Join(dir, "backend.hcl")

				if !force {
					if _, err := os.Stat(newBackendPath); err == nil {
						return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("backend.tf already exists (use --force to overwrite)"))
					}
					if _, err := os.Stat(newBackendHCLPath); err == nil {
						return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("backend.hcl already exists (use --force to overwrite)"))
					}
				}

				// Use first successful workspace for backend.hcl generation
				var firstWorkspaceUUID string
				for _, wm := range report.Workspaces {
					if wm.Success {
						firstWorkspaceUUID = wm.WorkspaceUUID
						break
					}
				}

				backendTF := terraformBackendTF()
				backendHCL := terraformBackendHCL(khEndpoint, projectUUID, firstWorkspaceUUID)

				if err := os.WriteFile(newBackendPath, []byte(backendTF), 0o644); err != nil {
					return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to write backend.tf: %w", err))
				}
				if err := os.WriteFile(newBackendHCLPath, []byte(backendHCL), 0o644); err != nil {
					return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to write backend.hcl: %w", err))
				}
			}

			// Finalize report
			report.EndTime = time.Now()
			report.Duration = report.EndTime.Sub(report.StartTime).String()
			report.Success = len(report.Errors) == 0

			// Write report to file if requested
			if reportPath != "" {
				reportData, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("failed to marshal report: %w", err))
				}
				if err := os.WriteFile(reportPath, reportData, 0o644); err != nil {
					return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to write report: %w", err))
				}
				logging.Debugf("Migration report written to %s", reportPath)
			}

			// Output summary
			if batchMode {
				summary := map[string]any{
					"action":          "migrate-batch",
					"total":           len(report.Workspaces),
					"successful":      successCount,
					"failed":          len(report.Workspaces) - successCount,
					"backend_type":    report.BackendType,
					"project":         report.Project,
					"module":          report.Module,
					"backup_location": report.BackupLocation,
					"duration":        report.Duration,
					"dry_run":         dryRun,
				}
				if reportPath != "" {
					summary["report"] = reportPath
				}
				if len(report.Errors) > 0 {
					summary["errors"] = report.Errors
				}
				if len(report.Warnings) > 0 {
					summary["warnings"] = report.Warnings
				}
				if !dryRun && successCount > 0 {
					summary["message"] = "Migration complete! Run: terraform init -reconfigure -backend-config=backend.hcl"
				} else if dryRun {
					summary["message"] = "Dry-run mode: no changes made"
				}
				return printer.JSON(summary)
			}

			// Single workspace output
			if len(report.Workspaces) > 0 {
				wm := report.Workspaces[0]
				result := map[string]any{
					"action":          "migrate",
					"backend_type":    report.BackendType,
					"project":         report.Project,
					"module":          report.Module,
					"workspace":       wm.Workspace,
					"state_id":        wm.StateID,
					"state_size":      wm.SourceSize,
					"state_lineage":   wm.Lineage,
					"state_serial":    wm.Serial,
					"source_checksum": wm.SourceChecksum,
					"target_checksum": wm.TargetChecksum,
					"success":         wm.Success,
					"dry_run":         dryRun,
				}
				if wm.BackupPath != "" {
					result["backup_path"] = wm.BackupPath
				}
				if reportPath != "" {
					result["report"] = reportPath
				}
				if wm.Error != "" {
					result["error"] = wm.Error
					return printer.JSON(result)
				}
				if !dryRun {
					result["message"] = "Migration complete! Run: terraform init -reconfigure -backend-config=backend.hcl"
				} else {
					result["message"] = "Dry-run mode: no changes made"
				}
				return printer.JSON(result)
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Terraform project directory")
	cmd.Flags().StringVar(&project, "project", "", "KeyHarbour project UUID (required, or set KH_PROJECT)")
	cmd.Flags().StringVarP(&module, "module", "m", "", "Module name (auto-detected or defaults to 'infra')")
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace name (auto-detected or defaults to 'default')")
	cmd.Flags().StringVar(&envName, "environment", "", "KeyHarbour environment name (defaults to workspace or KH_ENVIRONMENT)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without making changes")
	cmd.Flags().StringVar(&backupDir, "backup-dir", "", "Backup directory (defaults to .kh-migrate-backup)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing files")
	cmd.Flags().BoolVar(&skipBackup, "skip-backup", false, "Skip backing up current backend config")
	cmd.Flags().StringVar(&khEndpoint, "endpoint", "", "KeyHarbour API endpoint")
	cmd.Flags().StringVar(&khOrg, "org", "", "KeyHarbour organization")
	cmd.Flags().StringVar(&khProject, "kh-project", "", "KeyHarbour project (alternative to --project)")
	cmd.Flags().BoolVar(&batchMode, "batch", false, "Migrate all workspaces (discovers from terraform.tfstate.d/)")
	cmd.Flags().BoolVar(&validate, "validate", false, "Validate state before and after migration")
	cmd.Flags().StringVar(&reportPath, "report", "", "Write detailed migration report to file (JSON)")
	cmd.Flags().BoolVar(&rollback, "rollback", false, "Rollback migration from backup")
	cmd.Flags().StringVar(&rollbackFrom, "rollback-from", "", "Backup directory to rollback from (defaults to .kh-migrate-backup)")

	// TFC source options (for migrating from Terraform Cloud)
	cmd.Flags().StringVar(&tfcOrg, "tfc-org", "", "Terraform Cloud organization (overrides detected config, or TF_CLOUD_ORGANIZATION)")
	cmd.Flags().StringVar(&tfcWorkspace, "tfc-workspace", "", "Terraform Cloud workspace name (overrides detected config, or TF_WORKSPACE)")

	// Bulk migration options
	cmd.Flags().BoolVar(&migrateAll, "all", false, "Migrate all workspaces from TFC organization")
	cmd.Flags().BoolVar(&createWorkspace, "create-workspace", false, "Auto-create workspace in KeyHarbour if it doesn't exist")

	return cmd
}

func newMigrateBackendCmd() *cobra.Command {
	var from, to string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "backend",
		Short: "Migrate backend --from ... --to ... (legacy command)",
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if dryRun {
				return printer.JSON(map[string]any{"action": "migrate", "from": from, "to": to, "dry_run": true})
			}
			return fmt.Errorf("migrate backend not implemented yet; use 'kh migrate auto' instead")
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Source backend")
	cmd.Flags().StringVar(&to, "to", "", "Target backend")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without writing")

	return cmd
}

// detectBackend scans .tf files in dir for backend configuration
func detectBackend(dir string) (*BackendConfig, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.tf"))
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		content := string(data)

		// Simple regex-based detection (works for most common cases)
		// Format: terraform { backend "TYPE" { ... } }
		backendRE := regexp.MustCompile(`(?s)backend\s+"([^"]+)"\s*\{([^}]*)\}`)
		matches := backendRE.FindStringSubmatch(content)
		if len(matches) >= 2 {
			backendType := matches[1]
			backendBody := ""
			if len(matches) > 2 {
				backendBody = matches[2]
			}

			cfg := &BackendConfig{
				Type:     backendType,
				Config:   parseBackendBlock(backendBody),
				FilePath: f,
			}
			return cfg, nil
		}

		// Check for cloud block (Terraform Cloud)
		cloudRE := regexp.MustCompile(`(?s)cloud\s*\{([^}]*)\}`)
		if cloudRE.MatchString(content) {
			matches := cloudRE.FindStringSubmatch(content)
			cloudBody := ""
			if len(matches) > 1 {
				cloudBody = matches[1]
			}
			cfg := &BackendConfig{
				Type:     "tfc",
				Config:   parseBackendBlock(cloudBody),
				FilePath: f,
			}
			return cfg, nil
		}
	}

	// No backend found in config files; assume local backend
	return &BackendConfig{
		Type:     "local",
		Config:   map[string]string{"path": "terraform.tfstate"},
		FilePath: filepath.Join(dir, "terraform.tfstate"),
	}, nil
}

// parseBackendBlock extracts key-value pairs from backend block (simple HCL parsing)
func parseBackendBlock(body string) map[string]string {
	config := make(map[string]string)
	// Match: key = "value" or key = value
	kvRE := regexp.MustCompile(`(\w+)\s*=\s*"([^"]*)"`)
	matches := kvRE.FindAllStringSubmatch(body, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			config[match[1]] = match[2]
		}
	}
	return config
}

// retrieveState fetches state data from the detected backend
func retrieveState(ctx context.Context, cfg *BackendConfig, workspace string) ([]byte, backend.Object, error) {
	var reader backend.Reader

	switch cfg.Type {
	case "local":
		// Local backend: read from file
		path := cfg.Config["path"]
		if path == "" {
			path = "terraform.tfstate"
		}
		if workspace != "" && workspace != "default" {
			path = filepath.Join("terraform.tfstate.d", workspace, "terraform.tfstate")
		}
		reader = backend.NewLocalReader(path, nil)

	case "http":
		// HTTP backend: use address from config
		address := cfg.Config["address"]
		if address == "" {
			return nil, backend.Object{}, errors.New("http backend missing 'address' in config")
		}
		reader = backend.NewHTTPReader(address)

	case "tfc", "cloud":
		// Terraform Cloud: extract org and workspace
		org := cfg.Config["organization"]
		ws := cfg.Config["name"] // from workspaces { name = "..." }
		if ws == "" {
			ws = workspace
		}
		if ws == "" {
			return nil, backend.Object{}, errors.New("terraform cloud backend requires workspace name")
		}

		// Get token from env
		token := os.Getenv("TF_API_TOKEN")
		if token == "" {
			token = os.Getenv("TFC_TOKEN")
		}
		if token == "" {
			token = os.Getenv("TF_TOKEN_app_terraform_io")
		}
		if token == "" {
			return nil, backend.Object{}, errors.New("terraform cloud requires TF_API_TOKEN or TFC_TOKEN")
		}

		reader = backend.NewTFCReader("https://app.terraform.io", org, ws, token)

	case "s3", "azurerm", "gcs":
		return nil, backend.Object{}, fmt.Errorf("backend type '%s' not yet supported; export state manually with: terraform state pull > state.tfstate", cfg.Type)

	default:
		return nil, backend.Object{}, fmt.Errorf("unsupported backend type: %s", cfg.Type)
	}

	// List and get first state
	objs, err := reader.List(ctx)
	if err != nil {
		return nil, backend.Object{}, err
	}
	if len(objs) == 0 {
		return nil, backend.Object{}, errors.New("no state found in backend")
	}

	// Get the state data
	data, obj, err := reader.Get(ctx, objs[0].Key)
	if err != nil {
		return nil, backend.Object{}, err
	}

	// Parse state to extract metadata
	var tfState struct {
		Lineage string `json:"lineage"`
		Serial  int    `json:"serial"`
	}
	if err := json.Unmarshal(data, &tfState); err == nil {
		obj.Module = tfState.Lineage // Store lineage in Module field for now
	}

	return data, obj, nil
}

// backupBackendConfig creates a timestamped backup of the current backend config
func backupBackendConfig(cfg *BackendConfig, backupDir string) (string, error) {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("backend-%s-%s.tf.bak", cfg.Type, timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	// Read original backend file
	data, err := os.ReadFile(cfg.FilePath)
	if err != nil {
		// If file doesn't exist (e.g., local backend), create a note
		if os.IsNotExist(err) {
			note := fmt.Sprintf("# Backup of %s backend\n# Original state location: %s\n# Backed up at: %s\n",
				cfg.Type, cfg.FilePath, time.Now().Format(time.RFC3339))
			return backupPath, os.WriteFile(backupPath, []byte(note), 0o644)
		}
		return "", err
	}

	// Write backup
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return "", err
	}

	cfg.BackupPath = backupPath
	return backupPath, nil
}

// sanitizeID converts a string to a safe identifier for KeyHarbour state IDs
func sanitizeID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// sanitizeWorkspaceName converts a TFC workspace name to a KeyHarbour-compatible name
// KeyHarbour only allows letters and numbers in workspace names
// e.g., "app-staging" -> "appstaging", "infra_shared" -> "infrashared"
func sanitizeWorkspaceName(s string) string {
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// discoverWorkspaces finds all workspaces in a Terraform project
func discoverWorkspaces(dir string, cfg *BackendConfig) ([]string, error) {
	var workspaces []string

	switch cfg.Type {
	case "local":
		// Check for terraform.tfstate.d/ directory (workspace states)
		wsDir := filepath.Join(dir, "terraform.tfstate.d")
		if _, err := os.Stat(wsDir); err == nil {
			entries, err := os.ReadDir(wsDir)
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				if entry.IsDir() {
					stateFile := filepath.Join(wsDir, entry.Name(), "terraform.tfstate")
					if _, err := os.Stat(stateFile); err == nil {
						workspaces = append(workspaces, entry.Name())
					}
				}
			}
		}
		// Always include default workspace if terraform.tfstate exists
		if _, err := os.Stat(filepath.Join(dir, "terraform.tfstate")); err == nil {
			workspaces = append(workspaces, "default")
		}

	case "http", "tfc", "cloud":
		// For remote backends, we can only migrate the explicitly specified workspace
		// unless the backend supports listing (future enhancement)
		return nil, fmt.Errorf("batch mode not supported for %s backend; specify --workspace explicitly", cfg.Type)

	default:
		return nil, fmt.Errorf("workspace discovery not supported for backend type: %s", cfg.Type)
	}

	if len(workspaces) == 0 {
		workspaces = append(workspaces, "default")
	}

	return workspaces, nil
}

// validateState performs integrity checks on Terraform state data
func validateState(stateData []byte) ValidationResult {
	result := ValidationResult{
		Valid:  true,
		Checks: []string{},
		Errors: []string{},
	}

	// Check if valid JSON
	var tfState struct {
		Version          int    `json:"version"`
		TerraformVersion string `json:"terraform_version"`
		Lineage          string `json:"lineage"`
		Serial           int    `json:"serial"`
		Resources        []any  `json:"resources"`
	}

	if err := json.Unmarshal(stateData, &tfState); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("invalid JSON: %v", err))
		return result
	}

	result.Checks = append(result.Checks, "valid JSON format")
	result.StateVersion = tfState.Version
	result.TerraformVersion = tfState.TerraformVersion

	// Check state version
	if tfState.Version < 3 || tfState.Version > 4 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("unusual state version: %d (expected 3 or 4)", tfState.Version))
	} else {
		result.Checks = append(result.Checks, fmt.Sprintf("state version %d", tfState.Version))
	}

	// Check lineage exists
	if tfState.Lineage == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "missing lineage")
	} else {
		result.Checks = append(result.Checks, "lineage present")
	}

	// Check serial is non-negative
	if tfState.Serial < 0 {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("invalid serial: %d", tfState.Serial))
	} else {
		result.Checks = append(result.Checks, fmt.Sprintf("serial %d", tfState.Serial))
	}

	// Check terraform version format
	if tfState.TerraformVersion == "" {
		result.Warnings = append(result.Warnings, "missing terraform_version")
	} else {
		result.Checks = append(result.Checks, fmt.Sprintf("terraform version %s", tfState.TerraformVersion))
	}

	// Check state size
	if len(stateData) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "empty state")
	} else if len(stateData) > 100*1024*1024 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("large state file: %d MB", len(stateData)/(1024*1024)))
	}

	result.Checks = append(result.Checks, fmt.Sprintf("state size: %d bytes", len(stateData)))

	return result
}

// performRollback restores backend and state files from backup
func performRollback(backupFrom, dir string, printer output.Printer) error {
	if backupFrom == "" {
		backupFrom = filepath.Join(dir, ".kh-migrate-backup")
	}

	logging.Debugf("Rolling back from backup: %s", backupFrom)

	// Check if backup directory exists
	if _, err := os.Stat(backupFrom); os.IsNotExist(err) {
		return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("backup directory not found: %s", backupFrom))
	}

	// Find backup files
	backupFiles, err := filepath.Glob(filepath.Join(backupFrom, "*.bak"))
	if err != nil {
		return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to list backups: %w", err))
	}

	if len(backupFiles) == 0 {
		return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("no backup files found in %s", backupFrom))
	}

	restored := []string{}
	errors := []string{}

	// Restore state files
	for _, backupFile := range backupFiles {
		basename := filepath.Base(backupFile)

		// Determine original file path
		var originalPath string
		if strings.Contains(basename, "backend-") {
			// Backend config backup: backend-TYPE-TIMESTAMP.tf.bak
			originalPath = filepath.Join(dir, "backend.tf")
		} else {
			// State file backup: WORKSPACE-TIMESTAMP.tfstate.bak
			parts := strings.Split(basename, "-")
			if len(parts) >= 2 {
				workspace := parts[0]
				if workspace == "default" {
					originalPath = filepath.Join(dir, "terraform.tfstate")
				} else {
					originalPath = filepath.Join(dir, "terraform.tfstate.d", workspace, "terraform.tfstate")
				}
			}
		}

		if originalPath == "" {
			errors = append(errors, fmt.Sprintf("could not determine original path for %s", basename))
			continue
		}

		// Read backup
		data, err := os.ReadFile(backupFile)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to read %s: %v", basename, err))
			continue
		}

		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
			errors = append(errors, fmt.Sprintf("failed to create dir for %s: %v", originalPath, err))
			continue
		}

		// Restore file
		if err := os.WriteFile(originalPath, data, 0o644); err != nil {
			errors = append(errors, fmt.Sprintf("failed to restore %s: %v", originalPath, err))
			continue
		}

		restored = append(restored, originalPath)
		logging.Debugf("Restored %s from %s", originalPath, backupFile)
	}

	// Remove new backend files if they exist
	newBackendFiles := []string{
		filepath.Join(dir, "backend.tf"),
		filepath.Join(dir, "backend.hcl"),
	}
	for _, f := range newBackendFiles {
		if _, err := os.Stat(f); err == nil {
			// Check if this was NOT in the original backup (i.e., it's a new file from migration)
			isNew := true
			for _, restored := range restored {
				if restored == f {
					isNew = false
					break
				}
			}
			if isNew {
				if err := os.Remove(f); err != nil {
					errors = append(errors, fmt.Sprintf("failed to remove %s: %v", f, err))
				} else {
					logging.Debugf("Removed new file: %s", f)
				}
			}
		}
	}

	result := map[string]any{
		"action":   "rollback",
		"backup":   backupFrom,
		"restored": restored,
	}

	if len(errors) > 0 {
		result["errors"] = errors
		result["success"] = false
	} else {
		result["success"] = true
		result["message"] = fmt.Sprintf("Rollback complete: restored %d file(s)", len(restored))
	}

	return printer.JSON(result)
}
