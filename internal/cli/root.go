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
		Use:           "kh",
		Short:         "Key-Harbour CLI",
		Long:          "kh is the Key-Harbour CLI for managing Terraform state across backends and Key-Harbour.",
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
	}

	// Attach subcommands
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newWhoamiCmd())
	cmd.AddCommand(newConfigCmd())
	cmd.AddCommand(newStateCmd())
	cmd.AddCommand(newProjectsCmd())
	cmd.AddCommand(newWorkspacesCmd())
	cmd.AddCommand(newStatefilesCmd())
	cmd.AddCommand(newSyncCmd())
	cmd.AddCommand(newMigrateCmd())
	cmd.AddCommand(newVerifyCmd())
	cmd.AddCommand(newLockCmd())
	cmd.AddCommand(newUnlockCmd())
	cmd.AddCommand(newCompletionCmd(cmd))
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newTFCCmd())
	cmd.AddCommand(newHTTPCmd())
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
