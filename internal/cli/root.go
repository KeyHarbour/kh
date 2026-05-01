package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"kh/internal/exitcodes"
	"kh/internal/khclient"
	"kh/internal/kherrors"
	"kh/internal/logging"
	"kh/pkg/version"

	"github.com/spf13/cobra"
)

var (
	outputFormat string
	debug        bool
	showVersion  bool
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kh",
		Short: "KeyHarbour CLI",
		Long: `kh is the official CLI for KeyHarbour, a self-hosted Terraform state backend.

  kh auth      Authenticate and manage identity
  kh tf        Terraform state management (sync, version, lock, verify, init)
  kh project   Inspect projects
  kh workspace Inspect workspaces
  kh kv        Manage key/value pairs
  kh config    Manage CLI configuration
  kh license   Manage software licenses

Environment variables:
  KH_ENDPOINT          API base URL (e.g. https://app.keyharbour.ca/api/v2)
  KH_TOKEN             Bearer token for authentication
  KH_ORG               Default organization slug
  KH_PROJECT           Default project UUID
  KH_WORKSPACE         Default workspace UUID
  KH_CONCURRENCY       Default parallelism for bulk operations (default: 4)
  KH_OUTPUT            Default output format: table|json
  KH_DEBUG             Set to 1 for verbose debug logging
  KH_INSECURE          Set to 1 to skip TLS certificate verification (dev/test only)
  KH_ENCRYPTION_KEY_FILE  Path to a file containing the hex-encoded 256-bit AES key for client-side KV encryption`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table|json")
	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging (or set KH_DEBUG=1)")
	// Provide a global --version flag for quick version printing
	cmd.PersistentFlags().BoolVar(&showVersion, "version", false, "Show version and exit")

	// Configure debug logging prior to any subcommand execution
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		// If --version was passed, print version and exit immediately.
		if showVersion {
			if outputFormat == "json" {
				out := map[string]string{"version": version.Version}
				b, _ := json.Marshal(out)
				fmt.Println(string(b))
			} else {
				fmt.Println(version.Version)
			}
			os.Exit(0)
		}
		if !debug {
			if v := os.Getenv("KH_DEBUG"); v != "" {
				if v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
					debug = true
				}
			}
		}
		logging.SetDebug(debug)
		// Apply KH_OUTPUT env var only when --output was not explicitly passed.
		if !cmd.Root().PersistentFlags().Changed("output") {
			if v := os.Getenv("KH_OUTPUT"); v != "" {
				outputFormat = v
			}
		}
		// Warn when TLS verification is disabled so it is never silent.
		if v := os.Getenv("KH_INSECURE"); v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
			fmt.Fprintln(cmd.ErrOrStderr(), "warning: TLS certificate verification is disabled (KH_INSECURE)")
		}
	}

	// Attach subcommands
	cmd.AddCommand(newAuthCmd())
	cmd.AddCommand(newTFCmd())
	cmd.AddCommand(newProjectsCmd())
	cmd.AddCommand(newWorkspacesCmd())
	cmd.AddCommand(newKVCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newLicenseCmd())
	cmd.AddCommand(newCompletionCmd(cmd))
	// version is available via the global --version flag; no separate subcommand required

	// When no subcommand is provided, run this root handler.
	cmd.Run = func(cmd *cobra.Command, args []string) {
		if showVersion {
			if outputFormat == "json" {
				out := map[string]string{"version": version.Version}
				b, _ := json.Marshal(out)
				fmt.Println(string(b))
				return
			}
			fmt.Println(version.Version)
			return
		}
		_ = cmd.Help()
	}

	return cmd
}

// Execute runs the root command and maps errors to exit codes.
func Execute() int {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		khErr := classifyError(err)
		if outputFormat == "json" {
			enc := json.NewEncoder(os.Stderr)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{"error": khErr})
		} else {
			fmt.Fprintln(os.Stderr, err)
			if khErr.Hint != "" {
				fmt.Fprintf(os.Stderr, "hint: %s\n", khErr.Hint)
			}
		}
		return khErr.ExitCode()
	}
	return exitcodes.OK
}

// requireExactArgs wraps cobra.ExactArgs so that wrong-arity errors are
// classified as validation failures with the correct hint, rather than falling
// through to the generic "unexpected error" handler.
func requireExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return kherrors.ErrMissingFlag.Newf("usage: %s", cmd.UseLine())
		}
		if len(args) > n {
			return kherrors.ErrInvalidValue.Newf("too many arguments — usage: %s", cmd.UseLine())
		}
		return nil
	}
}

// classifyError converts any error to a *kherrors.KHError for structured
// output and exit-code resolution. It handles:
//   - *kherrors.KHError already in the chain (returned as-is)
//   - khclient.APIError mapped by HTTP status code
//   - All other errors wrapped as KH-INT-001
func classifyError(err error) *kherrors.KHError {
	var khErr *kherrors.KHError
	if errors.As(err, &khErr) {
		return khErr
	}
	var apiErr khclient.APIError
	if errors.As(err, &apiErr) {
		msg := apiErr.Error()
		switch {
		case apiErr.StatusCode == 401:
			return kherrors.ErrTokenInvalid.Wrap(msg, err)
		case apiErr.StatusCode == 403:
			return kherrors.ErrForbidden.Wrap(msg, err)
		case apiErr.StatusCode == 404:
			return kherrors.ErrNotFound.Wrap(msg, err)
		case apiErr.StatusCode == 409 || apiErr.StatusCode == 423:
			return kherrors.ErrStateLocked.Wrap(msg, err)
		case apiErr.StatusCode >= 500:
			return kherrors.ErrAPIError.Wrap(msg, err)
		}
	}
	return kherrors.ErrInternal.Wrap(kherrors.Redact(err.Error()), err)
}

// Ensure the old exitcodes.ExitCoder interface is still satisfied at compile
// time so any remaining exitcodes.With(...) call sites keep working until
// they are migrated.
var _ exitcodes.ExitCoder = (*kherrors.KHError)(nil)
