package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"kh/internal/config"
	"kh/internal/exitcodes"
	"kh/internal/khclient"
	"kh/internal/kvencrypt"
	"kh/internal/output"

	"github.com/spf13/cobra"
)

type kvCmdOpts struct {
	project           string
	workspace         string
	encryptionKeyFile string
}

func newKVCmd() *cobra.Command {
	opts := &kvCmdOpts{}
	cmd := &cobra.Command{
		Use:   "kv",
		Short: "Manage key/value pairs in a workspace",
		Long: `Manage key/value pairs stored in a KeyHarbour workspace.

Commands that operate on a specific key (get, update, delete) only require the
key name — no --project or --workspace flags needed.

Commands that operate on the workspace collection (ls, set) require --workspace
(or KH_WORKSPACE). If the workspace is identified by name rather than UUID,
--project (or KH_PROJECT) is also required for name resolution.`,
	}
	cmd.PersistentFlags().StringVar(&opts.project, "project", "", "Project UUID or name (or KH_PROJECT)")
	cmd.PersistentFlags().StringVar(&opts.workspace, "workspace", "", "Workspace UUID or name (or KH_WORKSPACE)")
	cmd.PersistentFlags().StringVar(&opts.encryptionKeyFile, "encryption-key-file", "", "Path to a file containing the hex-encoded 256-bit AES key (or KH_ENCRYPTION_KEY_FILE)")

	cmd.AddCommand(newKVListCmd(opts))
	cmd.AddCommand(newKVGetCmd(opts))
	cmd.AddCommand(newKVSetCmd(opts))
	cmd.AddCommand(newKVUpdateCmd(opts))
	cmd.AddCommand(newKVDeleteCmd(opts))
	cmd.AddCommand(newKVEnvCmd(opts))
	cmd.AddCommand(newKVRunCmd(opts))
	return cmd
}

func (o *kvCmdOpts) resolve(cfg config.Config) (workspaceUUID string, err error) {
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

	// If the workspace ref is already a UUID, resolve it directly — no project needed.
	if looksLikeUUID(workspaceRef) {
		ws, err := client.GetWorkspace(ctx, workspaceRef)
		if err != nil {
			return "", err
		}
		return ws.UUID, nil
	}

	// Name-based lookup requires a project reference.
	projectRef := projectRefOrEnv(o.project, cfg)
	if projectRef == "" {
		return "", errors.New("--project is required when workspace is specified by name (or set KH_PROJECT)")
	}
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
	keyFile := o.encryptionKeyFile
	if keyFile == "" {
		keyFile = os.Getenv("KH_ENCRYPTION_KEY_FILE")
	}
	if keyFile == "" {
		return nil, nil // encryption not requested
	}
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, exitcodes.With(exitcodes.ValidationError, fmt.Errorf("cannot read encryption key file: %w", err))
	}
	raw := strings.TrimSpace(string(data))
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

Requires --workspace (or KH_WORKSPACE). If the workspace is specified by name
rather than UUID, --project (or KH_PROJECT) is also required for name resolution.
Private values are masked as *** in table output; use -o json to see the raw
response (values remain masked server-side unless the token has reveal access).

Examples:
  kh kv ls --workspace <uuid>
  kh kv ls --workspace prod --project <uuid> -o json`,
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

			headers := []string{"KEY", "VALUE", "PRIVATE", "ENVIRONMENT", "EXPIRES AT"}
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
				rows = append(rows, []string{kv.Key, val, fmt.Sprintf("%v", kv.Private), orDash(kv.Environment), exp})
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
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: value appears encrypted; use --encryption-key-file or KH_ENCRYPTION_KEY_FILE to decrypt\n")
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
	var valueFile string
	cmd := &cobra.Command{
		Use:   "set <key> [value]",
		Short: "Create a new key/value in a workspace",
		Long: `Create a new key/value pair in a workspace.

Requires --workspace (or KH_WORKSPACE). If the workspace is specified by name
rather than UUID, --project (or KH_PROJECT) is also required for name resolution.

The value can be provided as a positional argument or read from a file with
--value-file. Exactly one of the two must be supplied.

Examples:
  kh kv set MY_KEY my-value --workspace <uuid>
  kh kv set MY_SECRET s3cr3t --workspace <uuid> --private
  kh kv set TEMP_KEY value --workspace prod --project <uuid> --expires-at 2026-12-31T00:00:00Z
  kh kv set CERT --value-file ./cert.pem --workspace <uuid>`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			hasValueArg := len(args) == 2
			hasValueFile := cmd.Flags().Changed("value-file")
			if hasValueArg && hasValueFile {
				return exitcodes.With(exitcodes.ValidationError, errors.New("provide either a positional value or --value-file, not both"))
			}
			if !hasValueArg && !hasValueFile {
				return exitcodes.With(exitcodes.ValidationError, errors.New("a value is required: provide it as an argument or via --value-file"))
			}

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
			var value string
			if hasValueFile {
				data, err := os.ReadFile(valueFile)
				if err != nil {
					return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("cannot read value file: %w", err))
				}
				value = string(data)
			} else {
				value = args[1]
			}
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
	cmd.Flags().StringVar(&valueFile, "value-file", "", "Read value from a file instead of a positional argument")
	return cmd
}

// ── update ────────────────────────────────────────────────────────────────────

func newKVUpdateCmd(opts *kvCmdOpts) *cobra.Command {
	var value string
	var valueFile string
	var private string // "true"|"false"|"" (unset = don't change)
	var expiresAt string
	cmd := &cobra.Command{
		Use:   "update <key>",
		Short: "Create or update a key/value",
		Long: `Update the value, private flag, or expiry of an existing key/value.

If the key does not exist and --workspace is provided (or KH_WORKSPACE is set),
the key is created automatically (upsert). If the workspace is specified by name
rather than UUID, --project (or KH_PROJECT) is also required.

The value can be supplied via --value or read from a file with --value-file.

Examples:
  kh kv update MY_KEY --value new-value
  kh kv update MY_KEY --value-file ./cert.pem
  kh kv update MY_KEY --value new-value --private true
  kh kv update MY_KEY --value new-value --expires-at 2027-01-01T00:00:00Z
  kh kv update MY_KEY --value new-value --workspace <uuid>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hasValue := cmd.Flags().Changed("value")
			hasValueFile := cmd.Flags().Changed("value-file")
			if hasValue && hasValueFile {
				return exitcodes.With(exitcodes.ValidationError, errors.New("provide either --value or --value-file, not both"))
			}
			if !hasValue && !hasValueFile {
				return errors.New("--value or --value-file is required")
			}
			cfg, _ := config.LoadWithEnv()
			client := khclient.New(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()

			encKey, err := opts.resolveEncryptionKey(cfg)
			if err != nil {
				return err
			}
			if hasValueFile {
				data, err := os.ReadFile(valueFile)
				if err != nil {
					return exitcodes.With(exitcodes.ValidationError, fmt.Errorf("cannot read value file: %w", err))
				}
				value = string(data)
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

			updateErr := client.UpdateKeyValue(ctx, args[0], req)
			if updateErr == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Key %q updated.\n", args[0])
				return nil
			}

			// On 404, fall back to create if a workspace can be resolved.
			var apiErr khclient.APIError
			if errors.As(updateErr, &apiErr) && apiErr.StatusCode == 404 {
				workspaceRef := opts.workspace
				if workspaceRef == "" {
					workspaceRef = config.FromEnvOr(cfg, "KH_WORKSPACE", "")
				}
				if workspaceRef != "" {
					workspaceUUID, rerr := opts.resolve(cfg)
					if rerr != nil {
						return rerr
					}
					isPrivate := private == "true"
					createReq := khclient.CreateKeyValueRequest{
						Key:     args[0],
						Value:   sendValue,
						Private: isPrivate,
					}
					if expiresAt != "" {
						createReq.ExpiresAt = &expiresAt
					}
					if cerr := client.CreateKeyValue(ctx, workspaceUUID, createReq); cerr != nil {
						return cerr
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Key %q created.\n", args[0])
					return nil
				}
			}
			return updateErr
		},
	}
	cmd.Flags().StringVar(&value, "value", "", "New value")
	cmd.Flags().StringVar(&valueFile, "value-file", "", "Read new value from a file")
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

// ── env ───────────────────────────────────────────────────────────────────────

// resolveKVPairs fetches KV pairs from the workspace, applies prefix/environment
// filters, decrypts values where possible, and returns a flat map of name→value.
// When prefix is non-empty, only keys with that prefix are included and the prefix
// is stripped from the resulting name.
func resolveKVPairs(
	cmd *cobra.Command,
	client interface {
		ListKeyValues(context.Context, string) ([]khclient.KeyValue, error)
	},
	ctx context.Context,
	workspaceUUID, prefix, environment string,
	encKey *[32]byte,
) []struct{ Name, Value string } {
	items, err := client.ListKeyValues(ctx, workspaceUUID)
	if err != nil {
		return nil
	}
	var out []struct{ Name, Value string }
	for _, kv := range items {
		if environment != "" && kv.Environment != environment {
			continue
		}
		if prefix != "" && !strings.HasPrefix(kv.Key, prefix) {
			continue
		}
		name := strings.TrimPrefix(kv.Key, prefix)
		val := kv.Value
		if kvencrypt.IsEncrypted(val) {
			if encKey == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping encrypted key %q (no --encryption-key-file)\n", kv.Key)
				continue
			}
			plain, err := kvencrypt.Decrypt(*encKey, val)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: skipping key %q: decryption failed: %v\n", kv.Key, err)
				continue
			}
			val = plain
		}
		out = append(out, struct{ Name, Value string }{name, val})
	}
	return out
}

func newKVEnvCmd(opts *kvCmdOpts) *cobra.Command {
	var format string
	var environment string
	var prefix string
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print workspace key/values as environment variable assignments",
		Long: `Fetch all key/value pairs from a workspace and print them as shell
variable assignments suitable for sourcing or eval.

Formats:
  export  (default) — "export KEY='VALUE'" lines, safe to eval in bash/zsh
  dotenv            — "KEY=VALUE" lines for .env files / Docker --env-file

Use --prefix to include only keys that start with the given prefix. The prefix
is stripped from the variable name before output, so KH_ENV_DATABASE_URL becomes
DATABASE_URL. Without --prefix all keys are included as-is.

Use --environment to filter to keys tagged with a specific environment label.
Encrypted values are decrypted automatically when --encryption-key-file is set.
Private values are included — secure your terminal session accordingly.

Examples:
  eval $(kh kv env --workspace prod)
  eval $(kh kv env --workspace prod --prefix KH_ENV_)
  kh kv env --workspace <uuid> --format dotenv > .env
  kh kv env --workspace <uuid> --prefix KH_ENV_ --format dotenv > .env
  kh kv env --workspace prod --environment staging`,
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

			pairs := resolveKVPairs(cmd, client, ctx, workspaceUUID, prefix, environment, encKey)
			out := cmd.OutOrStdout()
			for _, p := range pairs {
				// Single-quote the value and escape any embedded single quotes.
				escaped := strings.ReplaceAll(p.Value, "'", `'\''`)
				switch format {
				case "dotenv":
					fmt.Fprintf(out, "%s='%s'\n", p.Name, escaped)
				default: // "export"
					fmt.Fprintf(out, "export %s='%s'\n", p.Name, escaped)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "export", "Output format: export|dotenv")
	cmd.Flags().StringVar(&environment, "environment", "", "Filter to keys tagged with this environment label")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Only include keys with this prefix; strip it from the variable name (e.g. KH_ENV_)")
	return cmd
}

// ── run ───────────────────────────────────────────────────────────────────────

func newKVRunCmd(opts *kvCmdOpts) *cobra.Command {
	var environment string
	var prefix string
	cmd := &cobra.Command{
		Use:   "run -- <command> [args...]",
		Short: "Run a command with workspace key/values injected as environment variables",
		Long: `Fetch all key/value pairs from a workspace and exec a command with those
key/value pairs injected into its environment.

The child process inherits the current environment plus the workspace keys.
Workspace values override any existing environment variable with the same name.

Use --prefix to include only keys that start with the given prefix. The prefix
is stripped from the variable name before injection, so KH_ENV_DATABASE_URL
becomes DATABASE_URL in the child process environment.

Encrypted values are decrypted automatically when --encryption-key-file is set.
Use --environment to inject only keys tagged with a specific environment label.

Examples:
  kh kv run --workspace prod -- terraform apply
  kh kv run --workspace prod --prefix KH_ENV_ -- terraform apply
  kh kv run --workspace <uuid> -- ./deploy.sh
  kh kv run --workspace prod --environment staging -- printenv`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errors.New("a command to run is required after --")
			}
			return nil
		},
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

			// Start from the current process environment.
			env := os.Environ()
			for _, p := range resolveKVPairs(cmd, client, ctx, workspaceUUID, prefix, environment, encKey) {
				env = append(env, p.Name+"="+p.Value)
			}

			bin, err := exec.LookPath(args[0])
			if err != nil {
				return fmt.Errorf("command not found: %s", args[0])
			}
			return syscall.Exec(bin, args, env)
		},
	}
	cmd.Flags().StringVar(&environment, "environment", "", "Filter to keys tagged with this environment label")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Only include keys with this prefix; strip it from the variable name (e.g. KH_ENV_)")
	return cmd
}
