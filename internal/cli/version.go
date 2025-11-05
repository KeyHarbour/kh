package cli

import (
	"encoding/json"
	"fmt"

	"kh/pkg/version"

	"github.com/spf13/cobra"
)

// newVersionCmd returns a cobra command that prints the build version.
func newVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show kh version",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Respect output format
			if outputFormat == "json" {
				out := map[string]string{"version": version.Version}
				b, err := json.Marshal(out)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}
			fmt.Println(version.Version)
			return nil
		},
	}
	return cmd
}
