package cli

import "github.com/spf13/cobra"

func newTFCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tf",
		Short: "Terraform state management",
		Long:  `Manage Terraform state stored in KeyHarbour.`,
	}
	cmd.AddCommand(newStateCmd())
	cmd.AddCommand(newStatefilesCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newTFInitCmd())
	return cmd
}
