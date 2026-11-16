package cli

import (
	"errors"
	"fmt"

	"github.com/KeyHarbour/kh/internal/khclient"
	"github.com/KeyHarbour/kh/internal/kherrors"
	"github.com/KeyHarbour/kh/internal/output"

	"github.com/spf13/cobra"
)

func newUnlockCmd() *cobra.Command {
	var target lockTarget
	var force bool
	cmd := &cobra.Command{
		Use:   "unlock [lock-id]",
		Short: "Release a workspace's Terraform state lock",
		Long: `Release the Terraform state lock on a workspace.

Pass the ID of the lock to release, as printed by 'kh tf state lock' or shown
in Terraform's lock error. kh checks it against the current holder and refuses
to release a different lock. Use --force to release whatever lock is held
without that check.

Examples:
  kh tf state unlock --workspace <workspace-uuid> --environment prod <lock-id>
  kh tf state unlock --workspace <workspace-uuid> --environment prod --force`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return kherrors.ErrInvalidValue.New("unlock takes at most 1 argument: [lock-id]")
			}
			if len(args) == 0 && !force {
				return kherrors.ErrMissingFlag.New("unlock requires the lock ID (from 'kh tf state lock' or Terraform's lock error), or --force to release whatever lock is held")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ws, err := target.resolve()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			lockID := ""
			if len(args) == 1 {
				lockID = args[0]
			}

			// The backend releases whichever lock is held regardless of the ID
			// sent, so check the holder here unless --force says not to.
			if !force {
				holder, err := client.LockHolder(ctx, ws, target.environment, newLockInfo("kh-unlock-check", "checking the lock holder before unlock"))
				if err != nil {
					return err
				}
				if holder == nil {
					return kherrors.ErrNotFound.Newf("workspace %s is not locked", ws)
				}
				if holder.ID != lockID {
					return kherrors.ErrStateLocked.Newf("workspace %s is locked by a different lock: %s; pass that ID or use --force", ws, (&khclient.LockedError{Holder: *holder}).Error())
				}
			}

			released, err := client.UnlockWorkspace(ctx, ws, target.environment, lockID)
			if errors.Is(err, khclient.ErrNotLocked) {
				return kherrors.ErrNotFound.Newf("workspace %s is not locked", ws)
			}
			if err != nil {
				return err
			}
			if released == "" {
				released = lockID
			}
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(map[string]any{"workspace": ws, "environment": target.environment, "released": released})
			}
			if released != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Released lock %s on workspace %s\n", released, ws)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Released lock on workspace %s\n", ws)
			}
			return nil
		},
	}
	target.addFlags(cmd)
	cmd.Flags().BoolVar(&force, "force", false, "Release whatever lock is held without checking its ID")
	return cmd
}
