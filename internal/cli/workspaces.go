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
	cmd.AddCommand(newWorkspacesCreateCmd(opts))
	cmd.AddCommand(newWorkspacesUpdateCmd(opts))
	cmd.AddCommand(newWorkspacesDeleteCmd(opts))
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
			cfg, _ := config.LoadWithEnv()
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

func newWorkspacesCreateCmd(opts *workspaceCmdOpts) *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new workspace in a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
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
			ws, err := client.CreateWorkspace(ctx, project.UUID, khclient.CreateWorkspaceRequest{
				Name:        args[0],
				Description: description,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Workspace %q created (uuid: %s).\n", ws.Name, ws.UUID)
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Workspace description")
	return cmd
}

func newWorkspacesUpdateCmd(opts *workspaceCmdOpts) *cobra.Command {
	var name, description string
	cmd := &cobra.Command{
		Use:   "update <name-or-uuid>",
		Short: "Update a workspace name or description",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("description") {
				return fmt.Errorf("at least one of --name or --description is required")
			}
			cfg, _ := config.LoadWithEnv()
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
			// Fetch current values to fill in any unset fields
			if current, err := client.GetWorkspace(ctx, workspace.UUID); err == nil {
				if !cmd.Flags().Changed("name") {
					name = current.Name
				}
				if !cmd.Flags().Changed("description") {
					description = current.Description
				}
			}
			if err := client.UpdateWorkspace(ctx, workspace.UUID, khclient.UpdateWorkspaceRequest{
				Name:        name,
				Description: description,
			}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Workspace %q updated.\n", workspace.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New workspace name")
	cmd.Flags().StringVar(&description, "description", "", "New workspace description")
	return cmd
}

func newWorkspacesDeleteCmd(opts *workspaceCmdOpts) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <name-or-uuid>",
		Short: "Delete a workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete workspace %q? This cannot be undone. Pass --force to confirm.\n", args[0])
				return nil
			}
			cfg, _ := config.LoadWithEnv()
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
			if err := client.DeleteWorkspace(ctx, workspace.UUID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Workspace %q deleted.\n", workspace.Name)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm deletion without prompting")
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
			cfg, _ := config.LoadWithEnv()
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
			if detail, err := client.GetWorkspace(ctx, workspace.UUID); err == nil {
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
