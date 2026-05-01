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

func TestListStatefiles_NoEnvironmentFilter(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("environment"); got != "" {
			t.Errorf("expected no environment param, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.ListStatefiles(context.Background(), "ws", ""); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestListStatefiles_404ReturnsEmpty(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	c := New(config.Config{Endpoint: srv.URL})
	items, err := c.ListStatefiles(context.Background(), "ws", "")
	if err != nil {
		t.Fatalf("expected nil err on 404, got %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected empty list, got %d", len(items))
	}
}

func TestListStatefiles_RequiresWorkspaceUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.ListStatefiles(context.Background(), "", ""); err == nil {
		t.Fatal("expected error for empty workspace uuid")
	}
}

func TestListStatefiles_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.ListStatefiles(context.Background(), "ws", ""); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

func TestGetLastStatefile(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/workspaces/ws/statefiles/last" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Statefile{
			UUID:        "sf-last",
			Content:     `{"version":4}`,
			PublishedAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		})
	})

	c := New(config.Config{Endpoint: srv.URL})
	sf, err := c.GetLastStatefile(context.Background(), "ws")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if sf.UUID != "sf-last" {
		t.Fatalf("unexpected UUID: %q", sf.UUID)
	}
	if sf.Content != `{"version":4}` {
		t.Fatalf("unexpected content: %q", sf.Content)
	}
}

func TestGetLastStatefile_RequiresWorkspaceUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.GetLastStatefile(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty workspace uuid")
	}
}

func TestGetLastStatefile_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.GetLastStatefile(context.Background(), "ws"); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestGetStatefile(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/statefiles/sf-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Statefile{
			UUID:        "sf-1",
			Content:     `{"version":4}`,
			PublishedAt: time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC),
		})
	})

	c := New(config.Config{Endpoint: srv.URL})
	sf, err := c.GetStatefile(context.Background(), "sf-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if sf.UUID != "sf-1" {
		t.Fatalf("unexpected UUID: %q", sf.UUID)
	}
	if sf.Content != `{"version":4}` {
		t.Fatalf("unexpected content: %q", sf.Content)
	}
}

func TestGetStatefile_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.GetStatefile(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestGetStatefile_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.GetStatefile(context.Background(), "sf-missing"); err == nil {
		t.Fatal("expected error on 404, got nil")
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

func TestCreateStatefile_201Response(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(StatefileCreatedResponse{Status: "created"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	resp, err := c.CreateStatefile(context.Background(), "ws", CreateStatefileRequest{Content: "{}"})
	if err != nil {
		t.Fatalf("unexpected err on 201: %v", err)
	}
	if resp.Status != "created" {
		t.Fatalf("unexpected status: %q", resp.Status)
	}
}

func TestCreateStatefile_RequiresWorkspaceUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.CreateStatefile(context.Background(), "", CreateStatefileRequest{Content: "{}"}); err == nil {
		t.Fatal("expected error for empty workspace uuid")
	}
}

func TestCreateStatefile_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.CreateStatefile(context.Background(), "ws", CreateStatefileRequest{Content: "{}"}); err == nil {
		t.Fatal("expected error on 422, got nil")
	}
}

func TestDeleteStatefiles(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/workspaces/ws/statefiles" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteStatefiles(context.Background(), "ws"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 DELETE, got %d", hits)
	}
}

func TestDeleteStatefiles_RequiresWorkspaceUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteStatefiles(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty workspace uuid")
	}
}

func TestDeleteStatefiles_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteStatefiles(context.Background(), "ws"); err == nil {
		t.Fatal("expected error on 500, got nil")
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

func TestDeleteStatefile_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteStatefile(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestDeleteStatefile_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteStatefile(context.Background(), "sf-missing"); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}
