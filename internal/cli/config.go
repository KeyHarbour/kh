package cli

import (
	"fmt"
	"kh/internal/config"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage kh configuration"}
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			v, err := config.Get(cfg, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	}
	return cmd
}

func newConfigSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			if err := config.Set(&cfg, args[0], args[1]); err != nil {
				return err
			}
			return config.Save(cfg)
		},
	}
	return cmd
}
