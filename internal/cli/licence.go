package cli

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
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
		Long: `Create, inspect, update, and delete software license records in KeyHarbour.

Subcommands:
  ls           List all license records
  show         Show a license record's details
  create       Create a new license record
  update       Update a license record
  delete       Delete a license record
  import       Import license records from a CSV file
  instance     Manage instances of a licensed application
  licensee     Manage licensees assigned to an instance
  team-member  Manage team members associated with licenses
  users        Manage license team members`,
	}
	cmd.AddCommand(newLicenseListCmd())
	cmd.AddCommand(newLicenseShowCmd())
	cmd.AddCommand(newLicenseCreateCmd())
	cmd.AddCommand(newLicenseUpdateCmd())
	cmd.AddCommand(newLicenseDeleteCmd())
	cmd.AddCommand(newLicenseImportCmd())
	cmd.AddCommand(newLicenseInstanceCmd())
	cmd.AddCommand(newLicenseLicenseeCmd())
	cmd.AddCommand(newLicenseTeamMemberCmd())
	cmd.AddCommand(newUsersCmd())
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
	var unitCost float64
	var unitCostSet bool
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
			if unitCostSet {
				req.UnitCost = &unitCost
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
	cmd.Flags().Float64Var(&unitCost, "unit-cost", 0, "Unit cost per seat")
	cmd.MarkFlagRequired("short-name")
	cmd.MarkFlagRequired("owner")
	cmd.MarkFlagRequired("vendor")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		seatsSet = cmd.Flags().Changed("seats")
		unitCostSet = cmd.Flags().Changed("unit-cost")
		return nil
	}
	return cmd
}

// ── update ────────────────────────────────────────────────────────────────────

func newLicenseUpdateCmd() *cobra.Command {
	var name, shortName, owner, vendor, renewalDate, tier, status string
	var seats int
	var unitCost float64
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
				!cmd.Flags().Changed("unit-cost") &&
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
			if cmd.Flags().Changed("unit-cost") {
				req.UnitCost = &unitCost
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
	cmd.Flags().Float64Var(&unitCost, "unit-cost", 0, "New unit cost per seat")
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

// ── import ────────────────────────────────────────────────────────────────────

// newLicenseImportCmd imports applications from a CSV file.
//
// Expected columns (header row required):
//
//	name, short_name, owner, vendor, renewal_date, tier, seats, unit_cost
//
// Only name, short_name, owner, and vendor are required; the rest are optional.
func newLicenseImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file.csv>",
		Short: "Import license records from a CSV file",
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

				req := khclient.CreateApplicationRequest{
					Name:        csvField(record, idx, "name"),
					ShortName:   csvField(record, idx, "short_name"),
					Owner:       csvField(record, idx, "owner"),
					Vendor:      csvField(record, idx, "vendor"),
					RenewalDate: csvField(record, idx, "renewal_date"),
					Tier:        csvField(record, idx, "tier"),
				}
				if req.Name == "" || req.ShortName == "" || req.Owner == "" || req.Vendor == "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "line %d: skipping row — name, short_name, owner, vendor are required\n", line)
					skipped++
					continue
				}
				if s := csvField(record, idx, "seats"); s != "" {
					if n, err := strconv.Atoi(s); err == nil {
						req.Seats = &n
					}
				}
				if s := csvField(record, idx, "unit_cost"); s != "" {
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						req.UnitCost = &f
					}
				}

				ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
				err = client.CreateApplication(ctx, req)
				cancel()
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "line %d: %s — %v\n", line, req.Name, err)
					skipped++
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "created: %s\n", req.Name)
				created++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n%d created, %d skipped\n", created, skipped)
			return nil
		},
	}
	return cmd
}

// orDash returns s if non-empty, otherwise "-".
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// ── license instance ──────────────────────────────────────────────────────────

func newLicenseInstanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Manage instances of a license application",
	}
	cmd.AddCommand(newLicenseInstanceListCmd())
	cmd.AddCommand(newLicenseInstanceShowCmd())
	cmd.AddCommand(newLicenseInstanceCreateCmd())
	cmd.AddCommand(newLicenseInstanceUpdateCmd())
	cmd.AddCommand(newLicenseInstanceDeleteCmd())
	return cmd
}

func newLicenseInstanceListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls <application-uuid>",
		Short: "List instances of an application",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			items, err := client.ListInstances(ctx, args[0])
			if err != nil {
				return err
			}

			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}

			headers := []string{"UUID", "NAME", "SHORT NAME", "OWNER", "RENEWAL DATE", "STATUS"}
			rows := make([][]string, 0, len(items))
			for _, i := range items {
				rows = append(rows, []string{i.UUID, i.Name, i.ShortName, orDash(i.Owner), orDash(i.RenewalDate), orDash(i.Status)})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json")
	return cmd
}

func newLicenseInstanceShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <uuid>",
		Short: "Show instance details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			inst, err := client.GetInstance(ctx, args[0])
			if err != nil {
				return err
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(inst)
		},
	}
	return cmd
}

func newLicenseInstanceCreateCmd() *cobra.Command {
	var shortName, owner, renewalDate string
	var seats int
	var unitCost float64
	cmd := &cobra.Command{
		Use:   "create <application-uuid> <name>",
		Short: "Create a new instance under an application",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			req := khclient.CreateInstanceRequest{
				Name:        args[1],
				ShortName:   shortName,
				Owner:       owner,
				RenewalDate: renewalDate,
			}
			if cmd.Flags().Changed("seats") {
				req.Seats = &seats
			}
			if cmd.Flags().Changed("unit-cost") {
				req.UnitCost = &unitCost
			}

			if err := client.CreateInstance(ctx, args[0], req); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Instance %q created.\n", args[1])
			return nil
		},
	}
	cmd.Flags().StringVar(&shortName, "short-name", "", "Short name (required)")
	cmd.Flags().StringVar(&owner, "owner", "", "Owner")
	cmd.Flags().StringVar(&renewalDate, "renewal-date", "", "Renewal date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&seats, "seats", 0, "Number of seats")
	cmd.Flags().Float64Var(&unitCost, "unit-cost", 0, "Unit cost per seat")
	cmd.MarkFlagRequired("short-name")
	return cmd
}

func newLicenseInstanceUpdateCmd() *cobra.Command {
	var name, shortName, owner, renewalDate, status string
	var seats int
	var unitCost float64
	cmd := &cobra.Command{
		Use:   "update <uuid>",
		Short: "Update an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") &&
				!cmd.Flags().Changed("short-name") &&
				!cmd.Flags().Changed("owner") &&
				!cmd.Flags().Changed("renewal-date") &&
				!cmd.Flags().Changed("seats") &&
				!cmd.Flags().Changed("unit-cost") &&
				!cmd.Flags().Changed("status") {
				return fmt.Errorf("at least one flag is required")
			}

			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			req := khclient.UpdateInstanceRequest{
				Name:        name,
				ShortName:   shortName,
				Owner:       owner,
				RenewalDate: renewalDate,
				Status:      status,
			}
			if cmd.Flags().Changed("seats") {
				req.Seats = &seats
			}
			if cmd.Flags().Changed("unit-cost") {
				req.UnitCost = &unitCost
			}

			if err := client.UpdateInstance(ctx, args[0], req); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Instance %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New name")
	cmd.Flags().StringVar(&shortName, "short-name", "", "New short name")
	cmd.Flags().StringVar(&owner, "owner", "", "New owner")
	cmd.Flags().StringVar(&renewalDate, "renewal-date", "", "New renewal date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&seats, "seats", 0, "New seat count")
	cmd.Flags().Float64Var(&unitCost, "unit-cost", 0, "New unit cost per seat")
	cmd.Flags().StringVar(&status, "status", "", "New status (active|disabled|archived)")
	return cmd
}

func newLicenseInstanceDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <uuid>",
		Short: "Delete an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete instance %q? This cannot be undone. Pass --force to confirm.\n", args[0])
				return nil
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.DeleteInstance(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Instance %q deleted.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm deletion without prompting")
	return cmd
}

// ── license licensee ──────────────────────────────────────────────────────────

func newLicenseLicenseeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "licensee",
		Short: "Manage licensees on an instance",
	}
	cmd.AddCommand(newLicenseLicenseeListCmd())
	cmd.AddCommand(newLicenseLicenseeShowCmd())
	cmd.AddCommand(newLicenseLicenseeAddCmd())
	cmd.AddCommand(newLicenseLicenseeUpdateCmd())
	cmd.AddCommand(newLicenseLicenseeDeleteCmd())
	return cmd
}

func newLicenseLicenseeListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls <instance-uuid>",
		Short: "List licensees for an instance",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			items, err := client.ListLicensees(ctx, args[0])
			if err != nil {
				return err
			}

			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}

			headers := []string{"UUID", "STATUS"}
			rows := make([][]string, 0, len(items))
			for _, l := range items {
				rows = append(rows, []string{l.UUID, orDash(l.Status)})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json")
	return cmd
}

func newLicenseLicenseeShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <uuid>",
		Short: "Show licensee details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			l, err := client.GetLicensee(ctx, args[0])
			if err != nil {
				return err
			}
			return output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}.JSON(l)
		},
	}
	return cmd
}

func newLicenseLicenseeAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <instance-uuid> <member-uuid>",
		Short: "Add a licensee to an instance",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.CreateLicensee(ctx, args[0], khclient.CreateLicenseeRequest{UUID: args[1]}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Licensee %q added.\n", args[1])
			return nil
		},
	}
	return cmd
}

func newLicenseLicenseeUpdateCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "update <uuid>",
		Short: "Update a licensee's status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.UpdateLicensee(ctx, args[0], khclient.UpdateLicenseeRequest{UUID: args[0], Status: status}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Licensee %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "New status (active|disabled|archived)")
	cmd.MarkFlagRequired("status")
	return cmd
}

func newLicenseLicenseeDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <uuid>",
		Short: "Remove a licensee",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Remove licensee %q? This cannot be undone. Pass --force to confirm.\n", args[0])
				return nil
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.DeleteLicensee(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Licensee %q removed.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm removal without prompting")
	return cmd
}

// ── license team-member ───────────────────────────────────────────────────────

func newLicenseTeamMemberCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "team-member",
		Short: "Manage license team members",
	}
	cmd.AddCommand(newLicenseTeamMemberListCmd())
	cmd.AddCommand(newLicenseTeamMemberShowCmd())
	cmd.AddCommand(newLicenseTeamMemberAddCmd())
	cmd.AddCommand(newLicenseTeamMemberUpdateCmd())
	cmd.AddCommand(newLicenseTeamMemberDeleteCmd())
	return cmd
}

func newLicenseTeamMemberListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List team members",
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

func newLicenseTeamMemberShowCmd() *cobra.Command {
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

func newLicenseTeamMemberAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <uuid>",
		Short: "Add a team member",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.CreateTeamMember(ctx, khclient.CreateTeamMemberRequest{UUID: args[0]}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Team member %q added.\n", args[0])
			return nil
		},
	}
	return cmd
}

func newLicenseTeamMemberUpdateCmd() *cobra.Command {
	var managerUUID string
	cmd := &cobra.Command{
		Use:   "update <uuid>",
		Short: "Update a team member's manager",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.UpdateTeamMember(ctx, args[0], khclient.UpdateTeamMemberRequest{UUID: args[0], ManagerUUID: managerUUID}); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Team member %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&managerUUID, "manager-uuid", "", "UUID of the manager (required)")
	cmd.MarkFlagRequired("manager-uuid")
	return cmd
}

func newLicenseTeamMemberDeleteCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <uuid>",
		Short: "Remove a team member",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Remove team member %q? This cannot be undone. Pass --force to confirm.\n", args[0])
				return nil
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.DeleteTeamMember(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Team member %q removed.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm removal without prompting")
	return cmd
}
