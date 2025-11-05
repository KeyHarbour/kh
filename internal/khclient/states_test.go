package khclient

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "kh/internal/config"
)

func TestListStates_404ReturnsEmpty(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/api/v1/states" {
            http.NotFound(w, r)
            return
        }
        http.NotFound(w, r)
    }))
    defer srv.Close()

    cfg := config.Config{Endpoint: srv.URL}
    c := New(cfg)
    out, err := c.ListStates(context.Background(), ListStatesRequest{})
    if err != nil {
        t.Fatalf("expected nil err, got %v", err)
    }
    if len(out) != 0 {
        t.Fatalf("expected empty list, got %d", len(out))
    }
}
