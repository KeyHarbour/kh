package khclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDecodeJSON_NonJSONTruncates(t *testing.T) {
	longHTML := "<html><head><title>Test</title></head><body>" +
		"" +
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. " +
		"Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit " +
		"in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum." +
		"</body></html>"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(longHTML))
	}))
	defer srv.Close()

	c := &Client{Endpoint: srv.URL, HTTP: srv.Client()}
	resp, err := c.do(context.Background(), http.MethodGet, "/test", nil, nil, nil)
	if err != nil {
		t.Fatalf("do returned error: %v", err)
	}
	defer resp.Body.Close()
	var dest struct{}
	err = decodeJSON(resp, &dest)
	if err == nil {
		t.Fatalf("expected error for non-JSON content")
	}
	apiErr, ok := err.(APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", apiErr.StatusCode)
	}
	if len(apiErr.Body) == 0 {
		t.Errorf("expected body snippet, got empty string")
	}
	if len(apiErr.Body) > 320 { // 300 + suffix
		t.Errorf("expected truncated body <= 320 chars, got %d", len(apiErr.Body))
	}
}
