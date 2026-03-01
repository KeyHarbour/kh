package cli

import (
	"context"
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
	"kh/internal/workerpool"

	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	// Source flags
	var from string
	var localPath string
	var httpURL string
	var tfcOrg string
	var tfcWorkspace string
	var tfcHost string
	var tfcToken string

	// Destination flags
	var to string
	var outPath string
	var destHTTPURL string
	var destTFCOrg string
	var destTFCWorkspace string
	var destTFCHost string
	var destTFCToken string

	// KeyHarbour specific
	var project string
	var workspace string
	var env string
	var createWorkspace bool
	var srcWorkspace string // for --from=keyharbour
	var srcProject string   // for --from=keyharbour
	var stateID string      // for --from=keyharbour with specific state

	// Operation flags
	var dryRun bool
	var verifyChecksum bool
	var overwrite bool
	var lock bool
	var verifyAfterUpload bool
	var workspacePattern string
	var concurrency int
	var idempotencyKey string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Terraform state between backends",
		Long: `Sync reads state from a source backend and writes it to a destination backend.

Sources  (--from): local, http, tfc, keyharbour
Destinations (--to): keyharbour, file, http, tfc  (default: keyharbour)

Examples:
  # From local file to KeyHarbour
  kh sync --from=local --path=./terraform.tfstate --project=<uuid> --workspace=prod

  # From Terraform Cloud to KeyHarbour (auto-create workspace)
  kh sync --from=tfc --tfc-org=my-org --tfc-workspace=ws-name --project=<uuid> --create-workspace

  # From KeyHarbour to local file
  kh sync --from=keyharbour --src-project=<uuid> --src-workspace=prod --to=file --out=./backup.tfstate

  # From KeyHarbour to Terraform Cloud
  kh sync --from=keyharbour --src-project=<uuid> --src-workspace=ws1 \
    --to=tfc --dest-tfc-org=my-org --dest-tfc-workspace=ws-name

  # From HTTP backend to local file
  kh sync --from=http --url=https://old-backend.com/state --to=file --out=./imported.tfstate

  # Between two KeyHarbour workspaces
  kh sync --from=keyharbour --src-project=<proj1> --src-workspace=ws1 \
    --to=keyharbour --project=<proj2> --workspace=ws2

  # Dry-run: preview what would be synced
  kh sync --from=tfc --tfc-org=my-org --tfc-workspace=ws --project=<uuid> --dry-run
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithEnv()
			if err != nil {
				return exitcodes.With(exitcodes.UnknownError, err)
			}

			if concurrency == 0 {
				concurrency = cfg.Concurrency
			}
			if concurrency == 0 {
				concurrency = 4
			}

			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}

			// Validate source
			if from == "" {
				return exitcodes.With(exitcodes.ValidationError, errors.New("--from is required (local|http|tfc|keyharbour)"))
			}

			// Default destination to keyharbour for backward compatibility
			if to == "" {
				to = "keyharbour"
			}

			// Setup workspace pattern regex if provided
			var wsRE *regexp.Regexp
			if workspacePattern != "" {
				wsRE, err = regexp.Compile(workspacePattern)
				if err != nil {
					return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("invalid --workspace-pattern: %w", err))
				}
			}

			ctx := cmd.Context()
			client := khclient.New(cfg)

			// Setup Reader
			var r backend.Reader
			switch from {
			case "local":
				if localPath == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--path is required for --from=local"))
				}
				r = backend.NewLocalReader(localPath, wsRE)
			case "http":
				if httpURL == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--url is required for --from=http"))
				}
				r = backend.NewHTTPReader(httpURL)
			case "tfc":
				if tfcOrg == "" {
					tfcOrg = os.Getenv("TF_CLOUD_ORGANIZATION")
				}
				if tfcWorkspace == "" {
					tfcWorkspace = os.Getenv("TF_WORKSPACE")
				}
				if tfcToken == "" {
					if v := os.Getenv("TF_API_TOKEN"); v != "" {
						tfcToken = v
					} else if v := os.Getenv("TFC_TOKEN"); v != "" {
						tfcToken = v
					} else if v := os.Getenv("TF_TOKEN_app_terraform_io"); v != "" {
						tfcToken = v
					}
				}
				if tfcOrg == "" || tfcWorkspace == "" || tfcToken == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--tfc-org, --tfc-workspace and a token (TF_API_TOKEN/TFC_TOKEN) are required for --from=tfc"))
				}
				r = backend.NewTFCReader(tfcHost, tfcOrg, tfcWorkspace, tfcToken)
			case "keyharbour":
				if cfg.Token == "" {
					return exitcodes.With(exitcodes.AuthError, errors.New("not logged in: KH_TOKEN is missing"))
				}
				r = backend.NewKeyHarbourReader(client, srcProject, srcWorkspace, stateID, env)
			default:
				return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("unsupported --from: %s (supported: local,http,tfc,keyharbour)", from))
			}

			// Setup Writer
			var w backend.Writer
			var destProj *khclient.Project
			switch to {
			case "keyharbour":
				if cfg.Token == "" {
					return exitcodes.With(exitcodes.AuthError, errors.New("not logged in: KH_TOKEN is missing"))
				}
				projectRef := projectRefOrEnv(project, cfg)
				if projectRef == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--project is required for --to=keyharbour"))
				}
				proj, err := resolveProjectRef(ctx, client, projectRef)
				if err != nil {
					return exitcodes.With(exitcodes.ValidationError, err)
				}
				destProj = &proj
				w = backend.NewKeyHarbourWriter(client, destProj.UUID, workspace, createWorkspace)
			case "file":
				if outPath == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--out is required for --to=file"))
				}
				w = &backend.LocalWriter{}
			case "http":
				if destHTTPURL == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--dest-url is required for --to=http"))
				}
				headers := map[string]string{}
				if idempotencyKey != "" {
					headers["Idempotency-Key"] = idempotencyKey
				}
				w = backend.NewHTTPWriterWithHeaders(destHTTPURL, headers)
			case "tfc":
				if destTFCOrg == "" {
					if v := os.Getenv("TF_CLOUD_ORGANIZATION"); v != "" {
						destTFCOrg = v
					}
				}
				if destTFCWorkspace == "" {
					destTFCWorkspace = os.Getenv("TF_WORKSPACE")
				}
				if destTFCToken == "" {
					if v := os.Getenv("TF_API_TOKEN"); v != "" {
						destTFCToken = v
					} else if v := os.Getenv("TFC_TOKEN"); v != "" {
						destTFCToken = v
					} else if v := os.Getenv("TF_TOKEN_app_terraform_io"); v != "" {
						destTFCToken = v
					}
				}
				if destTFCHost == "" {
					destTFCHost = "https://app.terraform.io"
				}
				if destTFCOrg == "" || destTFCWorkspace == "" || destTFCToken == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--dest-tfc-org, --dest-tfc-workspace and a token are required for --to=tfc"))
				}
				w = backend.NewTFCWriter(destTFCHost, destTFCOrg, destTFCWorkspace, destTFCToken)
			default:
				return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("unsupported --to: %s (supported: keyharbour,file,http,tfc)", to))
			}

			// List objects from source
			fetchCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			objs, err := r.List(fetchCtx)
			if err != nil {
				return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to list source objects: %w", err))
			}
			if len(objs) == 0 {
				return exitcodes.With(exitcodes.BackendIOError, errors.New("no state files found in source"))
			}

			logging.Debugf("Found %d objects in source", len(objs))

			// Validate workspace constraints
			if to == "keyharbour" && workspace != "" && len(objs) > 1 {
				return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("source has %d items but --workspace specified a single target. Remove --workspace to infer names, or ensure source has only 1 item.", len(objs)))
			}

			if dryRun {
				summary := map[string]interface{}{
					"action":      "sync",
					"from":        from,
					"to":          to,
					"count":       len(objs),
					"dry_run":     true,
					"concurrency": concurrency,
				}
				if to == "keyharbour" && destProj != nil {
					summary["project"] = destProj.Name
					summary["workspace"] = workspace
					summary["environment"] = env
				}
				items := make([]map[string]interface{}, len(objs))
				for i, obj := range objs {
					items[i] = map[string]interface{}{
						"key":       obj.Key,
						"workspace": obj.Workspace,
						"size":      obj.Size,
					}
				}
				summary["items"] = items
				return printer.JSON(summary)
			}

			// Process objects with concurrency
			results := workerpool.Run(objs, concurrency, func(obj backend.Object) error {
				// Read data
				data, meta, err := r.Get(ctx, obj.Key)
				if err != nil {
					return fmt.Errorf("failed to read %s: %w", obj.Key, err)
				}

				// Verify checksum if requested
				if verifyChecksum && meta.Checksum != "" {
					calc := state.SHA256Hex(data)
					if calc != meta.Checksum {
						return fmt.Errorf("checksum mismatch for %s", meta.Key)
					}
				}

				// Determine write key based on destination
				writeKey := obj.Key
				if to == "keyharbour" {
					// Use workspace name for KeyHarbour
					targetWorkspaceName := workspace
					if targetWorkspaceName == "" {
						if obj.Workspace != "" {
							targetWorkspaceName = obj.Workspace
						} else {
							return fmt.Errorf("cannot determine target workspace for %s (use --workspace)", obj.Key)
						}
					}
					targetWorkspaceName = validateAndSanitizeWorkspaceName(targetWorkspaceName, cmd.ErrOrStderr())
					writeKey = targetWorkspaceName
				} else if to == "file" {
					// Use template substitution for file output
					writeKey = outPath
					writeKey = strings.ReplaceAll(writeKey, "{workspace}", obj.Workspace)
					writeKey = strings.ReplaceAll(writeKey, "{key}", filepath.Base(obj.Key))
				}

				// Lock if requested (only for KeyHarbour sources)
				if lock && from == "keyharbour" && meta.Module != "" {
					if err := client.AcquireLock(ctx, meta.Module); err != nil {
						return fmt.Errorf("failed to acquire lock for %s: %w", meta.Key, err)
					}
					defer func() { _ = client.ReleaseLock(ctx, meta.Module, false) }()
				}

				// Write
				logging.Debugf("Writing %s to %s (key=%s, %d bytes)", obj.Key, to, writeKey, len(data))
				written, err := w.Put(ctx, writeKey, data, overwrite)
				if err != nil {
					return fmt.Errorf("failed to write %s: %w", writeKey, err)
				}

				// Verify after upload for HTTP destinations
				if verifyAfterUpload && to == "http" && written.Checksum != "" {
					calc := state.SHA256Hex(data)
					if calc != written.Checksum {
						return fmt.Errorf("upload verification failed for %s: checksum mismatch", writeKey)
					}
				}

				logging.Debugf("Successfully synced %s -> %s (%d bytes)", obj.Key, writeKey, len(data))
				return nil
			})

			// Collect results
			successes := 0
			failures := []map[string]string{}
			for i, res := range results {
				if res.Err != nil {
					failures = append(failures, map[string]string{
						"key":   objs[i].Key,
						"error": res.Err.Error(),
					})
				} else {
					successes++
				}
			}

			summary := map[string]interface{}{
				"action":  "sync",
				"from":    from,
				"to":      to,
				"total":   len(objs),
				"success": successes,
				"failed":  len(failures),
			}
			if len(failures) > 0 {
				summary["failures"] = failures
			}

			if len(failures) > 0 {
				// Return with error but still show summary
				if err := printer.JSON(summary); err != nil {
					return err
				}
				return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("%d/%d operations failed", len(failures), len(objs)))
			}

			return printer.JSON(summary)
		},
	}

	// Source flags
	cmd.Flags().StringVar(&from, "from", "", "Source backend: local|http|tfc|keyharbour")
	cmd.Flags().StringVar(&localPath, "path", "", "Local file or directory for --from=local")
	cmd.Flags().StringVar(&httpURL, "url", "", "Source URL for --from=http")
	cmd.Flags().StringVar(&tfcOrg, "tfc-org", "", "Terraform Cloud organization for source")
	cmd.Flags().StringVar(&tfcWorkspace, "tfc-workspace", "", "Terraform Cloud workspace for source")
	cmd.Flags().StringVar(&tfcHost, "tfc-host", "https://app.terraform.io", "Terraform Cloud host URL for source")
	cmd.Flags().StringVar(&tfcToken, "tfc-token", "", "Terraform Cloud API token for source")
	cmd.Flags().StringVar(&srcProject, "src-project", "", "Source KeyHarbour project (for --from=keyharbour)")
	cmd.Flags().StringVar(&srcWorkspace, "src-workspace", "", "Source KeyHarbour workspace (for --from=keyharbour)")
	cmd.Flags().StringVar(&stateID, "state-id", "", "Specific state ID (for --from=keyharbour)")

	// Destination flags
	cmd.Flags().StringVar(&to, "to", "", "Destination backend: keyharbour|file|http|tfc (default: keyharbour)")
	cmd.Flags().StringVar(&project, "project", "", "Target KeyHarbour project (for --to=keyharbour)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Target KeyHarbour workspace (for --to=keyharbour)")
	cmd.Flags().StringVar(&env, "env", "", "Filter statefiles by environment name (for --from=keyharbour)")
	cmd.Flags().BoolVar(&createWorkspace, "create-workspace", false, "Create workspace if it does not exist (for --to=keyharbour)")
	cmd.Flags().StringVar(&outPath, "out", "", "Output path for --to=file (supports {workspace} and {key} templates)")
	cmd.Flags().StringVar(&destHTTPURL, "dest-url", "", "Destination URL for --to=http")
	cmd.Flags().StringVar(&destTFCOrg, "dest-tfc-org", "", "Terraform Cloud organization for destination")
	cmd.Flags().StringVar(&destTFCWorkspace, "dest-tfc-workspace", "", "Terraform Cloud workspace for destination")
	cmd.Flags().StringVar(&destTFCHost, "dest-tfc-host", "https://app.terraform.io", "Terraform Cloud host URL for destination")
	cmd.Flags().StringVar(&destTFCToken, "dest-tfc-token", "", "Terraform Cloud API token for destination")

	// Operation flags
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without writing")
	cmd.Flags().BoolVar(&verifyChecksum, "verify-checksum", false, "Verify checksums during sync")
	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "Allow overwriting existing files/states")
	cmd.Flags().BoolVar(&lock, "lock", false, "Acquire advisory lock during sync (for --from=keyharbour)")
	cmd.Flags().BoolVar(&verifyAfterUpload, "verify-after-upload", true, "Verify upload for HTTP destinations")
	cmd.Flags().StringVar(&workspacePattern, "workspace-pattern", "", "Workspace regex filter (for --from=local)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Parallelism for operations (defaults from KH_CONCURRENCY)")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Idempotency-Key header (for --to=http)")

	return cmd
}
