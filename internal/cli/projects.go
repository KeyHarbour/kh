package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/KeyHarbour/kh/internal/config"
	"github.com/KeyHarbour/kh/internal/khclient"
	"github.com/KeyHarbour/kh/internal/kherrors"
	"github.com/KeyHarbour/kh/internal/output"

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
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(proj)
			}
			return printer.Table(
				[]string{"UUID", "NAME", "DESCRIPTION", "ENVIRONMENTS"},
				[][]string{{proj.UUID, proj.Name, orDash(proj.Description), orDash(strings.Join(proj.Environments, ", "))}},
			)
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

Requires --org (organization UUID) or KH_ORG and a positional name argument.

--environment names environments to attach. The KeyHarbour API does not apply
them yet (KeyHarbour/KeyHarbourApi#19): kh checks the created project and warns
when they are missing, in which case add them in the KeyHarbour web app.

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
				uuid = "unknown, run 'kh project ls' to find it"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project %q created (uuid: %s).\n", proj.Name, uuid)
			warnUnappliedEnvironments(ctx, client, cmd.ErrOrStderr(), proj.UUID, environments)
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

Only the flags you pass are sent; everything else is left as it is.
--environment replaces the full environment list. The KeyHarbour API does not
apply environment names yet (KeyHarbour/KeyHarbourApi#19): kh checks the
project afterwards and warns when they are missing.

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

			if cmd.Flags().Changed("name") && strings.TrimSpace(name) == "" {
				return kherrors.ErrInvalidValue.New("--name cannot be empty")
			}
			if cmd.Flags().Changed("environment") && len(environments) == 0 {
				return kherrors.ErrInvalidValue.New("--environment cannot be empty")
			}
			if err := client.UpdateProject(ctx, proj.UUID, khclient.UpdateProjectRequest{
				Name:             name,
				EnvironmentNames: environments,
			}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Project %q updated.\n", proj.UUID)
			warnUnappliedEnvironments(ctx, client, cmd.ErrOrStderr(), proj.UUID, environments)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New project name")
	cmd.Flags().StringArrayVar(&environments, "environment", nil, "Environment name (repeatable); replaces the full list")
	return cmd
}

// warnUnappliedEnvironments re-reads the project and warns on w when any of
// the requested environments are missing. The KeyHarbour API currently
// accepts environments only as internal IDs and silently ignores names, so
// without this check create and update would report a success that did not
// happen.
func warnUnappliedEnvironments(ctx context.Context, client *khclient.Client, w io.Writer, projectUUID string, requested []string) {
	if len(requested) == 0 {
		return
	}
	if projectUUID == "" {
		fmt.Fprintf(w, "warning: could not check that environments %s were applied: the project UUID is unknown\n", strings.Join(requested, ", "))
		return
	}
	proj, err := client.GetProject(ctx, projectUUID)
	if err != nil {
		fmt.Fprintf(w, "warning: could not check that environments %s were applied: %v\n", strings.Join(requested, ", "), err)
		return
	}
	if missing := missingEnvironments(requested, proj.Environments); len(missing) > 0 {
		fmt.Fprintf(w, "warning: environments not applied: %s. The KeyHarbour API does not accept environment names yet (KeyHarbour/KeyHarbourApi#19); add them to the project in the KeyHarbour web app.\n", strings.Join(missing, ", "))
	}
}

// missingEnvironments returns the requested names absent from actual,
// compared case-insensitively as the API does.
func missingEnvironments(requested, actual []string) []string {
	have := make(map[string]bool, len(actual))
	for _, a := range actual {
		have[strings.ToLower(a)] = true
	}
	var missing []string
	for _, r := range requested {
		if !have[strings.ToLower(r)] {
			missing = append(missing, r)
		}
	}
	return missing
}
