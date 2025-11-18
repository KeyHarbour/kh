package cli

import (
	"context"
	"fmt"
	"time"

	"kh/internal/config"
	"kh/internal/khclient"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

func newProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Inspect Key-Harbour projects",
	}
	cmd.AddCommand(newProjectsListCmd())
	cmd.AddCommand(newProjectsShowCmd())
	return cmd
}

func newProjectsListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			items, err := client.ListProjects(ctx)
			if err != nil {
				return err
			}
			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}
			headers := []string{"UUID", "NAME"}
			rows := make([][]string, 0, len(items))
			for _, p := range items {
				rows = append(rows, []string{p.UUID, p.Name})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json (overrides global)")
	return cmd
}

func newProjectsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <name-or-uuid>",
		Short: "Show a project's details",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("projects show requires exactly one argument: <name-or-uuid>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			proj, err := resolveProjectRef(ctx, client, args[0])
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
