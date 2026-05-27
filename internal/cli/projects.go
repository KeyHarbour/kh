package cli

import (
	"context"
	"fmt"
	"time"

	"kh/internal/config"
	"kh/internal/khclient"
	"kh/internal/kherrors"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage KeyHarbour projects",
		Long: `Manage projects stored in KeyHarbour.

Subcommands:
  ls      List all projects in an organization (requires --org or KH_ORG)
  show    Show a project's details
  create  Create a new project (requires --org or KH_ORG)
  update  Update a project's name or environments`,
	}
	cmd.AddCommand(newProjectsListCmd())
	cmd.AddCommand(newProjectsShowCmd())
	cmd.AddCommand(newProjectsCreateCmd())
	cmd.AddCommand(newProjectsUpdateCmd())
	return cmd
}

func orgUUID(flagValue string, cfg config.Config) string {
	if flagValue != "" {
		return flagValue
	}
	return config.FromEnvOr(cfg, "KH_ORG", "")
}

func newProjectsListCmd() *cobra.Command {
	var org string
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List projects in an organization",
		Long: `List all projects belonging to an organization.

Requires --org (organization UUID) or KH_ORG.

Examples:
  kh project ls --org <org-uuid>
  KH_ORG=<org-uuid> kh project ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			orgRef := orgUUID(org, cfg)
			if orgRef == "" {
				return kherrors.ErrMissingFlag.New("--org is required (or set KH_ORG)")
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			projects, err := client.ListProjects(ctx, orgRef)
			if err != nil {
				return err
			}

			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(projects)
			}

			headers := []string{"NAME", "UUID"}
			rows := make([][]string, 0, len(projects))
			for _, p := range projects {
				rows = append(rows, []string{p.Name, p.UUID})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "Organization UUID (or KH_ORG)")
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json")
	return cmd
}

func newProjectsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [project-uuid]",
		Short: "Show a project's details",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return kherrors.ErrInvalidValue.New("projects show accepts at most one argument: <project-uuid>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()

			projectRef := ""
			if len(args) == 1 {
				projectRef = args[0]
			} else {
				projectRef = cfg.Project
			}

			if projectRef == "" {
				return kherrors.ErrMissingFlag.New("project uuid is required: provide as argument or set KH_PROJECT")
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

func newProjectsCreateCmd() *cobra.Command {
	var org string
	var description string
	var environments []string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new project in an organization",
		Long: `Create a new project under an organization.

Requires --org (organization UUID) or KH_ORG, a positional name argument, and
at least one --environment.

Examples:
  kh project create my-project --org <org-uuid> --environment production
  kh project create my-project --org <org-uuid> --environment production --environment staging
  kh project create my-project --org <org-uuid> --environment production --description "My project"`,
		Args: requireExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			orgRef := orgUUID(org, cfg)
			if orgRef == "" {
				return kherrors.ErrMissingFlag.New("--org is required (or set KH_ORG)")
			}
			if len(environments) == 0 {
				return kherrors.ErrMissingFlag.New("at least one --environment is required")
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			proj, err := client.CreateProject(ctx, orgRef, khclient.CreateProjectRequest{
				Name:             args[0],
				Description:      description,
				EnvironmentNames: environments,
			})
			if err != nil {
				return err
			}
			uuid := proj.UUID
			if uuid == "" {
				uuid = "(see server)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project %q created (uuid: %s).\n", proj.Name, uuid)
			return nil
		},
	}
	cmd.Flags().StringVar(&org, "org", "", "Organization UUID (or KH_ORG)")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().StringArrayVar(&environments, "environment", nil, "Environment name (repeatable, e.g. --environment production --environment staging)")
	return cmd
}

func newProjectsUpdateCmd() *cobra.Command {
	var name string
	var environments []string
	cmd := &cobra.Command{
		Use:   "update <project-uuid>",
		Short: "Update a project's name or environments",
		Long: `Update the name and environment list of an existing project.

The API requires both --name and at least one --environment to be provided.
To keep the current name or environments, fetch them first with 'kh project show'
and pass the same values.

Examples:
  kh project update <uuid> --name new-name --environment production
  kh project update <uuid> --name my-project --environment production --environment staging`,
		Args: requireExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("environment") {
				return kherrors.ErrMissingFlag.New("at least one of --name or --environment is required")
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			proj, err := resolveProjectRef(ctx, client, args[0])
			if err != nil {
				return err
			}

			// Fill in unchanged fields from the current project state.
			if current, err := client.GetProject(ctx, proj.UUID); err == nil {
				if !cmd.Flags().Changed("name") {
					name = current.Name
				}
				if !cmd.Flags().Changed("environment") {
					environments = current.Environments
				}
			}

			if len(environments) == 0 {
				return kherrors.ErrMissingFlag.New("at least one --environment is required (or the current project has none)")
			}

			if err := client.UpdateProject(ctx, proj.UUID, khclient.UpdateProjectRequest{
				Name:             name,
				EnvironmentNames: environments,
			}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project %q updated.\n", proj.UUID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New project name")
	cmd.Flags().StringArrayVar(&environments, "environment", nil, "Environment name (repeatable); replaces the full list")
	return cmd
}
