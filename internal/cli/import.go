package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kh/internal/backend"
	"kh/internal/config"
	"kh/internal/exitcodes"
	"kh/internal/logging"
	"kh/internal/output"
	"kh/internal/state"
	"kh/internal/workerpool"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	var from string
	var dryRun bool
	var concurrency int
	var verifyChecksum bool
	var project, module, env, workspacePattern string
	var report string
	var localPath string
	var httpURL string
	var tfcOrg string
	var tfcWorkspace string
	var tfcHost string
	var tfcToken string
	var outPath string
	var overwrite bool
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import data into Key-Harbour",
	}
	// Subcommand: tfstate
	tfstate := &cobra.Command{
		Use:   "tfstate",
		Short: "Import Terraform state from backends",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			if concurrency == 0 {
				concurrency = cfg.Concurrency
			}
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}

			var wsRE *regexp.Regexp
			var err error
			if workspacePattern != "" {
				wsRE, err = regexp.Compile(workspacePattern)
				if err != nil {
					return fmt.Errorf("invalid --workspace-pattern: %w", err)
				}
			}

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

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			objs, err := r.List(ctx)
			if err != nil {
				return err
			}
			logging.Debugf("import source=%s concurrency=%d items=%d", from, concurrency, len(objs))
			// Summarize
			sum := struct {
				Action      string           `json:"action"`
				Source      string           `json:"source"`
				Count       int              `json:"count"`
				Bytes       int64            `json:"bytes"`
				Items       []backend.Object `json:"items"`
				DryRun      bool             `json:"dry_run"`
				Project     string           `json:"project"`
				Module      string           `json:"module"`
				Env         string           `json:"env"`
				WorkspaceRe string           `json:"workspace_pattern"`
			}{Action: "import", Source: from, DryRun: dryRun, Project: project, Module: module, Env: env, WorkspaceRe: workspacePattern}
			for _, o := range objs {
				sum.Count++
				sum.Bytes += o.Size
			}
			sum.Items = objs

			if dryRun {
				return printer.JSON(sum)
			}
			// Concurrently fetch and verify
			results := workerpool.Run(objs, concurrency, func(o backend.Object) error {
				logging.Debugf("reading %s", o.URL)
				data, obj, err := r.Get(ctx, o.Key)
				if err != nil {
					return err
				}
				if verifyChecksum && obj.Checksum != "" {
					calc := state.SHA256Hex(data)
					if calc != obj.Checksum {
						return fmt.Errorf("checksum mismatch for %s", obj.Key)
					}
				}
				// Optional: write to file if --out provided
				if outPath != "" {
					path := outPath
					ws := obj.Workspace
					if ws == "" {
						ws = "default"
					}
					path = strings.ReplaceAll(path, "{workspace}", ws)
					// if source object has a key/module, allow {key} placeholder
					if obj.Key != "" {
						path = strings.ReplaceAll(path, "{key}", sanitizePath(obj.Key))
					}
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						return err
					}
					if !overwrite {
						if _, err := os.Stat(path); err == nil {
							return fmt.Errorf("file exists: %s (use --overwrite)", path)
						}
					}
					if err := os.WriteFile(path, data, 0o644); err != nil {
						return err
					}
					logging.Debugf("wrote file %s bytes=%d", path, len(data))
					return printer.JSON(map[string]any{
						"read":     obj.URL,
						"written":  path,
						"bytes":    obj.Size,
						"checksum": obj.Checksum,
					})
				}
				// TODO: send to KH ingest API when available
				logging.Debugf("read ok url=%s bytes=%d checksum=%s", obj.URL, obj.Size, obj.Checksum)
				return printer.JSON(map[string]any{
					"read":     obj.URL,
					"bytes":    obj.Size,
					"checksum": obj.Checksum,
				})
			})
			for _, r := range results {
				if r.Err != nil {
					return r.Err
				}
			}

			if report != "" {
				if err := writeImportReport(report, sum); err != nil {
					return fmt.Errorf("failed to write import report: %w", err)
				}
			}

			if outPath != "" || report != "" {
				return nil
			}

			// KH ingest not implemented yet
			return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("Key-Harbour API client not implemented: cannot import yet"))
		},
	}

	cmd.PersistentFlags().IntVar(&concurrency, "concurrency", 0, "Parallelism for I/O operations")

	tfstate.Flags().StringVar(&from, "from", "", "Source backend: http|local|tfc")
	tfstate.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without writing")
	tfstate.Flags().StringVar(&project, "project", "", "Key-Harbour project")
	tfstate.Flags().StringVar(&module, "module", "", "Module identifier (e.g. repo/path)")
	tfstate.Flags().StringVar(&env, "env", "", "Environment metadata")
	tfstate.Flags().StringVar(&workspacePattern, "workspace-pattern", ".*", "Workspace detection regex")
	tfstate.Flags().StringVar(&report, "report", "", "Write machine-readable report to file")
	tfstate.Flags().StringVar(&localPath, "path", "", "Local file or directory for --from=local")
	tfstate.Flags().StringVar(&httpURL, "url", "", "Source URL for --from=http")
	// Terraform Cloud options for --from=tfc
	tfstate.Flags().StringVar(&tfcOrg, "tfc-org", "", "Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)")
	tfstate.Flags().StringVar(&tfcWorkspace, "tfc-workspace", "", "Terraform Cloud workspace name (or TF_WORKSPACE)")
	tfstate.Flags().StringVar(&tfcHost, "tfc-host", "https://app.terraform.io", "Terraform Cloud host URL")
	tfstate.Flags().StringVar(&tfcToken, "tfc-token", "", "Terraform Cloud API token (or TF_API_TOKEN/TFC_TOKEN)")
	tfstate.Flags().BoolVar(&verifyChecksum, "verify-checksum", false, "Verify checksums before ingest")
	tfstate.Flags().StringVar(&outPath, "out", "", "Optional file path template to save downloaded state (supports {workspace} and {key})")
	tfstate.Flags().BoolVar(&overwrite, "overwrite", false, "Allow overwriting existing files when using --out")

	cmd.AddCommand(tfstate)
	return cmd
}

// sanitizePath converts arbitrary keys/URLs into safe file name components.
func sanitizePath(s string) string {
	// Replace common separators and URL fragments
	s = strings.ReplaceAll(s, "://", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "&", "_")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func writeImportReport(path string, data any) error {
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
