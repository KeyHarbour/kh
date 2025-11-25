package khclient

import (
	"context"
	"net/http"
	"net/url"
)

func (c *Client) GetProject(ctx context.Context, projectUUID string) (Project, error) {
	if projectUUID == "" {
		return Project{}, APIError{StatusCode: http.StatusBadRequest, Message: "project uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/v1/projects/"+url.PathEscape(projectUUID), nil, nil, nil)
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
	resp, err := c.do(ctx, http.MethodGet, "/v1/projects/"+url.PathEscape(projectUUID)+"/workspaces", nil, nil, nil)
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

func (c *Client) GetWorkspace(ctx context.Context, projectUUID, workspaceUUID string) (Workspace, error) {
	if projectUUID == "" || workspaceUUID == "" {
		return Workspace{}, APIError{StatusCode: http.StatusBadRequest, Message: "project and workspace uuid are required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/v1/projects/"+url.PathEscape(projectUUID)+"/workspaces/"+url.PathEscape(workspaceUUID), nil, nil, nil)
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
