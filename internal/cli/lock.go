package cli

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/KeyHarbour/kh/internal/config"
	"github.com/KeyHarbour/kh/internal/khclient"
	"github.com/KeyHarbour/kh/internal/kherrors"
	"github.com/KeyHarbour/kh/internal/output"
	"github.com/KeyHarbour/kh/pkg/version"

	"github.com/spf13/cobra"
)

// lockTarget is the workspace and environment addressed by the lock commands.
// Locks cover the whole workspace; the environment is only required because
// the backend route includes it.
type lockTarget struct {
	workspace   string
	environment string
}

func (t *lockTarget) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&t.workspace, "workspace", "", "Workspace UUID (or KH_WORKSPACE)")
	cmd.Flags().StringVar(&t.environment, "environment", "", "Environment name of the workspace's project (required)")
}

// resolve validates the target and returns a client for it.
func (t *lockTarget) resolve() (*khclient.Client, string, error) {
	cfg, err := config.LoadWithEnv()
	if err != nil {
		return nil, "", kherrors.ErrConfigLoad.Wrapf(err, "failed to load configuration: %s", err)
	}
	if cfg.Token == "" {
		return nil, "", kherrors.ErrMissingToken.New("not logged in: KH_TOKEN is missing")
	}
	ws := workspaceRefOrEnv(t.workspace, cfg)
	if ws == "" {
		return nil, "", kherrors.ErrMissingFlag.New("--workspace is required (or set KH_WORKSPACE)")
	}
	if !looksLikeUUID(ws) {
		return nil, "", kherrors.ErrInvalidValue.Newf("workspace %q is not a valid UUID", ws)
	}
	if t.environment == "" {
		return nil, "", kherrors.ErrMissingFlag.New("--environment is required")
	}
	return khclient.New(cfg), ws, nil
}

// newLockInfo describes a lock taken by kh, in the shape Terraform uses so the
// holder reads sensibly in Terraform's own lock errors.
func newLockInfo(operation, info string) khclient.LockInfo {
	return khclient.LockInfo{
		ID:        newLockID(),
		Operation: operation,
		Info:      info,
		Who:       lockWho(),
		Version:   "kh " + version.Version,
		Created:   time.Now().UTC().Format(time.RFC3339),
	}
}

// newLockID returns a random RFC 4122 version 4 UUID, as Terraform uses for
// lock IDs.
func newLockID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func lockWho() string {
	name := "unknown"
	if u, err := user.Current(); err == nil && u.Username != "" {
		name = u.Username
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		return name
	}
	return name + "@" + host
}

// lockConflictError converts a lock conflict into the KH-CONF-001 error.
func lockConflictError(err error, workspaceUUID string) error {
	var locked *khclient.LockedError
	if errors.As(err, &locked) {
		return kherrors.ErrStateLocked.Wrapf(err, "workspace %s: %s", workspaceUUID, err)
	}
	return err
}

func newLockCmd() *cobra.Command {
	var target lockTarget
	var reason string
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock a workspace's Terraform state",
		Long: `Acquire the Terraform state lock on a workspace. While it is held,
Terraform runs and 'kh tf sync --lock' against the workspace fail with a lock
error.

The lock covers the whole workspace. --environment is required by the backend
route but does not narrow the lock.

Release it with 'kh tf state unlock <lock-id>'.

Examples:
  kh tf state lock --workspace <workspace-uuid> --environment prod --reason "maintenance"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, ws, err := target.resolve()
			if err != nil {
				return err
			}
			info := newLockInfo("kh-lock", reason)
			if err := client.LockWorkspace(cmd.Context(), ws, target.environment, info); err != nil {
				return lockConflictError(err, ws)
			}
			printer := output.Printer{Format: outputFormat, W: cmd.OutOrStdout()}
			if printer.Format == "json" {
				return printer.JSON(map[string]any{"workspace": ws, "environment": target.environment, "lock": info})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Locked workspace %s (lock ID %s)\n", ws, info.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "Release with: kh tf state unlock --workspace %s --environment %s %s\n", ws, target.environment, info.ID)
			return nil
		},
	}
	target.addFlags(cmd)
	cmd.Flags().StringVar(&reason, "reason", "", "Why the lock is held (shown to anyone who hits it)")
	return cmd
}
