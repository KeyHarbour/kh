package cli

import (
	"fmt"
	"kh/internal/config"
	"os"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var token string
	var device bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Key-Harbour (OIDC device code or PAT)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			if token == "" && device {
				// Stub device flow
				fmt.Fprintln(os.Stderr, "Starting device flow (stub). Visit: https://app.key-harbour.example/devices and enter code: ABCD-EFGH")
				token = "device-token-stub"
			}
			if token == "" {
				return fmt.Errorf("provide --token or --device")
			}
			cfg.Token = token
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "login ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Personal access token (PAT)")
	cmd.Flags().BoolVar(&device, "device", false, "Use OIDC device code flow")
	return cmd
}
