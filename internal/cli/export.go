package cli

import (
	"context"
	"errors"
	"fmt"
	"kh/internal/backend"
	"kh/internal/config"
	"kh/internal/exitcodes"
	"kh/internal/khclient"
	"kh/internal/logging"
	"kh/internal/output"
	"kh/internal/state"
	"kh/internal/workerpool"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newExportCmd() *cobra.Command {
	var to, format, outPath, httpURL string
	var dryRun, verifyChecksum, overwrite bool
	var idempotencyKey string
	var project, module, workspace, stateID string
	var concurrency int
	var lock bool

	cmd := &cobra.Command{Use: "export", Short: "Export data from Key-Harbour"}

	tfstate := &cobra.Command{
		Use:   "tfstate",
		Short: "Export Terraform state to file or backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if dryRun {
				return printer.JSON(map[string]any{
					"action":          "export",
					"target":          to,
					"project":         project,
					"module":          module,
					"workspace":       workspace,
					"state_id":        stateID,
					"format":          format,
					"out":             outPath,
					"verify_checksum": verifyChecksum,
					"overwrite":       overwrite,
					"idempotency_key": idempotencyKey,
					"lock":            lock,
					"dry_run":         true,
				})
			}

			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			if concurrency == 0 {
				concurrency = config.FromEnvOrInt(cfg, "KH_CONCURRENCY", cfg.Concurrency)
			}
			var metas []khclient.StateMeta
			if stateID != "" {
				_, meta, err := client.GetStateRaw(ctx, stateID)
				logging.Debugf("export filters: project=%q module=%q workspace=%q stateID=%q to=%q", project, module, workspace, stateID, to)
				if err != nil {
					return err
				}
				metas = []khclient.StateMeta{meta}
			} else {
				list, err := client.ListStates(ctx, khclient.ListStatesRequest{Project: project, Module: module, Workspace: workspace})
				if err != nil {
					return err
				}
				metas = list
			}
			logging.Debugf("export metas: %d states to process (concurrency=%d)", len(metas), concurrency)

			var w backend.Writer
			switch to {
			case "file":
				if outPath == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--out is required for --to=file"))
				}
				w = &backend.LocalWriter{}
			case "http":
				if httpURL == "" {
					return exitcodes.With(exitcodes.ValidationError, errors.New("--url is required for --to=http"))
				}
				headers := map[string]string{}
				if idempotencyKey != "" {
					headers["Idempotency-Key"] = idempotencyKey
				}
				w = backend.NewHTTPWriterWithHeaders(httpURL, headers)
			default:
				return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("unsupported --to: %s (supported: file,http)", to))
			}

			results := workerpool.Run(metas, concurrency, func(meta khclient.StateMeta) error {
				if lock {
					if err := client.AcquireLock(ctx, meta.ID); err != nil {
						return err
					}
					defer func() { _ = client.ReleaseLock(ctx, meta.ID, false) }()
				}
				key := outPath
				if to == "http" {
					key = httpURL
				}
				key = strings.ReplaceAll(key, "{module}", meta.Module)
				ws := meta.Workspace
				if ws == "" {
					ws = "default"
				}
				key = strings.ReplaceAll(key, "{workspace}", ws)
				logging.Debugf("exporting state id=%s -> %s", meta.ID, key)

				b, _, err := client.GetStateRaw(ctx, meta.ID)
				if err != nil {
					return err
				}
				if verifyChecksum && meta.Checksum != "" {
					sum := state.SHA256Hex(b)
					if sum != meta.Checksum {
						return fmt.Errorf("checksum mismatch for %s", meta.ID)
					}
				}

				obj, err := w.Put(ctx, key, b, overwrite)
				if err != nil {
					return err
				}
				if verifyChecksum {
					sum := state.SHA256Hex(b)
					if sum != obj.Checksum {
						return fmt.Errorf("write checksum mismatch for %s", key)
					}
				}
				logging.Debugf("exported -> url=%s bytes=%d checksum=%s", obj.URL, obj.Size, obj.Checksum)

				return printer.JSON(map[string]any{
					"written":  obj.URL,
					"bytes":    obj.Size,
					"checksum": obj.Checksum,
				})
			})
			for _, r := range results {
				if r.Err != nil {
					return r.Err
				}
			}
			return nil
		},
	}

	tfstate.Flags().StringVar(&to, "to", "file", "Target: file|s3|gcs|azblob|http")
	tfstate.Flags().StringVar(&httpURL, "url", "", "Target URL for --to=http")
	tfstate.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without writing")
	tfstate.Flags().StringVar(&format, "format", "v4", "State format: v4")
	tfstate.Flags().StringVar(&outPath, "out", "", "Output path when --to=file")
	tfstate.Flags().StringVar(&project, "project", "", "Filter by project")
	tfstate.Flags().StringVar(&module, "module", "", "Filter by module")
	tfstate.Flags().StringVar(&workspace, "workspace", "", "Filter by workspace")
	tfstate.Flags().StringVar(&stateID, "state-id", "", "Select by state ID")
	tfstate.Flags().BoolVar(&verifyChecksum, "verify-checksum", false, "Verify checksums on export")
	tfstate.Flags().BoolVar(&overwrite, "overwrite", false, "Overwrite existing files/targets")
	tfstate.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Set Idempotency-Key header for HTTP targets")
	tfstate.Flags().IntVar(&concurrency, "concurrency", 0, "Parallelism for I/O operations (defaults from KH_CONCURRENCY)")
	tfstate.Flags().BoolVar(&lock, "lock", false, "Acquire advisory lock per state during export")

	cmd.AddCommand(tfstate)
	return cmd
}
