package cli

import (
	"bufio"
	"context"
	"fmt"
	"kh/internal/config"
	"kh/internal/khclient"
	"kh/internal/kherrors"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	var token string
	var tokenStdin bool
	var endpoint string
	var device bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with KeyHarbour (OIDC device code or PAT)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			org := config.FromEnvOr(cfg, "KH_ORG", cfg.Org)

			// Use endpoint from flag, env, or config (in that order)
			if endpoint == "" {
				endpoint = config.FromEnvOr(cfg, "KH_ENDPOINT", cfg.Endpoint)
			}

			if token != "" && tokenStdin {
				return kherrors.ErrConflictingFlags.New("provide only one of --token or --token-stdin")
			}

			// Read token from stdin if requested.
			if token == "" && tokenStdin {
				line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				if err != nil && err.Error() != "EOF" {
					return kherrors.ErrInvalidValue.Wrapf(err, "failed to read token from stdin: %s", err)
				}
				token = strings.TrimSpace(line)
			}

			// Use token from flag/stdin, env, or trigger device flow
			if token == "" {
				token = config.FromEnvOr(cfg, "KH_TOKEN", "")
			}

			if token == "" && device {
				// Stub device flow
				fmt.Fprintln(os.Stderr, "Starting device flow (stub). Visit: https://app.keyharbour.example/devices and enter code: ABCD-EFGH")
				token = "device-token-stub"
			}
			if token == "" {
				return kherrors.ErrMissingToken.New("provide --token-stdin, set KH_TOKEN environment variable, or use --device")
			}

			// Validate token by making a test API call
			testCfg := cfg
			testCfg.Token = token
			if endpoint != "" {
				testCfg.Endpoint = endpoint
			}
			if testCfg.Endpoint == "" {
				return kherrors.ErrMissingFlag.New("--endpoint or KH_ENDPOINT is required")
			}

			client := khclient.New(testCfg)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			// Validate token with an org-scoped call when org context is available.
			if org != "" {
				if _, err := client.ListProjects(ctx, org); err != nil {
					return kherrors.ErrTokenInvalid.Newf("token validation failed: %v", err)
				}
			}

			// Token is valid, save the config
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
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read token from stdin (preferred for shell safety)")
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
