package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newUnlockCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "unlock <state-id>",
		Short: "Release an advisory lock",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = force
			return fmt.Errorf("unlock not implemented yet")
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force unlock")
	return cmd
}
