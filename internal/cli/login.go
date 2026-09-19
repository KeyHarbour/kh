package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"kh/internal/config"
	"kh/internal/khclient"
	"kh/internal/kherrors"
	"net/http"
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
	var org string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with KeyHarbour (OIDC device code or PAT)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()

			// --org wins, then KH_ORG, then whatever is already in config.
			if org == "" {
				org = config.FromEnvOr(cfg, "KH_ORG", cfg.Org)
			}

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

			// The only token check available is org-scoped, so it can only run
			// when an org is known. Without one the token cannot be verified at
			// all, and saying so is the point: see the unverified branch below.
			verified := false
			if org != "" {
				if _, err := client.ListProjects(ctx, org); err != nil {
					return loginValidationError(org, err)
				}
				verified = true
			}

			cfg.Token = token
			if endpoint != "" {
				cfg.Endpoint = endpoint
			}
			if org != "" {
				cfg.Org = org
			}
			if err := config.Save(cfg); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if verified {
				fmt.Fprintf(out, "login ok\nendpoint: %s\n", cfg.Endpoint)
				return nil
			}

			// Never report "login ok" for a token nothing has checked: an
			// unusable token would otherwise surface much later, on an
			// unrelated command, with an error pointing nowhere near login.
			fmt.Fprintf(out, "token saved, but NOT verified\nendpoint: %s\n", cfg.Endpoint)
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: no organization is configured, so the token could not be checked against the API")
			fmt.Fprintln(cmd.ErrOrStderr(), "hint: re-run with --org <uuid> (or set KH_ORG) to verify it now")
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Personal access token (PAT) (or KH_TOKEN)")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read token from stdin (preferred for shell safety)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "KeyHarbour API endpoint (or KH_ENDPOINT)")
	cmd.Flags().BoolVar(&device, "device", false, "Use OIDC device code flow")
	cmd.Flags().StringVar(&org, "org", "", "Organization UUID used to verify the token (or KH_ORG)")
	return cmd
}

// loginValidationError distinguishes a token the API actively rejected from a
// call that merely could not be completed. The previous behaviour reported
// every failure as "token validation failed", which sent users off to
// regenerate a perfectly good token when the real problem was a typo'd
// endpoint, a wrong org, or an API that was simply down.
func loginValidationError(org string, err error) error {
	var apiErr khclient.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case http.StatusUnauthorized:
			return kherrors.ErrTokenInvalid.Wrapf(err, "the API rejected this token: %s", apiErr.Error())
		case http.StatusForbidden:
			return kherrors.ErrForbidden.Wrapf(err, "this token is not permitted to read organization %s", org)
		case http.StatusNotFound:
			return kherrors.ErrNotFound.Wrapf(err, "organization %s does not exist", org)
		}
	}
	return kherrors.ErrAPIError.Wrapf(err, "could not reach the API to verify the token: %s", err)
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
