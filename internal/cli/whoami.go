package cli

import (
	"fmt"
	"kh/internal/config"

	"github.com/spf13/cobra"
)

func newWhoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show the current authenticated identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			if cfg.Token == "" {
				return fmt.Errorf("not logged in: set token with kh login --token ...")
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
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "***" + s[len(s)-3:]
}
