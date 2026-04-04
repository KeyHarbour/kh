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
		workspaceRef = config.FromEnvOr(cfg, "KH_WORKSPACE", "")
	}
	if workspaceRef == "" {
		return "", "", exitcodes.With(exitcodes.ValidationError, errors.New("--workspace is required (or set KH_WORKSPACE)"))
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
		Use:   "version",
		Short: "Manage workspace statefile versions",
		Long: `Manage Terraform statefile versions stored in a KeyHarbour workspace.

Commands that operate on the workspace collection (ls, last, push, rm-all)
require --project and --workspace (or KH_PROJECT / KH_WORKSPACE env vars).

Commands that operate on a specific version (get, rm) only require the
statefile UUID — no --project or --workspace flags needed.`,
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
		Long: `List statefile versions for a workspace, ordered newest first.

Requires --project and --workspace (or KH_PROJECT / KH_WORKSPACE).
Use --environment to filter by environment name.

Examples:
  kh tf version ls --project <uuid> --workspace <uuid>
  kh tf version ls --project <uuid> --workspace prod -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			_, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			items, err := client.ListStatefiles(ctx, workspace, environment)
			if err != nil {
				return err
			}
			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}
			headers := []string{"UUID", "PUBLISHED_AT", "BYTES"}
			rows := make([][]string, 0, len(items))
			for _, sf := range items {
				rows = append(rows, []string{
					sf.UUID,
					sf.PublishedAt.Format(time.RFC3339),
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
	var raw bool
	cmd := &cobra.Command{
		Use:   "last",
		Short: "Show the latest statefile for a workspace",
		Long: `Retrieve the most recently uploaded statefile for a workspace.

Requires --project and --workspace (or KH_PROJECT / KH_WORKSPACE).

Examples:
  kh tf version last --project <uuid> --workspace <uuid>
  kh tf version last --project <uuid> --workspace prod --raw`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			_, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			item, err := client.GetLastStatefile(ctx, workspace)
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
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(item)
			}
			return printer.Table(
				[]string{"UUID", "PUBLISHED AT", "BYTES"},
				[][]string{{item.UUID, item.PublishedAt.Format(time.RFC3339), fmt.Sprint(len(item.Content))}},
			)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Print raw statefile content")
	return cmd
}

func newStatefilesGetCmd(target *statefileTarget) *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "get <uuid>",
		Short: "Show a specific statefile by UUID",
		Long: `Retrieve a specific statefile version by its UUID.

No --project or --workspace flags are required; the UUID uniquely identifies
the statefile. Use 'kh tf version ls' to find UUIDs.

Examples:
  kh tf version get <uuid>
  kh tf version get <uuid> --raw`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return exitcodes.With(exitcodes.ValidationError, errors.New("statefiles get requires exactly one argument: <uuid>"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			item, err := client.GetStatefile(ctx, args[0])
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
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(item)
			}
			return printer.Table(
				[]string{"UUID", "PUBLISHED AT", "BYTES"},
				[][]string{{item.UUID, item.PublishedAt.Format(time.RFC3339), fmt.Sprint(len(item.Content))}},
			)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Print raw statefile content")
	return cmd
}

func newStatefilesPushCmd(target *statefileTarget) *cobra.Command {
	var filePath string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "push",
		Short: "Upload a new statefile version to a workspace",
		Long: `Upload a Terraform state file as a new version in a workspace.

Requires --project and --workspace (or KH_PROJECT / KH_WORKSPACE).
Provide the state data via --file or --stdin.

Examples:
  kh tf version push --project <uuid> --workspace <uuid> --file ./terraform.tfstate
  terraform state pull | kh tf version push --project <uuid> --workspace prod --stdin`,
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

			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			_, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			resp, err := client.CreateStatefile(ctx, workspace, khclient.CreateStatefileRequest{Content: string(data)})
			if err != nil {
				return err
			}
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(struct {
					Action string `json:"action"`
					Status string `json:"status"`
					Bytes  int    `json:"bytes"`
				}{Action: "push", Status: resp.Status, Bytes: len(data)})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Statefile pushed (%d bytes)\n", len(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a Terraform state file")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read statefile content from stdin")
	return cmd
}

func newStatefilesDeleteCmd(target *statefileTarget) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <uuid>",
		Short: "Delete a specific statefile version by UUID",
		Long: `Delete a specific statefile version by its UUID.

No --project or --workspace flags are required; the UUID uniquely identifies
the statefile. Use 'kh tf version ls' to find UUIDs.

Examples:
  kh tf version rm <uuid>`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return exitcodes.With(exitcodes.ValidationError, errors.New("statefiles rm requires exactly one argument: <uuid>"))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if err := client.DeleteStatefile(ctx, args[0]); err != nil {
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
		Short: "Delete all statefile versions for a workspace",
		Long: `Delete every statefile version stored in a workspace. This is irreversible.

Requires --project and --workspace (or KH_PROJECT / KH_WORKSPACE).
Pass --force to confirm — the command refuses to proceed without it.

Examples:
  kh tf version rm-all --project <uuid> --workspace <uuid> --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				return exitcodes.With(exitcodes.ValidationError, errors.New("refusing to delete all statefiles without --force"))
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			resolver := clientReferenceResolver{client: client}
			_, workspace, err := target.resolve(ctx, resolver, cfg)
			if err != nil {
				return err
			}
			if err := client.DeleteStatefiles(ctx, workspace); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "all statefiles deleted")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm deletion of all statefiles")
	return cmd
}
