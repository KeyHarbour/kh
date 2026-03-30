package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"kh/internal/config"
	"kh/internal/exitcodes"
	"kh/internal/khclient"
	"kh/internal/kvencrypt"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

type kvCmdOpts struct {
	project       string
	workspace     string
	encryptionKey string
}

func newKVCmd() *cobra.Command {
	opts := &kvCmdOpts{}
	cmd := &cobra.Command{
		Use:   "kv",
		Short: "Manage key/value pairs in a workspace",
		Long: `Manage key/value pairs stored in a KeyHarbour workspace.

Commands that operate on a specific key (get, update, delete) only require the
key name — no --project or --workspace flags needed.

Commands that operate on the workspace collection (ls, set) require --project
and --workspace (or KH_PROJECT / KH_WORKSPACE env vars).`,
	}
	cmd.PersistentFlags().StringVar(&opts.project, "project", "", "Project UUID or name (or KH_PROJECT)")
	cmd.PersistentFlags().StringVar(&opts.workspace, "workspace", "", "Workspace UUID or name (or KH_WORKSPACE)")
	cmd.PersistentFlags().StringVar(&opts.encryptionKey, "encryption-key", "", "Hex-encoded 256-bit AES key for client-side encryption (or KH_ENCRYPTION_KEY)")

	cmd.AddCommand(newKVListCmd(opts))
	cmd.AddCommand(newKVGetCmd(opts))
	cmd.AddCommand(newKVSetCmd(opts))
	cmd.AddCommand(newKVUpdateCmd(opts))
	cmd.AddCommand(newKVDeleteCmd(opts))
	return cmd
}

func (o *kvCmdOpts) resolve(cfg config.Config) (workspaceUUID string, err error) {
	projectRef := projectRefOrEnv(o.project, cfg)
	if projectRef == "" {
		return "", errors.New("--project is required (or set KH_PROJECT)")
	}
	workspaceRef := o.workspace
	if workspaceRef == "" {
		workspaceRef = config.FromEnvOr(cfg, "KH_WORKSPACE", "")
	}
	if workspaceRef == "" {
		return "", errors.New("--workspace is required (or set KH_WORKSPACE)")
	}

	client := khclient.New(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	project, err := resolveProjectRef(ctx, client, projectRef)
	if err != nil {
		return "", err
	}
	workspace, err := resolveWorkspaceRef(ctx, client, project.UUID, workspaceRef)
	if err != nil {
		return "", err
	}
	return workspace.UUID, nil
}

func (o *kvCmdOpts) resolveEncryptionKey(cfg config.Config) (*[32]byte, error) {
	raw := o.encryptionKey
	if raw == "" {
		raw = config.FromEnvOr(cfg, "KH_ENCRYPTION_KEY", "")
	}
	if raw == "" {
		return nil, nil // encryption not requested
	}
	key, err := kvencrypt.ParseKey(raw)
	if err != nil {
		return nil, exitcodes.With(exitcodes.ValidationError, err)
	}
	return &key, nil
}

// ── ls ────────────────────────────────────────────────────────────────────────

func newKVListCmd(opts *kvCmdOpts) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List key/value pairs in a workspace",
		Long: `List all key/value pairs stored in a workspace.

Requires --project and --workspace (or KH_PROJECT / KH_WORKSPACE).
Private values are masked as *** in table output; use -o json to see the raw
response (values remain masked server-side unless the token has reveal access).

Examples:
  kh kv ls --project <uuid> --workspace <uuid>
  kh kv ls --project <uuid> --workspace prod -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			workspaceUUID, err := opts.resolve(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			items, err := client.ListKeyValues(ctx, workspaceUUID)
			if err != nil {
				return err
			}

			encKey, err := opts.resolveEncryptionKey(cfg)
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
				switch {
				case kvencrypt.IsEncrypted(val) && encKey != nil:
					plain, err := kvencrypt.Decrypt(*encKey, val)
					if err != nil {
						val = "[decryption failed]"
					} else {
						val = plain
					}
				case kvencrypt.IsEncrypted(val):
					val = "[encrypted]"
				case kv.Private:
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
		Long: `Retrieve a single key/value pair by its key name.

No --project or --workspace flags are required; the key name is globally unique
within your token scope.

Private values are masked unless --reveal is passed.

Examples:
  kh kv get MY_KEY
  kh kv get MY_SECRET --reveal
  kh kv get MY_KEY -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			kv, err := client.GetKeyValue(ctx, args[0])
			if err != nil {
				return err
			}

			encKey, err := opts.resolveEncryptionKey(cfg)
			if err != nil {
				return err
			}

			printer := output.Printer{Format: pick(format, outputFormat), W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(kv)
			}

			val := kv.Value
			switch {
			case kvencrypt.IsEncrypted(val) && encKey != nil:
				plain, err := kvencrypt.Decrypt(*encKey, val)
				if err != nil {
					return exitcodes.With(exitcodes.ValidationError, err)
				}
				val = plain
			case kvencrypt.IsEncrypted(val):
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: value appears encrypted; use --encryption-key to decrypt\n")
			case kv.Private && !reveal:
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
		Short: "Create a new key/value in a workspace",
		Long: `Create a new key/value pair in a workspace.

Requires --project and --workspace (or KH_PROJECT / KH_WORKSPACE).

Examples:
  kh kv set MY_KEY my-value --project <uuid> --workspace <uuid>
  kh kv set MY_SECRET s3cr3t --project <uuid> --workspace prod --private
  kh kv set TEMP_KEY value --project <uuid> --workspace prod --expires-at 2026-12-31T00:00:00Z`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.LoadWithEnv()
			workspaceUUID, err := opts.resolve(cfg)
			if err != nil {
				return err
			}
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			encKey, err := opts.resolveEncryptionKey(cfg)
			if err != nil {
				return err
			}
			value := args[1]
			if encKey != nil {
				value, err = kvencrypt.Encrypt(*encKey, value)
				if err != nil {
					return fmt.Errorf("encryption failed: %w", err)
				}
			}

			req := khclient.CreateKeyValueRequest{
				Key:     args[0],
				Value:   value,
				Private: private,
			}
			if expiresAt != "" {
				req.ExpiresAt = &expiresAt
			}

			if err := client.CreateKeyValue(ctx, workspaceUUID, req); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Key %q created.\n", args[0])
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
		Long: `Update the value, private flag, or expiry of an existing key/value.

No --project or --workspace flags are required; the key name uniquely identifies
the record within your token scope.

Examples:
  kh kv update MY_KEY --value new-value
  kh kv update MY_KEY --value new-value --private true
  kh kv update MY_KEY --value new-value --expires-at 2027-01-01T00:00:00Z`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("value") {
				return errors.New("--value is required")
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			encKey, err := opts.resolveEncryptionKey(cfg)
			if err != nil {
				return err
			}
			sendValue := value
			if encKey != nil {
				sendValue, err = kvencrypt.Encrypt(*encKey, value)
				if err != nil {
					return fmt.Errorf("encryption failed: %w", err)
				}
			}

			req := khclient.UpdateKeyValueRequest{Value: sendValue}
			if expiresAt != "" {
				req.ExpiresAt = &expiresAt
			}
			if cmd.Flags().Changed("private") {
				b := private == "true"
				req.Private = &b
			}

			if err := client.UpdateKeyValue(ctx, args[0], req); err != nil {
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
		Long: `Delete a key/value pair by key name.

No --project or --workspace flags are required; the key name uniquely identifies
the record within your token scope.

Pass --force to skip the confirmation prompt.

Examples:
  kh kv delete MY_KEY --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete key %q? This cannot be undone. Pass --force to confirm.\n", args[0])
				return nil
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			if err := client.DeleteKeyValue(ctx, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Key %q deleted.\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Confirm deletion without prompting")
	return cmd
}
