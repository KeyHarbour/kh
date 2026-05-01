package cli

import (
	"kh/internal/kherrors"

	"github.com/spf13/cobra"
)

func newUnlockCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:    "unlock <state-id>",
		Short:  "Release an advisory lock",
		Hidden: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return kherrors.ErrMissingFlag.New("unlock requires 1 argument: <state-id>. Tip: run 'kh tf state ls' to list IDs; use --force to override")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = force
			return kherrors.ErrInternal.New("unlock not implemented yet")
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Force unlock")
	return cmd
}
