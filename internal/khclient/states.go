package khclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

func (c *Client) ListStates(ctx context.Context, req ListStatesRequest) ([]StateMeta, error) {
	q := url.Values{}
	if req.Project != "" {
		q.Set("project", req.Project)
	}
	if req.Module != "" {
		q.Set("module", req.Module)
	}
	if req.Workspace != "" {
		q.Set("workspace", req.Workspace)
	}
	resp, err := c.do(ctx, http.MethodGet, "/v1/states", q, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return []StateMeta{}, nil
	}
	if err := expectStatus("list states", resp, http.StatusOK); err != nil {
		return nil, err
	}
	var out []StateMeta
	if err := decodeJSON(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetStateRaw(ctx context.Context, id string) ([]byte, StateMeta, error) {
	if id == "" {
		return nil, StateMeta{}, APIError{StatusCode: http.StatusBadRequest, Message: "state id is required"}
	}
	headers := map[string]string{
		"Accept": "application/vnd.terraform.state+json;version=4",
	}
	resp, err := c.do(ctx, http.MethodGet, "/v1/states/"+url.PathEscape(id), nil, nil, headers)
	if err != nil {
		return nil, StateMeta{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get state", resp, http.StatusOK); err != nil {
		return nil, StateMeta{}, err
	}
	var meta StateMeta
	if h := resp.Header.Get("X-State-Meta"); h != "" {
		_ = json.Unmarshal([]byte(h), &meta)
	}
	b, err := io.ReadAll(resp.Body)
	return b, meta, err
}

func (c *Client) AcquireLock(ctx context.Context, id string) error {
	if id == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "state id is required"}
	}
	resp, err := c.do(ctx, http.MethodPost, "/v1/states/"+url.PathEscape(id)+"/lock", nil, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("lock state", resp, http.StatusOK, http.StatusNoContent)
}

func (c *Client) ReleaseLock(ctx context.Context, id string, force bool) error {
	if id == "" {
		return APIError{StatusCode: http.StatusBadRequest, Message: "state id is required"}
	}
	q := url.Values{}
	if force {
		q.Set("force", "true")
	}
	resp, err := c.do(ctx, http.MethodPost, "/v1/states/"+url.PathEscape(id)+"/unlock", q, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("unlock state", resp, http.StatusOK, http.StatusNoContent)
}

// PutState uploads state data to KeyHarbour
func (c *Client) PutState(ctx context.Context, id string, data []byte, overwrite bool) (StateMeta, error) {
	if id == "" {
		return StateMeta{}, APIError{StatusCode: http.StatusBadRequest, Message: "state id is required"}
	}
	q := url.Values{}
	if overwrite {
		q.Set("overwrite", "true")
	}
	resp, err := c.do(ctx, http.MethodPut, "/v1/states/"+url.PathEscape(id), q, rawDataBody(data, "application/vnd.terraform.state+json;version=4"), nil)
	if err != nil {
		return StateMeta{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("put state", resp, http.StatusOK, http.StatusCreated); err != nil {
		return StateMeta{}, err
	}
	var meta StateMeta
	if err := decodeJSON(resp, &meta); err != nil {
		return StateMeta{}, err
	}
	return meta, nil
}
