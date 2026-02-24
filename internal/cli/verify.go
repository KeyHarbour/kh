package cli

import (
	"fmt"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

func newVerifyCmd() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "verify <state-id>",
		Short: "Validate a state's integrity",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("verify requires 1 argument: <state-id>. Tip: run 'kh state ls' to list IDs")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if full {
				return printer.JSON(map[string]any{"state_id": args[0], "checks": []string{"schema", "lineage", "serial", "checksum"}, "ok": true})
			}
			return fmt.Errorf("verify not implemented yet (use --full for stub output)")
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "Run full verification (stub)")
	return cmd
}
