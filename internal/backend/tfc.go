package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type TFCReader struct {
	Host      string // e.g., https://app.terraform.io
	Org       string
	Workspace string
	Token     string // API token for TFC
	HTTP      *http.Client
}

func NewTFCReader(host, org, workspace, token string) *TFCReader {
	if host == "" {
		host = "https://app.terraform.io"
	}
	return &TFCReader{
		Host:      strings.TrimRight(host, "/"),
		Org:       org,
		Workspace: workspace,
		Token:     token,
		HTTP:      http.DefaultClient,
	}
}

func (r *TFCReader) List(ctx context.Context) ([]Object, error) {
	if r.Org == "" || r.Workspace == "" {
		return nil, errors.New("tfc: org and workspace are required")
	}
	// Represent the current workspace state as a single object; key is the workspace name
	return []Object{{Key: r.Workspace, Workspace: r.Workspace, Module: "", URL: r.Host + "/" + r.Org + "/workspaces/" + r.Workspace}}, nil
}

func (r *TFCReader) Get(ctx context.Context, key string) ([]byte, Object, error) {
	// Resolve workspace id
	wsID, err := r.getWorkspaceID(ctx, r.Org, r.Workspace)
	if err != nil {
		return nil, Object{}, err
	}
	// Get the current state version and its download URL
	dl, err := r.getCurrentStateDownloadURL(ctx, wsID)
	if err != nil {
		return nil, Object{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dl, nil)
	if err != nil {
		return nil, Object{}, err
	}
	// Some download URLs require auth (e.g., /api/v2/state-versions/:id/download), others are pre-signed.
	// Including Authorization is safe and fixes 401s on protected URLs.
	if r.Token != "" {
		req.Header.Set("Authorization", "Bearer "+r.Token)
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return nil, Object{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, Object{}, fmt.Errorf("GET state: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, Object{}, err
	}
	// We don't have a checksum from TFC API here; leave empty.
	obj := Object{Key: key, Size: int64(len(b)), Workspace: r.Workspace, URL: dl}
	return b, obj, nil
}

func (r *TFCReader) getWorkspaceID(ctx context.Context, org, name string) (string, error) {
	u := r.Host + "/api/v2/organizations/" + url.PathEscape(org) + "/workspaces/" + url.PathEscape(name)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("tfc workspace get: %s", resp.Status)
	}
	var out struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Data.ID == "" {
		return "", errors.New("tfc: workspace id not found")
	}
	return out.Data.ID, nil
}

func (r *TFCReader) getCurrentStateDownloadURL(ctx context.Context, wsID string) (string, error) {
	// Use the dedicated endpoint for the current state version
	u := r.Host + "/api/v2/workspaces/" + url.PathEscape(wsID) + "/current-state-version"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("tfc current-state-version: %s", resp.Status)
	}
	var out struct {
		Data struct {
			Attributes struct {
				HostedStateDownloadURL string `json:"hosted-state-download-url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.Data.Attributes.HostedStateDownloadURL == "" {
		return "", errors.New("tfc: no current state version")
	}
	return out.Data.Attributes.HostedStateDownloadURL, nil
}

// TFCWorkspace represents a workspace from Terraform Cloud
type TFCWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListAllWorkspaces lists all workspaces in a Terraform Cloud organization
func (r *TFCReader) ListAllWorkspaces(ctx context.Context) ([]TFCWorkspace, error) {
	if r.Org == "" {
		return nil, errors.New("tfc: org is required")
	}

	var allWorkspaces []TFCWorkspace
	pageNumber := 1
	pageSize := 100

	for {
		u := fmt.Sprintf("%s/api/v2/organizations/%s/workspaces?page[number]=%d&page[size]=%d",
			r.Host, url.PathEscape(r.Org), pageNumber, pageSize)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+r.Token)
		req.Header.Set("Content-Type", "application/vnd.api+json")

		resp, err := r.HTTP.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("tfc list workspaces: %s", resp.Status)
		}

		var out struct {
			Data []struct {
				ID         string `json:"id"`
				Attributes struct {
					Name string `json:"name"`
				} `json:"attributes"`
			} `json:"data"`
			Meta struct {
				Pagination struct {
					CurrentPage int `json:"current-page"`
					TotalPages  int `json:"total-pages"`
				} `json:"pagination"`
			} `json:"meta"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return nil, err
		}

		for _, ws := range out.Data {
			allWorkspaces = append(allWorkspaces, TFCWorkspace{
				ID:   ws.ID,
				Name: ws.Attributes.Name,
			})
		}

		if pageNumber >= out.Meta.Pagination.TotalPages {
			break
		}
		pageNumber++
	}

	return allWorkspaces, nil
}

// end
