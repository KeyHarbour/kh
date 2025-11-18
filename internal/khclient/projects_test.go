package khclient

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"kh/internal/config"
)

func TestListProjects(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/projects" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Project{
			{UUID: "p-1", Name: "project-one"},
			{UUID: "p-2", Name: "project-two"},
		})
	})

	client := New(config.Config{Endpoint: srv.URL})
	items, err := client.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 2 || items[0].UUID != "p-1" || items[1].Name != "project-two" {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestGetWorkspace(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/projects/p-1/workspaces/w-1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(Workspace{Name: "workspace-one"})
		default:
			http.NotFound(w, r)
		}
	})

	client := New(config.Config{Endpoint: srv.URL})
	ws, err := client.GetWorkspace(context.Background(), "p-1", "w-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ws.UUID != "w-1" || ws.Name != "workspace-one" {
		t.Fatalf("unexpected workspace: %+v", ws)
	}
}
