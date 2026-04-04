package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lock <state-id>",
		Short: "Acquire an advisory lock",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("lock requires 1 argument: <state-id>. Tip: run 'kh tf state ls' to list IDs")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("lock not implemented yet")
		},
	}
	return cmd
}
