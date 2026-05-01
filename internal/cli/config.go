package cli

import (
	"fmt"
	"kh/internal/config"
	"kh/internal/kherrors"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long: `Read and write values in the local KeyHarbour CLI configuration file (~/.kh/config).

Subcommands:
  get <key>         Print the current value of a config key
  set <key> <value> Persist a config value

Valid keys: endpoint, token, org, project, concurrency

Configuration values can also be overridden at runtime via environment variables
(KH_ENDPOINT, KH_TOKEN, KH_ORG, KH_PROJECT, KH_CONCURRENCY).`,
	}
	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	return cmd
}

func newConfigGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a config value",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return kherrors.ErrMissingFlag.New("config get requires 1 argument: <key> (valid keys: endpoint, token, org, project, concurrency)")
			}
			return nil
		},
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
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return kherrors.ErrMissingFlag.New("config set requires 2 arguments: <key> <value> (valid keys: endpoint, token, org, project, concurrency)")
			}
			return nil
		},
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
