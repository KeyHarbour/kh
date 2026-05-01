package cli

import (
	"context"
	"errors"
	"strings"
	"testing"

	"kh/internal/config"
	"kh/internal/khclient"
)

type fakeResolver struct {
	projectRef   string
	workspaceRef string
}

func (f *fakeResolver) ResolveProject(ctx context.Context, ref string) (khclient.Project, error) {
	if ref == "" {
		return khclient.Project{}, errors.New("missing project")
	}
	f.projectRef = ref
	return khclient.Project{UUID: "proj-uuid"}, nil
}

func (f *fakeResolver) ResolveWorkspace(ctx context.Context, projectUUID, ref string) (khclient.Workspace, error) {
	if ref == "" {
		return khclient.Workspace{}, errors.New("missing workspace")
	}
	f.workspaceRef = ref
	return khclient.Workspace{UUID: "ws-uuid"}, nil
}

func TestStatefileTargetResolveUsesConfigDefaults(t *testing.T) {
	t.Setenv("KH_PROJECT", "")
	target := statefileTarget{workspace: "ws"}
	cfg := config.Config{Project: "cfg-project"}
	res := &fakeResolver{}
	project, workspace, err := target.resolve(context.Background(), res, cfg)
	if err != nil {
		t.Fatalf("expected nil err: %v", err)
	}
	if res.projectRef != "cfg-project" {
		t.Fatalf("expected resolver to receive project from config, got %s", res.projectRef)
	}
	if project != "proj-uuid" || workspace != "ws-uuid" {
		t.Fatalf("unexpected ids: project=%s workspace=%s", project, workspace)
	}
	if res.workspaceRef != "ws" {
		t.Fatalf("expected resolver to receive workspace flag, got %s", res.workspaceRef)
	}
}

func TestStatefileTargetResolveRequiresWorkspace(t *testing.T) {
	t.Setenv("KH_WORKSPACE", "") // ensure env var is not set
	target := statefileTarget{}
	cfg := config.Config{Project: "cfg"}
	if _, _, err := target.resolve(context.Background(), &fakeResolver{}, cfg); err == nil {
		t.Fatalf("expected error for missing workspace")
	}
}

func TestStatefileTargetResolveUsesKHWorkspace(t *testing.T) {
	t.Setenv("KH_WORKSPACE", "env-workspace")
	target := statefileTarget{} // no workspace flag
	cfg := config.Config{Project: "cfg-project"}
	res := &fakeResolver{}
	_, _, err := target.resolve(context.Background(), res, cfg)
	if err != nil {
		t.Fatalf("expected nil err with KH_WORKSPACE set: %v", err)
	}
	if res.workspaceRef != "env-workspace" {
		t.Fatalf("expected resolver to receive KH_WORKSPACE, got %q", res.workspaceRef)
	}
}

func TestStatefileTargetResolveRequiresProject(t *testing.T) {
	t.Setenv("KH_PROJECT", "")
	target := statefileTarget{workspace: "ws"}
	cfg := config.Config{}
	_, _, err := target.resolve(context.Background(), &fakeResolver{}, cfg)
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("expected missing project error, got %v", err)
	}
}

type errorResolver struct {
	projectErr   error
	workspaceErr error
}

func (r *errorResolver) ResolveProject(_ context.Context, _ string) (khclient.Project, error) {
	if r.projectErr != nil {
		return khclient.Project{}, r.projectErr
	}
	return khclient.Project{UUID: "proj-uuid"}, nil
}

func (r *errorResolver) ResolveWorkspace(_ context.Context, _, _ string) (khclient.Workspace, error) {
	if r.workspaceErr != nil {
		return khclient.Workspace{}, r.workspaceErr
	}
	return khclient.Workspace{UUID: "ws-uuid"}, nil
}

func TestStatefileTargetResolveProjectError(t *testing.T) {
	target := statefileTarget{project: "p", workspace: "w"}
	res := &errorResolver{projectErr: errors.New("project api error")}
	_, _, err := target.resolve(context.Background(), res, config.Config{})
	if err == nil || !strings.Contains(err.Error(), "project api error") {
		t.Fatalf("expected project error, got %v", err)
	}
}

func TestStatefileTargetResolveWorkspaceError(t *testing.T) {
	target := statefileTarget{project: "p", workspace: "w"}
	res := &errorResolver{workspaceErr: errors.New("workspace api error")}
	_, _, err := target.resolve(context.Background(), res, config.Config{})
	if err == nil || !strings.Contains(err.Error(), "workspace api error") {
		t.Fatalf("expected workspace error, got %v", err)
	}
}
