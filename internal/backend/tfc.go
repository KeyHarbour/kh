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
	u := r.Host + "/api/v2/workspaces/" + url.PathEscape(wsID) + "/state-versions?filter[current]=true"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+r.Token)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("tfc state-versions: %s", resp.Status)
	}
	var out struct {
		Data []struct {
			Attributes struct {
				HostedStateDownloadURL string `json:"hosted-state-download-url"`
			} `json:"attributes"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Data) == 0 || out.Data[0].Attributes.HostedStateDownloadURL == "" {
		return "", errors.New("tfc: no current state version")
	}
	return out.Data[0].Attributes.HostedStateDownloadURL, nil
}

// end
