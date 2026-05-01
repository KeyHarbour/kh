package cli

import (
	"os"

	"kh/internal/kherrors"

	"github.com/spf13/cobra"
)

func newCompletionCmd(root *cobra.Command) *cobra.Command {
	valid := []string{"bash", "zsh", "fish", "powershell"}
	cmd := &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate shell completion scripts",
		ValidArgs: valid,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return kherrors.ErrMissingFlag.Newf("completion requires exactly one argument: one of %v", valid)
			}
			a := args[0]
			for _, v := range valid {
				if a == v {
					return nil
				}
			}
			return kherrors.ErrInvalidValue.Newf("invalid shell %q; accepted values: %v", a, valid)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(os.Stdout)
			case "zsh":
				return root.GenZshCompletion(os.Stdout)
			case "fish":
				return root.GenFishCompletion(os.Stdout, true)
			case "powershell":
				return root.GenPowerShellCompletion(os.Stdout)
			}
			return nil
		},
	}
	return cmd
}
