package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"kh/internal/config"
	"kh/internal/exitcodes"
	"kh/internal/khclient"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

type statefileTarget struct {
	project   string
	workspace string
}

func (t statefileTarget) resolve(ctx context.Context, resolver referenceResolver, cfg config.Config) (string, string, error) {
	projectRef := projectRefOrEnv(t.project, cfg)
	if projectRef == "" {
		return "", "", exitcodes.With(exitcodes.ValidationError, errors.New("--project is required (or set KH_PROJECT)"))
	}
	workspaceRef := t.workspace
	if workspaceRef == "" {
		return "", "", exitcodes.With(exitcodes.ValidationError, errors.New("--workspace is required"))
	}
	project, err := resolver.ResolveProject(ctx, projectRef)
	if err != nil {
		return "", "", err
	}
	workspace, err := resolver.ResolveWorkspace(ctx, project.UUID, workspaceRef)
	if err != nil {
		return "", "", err
	}
	return project.UUID, workspace.UUID, nil
}

func newStatefilesCmd() *cobra.Command {
	target := &statefileTarget{}
	cmd := &cobra.Command{
		Use:   "statefiles",
		Short: "Manage workspace statefiles",
	}
	cmd.PersistentFlags().StringVar(&target.project, "project", "", "Project UUID (or KH_PROJECT)")
	cmd.PersistentFlags().StringVar(&target.workspace, "workspace", "", "Workspace name or UUID")
	cmd.AddCommand(newStatefilesListCmd(target))
	cmd.AddCommand(newStatefilesLastCmd(target))
	cmd.AddCommand(newStatefilesGetCmd(target))
	cmd.AddCommand(newStatefilesPushCmd(target))
	cmd.AddCommand(newStatefilesDeleteCmd(target))
	cmd.AddCommand(newStatefilesDeleteAllCmd(target))
	return cmd
}

func newStatefilesListCmd(target *statefileTarget) *cobra.Command {
	var format, environment string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List statefiles for a workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			project, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			items, err := client.ListStatefiles(ctx, project, workspace, environment)
			if err != nil {
				return err
			}
			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}
			headers := []string{"UUID", "PUBLISHED_AT", "ENVIRONMENT", "BYTES"}
			rows := make([][]string, 0, len(items))
			for _, sf := range items {
				rows = append(rows, []string{
					sf.UUID,
					sf.PublishedAt.Format(time.RFC3339),
					sf.Environment,
					fmt.Sprint(len(sf.Content)),
				})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVar(&environment, "environment", "", "Filter by environment name")
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json (overrides global)")
	return cmd
}

func newStatefilesLastCmd(target *statefileTarget) *cobra.Command {
	var environment string
	var raw bool
	cmd := &cobra.Command{
		Use:   "last",
		Short: "Show the latest statefile for a workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			project, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			item, err := client.GetLastStatefile(ctx, project, workspace, environment)
			if err != nil {
				return err
			}
			if raw {
				fmt.Fprint(cmd.OutOrStdout(), item.Content)
				if len(item.Content) > 0 && item.Content[len(item.Content)-1] != '\n' {
					fmt.Fprint(cmd.OutOrStdout(), "\n")
				}
				return nil
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(item)
		},
	}
	cmd.Flags().StringVar(&environment, "environment", "", "Filter by environment name")
	cmd.Flags().BoolVar(&raw, "raw", false, "Print raw statefile content")
	return cmd
}

func newStatefilesGetCmd(target *statefileTarget) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "get <uuid>",
		Short: "Show a specific statefile",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return exitcodes.With(exitcodes.ValidationError, errors.New("statefiles get requires exactly one argument: <uuid>"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			project, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			item, err := client.GetStatefile(ctx, project, workspace, args[0])
			if err != nil {
				return err
			}
			if raw {
				fmt.Fprint(cmd.OutOrStdout(), item.Content)
				if len(item.Content) > 0 && item.Content[len(item.Content)-1] != '\n' {
					fmt.Fprint(cmd.OutOrStdout(), "\n")
				}
				return nil
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(item)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Print raw statefile content")
	return cmd
}

func newStatefilesPushCmd(target *statefileTarget) *cobra.Command {
	var filePath string
	var fromStdin bool
	var environment string
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Upload a new statefile version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if filePath == "" && !fromStdin {
				return exitcodes.With(exitcodes.ValidationError, errors.New("provide --file or --stdin for statefiles push"))
			}
			if filePath != "" && fromStdin {
				return exitcodes.With(exitcodes.ValidationError, errors.New("--file and --stdin are mutually exclusive"))
			}
			var data []byte
			var err error
			if filePath != "" {
				data, err = os.ReadFile(filePath)
			} else {
				data, err = io.ReadAll(cmd.InOrStdin())
			}
			if err != nil {
				return err
			}
			if len(data) == 0 {
				return exitcodes.With(exitcodes.ValidationError, errors.New("statefile content is empty"))
			}

			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			project, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			resp, err := client.CreateStatefile(ctx, project, workspace, environment, khclient.CreateStatefileRequest{Content: string(data)})
			if err != nil {
				return err
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(struct {
				Action      string `json:"action"`
				Status      string `json:"status"`
				Bytes       int    `json:"bytes"`
				Environment string `json:"environment,omitempty"`
			}{
				Action:      "push",
				Status:      resp.Status,
				Bytes:       len(data),
				Environment: environment,
			})
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a Terraform state file")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read statefile content from stdin")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment tag for the statefile")
	return cmd
}

func newStatefilesDeleteCmd(target *statefileTarget) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <uuid>",
		Short: "Delete a single statefile version",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return exitcodes.With(exitcodes.ValidationError, errors.New("statefiles rm requires exactly one argument: <uuid>"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			project, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			if err := client.DeleteStatefile(ctx, project, workspace, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "statefile %s deleted\n", args[0])
			return nil
		},
	}
	return cmd
}

func newStatefilesDeleteAllCmd(target *statefileTarget) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm-all",
		Short: "Delete all statefiles for a workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return exitcodes.With(exitcodes.ValidationError, errors.New("refusing to delete all statefiles without --force"))
			}
			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			project, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			if err := client.DeleteStatefiles(ctx, project, workspace); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "all statefiles deleted")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm deletion of all statefiles")
	return cmd
}
