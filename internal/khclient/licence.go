package khclient

import (
	"context"
	"net/http"
	"net/url"
)

// ListApplications returns all applications for the organisation.
func (c *Client) ListApplications(ctx context.Context) ([]Application, error) {
	resp, err := c.do(ctx, http.MethodGet, "/licence/applications", nil, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := expectStatus("list applications", resp, http.StatusOK); err != nil {
		return nil, err
	}
	var out []Application
	return out, decodeJSON(resp, &out)
}

// GetApplication returns a single application by UUID.
func (c *Client) GetApplication(ctx context.Context, uuid string) (Application, error) {
	if uuid == "" {
		return Application{}, APIError{StatusCode: http.StatusBadRequest, Message: "application uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/licence/applications/"+url.PathEscape(uuid), nil, nil, nil)
	if err != nil {
		return Application{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get application", resp, http.StatusOK); err != nil {
		return Application{}, err
	}
	var out Application
	if err := decodeJSON(resp, &out); err != nil {
		return Application{}, err
	}
	if out.UUID == "" {
		out.UUID = uuid
	}
	return out, nil
}

// CreateApplication creates a new application.
func (c *Client) CreateApplication(ctx context.Context, req CreateApplicationRequest) error {
	body := struct {
		Application CreateApplicationRequest `json:"application"`
	}{Application: req}
	resp, err := c.do(ctx, http.MethodPost, "/licence/applications", nil, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("create application", resp, http.StatusCreated)
}

// UpdateApplication updates an existing application.
func (c *Client) UpdateApplication(ctx context.Context, uuid string, req UpdateApplicationRequest) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "application uuid is required"}
	}
	body := struct {
		Application UpdateApplicationRequest `json:"application"`
	}{Application: req}
	resp, err := c.do(ctx, http.MethodPatch, "/licence/applications/"+url.PathEscape(uuid), nil, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("update application", resp, http.StatusAccepted)
}

// DeleteApplication deletes an application by UUID.
func (c *Client) DeleteApplication(ctx context.Context, uuid string) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "application uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodDelete, "/licence/applications/"+url.PathEscape(uuid), nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete application", resp, http.StatusNoContent)
}
