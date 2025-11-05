package backend

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// TFCWriter uploads a new state version to a Terraform Cloud workspace.
type TFCWriter struct {
	Host      string // e.g., https://app.terraform.io
	Org       string
	Workspace string
	Token     string // API token for TFC
	HTTP      *http.Client
}

func NewTFCWriter(host, org, workspace, token string) *TFCWriter {
	if host == "" {
		host = "https://app.terraform.io"
	}
	return &TFCWriter{
		Host:      strings.TrimRight(host, "/"),
		Org:       org,
		Workspace: workspace,
		Token:     token,
		HTTP:      http.DefaultClient,
	}
}

func (w *TFCWriter) Put(ctx context.Context, key string, data []byte, overwrite bool) (Object, error) {
	if w.Org == "" || w.Workspace == "" {
		return Object{}, fmt.Errorf("tfc: org and workspace are required")
	}
	wsID, err := w.getWorkspaceID(ctx, w.Org, w.Workspace)
	if err != nil {
		return Object{}, err
	}
	// Extract metadata from state (serial, lineage, terraform_version)
	meta := struct {
		Serial           int    `json:"serial"`
		Lineage          string `json:"lineage"`
		TerraformVersion string `json:"terraform_version"`
	}{}
	_ = json.Unmarshal(data, &meta) // best-effort; ignore errors

	// Prepare attributes
	stateB64 := base64.StdEncoding.EncodeToString(data)
	md5sum := md5.Sum(data)
	md5b64 := base64.StdEncoding.EncodeToString(md5sum[:])

	attrs := map[string]any{
		"state": stateB64,
		"md5":   md5b64,
	}
	if meta.Serial > 0 {
		attrs["serial"] = meta.Serial
	}
	if meta.Lineage != "" {
		attrs["lineage"] = meta.Lineage
	}
	if meta.TerraformVersion != "" {
		attrs["terraform-version"] = meta.TerraformVersion
	}

	body := map[string]any{
		"data": map[string]any{
			"type":       "state-versions",
			"attributes": attrs,
		},
	}
	b, _ := json.Marshal(body)

	u := w.Host + "/api/v2/workspaces/" + url.PathEscape(wsID) + "/state-versions"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(string(b)))
	req.Header.Set("Authorization", "Bearer "+w.Token)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Accept", "application/vnd.api+json")
	resp, err := w.HTTP.Do(req)
	if err != nil {
		return Object{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// include body for diagnostics
		rb, _ := io.ReadAll(resp.Body)
		return Object{}, fmt.Errorf("tfc state upload: %s: %s", resp.Status, strings.TrimSpace(string(rb)))
	}

	// Return object summary with sha256 checksum of uploaded data.
	sha := sha256.Sum256(data)
	obj := Object{
		Key:      w.Workspace,
		Size:     int64(len(data)),
		Checksum: hex.EncodeToString(sha[:]),
		URL:      w.Host + "/app/" + w.Org + "/workspaces/" + w.Workspace,
	}
	return obj, nil
}

func (w *TFCWriter) getWorkspaceID(ctx context.Context, org, name string) (string, error) {
	u := w.Host + "/api/v2/organizations/" + url.PathEscape(org) + "/workspaces/" + url.PathEscape(name)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+w.Token)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	resp, err := w.HTTP.Do(req)
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
		return "", fmt.Errorf("tfc: workspace id not found")
	}
	return out.Data.ID, nil
}
