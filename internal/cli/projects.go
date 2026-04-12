package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"kh/internal/config"
	"kh/internal/exitcodes"
	"kh/internal/khclient"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Inspect KeyHarbour projects",
		Long: `Inspect projects stored in KeyHarbour.

Subcommands:
  show   Show a project's details (use KH_PROJECT or pass the UUID directly)`,
	}
	cmd.AddCommand(newProjectsListCmd())
	cmd.AddCommand(newProjectsShowCmd())
	return cmd
}

func newProjectsListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:    "ls",
		Short:  "List projects",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Friendly guidance since /v1/projects (index) isn't implemented in the API spec
			msg := "projects listing is not supported by the server API yet. Use 'kh project show <uuid>' or 'kh workspace ls --project <uuid>'."
			return exitcodes.With(exitcodes.ValidationError, errors.New(msg))
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json (overrides global)")
	return cmd
}

func newProjectsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [project-uuid]",
		Short: "Show a project's details",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("projects show accepts at most one argument: <project-uuid>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()

			// Use argument or fall back to KH_PROJECT
			projectRef := ""
			if len(args) == 1 {
				projectRef = args[0]
			} else {
				projectRef = cfg.Project
			}

			if projectRef == "" {
				return fmt.Errorf("project uuid is required: provide as argument or set KH_PROJECT")
			}

			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			proj, err := resolveProjectRef(ctx, client, projectRef)
			if err != nil {
				return err
			}
			if detail, err := client.GetProject(ctx, proj.UUID); err == nil {
				proj = detail
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(proj)
		},
	}
	return cmd
}
