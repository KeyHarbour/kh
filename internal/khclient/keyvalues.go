package khclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	resp, err := c.do(ctx, http.MethodGet, p, nil, nil, map[string]string{"Accept": "*/*"})
	if err != nil {
		return KeyValue{}, err
	}
	defer resp.Body.Close()
	if err := expectStatus("get keyvalue", resp, http.StatusOK); err != nil {
		return KeyValue{}, err
	}

	out := KeyValue{Key: key}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(ct, "application/json") {
		// The single-key JSON response doesn't include the key name; embed it.
		if err := decodeJSON(resp, &out); err != nil {
			return KeyValue{}, err
		}
		out.Key = key
		out.RawValue = []byte(out.Value)
		return out, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return KeyValue{}, err
	}
	out.Value = string(data)
	out.RawValue = data
	return out, nil
}

// CreateKeyValue creates a new key/value entry under the given workspace.
func (c *Client) CreateKeyValue(ctx context.Context, workspaceUUID string, req CreateKeyValueRequest) error {
	if workspaceUUID == "" {
		return fmt.Errorf("workspace uuid is required")
	}
	body, err := buildKeyValueMultipartBody(req.Key, req.Value, req.ExpiresAt, &req.Private, req.ValueFile)
	if err != nil {
		return err
	}
	p := "/workspaces/" + url.PathEscape(workspaceUUID) + "/keyvalues"
	resp, err := c.do(ctx, http.MethodPost, p, nil, body, nil)
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
	body, err := buildKeyValueMultipartBody(key, req.Value, req.ExpiresAt, req.Private, req.ValueFile)
	if err != nil {
		return err
	}
	p := "/keyvalues/" + url.PathEscape(key)
	resp, err := c.do(ctx, http.MethodPatch, p, nil, body, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return expectStatus("update keyvalue", resp, http.StatusAccepted)
}

func buildKeyValueMultipartBody(key, value string, expiresAt *string, private *bool, valueFromFile bool) (requestBody, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if key != "" {
		if err := w.WriteField("key", key); err != nil {
			return requestBody{}, err
		}
	}

	if valueFromFile {
		vw, err := w.CreateFormFile("value-file", "value")
		if err != nil {
			return requestBody{}, err
		}
		if _, err := io.Copy(vw, strings.NewReader(value)); err != nil {
			return requestBody{}, err
		}
	} else {
		if err := w.WriteField("value", value); err != nil {
			return requestBody{}, err
		}
	}

	if expiresAt != nil && *expiresAt != "" {
		if err := w.WriteField("expires_at", *expiresAt); err != nil {
			return requestBody{}, err
		}
	}
	if private != nil {
		if err := w.WriteField("private", strconv.FormatBool(*private)); err != nil {
			return requestBody{}, err
		}
	}

	if err := w.Close(); err != nil {
		return requestBody{}, err
	}
	return rawDataBody(buf.Bytes(), w.FormDataContentType()), nil
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
