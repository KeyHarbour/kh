package cli

import (
	"context"
	"errors"
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
	target := statefileTarget{}
	cfg := config.Config{Project: "cfg"}
	if _, _, err := target.resolve(context.Background(), &fakeResolver{}, cfg); err == nil {
		t.Fatalf("expected error for missing workspace")
	}
}
