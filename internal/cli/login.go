package cli

import (
	"fmt"
	"kh/internal/config"
	"os"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var token string
	var endpoint string
	var device bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Key-Harbour (OIDC device code or PAT)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()

			// Use endpoint from flag, env, or config (in that order)
			if endpoint == "" {
				endpoint = config.FromEnvOr(cfg, "KH_ENDPOINT", cfg.Endpoint)
			}

			// Use token from flag, env, or trigger device flow
			if token == "" {
				token = config.FromEnvOr(cfg, "KH_TOKEN", "")
			}

			if token == "" && device {
				// Stub device flow
				fmt.Fprintln(os.Stderr, "Starting device flow (stub). Visit: https://app.key-harbour.example/devices and enter code: ABCD-EFGH")
				token = "device-token-stub"
			}
			if token == "" {
				return fmt.Errorf("provide --token, set KH_TOKEN environment variable, or use --device")
			}
			cfg.Token = token
			if endpoint != "" {
				cfg.Endpoint = endpoint
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "login ok\nendpoint: %s\n", cfg.Endpoint)
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Personal access token (PAT) (or KH_TOKEN)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "KeyHarbour API endpoint (or KH_ENDPOINT)")
	cmd.Flags().BoolVar(&device, "device", false, "Use OIDC device code flow")
	return cmd
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored API token from the local config",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if cfg.Token == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "already logged out")
				return nil
			}
			cfg.Token = ""
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "logged out")
			return nil
		},
	}
}
