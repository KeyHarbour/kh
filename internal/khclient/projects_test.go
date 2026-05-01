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

func TestGetProject_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.GetProject(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestGetProject_BackfillsUUIDFromRequest(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Response omits UUID — client should backfill from the request path.
		json.NewEncoder(w).Encode(Project{Name: "project-one"})
	})

	client := New(config.Config{Endpoint: srv.URL})
	proj, err := client.GetProject(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if proj.UUID != "p-1" {
		t.Fatalf("expected UUID to be backfilled to p-1, got %q", proj.UUID)
	}
}

func TestGetProject_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.GetProject(context.Background(), "p-1"); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestListWorkspaces(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/p-1/workspaces" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Workspace{
			{UUID: "w-1", Name: "workspace-one"},
			{UUID: "w-2", Name: "workspace-two"},
		})
	})

	client := New(config.Config{Endpoint: srv.URL})
	workspaces, err := client.ListWorkspaces(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
	if workspaces[0].UUID != "w-1" || workspaces[0].Name != "workspace-one" {
		t.Fatalf("unexpected workspace[0]: %+v", workspaces[0])
	}
}

func TestListWorkspaces_RequiresProjectUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.ListWorkspaces(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty project uuid")
	}
}

func TestListWorkspaces_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.ListWorkspaces(context.Background(), "p-1"); err == nil {
		t.Fatal("expected error on 500, got nil")
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

func TestGetWorkspace_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.GetWorkspace(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestGetWorkspace_BackfillsUUIDFromRequest(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Workspace{Name: "workspace-one"}) // UUID omitted
	})

	client := New(config.Config{Endpoint: srv.URL})
	ws, err := client.GetWorkspace(context.Background(), "w-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ws.UUID != "w-1" {
		t.Fatalf("expected UUID backfilled to w-1, got %q", ws.UUID)
	}
}

func TestGetWorkspace_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.GetWorkspace(context.Background(), "w-1"); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestCreateWorkspace(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/projects/p-1/workspaces" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			Workspace struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"workspace"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.Workspace.Name != "myworkspace" {
			t.Fatalf("unexpected name: %q", body.Workspace.Name)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Workspace{UUID: "w-new", Name: "myworkspace"})
	})

	client := New(config.Config{Endpoint: srv.URL})
	ws, err := client.CreateWorkspace(context.Background(), "p-1", CreateWorkspaceRequest{Name: "myworkspace"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ws.UUID != "w-new" || ws.Name != "myworkspace" {
		t.Fatalf("unexpected workspace: %+v", ws)
	}
}

func TestCreateWorkspace_RequiresProjectUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.CreateWorkspace(context.Background(), "", CreateWorkspaceRequest{Name: "myworkspace"}); err == nil {
		t.Fatal("expected error for empty project uuid")
	}
}

func TestCreateWorkspace_RequiresName(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.CreateWorkspace(context.Background(), "p-1", CreateWorkspaceRequest{Name: ""}); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestCreateWorkspace_RejectsNonAlphanumericName(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	for _, name := range []string{"my-workspace", "my workspace", "my_workspace", "my.workspace"} {
		if _, err := client.CreateWorkspace(context.Background(), "p-1", CreateWorkspaceRequest{Name: name}); err == nil {
			t.Fatalf("expected error for name %q, got nil", name)
		}
	}
}

func TestCreateWorkspace_AcceptsAlphanumericName(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Workspace{UUID: "w-new", Name: "myWorkspace123"})
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.CreateWorkspace(context.Background(), "p-1", CreateWorkspaceRequest{Name: "myWorkspace123"}); err != nil {
		t.Fatalf("unexpected err for alphanumeric name: %v", err)
	}
}

func TestCreateWorkspace_FallsBackToListWhenUUIDMissing(t *testing.T) {
	var requestCount int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			// Response omits UUID — client must list to find it.
			json.NewEncoder(w).Encode(Workspace{Name: "myworkspace"})
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]Workspace{
				{UUID: "w-found", Name: "myworkspace"},
			})
		}
	})

	client := New(config.Config{Endpoint: srv.URL})
	ws, err := client.CreateWorkspace(context.Background(), "p-1", CreateWorkspaceRequest{Name: "myworkspace"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ws.UUID != "w-found" {
		t.Fatalf("expected UUID w-found from list fallback, got %q", ws.UUID)
	}
	if requestCount != 2 {
		t.Fatalf("expected 2 requests (create + list), got %d", requestCount)
	}
}

func TestCreateWorkspace_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"errors":["name has already been taken"]}`)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.CreateWorkspace(context.Background(), "p-1", CreateWorkspaceRequest{Name: "taken"}); err == nil {
		t.Fatal("expected error on 422, got nil")
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

func TestUpdateWorkspace_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if err := client.UpdateWorkspace(context.Background(), "w-1", UpdateWorkspaceRequest{Name: "x"}); err == nil {
		t.Fatal("expected error on 404, got nil")
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

func TestDeleteWorkspace_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if err := client.DeleteWorkspace(context.Background(), "w-1"); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestGetOrCreateWorkspace_ExistingWorkspace(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET for list, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Workspace{
			{UUID: "w-existing", Name: "myworkspace"},
		})
	})

	client := New(config.Config{Endpoint: srv.URL})
	ws, created, err := client.GetOrCreateWorkspace(context.Background(), "p-1", "myworkspace")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if created {
		t.Fatal("expected created=false for existing workspace")
	}
	if ws.UUID != "w-existing" {
		t.Fatalf("unexpected UUID: %q", ws.UUID)
	}
}

func TestGetOrCreateWorkspace_CreatesWhenMissing(t *testing.T) {
	var requestCount int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			// First call: list returns empty.
			json.NewEncoder(w).Encode([]Workspace{})
		case http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(Workspace{UUID: "w-new", Name: "newworkspace"})
		}
	})

	client := New(config.Config{Endpoint: srv.URL})
	ws, created, err := client.GetOrCreateWorkspace(context.Background(), "p-1", "newworkspace")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !created {
		t.Fatal("expected created=true for new workspace")
	}
	if ws.UUID != "w-new" {
		t.Fatalf("unexpected UUID: %q", ws.UUID)
	}
}

func TestGetOrCreateWorkspace_ListError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, _, err := client.GetOrCreateWorkspace(context.Background(), "p-1", "myworkspace"); err == nil {
		t.Fatal("expected error when list fails, got nil")
	}
}
