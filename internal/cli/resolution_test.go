package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"kh/internal/config"
	"kh/internal/khclient"
)

func TestResolveProjectRefByName(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/projects":
			json.NewEncoder(w).Encode([]khclient.Project{{UUID: "p-1", Name: "demo"}})
		case "/v1/projects/p-1":
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
			json.NewEncoder(w).Encode([]khclient.Project{{UUID: "p-1", Name: "demo"}})
		case "/v1/projects/p-1/workspaces":
			json.NewEncoder(w).Encode([]khclient.Workspace{{UUID: "w-1", Name: "default"}})
		case "/v1/projects/p-1/workspaces/w-1":
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
