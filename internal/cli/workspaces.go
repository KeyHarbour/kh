package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"kh/internal/config"
	"kh/internal/khclient"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

type workspaceCmdOpts struct {
	project string
}

func newWorkspacesCmd() *cobra.Command {
	opts := &workspaceCmdOpts{}
	cmd := &cobra.Command{
		Use:   "workspaces",
		Short: "Inspect project workspaces",
	}
	cmd.PersistentFlags().StringVar(&opts.project, "project", "", "Project UUID (or KH_PROJECT)")
	cmd.AddCommand(newWorkspacesListCmd(opts))
	cmd.AddCommand(newWorkspacesShowCmd(opts))
	return cmd
}

func (o *workspaceCmdOpts) projectRef(cfg config.Config) (string, error) {
	ref := projectRefOrEnv(o.project, cfg)
	if ref == "" {
		return "", errors.New("--project is required (or set KH_PROJECT)")
	}
	return ref, nil
}

func newWorkspacesListCmd(opts *workspaceCmdOpts) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List workspaces for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			ref, err := opts.projectRef(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			project, err := resolveProjectRef(ctx, client, ref)
			if err != nil {
				return err
			}
			items, err := client.ListWorkspaces(ctx, project.UUID)
			if err != nil {
				return err
			}
			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}
			headers := []string{"UUID", "NAME"}
			rows := make([][]string, 0, len(items))
			for _, w := range items {
				rows = append(rows, []string{w.UUID, w.Name})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json (overrides global)")
	return cmd
}

func newWorkspacesShowCmd(opts *workspaceCmdOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name-or-uuid>",
		Short: "Show workspace details",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("workspaces show requires exactly one argument: <name-or-uuid>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			ref, err := opts.projectRef(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			project, err := resolveProjectRef(ctx, client, ref)
			if err != nil {
				return err
			}
			workspace, err := resolveWorkspaceRef(ctx, client, project.UUID, args[0])
			if err != nil {
				return err
			}
			if detail, err := client.GetWorkspace(ctx, project.UUID, workspace.UUID); err == nil {
				workspace = detail
			}
			payload := struct {
				Project   khclient.Project   `json:"project"`
				Workspace khclient.Workspace `json:"workspace"`
			}{Project: project, Workspace: workspace}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(payload)
		},
	}
	return cmd
}
