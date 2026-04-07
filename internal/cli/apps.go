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

func newAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apps",
		Short: "List license applications",
	}
	cmd.AddCommand(newAppsListCmd())
	cmd.AddCommand(newAppsShowCmd())
	cmd.AddCommand(newAppsImportCmd())
	return cmd
}

func newAppsListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all license applications",
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

func newAppsShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <uuid>",
		Short: "Show application details",
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

// newAppsImportCmd imports applications from a CSV file.
//
// Expected columns (header row required):
//
//	name, short_name, owner, vendor, renewal_date, tier, seats, unit_cost
//
// Only name, short_name, owner, and vendor are required; the rest are optional.
func newAppsImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file.csv>",
		Short: "Import applications from a CSV file",
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

// csvIndex builds a column-name → index map from a header row.
func csvIndex(header []string) map[string]int {
	m := make(map[string]int, len(header))
	for i, h := range header {
		m[h] = i
	}
	return m
}

// csvField returns the value at the named column, or "" if absent.
func csvField(record []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i >= len(record) {
		return ""
	}
	return record[i]
}
