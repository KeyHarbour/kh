package khclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/KeyHarbour/kh/internal/config"
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

// KeyHarbourApi's show returns {"name", "environments"}: no uuid, and the
// environment list under "environments" rather than "environment_names".
func TestGetProject_DecodesAPIShape(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"name":"demo","environments":["production","staging"]}`)
	})

	client := New(config.Config{Endpoint: srv.URL})
	proj, err := client.GetProject(context.Background(), "p-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if proj.UUID != "p-1" || proj.Name != "demo" {
		t.Fatalf("unexpected project: %+v", proj)
	}
	if strings.Join(proj.Environments, ",") != "production,staging" {
		t.Fatalf("environments were dropped: %+v", proj.Environments)
	}
	out, _ := json.Marshal(proj)
	if !strings.Contains(string(out), `"environment_names":["production","staging"]`) {
		t.Fatalf("JSON output must keep the environment_names key, got %s", out)
	}
}

func TestProjectUnmarshal_EnvironmentKeys(t *testing.T) {
	tests := map[string]string{
		"environment_names": `{"uuid":"p","environment_names":["prod"]}`,
		"environments":      `{"uuid":"p","environments":["prod"]}`,
		"both, spec wins":   `{"uuid":"p","environment_names":["prod"],"environments":["other"]}`,
	}
	for name, body := range tests {
		var p Project
		if err := json.Unmarshal([]byte(body), &p); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if p.UUID != "p" || len(p.Environments) != 1 || p.Environments[0] != "prod" {
			t.Errorf("%s: got %+v", name, p)
		}
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

func TestListProjects(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations/org-1/projects" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Project{
			{UUID: "p-1", Name: "alpha"},
			{UUID: "p-2", Name: "beta"},
		})
	})

	client := New(config.Config{Endpoint: srv.URL})
	projects, err := client.ListProjects(context.Background(), "org-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(projects))
	}
	if projects[0].UUID != "p-1" || projects[0].Name != "alpha" {
		t.Fatalf("unexpected project[0]: %+v", projects[0])
	}
}

func TestListProjects_RequiresOrgUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.ListProjects(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty org uuid")
	}
}

func TestListProjects_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.ListProjects(context.Background(), "org-1"); err == nil {
		t.Fatal("expected error on 401, got nil")
	}
}

// KeyHarbourApi answers a create with {"status":"updated"} and no UUID, and
// project names are not unique, so the new project must be told apart from an
// existing one with the same name.
func TestCreateProject(t *testing.T) {
	var gotBody []byte
	created := false
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/organizations/org-1/projects" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			projects := []Project{{UUID: "p-old", Name: "my-project"}}
			if created {
				projects = append(projects, Project{UUID: "p-new", Name: "my-project"})
			}
			json.NewEncoder(w).Encode(projects)
		case http.MethodPost:
			var err error
			gotBody, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			created = true
			w.WriteHeader(http.StatusCreated)
			io.WriteString(w, `{"status":"updated"}`)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})

	client := New(config.Config{Endpoint: srv.URL})
	proj, err := client.CreateProject(context.Background(), "org-1", CreateProjectRequest{
		Name:             "my-project",
		EnvironmentNames: []string{"production", "staging"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if proj.UUID != "p-new" || proj.Name != "my-project" {
		t.Fatalf("expected the newly created project p-new, got %+v", proj)
	}
	var m map[string]any
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	inner, _ := m["project"].(map[string]any)
	if inner == nil || inner["name"] != "my-project" {
		t.Fatalf("expected project.name in body, got: %s", gotBody)
	}
}

func TestCreateProject_ListingFailsReturnsWithoutUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"status":"updated"}`)
	})

	client := New(config.Config{Endpoint: srv.URL})
	proj, err := client.CreateProject(context.Background(), "org-1", CreateProjectRequest{Name: "my-project"})
	if err != nil {
		t.Fatalf("a failed lookup must not fail the create, got %v", err)
	}
	if proj.UUID != "" || proj.Name != "my-project" {
		t.Fatalf("unexpected project: %+v", proj)
	}
}

func TestNewProjectUUID(t *testing.T) {
	after := []Project{{UUID: "a", Name: "web"}, {UUID: "b", Name: "web"}, {UUID: "c", Name: "api"}}
	tests := []struct {
		name     string
		existing map[string]bool
		want     string
	}{
		{"new duplicate name told apart", map[string]bool{"a": true, "c": true}, "b"},
		{"no listing before, ambiguous name", nil, ""},
		{"nothing new", map[string]bool{"a": true, "b": true, "c": true}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newProjectUUID(after, tt.existing, "web"); got != tt.want {
				t.Errorf("newProjectUUID = %q, want %q", got, tt.want)
			}
		})
	}
	if got := newProjectUUID(after, nil, "api"); got != "c" {
		t.Errorf("unique name without a prior listing = %q, want c", got)
	}
}

func TestCreateProject_RequiresOrgUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.CreateProject(context.Background(), "", CreateProjectRequest{Name: "x"}); err == nil {
		t.Fatal("expected error for empty org uuid")
	}
}

func TestCreateProject_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"errors":["name has already been taken"]}`)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if _, err := client.CreateProject(context.Background(), "org-1", CreateProjectRequest{Name: "taken"}); err == nil {
		t.Fatal("expected error on 422, got nil")
	}
}

// KeyHarbourApi reads params.expect(project: [...]), so the body must be
// wrapped in "project"; an unwrapped body is rejected with 400.
func TestUpdateProject(t *testing.T) {
	var gotBody []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/projects/p-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var err error
		gotBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	client := New(config.Config{Endpoint: srv.URL})
	err := client.UpdateProject(context.Background(), "p-1", UpdateProjectRequest{
		Name:             "new-name",
		EnvironmentNames: []string{"production"},
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m struct {
		Project map[string]any `json:"project"`
	}
	if err := json.Unmarshal(gotBody, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	if m.Project == nil || m.Project["name"] != "new-name" {
		t.Fatalf("expected project.name=new-name in body, got: %s", gotBody)
	}
	if envs, _ := m.Project["environment_names"].([]any); len(envs) != 1 || envs[0] != "production" {
		t.Fatalf("expected project.environment_names in body, got: %s", gotBody)
	}
}

func TestUpdateProject_SendsOnlySetFields(t *testing.T) {
	var gotBody string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusAccepted)
	})

	client := New(config.Config{Endpoint: srv.URL})
	if err := client.UpdateProject(context.Background(), "p-1", UpdateProjectRequest{Name: "renamed"}); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotBody != `{"project":{"name":"renamed"}}` {
		t.Fatalf("a rename must not send environment_names, got: %s", gotBody)
	}
}

func TestUpdateProject_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	client := New(config.Config{Endpoint: srv.URL})
	if err := client.UpdateProject(context.Background(), "", UpdateProjectRequest{Name: "x"}); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestUpdateProject_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	client := New(config.Config{Endpoint: srv.URL})
	if err := client.UpdateProject(context.Background(), "p-1", UpdateProjectRequest{Name: "x"}); err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}
