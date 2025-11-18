package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"kh/internal/config"
	"kh/internal/khclient"
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
		return khclient.Project{}, fmt.Errorf("project reference is required")
	}
	if looksLikeUUID(ref) {
		return client.GetProject(ctx, ref)
	}
	projects, err := client.ListProjects(ctx)
	if err != nil {
		return khclient.Project{}, err
	}
	for _, p := range projects {
		if strings.EqualFold(p.UUID, ref) || strings.EqualFold(p.Name, ref) {
			return p, nil
		}
	}
	return khclient.Project{}, fmt.Errorf("project %q not found", ref)
}

func resolveWorkspaceRef(ctx context.Context, client *khclient.Client, projectUUID, ref string) (khclient.Workspace, error) {
	if projectUUID == "" {
		return khclient.Workspace{}, fmt.Errorf("project uuid is required to resolve workspace")
	}
	if ref == "" {
		return khclient.Workspace{}, fmt.Errorf("workspace reference is required")
	}
	if looksLikeUUID(ref) {
		return client.GetWorkspace(ctx, projectUUID, ref)
	}
	workspaces, err := client.ListWorkspaces(ctx, projectUUID)
	if err != nil {
		return khclient.Workspace{}, err
	}
	for _, w := range workspaces {
		if strings.EqualFold(w.UUID, ref) || strings.EqualFold(w.Name, ref) {
			return w, nil
		}
	}
	return khclient.Workspace{}, fmt.Errorf("workspace %q not found in project %s", ref, projectUUID)
}

func projectRefOrEnv(flagValue string, cfg config.Config) string {
	if flagValue != "" {
		return flagValue
	}
	return config.FromEnvOr(cfg, "KH_PROJECT", "")
}
