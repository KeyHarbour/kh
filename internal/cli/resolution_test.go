package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"kh/internal/config"
	"kh/internal/khclient"
)

func newIPv4Server(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func TestResolveProjectRefByName(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/projects":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]khclient.Project{{UUID: "p-1", Name: "demo"}})
		case "/v1/projects/p-1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(khclient.Project{Name: "demo"})
		default:
			http.NotFound(w, r)
		}
	})

	client := khclient.New(config.Config{Endpoint: srv.URL})
	proj, err := resolveProjectRef(context.Background(), client, "demo")
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if proj.UUID != "p-1" || proj.Name != "demo" {
		t.Fatalf("unexpected project: %+v", proj)
	}
}

func TestResolveWorkspaceRefByUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/projects":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]khclient.Project{{UUID: "p-1", Name: "demo"}})
		case "/v1/projects/p-1/workspaces":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]khclient.Workspace{{UUID: "w-1", Name: "default"}})
		case "/v1/projects/p-1/workspaces/w-1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(khclient.Workspace{Name: "default"})
		default:
			http.NotFound(w, r)
		}
	})

	client := khclient.New(config.Config{Endpoint: srv.URL})
	ws, err := resolveWorkspaceRef(context.Background(), client, "p-1", "default")
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if ws.UUID != "w-1" {
		t.Fatalf("unexpected workspace: %+v", ws)
	}
}
