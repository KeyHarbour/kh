package khclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type CreateStatefileRequest struct {
	Content string `json:"content"`
}

func (c *Client) ListStatefiles(ctx context.Context, projectUUID, workspaceUUID, environment string) ([]Statefile, error) {
	path, err := statefilesPath(projectUUID, workspaceUUID, "")
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if environment != "" {
		q.Set("environment", environment)
	}
	resp, err := c.do(ctx, http.MethodGet, path, q, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return []Statefile{}, nil
	}
	if err := expectStatus("list statefiles", resp, http.StatusOK); err != nil {
		return nil, err
	}
	var out []Statefile
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetLastStatefile(ctx context.Context, projectUUID, workspaceUUID, environment string) (Statefile, error) {
	path, err := statefilesPath(projectUUID, workspaceUUID, "last")
	if err != nil {
		return Statefile{}, err
	}
	q := url.Values{}
	if environment != "" {
		q.Set("environment", environment)
	}
	resp, err := c.do(ctx, http.MethodGet, path, q, nil, nil)
	if err != nil {
		return Statefile{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get last statefile", resp, http.StatusOK); err != nil {
		return Statefile{}, err
	}
	var out Statefile
	if err := decodeJSON(resp, &out); err != nil {
		return Statefile{}, err
	}
	return out, nil
}

func (c *Client) GetStatefile(ctx context.Context, projectUUID, workspaceUUID, uuid string) (Statefile, error) {
	if uuid == "" {
		return Statefile{}, APIError{StatusCode: http.StatusBadRequest, Message: "statefile uuid is required"}
	}
	path, err := statefilesPath(projectUUID, workspaceUUID, uuid)
	if err != nil {
		return Statefile{}, err
	}
	resp, err := c.do(ctx, http.MethodGet, path, nil, nil, nil)
	if err != nil {
		return Statefile{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get statefile", resp, http.StatusOK); err != nil {
		return Statefile{}, err
	}
	var out Statefile
	if err := decodeJSON(resp, &out); err != nil {
		return Statefile{}, err
	}
	return out, nil
}

func (c *Client) CreateStatefile(ctx context.Context, projectUUID, workspaceUUID, environment string, body CreateStatefileRequest) (StatefileCreatedResponse, error) {
	path, err := statefilesPath(projectUUID, workspaceUUID, "")
	if err != nil {
		return StatefileCreatedResponse{}, err
	}
	q := url.Values{}
	if environment != "" {
		q.Set("environment", environment)
	}
	resp, err := c.do(ctx, http.MethodPost, path, q, body, nil)
	if err != nil {
		return StatefileCreatedResponse{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("create statefile", resp, http.StatusCreated, http.StatusOK); err != nil {
		return StatefileCreatedResponse{}, err
	}
	var out StatefileCreatedResponse
	if err := decodeJSON(resp, &out); err != nil {
		return StatefileCreatedResponse{}, err
	}
	return out, nil
}

func (c *Client) DeleteStatefiles(ctx context.Context, projectUUID, workspaceUUID string) error {
	path, err := statefilesPath(projectUUID, workspaceUUID, "")
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodDelete, path, nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete statefiles", resp, http.StatusNoContent)
}

func (c *Client) DeleteStatefile(ctx context.Context, projectUUID, workspaceUUID, uuid string) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "statefile uuid is required"}
	}
	path, err := statefilesPath(projectUUID, workspaceUUID, uuid)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodDelete, path, nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete statefile", resp, http.StatusNoContent)
}

func statefilesPath(projectUUID, workspaceUUID, suffix string) (string, error) {
	if projectUUID == "" {
		return "", fmt.Errorf("project uuid is required")
	}
	if workspaceUUID == "" {
		return "", fmt.Errorf("workspace uuid is required")
	}
	base := fmt.Sprintf("/v1/projects/%s/workspaces/%s/statefiles", url.PathEscape(projectUUID), url.PathEscape(workspaceUUID))
	if suffix != "" {
		base = base + "/" + url.PathEscape(suffix)
	}
	return base, nil
}
