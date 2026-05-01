package khclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CreateWorkspaceRequest represents the request body for creating a workspace
type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// CreateWorkspaceResponse represents the response from creating a workspace
type CreateWorkspaceResponse struct {
	Status string `json:"status"`
	UUID   string `json:"uuid,omitempty"`
}

func (c *Client) GetProject(ctx context.Context, projectUUID string) (Project, error) {
	if projectUUID == "" {
		return Project{}, APIError{StatusCode: http.StatusBadRequest, Message: "project uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectUUID), nil, nil, nil)
	if err != nil {
		return Project{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get project", resp, http.StatusOK); err != nil {
		return Project{}, err
	}
	var out Project
	if err := decodeJSON(resp, &out); err != nil {
		return Project{}, err
	}
	if out.UUID == "" {
		out.UUID = projectUUID
	}
	return out, nil
}

func (c *Client) ListWorkspaces(ctx context.Context, projectUUID string) ([]Workspace, error) {
	if projectUUID == "" {
		return nil, APIError{StatusCode: http.StatusBadRequest, Message: "project uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/projects/"+url.PathEscape(projectUUID)+"/workspaces", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := expectStatus("list workspaces", resp, http.StatusOK); err != nil {
		return nil, err
	}
	var out []Workspace
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetWorkspace(ctx context.Context, workspaceUUID string) (Workspace, error) {
	if workspaceUUID == "" {
		return Workspace{}, APIError{StatusCode: http.StatusBadRequest, Message: "workspace uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/workspaces/"+url.PathEscape(workspaceUUID), nil, nil, nil)
	if err != nil {
		return Workspace{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get workspace", resp, http.StatusOK); err != nil {
		return Workspace{}, err
	}
	var out Workspace
	if err := decodeJSON(resp, &out); err != nil {
		return Workspace{}, err
	}
	if out.UUID == "" {
		out.UUID = workspaceUUID
	}
	return out, nil
}

// CreateWorkspace creates a new workspace in a project
func (c *Client) CreateWorkspace(ctx context.Context, projectUUID string, req CreateWorkspaceRequest) (Workspace, error) {
	if projectUUID == "" {
		return Workspace{}, APIError{StatusCode: http.StatusBadRequest, Message: "project uuid is required"}
	}
	if req.Name == "" {
		return Workspace{}, APIError{StatusCode: http.StatusBadRequest, Message: "workspace name is required"}
	}
	// Validate workspace name (alphanumeric only)
	if !isAlphanumeric(req.Name) {
		return Workspace{}, APIError{StatusCode: http.StatusBadRequest, Message: fmt.Sprintf("workspace name %q must contain only letters and numbers (no hyphens or special characters)", req.Name)}
	}
	body := struct {
		Workspace CreateWorkspaceRequest `json:"workspace"`
	}{Workspace: req}
	resp, err := c.do(ctx, http.MethodPost, "/projects/"+url.PathEscape(projectUUID)+"/workspaces", nil, body, nil)
	if err != nil {
		return Workspace{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("create workspace", resp, http.StatusCreated, http.StatusOK); err != nil {
		return Workspace{}, err
	}
	// Try to decode as workspace response (some APIs return the created object)
	var out Workspace
	if err := decodeJSON(resp, &out); err != nil {
		// If it's just a status response, return a workspace with the name
		return Workspace{Name: req.Name}, nil
	}
	if out.Name == "" {
		out.Name = req.Name
	}
	// If UUID is not in the response, we need to fetch the workspace by name
	if out.UUID == "" {
		// List workspaces and find by name
		workspaces, listErr := c.ListWorkspaces(ctx, projectUUID)
		if listErr != nil {
			return out, nil // Return without UUID rather than failing
		}
		for _, ws := range workspaces {
			if ws.Name == req.Name {
				out.UUID = ws.UUID
				break
			}
		}
	}
	return out, nil
}

// isAlphanumeric checks if a string contains only letters and numbers
func isAlphanumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// UpdateWorkspace updates the name and/or description of an existing workspace.
func (c *Client) UpdateWorkspace(ctx context.Context, workspaceUUID string, req UpdateWorkspaceRequest) error {
	if workspaceUUID == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "workspace uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodPatch, "/workspaces/"+url.PathEscape(workspaceUUID), nil, req, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("update workspace", resp, http.StatusAccepted)
}

// DeleteWorkspace permanently deletes a workspace.
func (c *Client) DeleteWorkspace(ctx context.Context, workspaceUUID string) error {
	if workspaceUUID == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "workspace uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodDelete, "/workspaces/"+url.PathEscape(workspaceUUID), nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete workspace", resp, http.StatusNoContent)
}

// GetOrCreateWorkspace gets a workspace by name, or creates it if it doesn't exist
func (c *Client) GetOrCreateWorkspace(ctx context.Context, projectUUID, workspaceName string) (Workspace, bool, error) {
	// First, list workspaces to find by name
	workspaces, err := c.ListWorkspaces(ctx, projectUUID)
	if err != nil {
		return Workspace{}, false, err
	}

	// Look for existing workspace by name
	for _, ws := range workspaces {
		if ws.Name == workspaceName {
			return ws, false, nil // existed
		}
	}

	// Create new workspace
	ws, err := c.CreateWorkspace(ctx, projectUUID, CreateWorkspaceRequest{Name: workspaceName})
	if err != nil {
		return Workspace{}, false, err
	}
	return ws, true, nil // created
}
