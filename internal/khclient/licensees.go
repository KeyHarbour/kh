package khclient

import (
	"context"
	"net/http"
	"net/url"
)

// ListLicensees returns all licensees for the given instance.
func (c *Client) ListLicensees(ctx context.Context, instanceUUID string) ([]Licensee, error) {
	if instanceUUID == "" {
		return nil, APIError{StatusCode: http.StatusBadRequest, Message: "instance uuid is required"}
	}
	p := "/license/instances/" + url.PathEscape(instanceUUID) + "/licensees"
	resp, err := c.do(ctx, http.MethodGet, p, nil, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if err := expectStatus("list licensees", resp, http.StatusOK); err != nil {
		return nil, err
	}
	var out []Licensee
	return out, decodeJSON(resp, &out)
}

// GetLicensee returns a single licensee by UUID.
func (c *Client) GetLicensee(ctx context.Context, uuid string) (Licensee, error) {
	if uuid == "" {
		return Licensee{}, APIError{StatusCode: http.StatusBadRequest, Message: "licensee uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodGet, "/license/licensees/"+url.PathEscape(uuid), nil, nil, nil)
	if err != nil {
		return Licensee{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get licensee", resp, http.StatusOK); err != nil {
		return Licensee{}, err
	}
	var out Licensee
	if err := decodeJSON(resp, &out); err != nil {
		return Licensee{}, err
	}
	if out.UUID == "" {
		out.UUID = uuid
	}
	return out, nil
}

// CreateLicensee adds a licensee to the given instance.
func (c *Client) CreateLicensee(ctx context.Context, instanceUUID string, req CreateLicenseeRequest) error {
	if instanceUUID == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "instance uuid is required"}
	}
	body := struct {
		Licensee CreateLicenseeRequest `json:"licensee"`
	}{Licensee: req}
	p := "/license/instances/" + url.PathEscape(instanceUUID) + "/licensees"
	resp, err := c.do(ctx, http.MethodPost, p, nil, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("create licensee", resp, http.StatusCreated)
}

// UpdateLicensee updates an existing licensee.
func (c *Client) UpdateLicensee(ctx context.Context, uuid string, req UpdateLicenseeRequest) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "licensee uuid is required"}
	}
	body := struct {
		Licensee UpdateLicenseeRequest `json:"licensee"`
	}{Licensee: req}
	resp, err := c.do(ctx, http.MethodPatch, "/license/licensees/"+url.PathEscape(uuid), nil, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("update licensee", resp, http.StatusAccepted)
}

// DeleteLicensee removes a licensee by UUID.
func (c *Client) DeleteLicensee(ctx context.Context, uuid string) error {
	if uuid == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "licensee uuid is required"}
	}
	resp, err := c.do(ctx, http.MethodDelete, "/license/licensees/"+url.PathEscape(uuid), nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("delete licensee", resp, http.StatusNoContent)
}
