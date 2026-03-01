package khclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"kh/internal/config"
)

func TestListStatefiles(t *testing.T) {
	var called bool
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/workspaces/ws/statefiles" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("environment"); got != "prod" {
			t.Fatalf("expected environment query, got %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"uuid":"123","content":"{}","published_at":"2024-01-01T00:00:00Z"}]`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	items, err := c.ListStatefiles(context.Background(), "ws", "prod")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !called {
		t.Fatalf("server not called")
	}
	if len(items) != 1 || items[0].UUID != "123" {
		t.Fatalf("unexpected items: %+v", items)
	}
	if !items[0].PublishedAt.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected published_at: %v", items[0].PublishedAt)
	}
}

func TestCreateStatefile(t *testing.T) {
	var body []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws/statefiles" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("environment"); got != "" {
			t.Fatalf("expected no environment query, got %s", got)
		}
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(StatefileCreatedResponse{Status: "accepted"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	resp, err := c.CreateStatefile(context.Background(), "ws", CreateStatefileRequest{Content: "{}"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if resp.Status != "accepted" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if !strings.Contains(string(body), `"content":"{}"`) {
		t.Fatalf("expected content in body: %s", string(body))
	}
}

func TestDeleteStatefile(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path != "/statefiles/abc" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteStatefile(context.Background(), "abc"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 hit, got %d", hits)
	}
}
