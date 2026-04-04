package cli

import "github.com/spf13/cobra"

func newTFCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tf",
		Short: "Terraform state management",
		Long: `Manage Terraform state stored in KeyHarbour.

Subcommands:
  state     Inspect and manage states (ls, show, lock, unlock, verify)
  version   Manage statefile versions for a workspace
  sync      Migrate state between backends
  init      Scaffold a Terraform project for KeyHarbour`,
	}
	cmd.AddCommand(newStateCmd())
	cmd.AddCommand(newStatefilesCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newTFInitCmd())
	return cmd
}
