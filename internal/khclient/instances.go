package khclient

import (
	"context"
	"net/http"
	"net/url"
)

// ListInstances returns all instances for the given application.
func (c *Client) ListInstances(ctx context.Context, applicationUUID string) ([]Instance, error) {
	if applicationUUID == "" {
		return nil, APIError{StatusCode: http.StatusBadRequest, Message: "application uuid is required"}
	}
	p := "/license/applications/" + url.PathEscape(applicationUUID) + "/instances"
	resp, err := c.do(ctx, http.MethodGet, p, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := expectStatus("list instances", resp, http.StatusOK); err != nil {
		return nil, err
	}
	var out []Instance
	return out, decodeJSON(resp, &out)
}

// GetInstance returns a single instance by UUID.
func (c *Client) GetInstance(ctx context.Context, uuid string) (Instance, error) {
	if uuid == "" {
		return Instance{}, APIError{StatusCode: http.StatusBadRequest, Message: "instance uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/license/instances/"+url.PathEscape(uuid), nil, nil, nil)
	if err != nil {
		return Instance{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get instance", resp, http.StatusOK); err != nil {
		return Instance{}, err
	}
	var out Instance
	if err := decodeJSON(resp, &out); err != nil {
		return Instance{}, err
	}
	if out.UUID == "" {
		out.UUID = uuid
	}
	return out, nil
}

// CreateInstance creates a new instance under the given application and returns the created record.
func (c *Client) CreateInstance(ctx context.Context, applicationUUID string, req CreateInstanceRequest) (Instance, error) {
	if applicationUUID == "" {
		return Instance{}, APIError{StatusCode: http.StatusBadRequest, Message: "application uuid is required"}
	}
	body := struct {
		Instance CreateInstanceRequest `json:"instance"`
	}{Instance: req}
	p := "/license/applications/" + url.PathEscape(applicationUUID) + "/instances"
	resp, err := c.do(ctx, http.MethodPost, p, nil, body, nil)
	if err != nil {
		return Instance{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("create instance", resp, http.StatusCreated); err != nil {
		return Instance{}, err
	}
	var out Instance
	if err := decodeJSON(resp, &out); err != nil {
		return Instance{}, err
	}
	return out, nil
}

// UpdateInstance updates an existing instance.
func (c *Client) UpdateInstance(ctx context.Context, uuid string, req UpdateInstanceRequest) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "instance uuid is required"}
	}
	body := struct {
		Instance UpdateInstanceRequest `json:"instance"`
	}{Instance: req}
	resp, err := c.do(ctx, http.MethodPatch, "/license/instances/"+url.PathEscape(uuid), nil, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("update instance", resp, http.StatusAccepted)
}

// DeleteInstance deletes an instance by UUID.
func (c *Client) DeleteInstance(ctx context.Context, uuid string) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "instance uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodDelete, "/license/instances/"+url.PathEscape(uuid), nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete instance", resp, http.StatusNoContent)
}
