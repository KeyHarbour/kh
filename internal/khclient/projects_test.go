package khclient

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"kh/internal/config"
)

func TestGetProject(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/p-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Project{UUID: "p-1", Name: "project-one"})
	})

	client := New(config.Config{Endpoint: srv.URL})
	proj, err := client.GetProject(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if proj.UUID != "p-1" || proj.Name != "project-one" {
		t.Fatalf("unexpected project: %+v", proj)
	}
}

func TestGetWorkspace(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/w-1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Workspace{Name: "workspace-one"})
		default:
			http.NotFound(w, r)
		}
	})

	client := New(config.Config{Endpoint: srv.URL})
	ws, err := client.GetWorkspace(context.Background(), "w-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ws.UUID != "w-1" || ws.Name != "workspace-one" {
		t.Fatalf("unexpected workspace: %+v", ws)
	}
}
