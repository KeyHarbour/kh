package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"kh/internal/config"
	"kh/internal/khclient"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

type kvCmdOpts struct {
	project   string
	workspace string
	env       string
}

func newKVCmd() *cobra.Command {
	opts := &kvCmdOpts{}
	cmd := &cobra.Command{
		Use:   "kv",
		Short: "Manage key/value pairs in a workspace",
	}
	cmd.PersistentFlags().StringVar(&opts.project, "project", "", "Project UUID or name (or KH_PROJECT)")
	cmd.PersistentFlags().StringVar(&opts.workspace, "workspace", "", "Workspace UUID or name (or KH_WORKSPACE)")
	cmd.PersistentFlags().StringVar(&opts.env, "env", os.Getenv("KH_ENVIRONMENT"), "Environment name (or KH_ENVIRONMENT)")

	cmd.AddCommand(newKVListCmd(opts))
	cmd.AddCommand(newKVGetCmd(opts))
	cmd.AddCommand(newKVSetCmd(opts))
	cmd.AddCommand(newKVUpdateCmd(opts))
	cmd.AddCommand(newKVDeleteCmd(opts))
	return cmd
}

func (o *kvCmdOpts) resolve(cfg config.Config) (projectUUID, workspaceUUID string, err error) {
	projectRef := projectRefOrEnv(o.project, cfg)
	if projectRef == "" {
		return "", "", errors.New("--project is required (or set KH_PROJECT)")
	}
	workspaceRef := o.workspace
	if workspaceRef == "" {
		workspaceRef = config.FromEnvOr(cfg, "KH_WORKSPACE", "")
	}
	if workspaceRef == "" {
		return "", "", errors.New("--workspace is required (or set KH_WORKSPACE)")
	}

	client := khclient.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	project, err := resolveProjectRef(ctx, client, projectRef)
	if err != nil {
		return "", "", err
	}
	workspace, err := resolveWorkspaceRef(ctx, client, project.UUID, workspaceRef)
	if err != nil {
		return "", "", err
	}
	return project.UUID, workspace.UUID, nil
}

// ── ls ────────────────────────────────────────────────────────────────────────

func newKVListCmd(opts *kvCmdOpts) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List key/value pairs in a workspace",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			projectUUID, workspaceUUID, err := opts.resolve(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			items, err := client.ListKeyValues(ctx, projectUUID, workspaceUUID, opts.env)
			if err != nil {
				return err
			}

			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(items)
			}

			headers := []string{"KEY", "VALUE", "PRIVATE", "EXPIRES AT"}
			rows := make([][]string, 0, len(items))
			for _, kv := range items {
				exp := "-"
				if kv.ExpiresAt != nil {
					exp = *kv.ExpiresAt
				}
				val := kv.Value
				if kv.Private {
					val = "***"
				}
				rows = append(rows, []string{kv.Key, val, fmt.Sprintf("%v", kv.Private), exp})
			}
			return printer.Table(headers, rows)
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json")
	return cmd
}

// ── get ───────────────────────────────────────────────────────────────────────

func newKVGetCmd(opts *kvCmdOpts) *cobra.Command {
	var format string
	var reveal bool
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a key/value by key name",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			projectUUID, workspaceUUID, err := opts.resolve(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			kv, err := client.GetKeyValue(ctx, projectUUID, workspaceUUID, args[0])
			if err != nil {
				return err
			}

			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(kv)
			}

			val := kv.Value
			if kv.Private && !reveal {
				val = "*** (use --reveal to show)"
			}
			exp := "-"
			if kv.ExpiresAt != nil {
				exp = *kv.ExpiresAt
			}
			headers := []string{"KEY", "VALUE", "PRIVATE", "EXPIRES AT"}
			return printer.Table(headers, [][]string{{kv.Key, val, fmt.Sprintf("%v", kv.Private), exp}})
		},
	}
	cmd.Flags().StringVarP(&format, "output", "o", "", "Output format: table|json")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Show value even if the key is private")
	return cmd
}

// ── set (create) ──────────────────────────────────────────────────────────────

func newKVSetCmd(opts *kvCmdOpts) *cobra.Command {
	var private bool
	var expiresAt string
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Create a new key/value (requires --env)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.env == "" {
				return errors.New("--env is required to create a key/value")
			}
			cfg, _ := config.LoadWithEnv()
			projectUUID, workspaceUUID, err := opts.resolve(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			req := khclient.CreateKeyValueRequest{
				Key:     args[0],
				Value:   args[1],
				Private: private,
			}
			if expiresAt != "" {
				req.ExpiresAt = &expiresAt
			}

			if err := client.CreateKeyValue(ctx, projectUUID, workspaceUUID, opts.env, req); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Key %q created in environment %q.\n", args[0], opts.env)
			return nil
		},
	}
	cmd.Flags().BoolVar(&private, "private", false, "Mark the value as private (masked in list output)")
	cmd.Flags().StringVar(&expiresAt, "expires-at", "", "Expiry date/time (ISO 8601)")
	return cmd
}

// ── update ────────────────────────────────────────────────────────────────────

func newKVUpdateCmd(opts *kvCmdOpts) *cobra.Command {
	var value string
	var private string // "true"|"false"|"" (unset = don't change)
	var expiresAt string
	cmd := &cobra.Command{
		Use:   "update <key>",
		Short: "Update an existing key/value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("value") {
				return errors.New("--value is required")
			}
			cfg, _ := config.LoadWithEnv()
			projectUUID, workspaceUUID, err := opts.resolve(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			req := khclient.UpdateKeyValueRequest{Value: value}
			if expiresAt != "" {
				req.ExpiresAt = &expiresAt
			}
			if cmd.Flags().Changed("private") {
				b := private == "true"
				req.Private = &b
			}

			if err := client.UpdateKeyValue(ctx, projectUUID, workspaceUUID, args[0], req); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Key %q updated.\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&value, "value", "", "New value")
	cmd.Flags().StringVar(&private, "private", "", "Set private flag: true|false")
	cmd.Flags().StringVar(&expiresAt, "expires-at", "", "Expiry date/time (ISO 8601)")
	return cmd
}

// ── delete ────────────────────────────────────────────────────────────────────

func newKVDeleteCmd(opts *kvCmdOpts) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a key/value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete key %q? This cannot be undone. Pass --force to confirm.\n", args[0])
				return nil
			}
			cfg, _ := config.LoadWithEnv()
			projectUUID, workspaceUUID, err := opts.resolve(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.DeleteKeyValue(ctx, projectUUID, workspaceUUID, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Key %q deleted.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm deletion without prompting")
	return cmd
}
