package cli

import (
	"fmt"
	"os"
	"strings"

	"kh/internal/exitcodes"
	"kh/internal/logging"

	"github.com/spf13/cobra"
)

var (
	outputFormat string
	debug        bool
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

	// Configure debug logging prior to any subcommand execution
	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
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
	cmd.AddCommand(newImportCmd())
	cmd.AddCommand(newExportCmd())
	cmd.AddCommand(newMigrateCmd())
	cmd.AddCommand(newVerifyCmd())
	cmd.AddCommand(newLockCmd())
	cmd.AddCommand(newUnlockCmd())
	cmd.AddCommand(newCompletionCmd(cmd))

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
