package cli

import (
	"fmt"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	var from, to string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate between backends (convenience wrapper)",
	}

	backend := &cobra.Command{
		Use:   "backend",
		Short: "Migrate backend --from ... --to ...",
		RunE: func(cmd *cobra.Command, args []string) error {
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if dryRun {
				return printer.JSON(map[string]any{"action": "migrate", "from": from, "to": to, "dry_run": true})
			}
			return fmt.Errorf("migrate not implemented yet")
		},
	}

	backend.Flags().StringVar(&from, "from", "", "Source backend")
	backend.Flags().StringVar(&to, "to", "", "Target backend")
	backend.Flags().BoolVar(&dryRun, "dry-run", false, "Preview actions without writing")

	cmd.AddCommand(backend)
	return cmd
}
