package khclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientNewReq_PreservesEscapedPathSegments(t *testing.T) {
	c := &Client{Endpoint: "https://example.test/api/v2"}

	req, err := c.newReq(context.Background(), http.MethodGet, "/keyvalues/my%20key%2Fpart", nil, nil)
	if err != nil {
		t.Fatalf("newReq error: %v", err)
	}

	got := req.URL.String()
	if !strings.Contains(got, "/api/v2/keyvalues/my%20key%2Fpart") {
		t.Fatalf("expected escaped path segments to be preserved, got %q", got)
	}
	if strings.Contains(got, "%2520") || strings.Contains(got, "%252F") {
		t.Fatalf("expected no double-encoding in URL, got %q", got)
	}
}

func TestClientDo_FinalRetryReturnsAPIErrorBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "backend unavailable"})
	}))
	defer srv.Close()

	c := &Client{
		Endpoint:  srv.URL,
		HTTP:      srv.Client(),
		Retries:   1,
		RetryWait: 0,
	}

	_, err := c.do(context.Background(), http.MethodGet, "/health", nil, nil, nil)
	if err == nil {
		t.Fatal("expected final retry to return an error")
	}

	var apiErr APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError on final retry, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Error(), "backend unavailable") {
		t.Fatalf("expected server diagnostic message in error, got %v", apiErr)
	}
}
