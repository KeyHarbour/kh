package khclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("api error (%d): %s", e.StatusCode, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("api error (%d): %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("api error (%d)", e.StatusCode)
}

func expectStatus(op string, resp *http.Response, allowed ...int) error {
	for _, code := range allowed {
		if resp.StatusCode == code {
			return nil
		}
	}
	return fmt.Errorf("%s: %w", op, parseAPIError(resp))
}

func parseAPIError(resp *http.Response) error {
	defer resp.Body.Close()
	// Limit read to 8KB then truncate to 300 chars for display
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	var payload struct {
		Error   string   `json:"error"`
		Message string   `json:"message"`
		Detail  string   `json:"detail"`
		Errors  []string `json:"errors"`
		Status  string   `json:"status"`
	}
	if len(data) > 0 {
		_ = json.Unmarshal(data, &payload)
	}
	msg := payload.Message
	if msg == "" {
		msg = payload.Error
	}
	if msg == "" {
		msg = payload.Detail
	}
	// Check for validation errors array
	if len(payload.Errors) > 0 {
		msg = strings.Join(payload.Errors, "; ")
	}
	// Add helpful hints for common 422 errors
	if resp.StatusCode == 422 && msg == "" {
		if payload.Status == "unprocessable_entity" {
			msg = "validation failed (check: workspace name must be alphanumeric, environment must match project environments)"
		}
	}
	bodySnippet := strings.TrimSpace(string(data))
	if len(bodySnippet) > 300 {
		bodySnippet = bodySnippet[:300] + "... (truncated)"
	}
	return APIError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Body:       bodySnippet,
	}
}

func decodeJSON(resp *http.Response, dest any) error {
	if dest == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(ct, "json") {
		// Read a sample of the body and truncate to 300 chars for error reporting
		sampleBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		snippet := strings.TrimSpace(string(sampleBytes))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "... (truncated)"
		}
		return APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("unexpected content-type %s", ct), Body: snippet}
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(dest); err != nil {
		// Attempt to read remainder for context (best-effort)
		rest, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		snippet := strings.TrimSpace(string(rest))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "... (truncated)"
		}
		return APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("json decode error: %v", err), Body: snippet}
	}
	return nil
}
