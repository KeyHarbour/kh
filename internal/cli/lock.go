package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock <state-id>",
		Short: "Acquire an advisory lock",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("lock not implemented yet")
		},
	}
	return cmd
}
