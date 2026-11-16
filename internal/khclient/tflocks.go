package khclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// The Terraform HTTP backend protocol is served at the host root under
// /statefiles/v1/{workspace_uuid}/{environment}, outside the /api/v2 prefix the
// rest of the API uses. Locking is only available through this protocol: PUT
// locks and DELETE unlocks. The lock is held per workspace; the environment
// segment is required by the route but does not scope the lock.

// ErrNotLocked is returned by UnlockWorkspace when the workspace holds no lock.
var ErrNotLocked = errors.New("workspace is not locked")

// LockInfo mirrors Terraform's state lock info, which the backend stores and
// echoes back as the current holder when a lock attempt conflicts.
type LockInfo struct {
	ID        string `json:"ID"`
	Operation string `json:"Operation,omitempty"`
	Info      string `json:"Info,omitempty"`
	Who       string `json:"Who,omitempty"`
	Version   string `json:"Version,omitempty"`
	Created   string `json:"Created,omitempty"`
	Path      string `json:"Path,omitempty"`
}

// LockedError reports that the workspace is already locked by someone else.
type LockedError struct {
	Holder LockInfo
}

func (e *LockedError) Error() string {
	var parts []string
	if e.Holder.Who != "" {
		parts = append(parts, "by "+e.Holder.Who)
	}
	if e.Holder.Operation != "" {
		parts = append(parts, "for "+e.Holder.Operation)
	}
	if e.Holder.Created != "" {
		parts = append(parts, "since "+e.Holder.Created)
	}
	if e.Holder.ID != "" {
		parts = append(parts, "(lock ID "+e.Holder.ID+")")
	}
	if len(parts) == 0 {
		return "workspace is already locked"
	}
	return "workspace is already locked " + strings.Join(parts, " ")
}

// HostRoot returns the configured endpoint without its /api/v2 suffix.
func HostRoot(endpoint string) string {
	return strings.TrimSuffix(strings.TrimRight(endpoint, "/"), "/api/v2")
}

// TerraformBackendPath returns the Terraform HTTP backend path for a workspace
// and environment, relative to HostRoot.
func TerraformBackendPath(workspaceUUID, environment string) string {
	return fmt.Sprintf("/statefiles/v1/%s/%s", url.PathEscape(workspaceUUID), url.PathEscape(environment))
}

func terraformBackendPath(workspaceUUID, environment string) (string, error) {
	if workspaceUUID == "" {
		return "", fmt.Errorf("workspace uuid is required")
	}
	if environment == "" {
		return "", fmt.Errorf("environment is required")
	}
	return TerraformBackendPath(workspaceUUID, environment), nil
}

// LockWorkspace acquires the workspace lock. When the workspace is already
// locked it returns a *LockedError describing the current holder.
func (c *Client) LockWorkspace(ctx context.Context, workspaceUUID, environment string, info LockInfo) error {
	p, err := terraformBackendPath(workspaceUUID, environment)
	if err != nil {
		return err
	}
	resp, err := c.doAt(ctx, HostRoot(c.Endpoint), http.MethodPut, p, nil, info, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict || resp.StatusCode == http.StatusLocked {
		var holder LockInfo
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = json.Unmarshal(data, &holder)
		return &LockedError{Holder: holder}
	}
	return expectStatus("lock workspace", resp, http.StatusOK, http.StatusNoContent)
}

// UnlockWorkspace releases the workspace lock and returns the ID of the lock
// that was released. It returns ErrNotLocked when there was no lock.
//
// The backend does not yet check lockID against the holder, so callers that
// need that guarantee must check it themselves (see LockHolder).
func (c *Client) UnlockWorkspace(ctx context.Context, workspaceUUID, environment, lockID string) (string, error) {
	p, err := terraformBackendPath(workspaceUUID, environment)
	if err != nil {
		return "", err
	}
	resp, err := c.doAt(ctx, HostRoot(c.Endpoint), http.MethodDelete, p, nil, LockInfo{ID: lockID}, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusConflict {
		return "", ErrNotLocked
	}
	if err := expectStatus("unlock workspace", resp, http.StatusOK, http.StatusNoContent); err != nil {
		return "", err
	}
	var released LockInfo
	if resp.StatusCode == http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
		_ = json.Unmarshal(data, &released)
	}
	return released.ID, nil
}

// LockHolder reports the current lock on a workspace, or nil when it is not
// locked.
//
// The API has no lock inspection endpoint, so this probes with a lock attempt:
// a conflict reveals the holder, and a successful attempt is released straight
// away. The probe briefly holds the lock, so a Terraform run that tries to lock
// at that exact moment fails and must be retried.
func (c *Client) LockHolder(ctx context.Context, workspaceUUID, environment string, probe LockInfo) (*LockInfo, error) {
	err := c.LockWorkspace(ctx, workspaceUUID, environment, probe)
	var locked *LockedError
	if errors.As(err, &locked) {
		return &locked.Holder, nil
	}
	if err != nil {
		return nil, err
	}
	if _, err := c.UnlockWorkspace(ctx, workspaceUUID, environment, probe.ID); err != nil && !errors.Is(err, ErrNotLocked) {
		return nil, fmt.Errorf("release probe lock %s: %w", probe.ID, err)
	}
	return nil, nil
}
