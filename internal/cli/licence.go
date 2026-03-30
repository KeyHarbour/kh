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

func newLicenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "license",
		Short: "Manage software license records",
	}
	cmd.AddCommand(newLicenseListCmd())
	cmd.AddCommand(newLicenseShowCmd())
	cmd.AddCommand(newLicenseCreateCmd())
	cmd.AddCommand(newLicenseUpdateCmd())
	cmd.AddCommand(newLicenseDeleteCmd())
	return cmd
}

// ── ls ────────────────────────────────────────────────────────────────────────

func newLicenseListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all license records",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			items, err := client.ListApplications(ctx)
			if err != nil {
				return err
			}

			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}

			headers := []string{"UUID", "NAME", "SHORT NAME", "VENDOR", "OWNER", "TIER", "RENEWAL DATE", "STATUS"}
			rows := make([][]string, 0, len(items))
			for _, a := range items {
				seats := "-"
				if a.Seats != nil {
					seats = fmt.Sprintf("%d", *a.Seats)
				}
				_ = seats // seats shown in show, not ls to keep table narrow
				rows = append(rows, []string{
					a.UUID, a.Name, a.ShortName, a.Vendor, a.Owner,
					orDash(a.Tier), orDash(a.RenewalDate), orDash(a.Status),
				})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json")
	return cmd
}

// ── show ──────────────────────────────────────────────────────────────────────

func newLicenseShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <uuid>",
		Short: "Show license record details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			app, err := client.GetApplication(ctx, args[0])
			if err != nil {
				return err
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(app)
		},
	}
	return cmd
}

// ── create ────────────────────────────────────────────────────────────────────

func newLicenseCreateCmd() *cobra.Command {
	var shortName, owner, vendor, renewalDate, tier string
	var seats int
	var seatsSet bool
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new license record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			req := khclient.CreateApplicationRequest{
				Name:        args[0],
				ShortName:   shortName,
				Owner:       owner,
				Vendor:      vendor,
				RenewalDate: renewalDate,
				Tier:        tier,
			}
			if seatsSet {
				req.Seats = &seats
			}

			if err := client.CreateApplication(ctx, req); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "License %q created.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&shortName, "short-name", "", "Short name or abbreviation (required)")
	cmd.Flags().StringVar(&owner, "owner", "", "License owner (required)")
	cmd.Flags().StringVar(&vendor, "vendor", "", "Software vendor (required)")
	cmd.Flags().StringVar(&renewalDate, "renewal-date", "", "Renewal date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&tier, "tier", "", "License tier or edition (e.g. Enterprise, Plus)")
	cmd.Flags().IntVar(&seats, "seats", 0, "Number of licensed seats")
	cmd.MarkFlagRequired("short-name")
	cmd.MarkFlagRequired("owner")
	cmd.MarkFlagRequired("vendor")
	// track whether --seats was explicitly set
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		seatsSet = cmd.Flags().Changed("seats")
		return nil
	}
	return cmd
}

// ── update ────────────────────────────────────────────────────────────────────

func newLicenseUpdateCmd() *cobra.Command {
	var name, shortName, owner, vendor, renewalDate, tier, status string
	var seats int
	cmd := &cobra.Command{
		Use:   "update <uuid>",
		Short: "Update a license record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") &&
				!cmd.Flags().Changed("short-name") &&
				!cmd.Flags().Changed("owner") &&
				!cmd.Flags().Changed("vendor") &&
				!cmd.Flags().Changed("renewal-date") &&
				!cmd.Flags().Changed("tier") &&
				!cmd.Flags().Changed("seats") &&
				!cmd.Flags().Changed("status") {
				return fmt.Errorf("at least one flag is required")
			}

			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			req := khclient.UpdateApplicationRequest{
				Name:        name,
				ShortName:   shortName,
				Owner:       owner,
				Vendor:      vendor,
				RenewalDate: renewalDate,
				Tier:        tier,
				Status:      status,
			}
			if cmd.Flags().Changed("seats") {
				req.Seats = &seats
			}

			if err := client.UpdateApplication(ctx, args[0], req); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "License %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&shortName, "short-name", "", "New short name")
	cmd.Flags().StringVar(&owner, "owner", "", "New owner")
	cmd.Flags().StringVar(&vendor, "vendor", "", "New vendor")
	cmd.Flags().StringVar(&renewalDate, "renewal-date", "", "New renewal date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&tier, "tier", "", "New tier or edition")
	cmd.Flags().IntVar(&seats, "seats", 0, "New seat count")
	cmd.Flags().StringVar(&status, "status", "", "New status (active|disabled|archived)")
	return cmd
}

// ── delete ────────────────────────────────────────────────────────────────────

func newLicenseDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <uuid>",
		Short: "Delete a license record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete license %q? This cannot be undone. Pass --force to confirm.\n", args[0])
				return nil
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.DeleteApplication(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "License %q deleted.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm deletion without prompting")
	return cmd
}

// orDash returns s if non-empty, otherwise "-".
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
