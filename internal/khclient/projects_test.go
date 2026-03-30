package khclient

import (
	"context"
	"encoding/json"
	"io"
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

func TestUpdateWorkspace(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/workspaces/w-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	client := New(config.Config{Endpoint: srv.URL})
	err := client.UpdateWorkspace(context.Background(), "w-1", UpdateWorkspaceRequest{
		Name:        "new-name",
		Description: "new-desc",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	if m["name"] != "new-name" {
		t.Fatalf("expected name=new-name in body, got: %s", bodyBytes)
	}
	if m["description"] != "new-desc" {
		t.Fatalf("expected description=new-desc in body, got: %s", bodyBytes)
	}
}

func TestUpdateWorkspace_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if err := client.UpdateWorkspace(context.Background(), "", UpdateWorkspaceRequest{Name: "x"}); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestDeleteWorkspace(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/workspaces/w-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	client := New(config.Config{Endpoint: srv.URL})
	if err := client.DeleteWorkspace(context.Background(), "w-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 DELETE, got %d", hits)
	}
}

func TestDeleteWorkspace_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if err := client.DeleteWorkspace(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}
