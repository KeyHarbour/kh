package cli

import (
	"kh/internal/kherrors"

	"github.com/spf13/cobra"
)

func newLockCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "lock <state-id>",
		Short:  "Acquire an advisory lock",
		Hidden: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return kherrors.ErrMissingFlag.New("lock requires 1 argument: <state-id>. Tip: run 'kh tf state ls' to list IDs")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return kherrors.ErrInternal.New("lock not implemented yet")
		},
	}
	return cmd
}
