package backend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kh/internal/config"
	"kh/internal/khclient"
)

func TestKeyHarbourReaderList_WithSpecificStatefile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/proj-ref":
			writeJSON(t, w, khclient.Project{UUID: "proj-1", Name: "project-one"})
		case "/projects/proj-1/workspaces":
			writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "dev"}})
		case "/statefiles/sf-1":
			writeJSON(t, w, khclient.Statefile{UUID: "sf-1", Content: "{}"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "dev", "sf-1", "prod")
	objects, err := reader.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}
	if objects[0].Key != "sf-1" {
		t.Fatalf("expected statefile key sf-1, got %s", objects[0].Key)
	}
	if objects[0].Workspace != "dev" {
		t.Fatalf("expected workspace dev, got %s", objects[0].Workspace)
	}
	if objects[0].URL != "/workspaces/ws-1/statefiles/sf-1" {
		t.Fatalf("unexpected URL %s", objects[0].URL)
	}
	if objects[0].Size != 2 {
		t.Fatalf("expected size 2, got %d", objects[0].Size)
	}
}

func TestKeyHarbourReaderList_ListsWorkspaceStatefiles(t *testing.T) {
	var gotEnvironment string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/proj-ref":
			writeJSON(t, w, khclient.Project{UUID: "proj-1", Name: "project-one"})
		case "/projects/proj-1/workspaces":
			writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "dev"}})
		case "/workspaces/ws-1/statefiles":
			gotEnvironment = r.URL.Query().Get("environment")
			writeJSON(t, w, []khclient.Statefile{
				{UUID: "sf-1", Content: "{}"},
				{UUID: "sf-2", Content: "{\"serial\":1}"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "dev", "", "prod")
	objects, err := reader.List(context.Background())
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if gotEnvironment != "prod" {
		t.Fatalf("expected environment query prod, got %q", gotEnvironment)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objects))
	}
	if objects[1].Key != "sf-2" {
		t.Fatalf("expected second key sf-2, got %s", objects[1].Key)
	}
	if objects[1].Workspace != "dev" {
		t.Fatalf("expected workspace dev, got %s", objects[1].Workspace)
	}
	if objects[1].URL != "/workspaces/ws-1/statefiles/sf-2" {
		t.Fatalf("unexpected URL %s", objects[1].URL)
	}
	if objects[1].Size == 0 {
		t.Fatal("expected non-zero size")
	}
}

func TestKeyHarbourReaderList_ResolveErrors(t *testing.T) {
	t.Run("project resolution fails", func(t *testing.T) {
		reader := NewKeyHarbourReader(newTestKHClient("http://example.test"), "", "dev", "", "")
		objects, err := reader.List(context.Background())
		if err == nil || !strings.Contains(err.Error(), "project is required") {
			t.Fatalf("expected project error, got %v", err)
		}
		if objects != nil {
			t.Fatalf("expected nil objects, got %+v", objects)
		}
	})

	t.Run("workspace resolution fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/projects/proj-ref" {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, khclient.Project{UUID: "proj-1", Name: "project-one"})
		}))
		defer srv.Close()

		reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "", "", "")
		objects, err := reader.List(context.Background())
		if err == nil || !strings.Contains(err.Error(), "workspace is required") {
			t.Fatalf("expected workspace error, got %v", err)
		}
		if objects != nil {
			t.Fatalf("expected nil objects, got %+v", objects)
		}
	})
}

func TestKeyHarbourReaderGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/proj-ref":
			writeJSON(t, w, khclient.Project{UUID: "proj-1", Name: "project-one"})
		case "/projects/proj-1/workspaces":
			writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "dev"}})
		case "/statefiles/sf-9":
			writeJSON(t, w, khclient.Statefile{UUID: "sf-9", Content: "terraform-state"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "ws-1", "", "")
	data, obj, err := reader.Get(context.Background(), "sf-9")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if string(data) != "terraform-state" {
		t.Fatalf("unexpected content %q", string(data))
	}
	if obj.Key != "sf-9" {
		t.Fatalf("expected key sf-9, got %s", obj.Key)
	}
	if obj.Workspace != "dev" {
		t.Fatalf("expected workspace dev, got %s", obj.Workspace)
	}
	if obj.URL != "/workspaces/ws-1/statefiles/sf-9" {
		t.Fatalf("unexpected URL %s", obj.URL)
	}
}

func TestKeyHarbourReaderGet_ResolveErrors(t *testing.T) {
	t.Run("project resolution fails", func(t *testing.T) {
		reader := NewKeyHarbourReader(newTestKHClient("http://example.test"), "", "dev", "", "")
		_, _, err := reader.Get(context.Background(), "sf-1")
		if err == nil || !strings.Contains(err.Error(), "project is required") {
			t.Fatalf("expected project error, got %v", err)
		}
	})

	t.Run("workspace resolution fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/projects/proj-ref" {
				http.NotFound(w, r)
				return
			}
			writeJSON(t, w, khclient.Project{UUID: "proj-1", Name: "project-one"})
		}))
		defer srv.Close()

		reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "", "", "")
		_, _, err := reader.Get(context.Background(), "sf-1")
		if err == nil || !strings.Contains(err.Error(), "workspace is required") {
			t.Fatalf("expected workspace error, got %v", err)
		}
	})
}

func TestKeyHarbourReader_ErrorPaths(t *testing.T) {
	t.Run("project required", func(t *testing.T) {
		reader := NewKeyHarbourReader(newTestKHClient("http://example.test"), "", "dev", "", "")
		_, err := reader.resolveProject(context.Background())
		if err == nil || !strings.Contains(err.Error(), "project is required") {
			t.Fatalf("expected missing project error, got %v", err)
		}
	})

	t.Run("workspace required", func(t *testing.T) {
		reader := NewKeyHarbourReader(newTestKHClient("http://example.test"), "proj-ref", "", "", "")
		_, _, err := reader.resolveWorkspace(context.Background(), "proj-1")
		if err == nil || !strings.Contains(err.Error(), "workspace is required") {
			t.Fatalf("expected missing workspace error, got %v", err)
		}
	})

	t.Run("project lookup fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "missing", http.StatusNotFound)
		}))
		defer srv.Close()

		reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "dev", "", "")
		_, err := reader.resolveProject(context.Background())
		if err == nil || !strings.Contains(err.Error(), "failed to get project") {
			t.Fatalf("expected project lookup error, got %v", err)
		}
	})

	t.Run("workspace list fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "dev", "", "")
		_, _, err := reader.resolveWorkspace(context.Background(), "proj-1")
		if err == nil || !strings.Contains(err.Error(), "failed to list workspaces") {
			t.Fatalf("expected workspace list error, got %v", err)
		}
	})

	t.Run("workspace not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "dev"}})
		}))
		defer srv.Close()

		reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "prod", "", "")
		_, _, err := reader.resolveWorkspace(context.Background(), "proj-1")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("expected workspace not found error, got %v", err)
		}
	})

	t.Run("specific statefile get fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects/proj-ref":
				writeJSON(t, w, khclient.Project{UUID: "proj-1", Name: "project-one"})
			case "/projects/proj-1/workspaces":
				writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "dev"}})
			case "/statefiles/sf-1":
				http.Error(w, "missing", http.StatusNotFound)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "dev", "sf-1", "")
		_, err := reader.List(context.Background())
		if err == nil || !strings.Contains(err.Error(), "failed to get statefile sf-1") {
			t.Fatalf("expected get statefile error, got %v", err)
		}
	})

	t.Run("list statefiles fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects/proj-ref":
				writeJSON(t, w, khclient.Project{UUID: "proj-1", Name: "project-one"})
			case "/projects/proj-1/workspaces":
				writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "dev"}})
			case "/workspaces/ws-1/statefiles":
				http.Error(w, "boom", http.StatusInternalServerError)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "dev", "", "prod")
		_, err := reader.List(context.Background())
		if err == nil || !strings.Contains(err.Error(), "failed to list statefiles") {
			t.Fatalf("expected list statefiles error, got %v", err)
		}
	})

	t.Run("get statefile fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects/proj-ref":
				writeJSON(t, w, khclient.Project{UUID: "proj-1", Name: "project-one"})
			case "/projects/proj-1/workspaces":
				writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "dev"}})
			case "/statefiles/sf-2":
				http.Error(w, "missing", http.StatusNotFound)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		reader := NewKeyHarbourReader(newTestKHClient(srv.URL), "proj-ref", "dev", "", "")
		_, _, err := reader.Get(context.Background(), "sf-2")
		if err == nil || !strings.Contains(err.Error(), "failed to get statefile sf-2") {
			t.Fatalf("expected get statefile error, got %v", err)
		}
	})
}

func TestKeyHarbourWriterPut_ExistingWorkspace(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/proj-1/workspaces":
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET, got %s", r.Method)
			}
			writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "prod"}})
		case "/workspaces/ws-1/statefiles":
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST, got %s", r.Method)
			}
			var err error
			body, err = io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			writeJSONStatus(t, w, http.StatusCreated, khclient.StatefileCreatedResponse{Status: "accepted"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	writer := NewKeyHarbourWriter(newTestKHClient(srv.URL), "proj-1", "", false)
	obj, err := writer.Put(context.Background(), "prod", []byte("{}"), true)
	if err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if !strings.Contains(string(body), `"content":"{}"`) {
		t.Fatalf("expected state content in body, got %s", string(body))
	}
	if obj.Key != "prod" || obj.Workspace != "prod" {
		t.Fatalf("unexpected object %+v", obj)
	}
	if obj.URL != "/workspaces/ws-1/statefiles" {
		t.Fatalf("unexpected URL %s", obj.URL)
	}
}

func TestKeyHarbourWriterPut_CreatesWorkspace(t *testing.T) {
	var createWorkspaceCalled bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/proj-1/workspaces":
			if r.Method == http.MethodGet {
				writeJSON(t, w, []khclient.Workspace{})
				return
			}
			if r.Method == http.MethodPost {
				createWorkspaceCalled = true
				writeJSONStatus(t, w, http.StatusCreated, khclient.Workspace{UUID: "ws-new", Name: "prod"})
				return
			}
			t.Fatalf("unexpected method %s", r.Method)
		case "/workspaces/ws-new/statefiles":
			writeJSONStatus(t, w, http.StatusCreated, khclient.StatefileCreatedResponse{Status: "accepted"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	writer := NewKeyHarbourWriter(newTestKHClient(srv.URL), "proj-1", "", true)
	obj, err := writer.Put(context.Background(), "prod", []byte("terraform"), true)
	if err != nil {
		t.Fatalf("Put error: %v", err)
	}
	if !createWorkspaceCalled {
		t.Fatal("expected workspace creation call")
	}
	if obj.URL != "/workspaces/ws-new/statefiles" {
		t.Fatalf("unexpected URL %s", obj.URL)
	}
}

func TestKeyHarbourWriterPut_ResolveWorkspaceError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, []khclient.Workspace{})
	}))
	defer srv.Close()

	writer := NewKeyHarbourWriter(newTestKHClient(srv.URL), "proj-1", "", false)
	_, err := writer.Put(context.Background(), "prod", []byte("{}"), true)
	if err == nil || !strings.Contains(err.Error(), "use --create-workspace") {
		t.Fatalf("expected workspace resolution error, got %v", err)
	}
}

func TestKeyHarbourWriter_ErrorPaths(t *testing.T) {
	t.Run("list workspaces fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		writer := NewKeyHarbourWriter(newTestKHClient(srv.URL), "proj-1", "", false)
		_, err := writer.resolveWorkspace(context.Background(), "prod")
		if err == nil || !strings.Contains(err.Error(), "failed to list workspaces") {
			t.Fatalf("expected list error, got %v", err)
		}
	})

	t.Run("workspace not found without create", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(t, w, []khclient.Workspace{})
		}))
		defer srv.Close()

		writer := NewKeyHarbourWriter(newTestKHClient(srv.URL), "proj-1", "", false)
		_, err := writer.resolveWorkspace(context.Background(), "prod")
		if err == nil || !strings.Contains(err.Error(), "use --create-workspace") {
			t.Fatalf("expected create workspace hint, got %v", err)
		}
	})

	t.Run("create workspace fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				writeJSON(t, w, []khclient.Workspace{})
				return
			}
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		writer := NewKeyHarbourWriter(newTestKHClient(srv.URL), "proj-1", "", true)
		_, err := writer.resolveWorkspace(context.Background(), "prod")
		if err == nil || !strings.Contains(err.Error(), "failed to create workspace") {
			t.Fatalf("expected create workspace error, got %v", err)
		}
	})

	t.Run("create statefile fails", func(t *testing.T) {
		var hitPost bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/projects/proj-1/workspaces":
				writeJSON(t, w, []khclient.Workspace{{UUID: "ws-1", Name: "prod"}})
			case "/workspaces/ws-1/statefiles":
				hitPost = true
				http.Error(w, "boom", http.StatusInternalServerError)
			default:
				http.NotFound(w, r)
			}
		}))
		defer srv.Close()

		writer := NewKeyHarbourWriter(newTestKHClient(srv.URL), "proj-1", "", false)
		_, err := writer.Put(context.Background(), "prod", []byte("{}"), true)
		if !hitPost {
			t.Fatal("expected statefile create request")
		}
		if err == nil || !strings.Contains(err.Error(), "failed to create statefile") {
			t.Fatalf("expected create statefile error, got %v", err)
		}
	})
}

func newTestKHClient(endpoint string) *khclient.Client {
	return khclient.New(config.Config{Endpoint: endpoint})
}

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
}

func writeJSONStatus(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
}
