package khclient

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// ListKeyValues returns all key/value pairs for the given workspace.
func (c *Client) ListKeyValues(ctx context.Context, workspaceUUID string) ([]KeyValue, error) {
	if workspaceUUID == "" {
		return nil, fmt.Errorf("workspace uuid is required")
	}
	p := "/workspaces/" + url.PathEscape(workspaceUUID) + "/keyvalues"
	resp, err := c.do(ctx, http.MethodGet, p, nil, nil, nil)
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
func (c *Client) GetKeyValue(ctx context.Context, key string) (KeyValue, error) {
	if key == "" {
		return KeyValue{}, APIError{StatusCode: http.StatusBadRequest, Message: "key is required"}
	}
	p := "/keyvalues/" + url.PathEscape(key)
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

// CreateKeyValue creates a new key/value entry under the given workspace.
func (c *Client) CreateKeyValue(ctx context.Context, workspaceUUID string, req CreateKeyValueRequest) error {
	if workspaceUUID == "" {
		return fmt.Errorf("workspace uuid is required")
	}
	p := "/workspaces/" + url.PathEscape(workspaceUUID) + "/keyvalues"
	resp, err := c.do(ctx, http.MethodPost, p, nil, req, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("create keyvalue", resp, http.StatusCreated)
}

// UpdateKeyValue updates an existing key/value entry.
func (c *Client) UpdateKeyValue(ctx context.Context, key string, req UpdateKeyValueRequest) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	p := "/keyvalues/" + url.PathEscape(key)
	resp, err := c.do(ctx, http.MethodPatch, p, nil, req, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("update keyvalue", resp, http.StatusAccepted)
}

// DeleteKeyValue removes a key/value entry.
func (c *Client) DeleteKeyValue(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("key is required")
	}
	p := "/keyvalues/" + url.PathEscape(key)
	resp, err := c.do(ctx, http.MethodDelete, p, nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete keyvalue", resp, http.StatusNoContent)
}
