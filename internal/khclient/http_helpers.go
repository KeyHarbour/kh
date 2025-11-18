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
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var payload struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
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
	return APIError{
		StatusCode: resp.StatusCode,
		Message:    msg,
		Body:       strings.TrimSpace(string(data)),
	}
}

func decodeJSON(resp *http.Response, dest any) error {
	if dest == nil {
		_, err := io.Copy(io.Discard, resp.Body)
		return err
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "json") {
		return fmt.Errorf("unexpected content type: %s", ct)
	}
	dec := json.NewDecoder(resp.Body)
	return dec.Decode(dest)
}
