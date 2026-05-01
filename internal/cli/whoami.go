package cli

import (
	"fmt"
	"kh/internal/config"
	"kh/internal/kherrors"

	"github.com/spf13/cobra"
)

func newWhoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the current authenticated identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			if cfg.Token == "" {
				return kherrors.ErrMissingToken.New("not logged in: set token with kh login --token ... or KH_TOKEN env var")
			}
			org := cfg.Org
			if org == "" {
				org = "(no org set)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "token: %s\norg: %s\n", mask(cfg.Token), org)
			return nil
		},
	}
	return cmd
}

func mask(s string) string {
	if len(s) <= 3 {
		return "***"
	}
	return s[:3] + "***"
}
