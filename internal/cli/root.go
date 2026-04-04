package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"kh/internal/exitcodes"
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
  KH_ENDPOINT     API base URL (e.g. https://app.keyharbour.ca/api/v2)
  KH_TOKEN        Bearer token for authentication
  KH_PROJECT      Default project UUID
  KH_WORKSPACE    Default workspace name or UUID
  KH_DEBUG        Set to 1 for verbose debug logging
  KH_INSECURE     Set to 1 to skip TLS certificate verification (dev/test only)
  KH_ENCRYPTION_KEY  Hex-encoded 256-bit AES key for client-side KV encryption`,
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
		fmt.Fprintln(os.Stderr, err)
		if ec, ok := err.(exitcodes.ExitCoder); ok {
			return ec.ExitCode()
		}
		return exitcodes.UnknownError
	}
	return exitcodes.OK
}
