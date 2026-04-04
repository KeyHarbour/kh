package cli

import "github.com/spf13/cobra"

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate and manage identity",
	}
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newWhoamiCmd())
	return cmd
}
