package backend

import (
	"context"
	"fmt"

	"kh/internal/khclient"
)

// KeyHarbourReader reads state from KeyHarbour
type KeyHarbourReader struct {
	client      *khclient.Client
	project     string
	workspace   string
	statefileID string
	env         string
}

func NewKeyHarbourReader(client *khclient.Client, project, workspace, statefileID, env string) *KeyHarbourReader {
	return &KeyHarbourReader{
		client:      client,
		project:     project,
		workspace:   workspace,
		statefileID: statefileID,
		env:         env,
	}
}

func (r *KeyHarbourReader) List(ctx context.Context) ([]Object, error) {
	// Resolve project
	projectUUID, err := r.resolveProject(ctx)
	if err != nil {
		return nil, err
	}

	// Resolve workspace
	workspaceUUID, workspaceName, err := r.resolveWorkspace(ctx, projectUUID)
	if err != nil {
		return nil, err
	}

	// If specific statefile ID is provided, return just that one
	if r.statefileID != "" {
		sf, err := r.client.GetStatefile(ctx, projectUUID, workspaceUUID, r.statefileID)
		if err != nil {
			return nil, fmt.Errorf("failed to get statefile %s: %w", r.statefileID, err)
		}
		return []Object{{
			Key:       sf.UUID,
			Size:      int64(len(sf.Content)),
			Checksum:  "", // Statefile doesn't have checksum
			Workspace: workspaceName,
			URL:       fmt.Sprintf("/projects/%s/workspaces/%s/statefiles/%s", projectUUID, workspaceUUID, sf.UUID),
		}}, nil
	}

	// Otherwise list statefiles
	statefiles, err := r.client.ListStatefiles(ctx, projectUUID, workspaceUUID, r.env)
	if err != nil {
		return nil, fmt.Errorf("failed to list statefiles: %w", err)
	}

	objs := make([]Object, 0, len(statefiles))
	for _, sf := range statefiles {
		objs = append(objs, Object{
			Key:       sf.UUID,
			Size:      int64(len(sf.Content)),
			Workspace: workspaceName,
			URL:       fmt.Sprintf("/projects/%s/workspaces/%s/statefiles/%s", projectUUID, workspaceUUID, sf.UUID),
		})
	}

	return objs, nil
}

func (r *KeyHarbourReader) Get(ctx context.Context, key string) ([]byte, Object, error) {
	// Resolve project and workspace
	projectUUID, err := r.resolveProject(ctx)
	if err != nil {
		return nil, Object{}, err
	}

	workspaceUUID, workspaceName, err := r.resolveWorkspace(ctx, projectUUID)
	if err != nil {
		return nil, Object{}, err
	}

	// Get statefile by UUID (key)
	sf, err := r.client.GetStatefile(ctx, projectUUID, workspaceUUID, key)
	if err != nil {
		return nil, Object{}, fmt.Errorf("failed to get statefile %s: %w", key, err)
	}

	obj := Object{
		Key:       key,
		Size:      int64(len(sf.Content)),
		Workspace: workspaceName,
		URL:       fmt.Sprintf("/projects/%s/workspaces/%s/statefiles/%s", projectUUID, workspaceUUID, key),
	}

	return []byte(sf.Content), obj, nil
}

func (r *KeyHarbourReader) resolveProject(ctx context.Context) (string, error) {
	if r.project == "" {
		return "", fmt.Errorf("project is required for --from=keyharbour (use --src-project)")
	}

	// Try to get project directly (assume it's a UUID)
	proj, err := r.client.GetProject(ctx, r.project)
	if err != nil {
		return "", fmt.Errorf("failed to get project %q: %w", r.project, err)
	}

	return proj.UUID, nil
}

func (r *KeyHarbourReader) resolveWorkspace(ctx context.Context, projectUUID string) (string, string, error) {
	if r.workspace == "" {
		return "", "", fmt.Errorf("workspace is required for --from=keyharbour (use --src-workspace)")
	}

	// List workspaces
	workspaces, err := r.client.ListWorkspaces(ctx, projectUUID)
	if err != nil {
		return "", "", fmt.Errorf("failed to list workspaces: %w", err)
	}

	// Try by name or UUID
	for _, ws := range workspaces {
		if ws.Name == r.workspace || ws.UUID == r.workspace {
			return ws.UUID, ws.Name, nil
		}
	}

	return "", "", fmt.Errorf("workspace %q not found in project", r.workspace)
}

// KeyHarbourWriter writes state to KeyHarbour
type KeyHarbourWriter struct {
	client          *khclient.Client
	projectUUID     string
	workspace       string
	env             string
	createWorkspace bool
}

func NewKeyHarbourWriter(client *khclient.Client, projectUUID, workspace, env string, createWorkspace bool) *KeyHarbourWriter {
	return &KeyHarbourWriter{
		client:          client,
		projectUUID:     projectUUID,
		workspace:       workspace,
		env:             env,
		createWorkspace: createWorkspace,
	}
}

func (w *KeyHarbourWriter) Put(ctx context.Context, key string, data []byte, overwrite bool) (Object, error) {
	// key is the workspace name for KeyHarbour destinations
	workspaceName := key

	// Resolve or create workspace
	ws, err := w.resolveWorkspace(ctx, workspaceName)
	if err != nil {
		return Object{}, err
	}

	// Determine environment
	envTag := w.env
	if envTag == "" {
		// Try to get project environments
		proj, err := w.client.GetProject(ctx, w.projectUUID)
		if err == nil && len(proj.Environments) > 0 {
			envTag = proj.Environments[0]
		} else {
			envTag = "default"
		}
	}

	// Create statefile
	_, err = w.client.CreateStatefile(ctx, w.projectUUID, ws.UUID, envTag, khclient.CreateStatefileRequest{
		Content: string(data),
	})
	if err != nil {
		return Object{}, fmt.Errorf("failed to create statefile: %w", err)
	}

	return Object{
		Key:       workspaceName,
		Size:      int64(len(data)),
		Workspace: workspaceName,
		URL:       fmt.Sprintf("/projects/%s/workspaces/%s/statefiles", w.projectUUID, ws.UUID),
	}, nil
}

func (w *KeyHarbourWriter) resolveWorkspace(ctx context.Context, name string) (*khclient.Workspace, error) {
	// Try to find existing workspace
	workspaces, err := w.client.ListWorkspaces(ctx, w.projectUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}

	for _, ws := range workspaces {
		if ws.Name == name {
			return &ws, nil
		}
	}

	// Not found - create if allowed
	if !w.createWorkspace {
		return nil, fmt.Errorf("workspace %q not found (use --create-workspace)", name)
	}

	newWs, err := w.client.CreateWorkspace(ctx, w.projectUUID, khclient.CreateWorkspaceRequest{
		Name: name,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace %q: %w", name, err)
	}

	return &khclient.Workspace{
		UUID: newWs.UUID,
		Name: name,
	}, nil
}
