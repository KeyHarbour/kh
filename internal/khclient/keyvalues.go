package khclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

func kvPath(projectUUID, workspaceUUID, key string) (string, error) {
	if projectUUID == "" {
		return "", fmt.Errorf("project uuid is required")
	}
	if workspaceUUID == "" {
		return "", fmt.Errorf("workspace uuid is required")
	}
	base := fmt.Sprintf("/v1/projects/%s/workspaces/%s/keyvalues",
		url.PathEscape(projectUUID), url.PathEscape(workspaceUUID))
	if key != "" {
		base = base + "/" + url.PathEscape(key)
	}
	return base, nil
}

// ListKeyValues returns all key/value pairs for the given workspace, optionally
// filtered by environment.
func (c *Client) ListKeyValues(ctx context.Context, projectUUID, workspaceUUID, environment string) ([]KeyValue, error) {
	p, err := kvPath(projectUUID, workspaceUUID, "")
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if environment != "" {
		q.Set("environment", environment)
	}
	resp, err := c.do(ctx, http.MethodGet, p, q, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return []KeyValue{}, nil
	}
	if err := expectStatus("list keyvalues", resp, http.StatusOK); err != nil {
		return nil, err
	}
	var out []KeyValue
	return out, decodeJSON(resp, &out)
}

// GetKeyValue fetches a single key/value by key name.
func (c *Client) GetKeyValue(ctx context.Context, projectUUID, workspaceUUID, key string) (KeyValue, error) {
	if key == "" {
		return KeyValue{}, APIError{StatusCode: http.StatusBadRequest, Message: "key is required"}
	}
	p, err := kvPath(projectUUID, workspaceUUID, key)
	if err != nil {
		return KeyValue{}, err
	}
	resp, err := c.do(ctx, http.MethodGet, p, nil, nil, nil)
	if err != nil {
		return KeyValue{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get keyvalue", resp, http.StatusOK); err != nil {
		return KeyValue{}, err
	}
	// The single-key response doesn't include the key name; embed it.
	var out KeyValue
	if err := decodeJSON(resp, &out); err != nil {
		return KeyValue{}, err
	}
	out.Key = key
	return out, nil
}

// CreateKeyValue creates a new key/value entry under the given workspace and environment.
func (c *Client) CreateKeyValue(ctx context.Context, projectUUID, workspaceUUID, environment string, req CreateKeyValueRequest) error {
	if environment == "" {
		return fmt.Errorf("environment is required to create a key/value")
	}
	p, err := kvPath(projectUUID, workspaceUUID, "")
	if err != nil {
		return err
	}
	q := url.Values{}
	q.Set("environment", environment)
	resp, err := c.do(ctx, http.MethodPost, p, q, req, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("create keyvalue", resp, http.StatusCreated)
}

// UpdateKeyValue updates an existing key/value entry.
func (c *Client) UpdateKeyValue(ctx context.Context, projectUUID, workspaceUUID, key string, req UpdateKeyValueRequest) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	p, err := kvPath(projectUUID, workspaceUUID, key)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodPatch, p, nil, req, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("update keyvalue", resp, http.StatusAccepted)
}

// DeleteKeyValue removes a key/value entry.
func (c *Client) DeleteKeyValue(ctx context.Context, projectUUID, workspaceUUID, key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	p, err := kvPath(projectUUID, workspaceUUID, key)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodDelete, p, nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete keyvalue", resp, http.StatusNoContent)
}
