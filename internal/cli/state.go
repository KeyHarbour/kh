package cli

import (
	"github.com/spf13/cobra"
)

func newStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Lock and verify Terraform state",
		Long: `Lock, unlock and verify Terraform state stored in KeyHarbour.

To list or read statefile versions, use 'kh tf version ls' and 'kh tf version get'.`,
	}
	cmd.AddCommand(newLockCmd())
	cmd.AddCommand(newUnlockCmd())
	cmd.AddCommand(newVerifyCmd())
	return cmd
}

func pick(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return values[len(values)-1]
}
