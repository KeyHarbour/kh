package cli

import "github.com/spf13/cobra"

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate and manage identity",
		Long: `Authenticate with a KeyHarbour instance and manage the stored identity.

Subcommands:
  login   Save a token and endpoint to ~/.kh/config
  logout  Remove the stored token from ~/.kh/config
  whoami  Show the currently authenticated identity`,
	}
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newWhoamiCmd())
	return cmd
}
