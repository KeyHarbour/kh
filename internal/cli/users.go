package cli

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"time"

	"kh/internal/config"
	"kh/internal/khclient"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

func newUsersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "users",
		Short: "List license team members",
	}
	cmd.AddCommand(newUsersListCmd())
	cmd.AddCommand(newUsersShowCmd())
	cmd.AddCommand(newUsersImportCmd())
	return cmd
}

func newUsersListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all team members",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			items, err := client.ListTeamMembers(ctx)
			if err != nil {
				return err
			}

			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}

			headers := []string{"UUID", "MANAGER UUID"}
			rows := make([][]string, 0, len(items))
			for _, m := range items {
				mgr := "-"
				if m.ManagerUUID != nil {
					mgr = *m.ManagerUUID
				}
				rows = append(rows, []string{m.UUID, mgr})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json")
	return cmd
}

func newUsersShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <uuid>",
		Short: "Show team member details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			m, err := client.GetTeamMember(ctx, args[0])
			if err != nil {
				return err
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(m)
		},
	}
	return cmd
}

// newUsersImportCmd imports team members from a CSV file.
//
// Expected columns (header row required):
//
//	uuid, manager_uuid
//
// Only uuid is required; manager_uuid is optional.
func newUsersImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file.csv>",
		Short: "Import team members from a CSV file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return fmt.Errorf("open %s: %w", args[0], err)
			}
			defer f.Close()

			r := csv.NewReader(f)
			r.TrimLeadingSpace = true

			header, err := r.Read()
			if err != nil {
				return fmt.Errorf("read header: %w", err)
			}
			idx := csvIndex(header)

			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)

			var created, skipped int
			for line := 2; ; line++ {
				record, err := r.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					return fmt.Errorf("line %d: %w", line, err)
				}

				uuid := csvField(record, idx, "uuid")
				if uuid == "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "line %d: skipping row — uuid is required\n", line)
					skipped++
					continue
				}

				ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
				err = client.CreateTeamMember(ctx, khclient.CreateTeamMemberRequest{UUID: uuid})
				cancel()
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "line %d: %s — %v\n", line, uuid, err)
					skipped++
					continue
				}

				// Set manager if provided.
				if managerUUID := csvField(record, idx, "manager_uuid"); managerUUID != "" {
					ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
					_ = client.UpdateTeamMember(ctx, uuid, khclient.UpdateTeamMemberRequest{UUID: uuid, ManagerUUID: managerUUID})
					cancel()
				}

				fmt.Fprintf(cmd.OutOrStdout(), "created: %s\n", uuid)
				created++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n%d created, %d skipped\n", created, skipped)
			return nil
		},
	}
	return cmd
}
