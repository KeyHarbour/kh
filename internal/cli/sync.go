package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"kh/internal/backend"
	"kh/internal/config"
	"kh/internal/exitcodes"
	"kh/internal/khclient"
	"kh/internal/logging"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var from string
	var localPath string
	var httpURL string
	var tfcOrg string
	var tfcWorkspace string
	var tfcHost string
	var tfcToken string
	var project string
	var workspace string
	var env string
	var createWorkspace bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync state from a backend to Key-Harbour",
		Long: `Sync reads state from a source backend (local, http, or Terraform Cloud) and uploads it directly to a KeyHarbour workspace (as a new state version).

Unlike 'migrate', this command does not modify any local files. It is purely for uploading state data.

Examples:
  # Sync a local file to a specific workspace
  kh sync --from=local --path=./terraform.tfstate --project=my-project --workspace=prod

  # Sync from HTTP backend
  kh sync --from=http --url=https://old-backend.com/state --project=my-project --workspace=prod

  # Sync from Terraform Cloud
  kh sync --from=tfc --tfc-org=my-org --tfc-workspace=ws-name --project=my-project --create-workspace
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return exitcodes.With(exitcodes.UnknownError, err)
			}

			endpoint := config.FromEnvOr(cfg, "KH_ENDPOINT", cfg.Endpoint)
			token := config.FromEnvOr(cfg, "KH_TOKEN", cfg.Token)

			// 1. Setup Client
			client := khclient.New(config.Config{Endpoint: endpoint, Token: token})
			if token == "" {
				return exitcodes.With(exitcodes.AuthError, errors.New("not logged in: KH_TOKEN is missing"))
			}

			// 2. Resolve Project
			projectRef := projectRefOrEnv(project, cfg)
			if projectRef == "" {
				return exitcodes.With(exitcodes.ValidationError, errors.New("--project is required"))
			}
			ctx := cmd.Context()
			proj, err := resolveProjectRef(ctx, client, projectRef)
			if err != nil {
				return exitcodes.With(exitcodes.ValidationError, err)
			}

			// 3. Setup Reader
			var r backend.Reader
			switch from {
			case "local":
				if localPath == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--path is required for --from=local"))
				}
				r = backend.NewLocalReader(localPath, nil)
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
					// common envs: TF_API_TOKEN (preferred), TFC_TOKEN, TF_TOKEN_app_terraform_io
					if v := os.Getenv("TF_API_TOKEN"); v != "" {
						tfcToken = v
					}
					if tfcToken == "" {
						if v := os.Getenv("TFC_TOKEN"); v != "" {
							tfcToken = v
						}
					}
					if tfcToken == "" {
						if v := os.Getenv("TF_TOKEN_app_terraform_io"); v != "" {
							tfcToken = v
						}
					}
				}
				if tfcOrg == "" || tfcWorkspace == "" || tfcToken == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--tfc-org, --tfc-workspace and a token (TF_API_TOKEN/TFC_TOKEN) are required for --from=tfc"))
				}
				r = backend.NewTFCReader(tfcHost, tfcOrg, tfcWorkspace, tfcToken)
			default:
				return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("unsupported --from: %s (supported: local,http,tfc)", from))
			}

			// 4. List Objects from Source
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

			// If explicitly syncing ONE workspace, we expect 1 object, or we must map them.
			// Currently, we'll iterate and sync all found, but we need to know the target workspace name for each.
			// If --workspace is provided, implies 1 destination.
			if workspace != "" && len(objs) > 1 {
				return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("source has %d items but --workspace specified a single target. Remove --workspace to infer names, or ensure source has only 1 item.", len(objs)))
			}

			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			results := []map[string]string{}

			for _, obj := range objs {
				// Read data
				data, _, err := r.Get(ctx, obj.Key)
				if err != nil {
					return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to read %s: %w", obj.Key, err))
				}

				targetWorkspaceName := workspace
				if targetWorkspaceName == "" {
					if obj.Workspace != "" {
						targetWorkspaceName = obj.Workspace
					} else {
						// Fallback: use file name or key logic if needed, but for now error if ambiguous
						return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("cannot determine target workspace for %s (use --workspace)", obj.Key))
					}
				}

targetWorkspaceName = validateAndSanitizeWorkspaceName(targetWorkspaceName, cmd.ErrOrStderr())

				// Resolve target workspace
				var wsUUID string
				// Try to find
				ws, err := resolveWorkspaceRef(ctx, client, proj.UUID, targetWorkspaceName)
				if err != nil {
					// Check if not found
					if strings.Contains(err.Error(), "not found") {
						if createWorkspace {
							logging.Debugf("Creating workspace: %s", targetWorkspaceName)
							newWs, createErr := client.CreateWorkspace(ctx, proj.UUID, khclient.CreateWorkspaceRequest{Name: targetWorkspaceName})
							if createErr != nil {
								return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to create workspace %s: %w", targetWorkspaceName, createErr))
							}
							wsUUID = newWs.UUID
						} else {
							return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("workspace %q not found (use --create-workspace to create)", targetWorkspaceName))
						}
					} else {
						return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to resolve workspace %s: %w", targetWorkspaceName, err))
					}
				} else {
					wsUUID = ws.UUID
				}

				// Upload
				envTag := env
				if envTag == "" {
					// Auto-detect environment from project instead of using "default"
					if len(proj.Environments) > 0 {
						envTag = proj.Environments[0]
						logging.Debugf("No --env specified, using project's first environment: %s", envTag)
					} else {
						envTag = "default"
					}
				}

				logging.Debugf("Uploading state to project=%s workspace=%s (%s) env=%s", proj.Name, targetWorkspaceName, wsUUID, envTag)
				_, err = client.CreateStatefile(ctx, proj.UUID, wsUUID, envTag, khclient.CreateStatefileRequest{Content: string(data)})
				if err != nil {
					return exitcodes.With(exitcodes.BackendIOError, fmt.Errorf("failed to upload state: %w", err))
				}

				results = append(results, map[string]string{
					"source":    obj.Key,
					"target_ws": targetWorkspaceName,
					"bytes":     fmt.Sprintf("%d", len(data)),
					"status":    "synced",
				})
			}

			return printer.JSON(results)
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Source backend: local|http|tfc")
	cmd.Flags().StringVar(&localPath, "path", "", "Local file or directory for --from=local")
	cmd.Flags().StringVar(&httpURL, "url", "", "Source URL for --from=http")
	cmd.Flags().StringVar(&tfcOrg, "tfc-org", "", "Terraform Cloud organization")
	cmd.Flags().StringVar(&tfcWorkspace, "tfc-workspace", "", "Terraform Cloud workspace")
	cmd.Flags().StringVar(&tfcHost, "tfc-host", "https://app.terraform.io", "Terraform Cloud host URL")
	cmd.Flags().StringVar(&tfcToken, "tfc-token", "", "Terraform Cloud API token")

	cmd.Flags().StringVar(&project, "project", "", "Key-Harbour project name or UUID")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Target workspace name or UUID (optional if inferable)")
	cmd.Flags().BoolVar(&createWorkspace, "create-workspace", false, "Create workspace if it does not exist")
	cmd.Flags().StringVar(&env, "env", "", "Environment tag for state version")

	return cmd
}
