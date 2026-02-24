package cli

import (
	"context"
	"encoding/json"
	"errors"
	"kh/internal/backend"
	"kh/internal/output"
	"os"
	"time"

	"github.com/spf13/cobra"
)

func newTFCCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tfc",
		Short: "Terraform Cloud utilities",
	}
	cmd.AddCommand(newTFCUploadStateCmd())
	cmd.AddCommand(newTFCListWorkspacesCmd())
	return cmd
}

func newTFCUploadStateCmd() *cobra.Command {
	var filePath string
	var org, workspace, host, token string
	var adoptLineage bool

	cmd := &cobra.Command{
		Use:   "upload-state",
		Short: "Upload a local .tfstate file as a new state version to Terraform Cloud",
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if filePath == "" {
				return errors.New("--file is required")
			}
			if org == "" {
				org = os.Getenv("TF_CLOUD_ORGANIZATION")
			}
			if workspace == "" {
				workspace = os.Getenv("TF_WORKSPACE")
			}
			if token == "" {
				if v := os.Getenv("TF_API_TOKEN"); v != "" {
					token = v
				}
				if token == "" {
					if v := os.Getenv("TFC_TOKEN"); v != "" {
						token = v
					}
				}
				if token == "" {
					if v := os.Getenv("TF_TOKEN_app_terraform_io"); v != "" {
						token = v
					}
				}
			}
			if host == "" {
				host = "https://app.terraform.io"
			}
			if org == "" || workspace == "" || token == "" {
				return errors.New("--tfc-org, --tfc-workspace and a token (TF_API_TOKEN/TFC_TOKEN) are required")
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			// Optionally adopt the current workspace lineage to avoid 409 conflicts
			if adoptLineage {
				r := backend.NewTFCReader(host, org, workspace, token)
				ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
				defer cancel()
				curr, _, err := r.Get(ctx, workspace)
				if err == nil && len(curr) > 0 {
					var currMeta struct {
						Lineage string `json:"lineage"`
						Serial  int    `json:"serial"`
					}
					_ = json.Unmarshal(curr, &currMeta)
					if currMeta.Lineage != "" {
						var local map[string]any
						if err := json.Unmarshal(data, &local); err == nil {
							local["lineage"] = currMeta.Lineage
							if currMeta.Serial > 0 {
								// bump serial to be safe
								local["serial"] = currMeta.Serial + 1
							}
							if b, err := json.Marshal(local); err == nil {
								data = b
							}
						}
					}
				}
			}

			w := backend.NewTFCWriter(host, org, workspace, token)
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			obj, err := w.Put(ctx, workspace, data, true)
			if err != nil {
				return err
			}
			return printer.JSON(map[string]any{
				"action":    "tfc.upload-state",
				"org":       org,
				"workspace": workspace,
				"bytes":     obj.Size,
				"checksum":  obj.Checksum,
				"url":       obj.URL,
				"file":      filePath,
			})
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to local .tfstate file")
	cmd.Flags().StringVar(&org, "tfc-org", "", "Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)")
	cmd.Flags().StringVar(&workspace, "tfc-workspace", "", "Terraform Cloud workspace name (or TF_WORKSPACE)")
	cmd.Flags().StringVar(&host, "tfc-host", "https://app.terraform.io", "Terraform Cloud host URL")
	cmd.Flags().StringVar(&token, "tfc-token", "", "Terraform Cloud API token (or TF_API_TOKEN/TFC_TOKEN)")
	cmd.Flags().BoolVar(&adoptLineage, "adopt-lineage", false, "Fetch current workspace lineage and adopt it into the local state before upload (resolves 409 lineage conflicts)")
	return cmd
}

func newTFCListWorkspacesCmd() *cobra.Command {
	var org, host, token string

	cmd := &cobra.Command{
		Use:   "list-workspaces",
		Short: "List all workspaces in a Terraform Cloud organization",
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}

			if org == "" {
				org = os.Getenv("TF_CLOUD_ORGANIZATION")
			}
			if token == "" {
				if v := os.Getenv("TF_API_TOKEN"); v != "" {
					token = v
				}
				if token == "" {
					if v := os.Getenv("TFC_TOKEN"); v != "" {
						token = v
					}
				}
				if token == "" {
					if v := os.Getenv("TF_TOKEN_app_terraform_io"); v != "" {
						token = v
					}
				}
			}
			if host == "" {
				host = "https://app.terraform.io"
			}
			if org == "" || token == "" {
				return errors.New("--tfc-org and a token (TF_API_TOKEN/TFC_TOKEN) are required")
			}

			r := backend.NewTFCReader(host, org, "", token)
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			workspaces, err := r.ListAllWorkspaces(ctx)
			if err != nil {
				return err
			}

			// Convert to output format
			wsOutput := make([]map[string]string, len(workspaces))
			for i, ws := range workspaces {
				wsOutput[i] = map[string]string{
					"id":   ws.ID,
					"name": ws.Name,
				}
			}

			return printer.JSON(map[string]any{
				"organization": org,
				"count":        len(workspaces),
				"workspaces":   wsOutput,
			})
		},
	}
	cmd.Flags().StringVar(&org, "tfc-org", "", "Terraform Cloud organization (or TF_CLOUD_ORGANIZATION)")
	cmd.Flags().StringVar(&host, "tfc-host", "https://app.terraform.io", "Terraform Cloud host URL")
	cmd.Flags().StringVar(&token, "tfc-token", "", "Terraform Cloud API token (or TF_API_TOKEN/TFC_TOKEN)")
	return cmd
}
