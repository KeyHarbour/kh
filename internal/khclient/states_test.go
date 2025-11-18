package khclient

import (
	"context"
	"kh/internal/config"
	"net/http"
	"testing"
)

func TestListStates_404ReturnsEmpty(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/states" {
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	})

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
