package khclient

import (
	"context"
	"encoding/json"
	"fmt"
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
	r, err := c.newReq(ctx, http.MethodGet, "/api/v1/states", q, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		// Treat 404 Not Found as no states present for the given filters
		return []StateMeta{}, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list states: %s", resp.Status)
	}
	var out []StateMeta
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetStateRaw(ctx context.Context, id string) ([]byte, StateMeta, error) {
	r, err := c.newReq(ctx, http.MethodGet, "/api/v1/states/"+id, nil, nil)
	if err != nil {
		return nil, StateMeta{}, err
	}
	r.Header.Set("Accept", "application/vnd.terraform.state+json;version=4")
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return nil, StateMeta{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, StateMeta{}, fmt.Errorf("get state: %s", resp.Status)
	}
	// Assume metadata is in header or a sidecar endpoint; for MVP, try X-State-Meta header as JSON else zero
	var meta StateMeta
	if h := resp.Header.Get("X-State-Meta"); h != "" {
		_ = json.Unmarshal([]byte(h), &meta)
	}
	b, err := io.ReadAll(resp.Body)
	return b, meta, err
}

func (c *Client) AcquireLock(ctx context.Context, id string) error {
	r, err := c.newReq(ctx, http.MethodPost, "/api/v1/states/"+id+"/lock", nil, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("lock: %s", resp.Status)
	}
	return nil
}

func (c *Client) ReleaseLock(ctx context.Context, id string, force bool) error {
	q := url.Values{}
	if force {
		q.Set("force", "true")
	}
	r, err := c.newReq(ctx, http.MethodPost, "/api/v1/states/"+id+"/unlock", q, nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("unlock: %s", resp.Status)
	}
	return nil
}
