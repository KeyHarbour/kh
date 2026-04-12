package cli

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"kh/internal/config"
	"kh/internal/kherrors"
	"kh/internal/khclient"
	"kh/internal/kherrors"
)

type referenceResolver interface {
	ResolveProject(ctx context.Context, ref string) (khclient.Project, error)
	ResolveWorkspace(ctx context.Context, projectUUID, ref string) (khclient.Workspace, error)
}

type clientReferenceResolver struct {
	client *khclient.Client
}

func (r clientReferenceResolver) ResolveProject(ctx context.Context, ref string) (khclient.Project, error) {
	return resolveProjectRef(ctx, r.client, ref)
}

func (r clientReferenceResolver) ResolveWorkspace(ctx context.Context, projectUUID, ref string) (khclient.Workspace, error) {
	return resolveWorkspaceRef(ctx, r.client, projectUUID, ref)
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func looksLikeUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

func resolveProjectRef(ctx context.Context, client *khclient.Client, ref string) (khclient.Project, error) {
	if ref == "" {
		return khclient.Project{}, kherrors.ErrMissingFlag.New("project reference is required")
	}
	proj, err := client.GetProject(ctx, ref)
	if err == nil {
		return proj, nil
	}
	var apiErr khclient.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
		return khclient.Project{}, kherrors.ErrNotFound.Newf("project %q not found. Provide the project UUID exactly as shown in KeyHarbour", ref)
	}
	return khclient.Project{}, err
}

func resolveWorkspaceRef(ctx context.Context, client *khclient.Client, projectUUID, ref string) (khclient.Workspace, error) {
	if ref == "" {
		return khclient.Workspace{}, kherrors.ErrMissingFlag.New("workspace uuid is required")
	}
	if !looksLikeUUID(ref) {
		return khclient.Workspace{}, kherrors.ErrInvalidValue.Newf("workspace %q is not a valid UUID — workspace names are no longer supported, use the workspace UUID", ref)
	}
	return client.GetWorkspace(ctx, ref)
}

func projectRefOrEnv(flagValue string, cfg config.Config) string {
	if flagValue != "" {
		return flagValue
	}
	return config.FromEnvOr(cfg, "KH_PROJECT", "")
}
