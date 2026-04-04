package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"kh/internal/config"
	"kh/internal/khclient"
	"kh/internal/output"
	"time"

	"github.com/spf13/cobra"
)

type StateMeta struct {
	ID        string `json:"id"`
	Project   string `json:"project"`
	Module    string `json:"module"`
	Workspace string `json:"workspace"`
	Size      int64  `json:"size"`
}

func newStateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "state", Short: "Inspect and manage states"}
	cmd.AddCommand(newStateLsCmd())
	cmd.AddCommand(newStateShowCmd())
	cmd.AddCommand(newLockCmd())
	cmd.AddCommand(newUnlockCmd())
	cmd.AddCommand(newVerifyCmd())
	return cmd
}

func newStateLsCmd() *cobra.Command {
	var project, module, workspace, format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List states known by Key-Harbour",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			items, err := client.ListStates(ctx, khclient.ListStatesRequest{Project: project, Module: module, Workspace: workspace})
			if err != nil {
				return err
			}
			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}
			headers := []string{"ID", "PROJECT", "MODULE", "WORKSPACE", "SIZE"}
			rows := make([][]string, 0, len(items))
			for _, s := range items {
				rows = append(rows, []string{s.ID, s.Project, s.Module, s.Workspace, fmt.Sprint(s.Size)})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Filter by project")
	cmd.Flags().StringVar(&module, "module", "", "Filter by module")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Filter by workspace")
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json (overrides global)")
	return cmd
}

func newStateShowCmd() *cobra.Command {
	var raw bool
	cmd := &cobra.Command{
		Use:   "show <state-id>",
		Short: "Show a state's JSON (Terraform v4)",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("state show requires 1 argument: <state-id>. Tip: run 'kh tf state ls' to list IDs")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			b, meta, err := client.GetStateRaw(ctx, args[0])
			if err != nil {
				return err
			}
			if raw {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(json.RawMessage(b))
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(meta)
		},
	}
	cmd.Flags().BoolVar(&raw, "raw", false, "Print raw Terraform v4 JSON")
	return cmd
}

func pick(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return values[len(values)-1]
}
