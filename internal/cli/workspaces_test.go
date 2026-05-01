package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newWorkspacesTestServer starts a test server pre-wired with project and workspace
// resolution endpoints, delegating workspace-specific requests to wsHandler.
func newWorkspacesTestServer(t *testing.T, wsHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v2/projects/proj-uuid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "proj-uuid", "name": "my-project"})
	})
	mux.HandleFunc("/api/v2/projects/proj-uuid/workspaces", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]any{{"uuid": "11111111-2222-3333-4444-555555555555", "name": "my-workspace"}})
			return
		}
		wsHandler(w, r)
	})
	mux.HandleFunc("/api/v2/workspaces/11111111-2222-3333-4444-555555555555", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "name": "my-workspace", "description": "old-desc"})
			return
		}
		wsHandler(w, r)
	})
	mux.HandleFunc("/api/v2/workspaces/", wsHandler)

	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: mux}}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func runWorkspacesCmd(t *testing.T, srv *httptest.Server, args ...string) (string, error) {
	t.Helper()
	t.Setenv("KH_ENDPOINT", srv.URL)
	t.Setenv("KH_TOKEN", "test-token")

	buf := &bytes.Buffer{}
	cmd := newWorkspacesCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestWorkspacesCreate_SendsCorrectPayload(t *testing.T) {
	var bodyBytes []byte
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	out, err := runWorkspacesCmd(t, srv, "create", "newws", "--project", "proj-uuid", "--description", "my desc")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "newws") {
		t.Errorf("expected workspace name in output, got: %s", out)
	}
	if !strings.Contains(string(bodyBytes), `"name":"newws"`) {
		t.Errorf("expected name in body, got: %s", bodyBytes)
	}
	if !strings.Contains(string(bodyBytes), `"description":"my desc"`) {
		t.Errorf("expected description in body, got: %s", bodyBytes)
	}
}

func TestWorkspacesCreate_RequiresProject(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runWorkspacesCmd(t, srv, "create", "newws")
	if err == nil {
		t.Fatal("expected error when --project is missing")
	}
}

func TestWorkspacesUpdate_SendsCorrectPayload(t *testing.T) {
	var bodyBytes []byte
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	out, err := runWorkspacesCmd(t, srv, "update", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid", "--name", "renamed")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "updated") {
		t.Errorf("expected updated confirmation in output, got: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	if m["name"] != "renamed" {
		t.Errorf("expected name=renamed in body, got: %s", bodyBytes)
	}
}

func TestWorkspacesUpdate_RequiresAtLeastOneFlag(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runWorkspacesCmd(t, srv, "update", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid")
	if err == nil {
		t.Fatal("expected error when no --name or --description provided")
	}
}

func TestWorkspacesDelete_RequiresForce(t *testing.T) {
	var deleteCalled bool
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, _ = runWorkspacesCmd(t, srv, "delete", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid")
	if deleteCalled {
		t.Fatal("DELETE should not be called without --force")
	}
}

func TestWorkspacesDelete_WithForce(t *testing.T) {
	var deleteCalled bool
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, err := runWorkspacesCmd(t, srv, "delete", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid", "--force")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected DELETE to be called with --force")
	}
}

func TestWorkspacesDelete_RequiresProject(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runWorkspacesCmd(t, srv, "delete", "11111111-2222-3333-4444-555555555555", "--force")
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("expected missing project error, got %v", err)
	}
}

func TestWorkspacesUpdate_OnlyDescription(t *testing.T) {
	var bodyBytes []byte
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	_, err := runWorkspacesCmd(t, srv, "update", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid", "--description", "new-desc")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	// Name should be preserved from current workspace (fetched via GetWorkspace)
	if m["name"] != "my-workspace" {
		t.Errorf("expected preserved name=my-workspace in body, got: %s", bodyBytes)
	}
	if m["description"] != "new-desc" {
		t.Errorf("expected description=new-desc in body, got: %s", bodyBytes)
	}
}

func TestWorkspacesList_TableOutput(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to wsHandler: %s %s", r.Method, r.URL.Path)
	})

	out, err := runWorkspacesCmd(t, srv, "ls", "--project", "proj-uuid")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "my-workspace") {
		t.Errorf("expected workspace name in output, got: %s", out)
	}
	if !strings.Contains(out, "11111111-2222-3333-4444-555555555555") {
		t.Errorf("expected workspace UUID in output, got: %s", out)
	}
}

func TestWorkspacesList_JSONOutput(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to wsHandler: %s %s", r.Method, r.URL.Path)
	})

	out, err := runWorkspacesCmd(t, srv, "ls", "--project", "proj-uuid", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(out), &items); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	if len(items) != 1 || items[0]["name"] != "my-workspace" {
		t.Errorf("unexpected JSON output: %s", out)
	}
}

func TestWorkspacesList_RequiresProject(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runWorkspacesCmd(t, srv, "ls")
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("expected missing project error, got %v", err)
	}
}

func TestWorkspacesShow_TableOutput(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to wsHandler: %s %s", r.Method, r.URL.Path)
	})

	out, err := runWorkspacesCmd(t, srv, "show", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "my-workspace") {
		t.Errorf("expected workspace name in output, got: %s", out)
	}
	if !strings.Contains(out, "11111111-2222-3333-4444-555555555555") {
		t.Errorf("expected workspace UUID in output, got: %s", out)
	}
}

func TestWorkspacesShow_JSONOutput(t *testing.T) {
	outputFormat = "json"
	defer func() { outputFormat = "" }()

	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to wsHandler: %s %s", r.Method, r.URL.Path)
	})

	out, err := runWorkspacesCmd(t, srv, "show", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out)
	}
	ws, _ := payload["workspace"].(map[string]any)
	if ws == nil || ws["name"] != "my-workspace" {
		t.Errorf("unexpected JSON payload: %s", out)
	}
}

func TestWorkspacesShow_RequiresProject(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runWorkspacesCmd(t, srv, "show", "11111111-2222-3333-4444-555555555555")
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("expected missing project error, got %v", err)
	}
}

func TestWorkspacesShow_RequiresExactlyOneArg(t *testing.T) {
	cmd := newWorkspacesShowCmd(&workspaceCmdOpts{})
	if err := cmd.Args(cmd, []string{}); err == nil || !strings.Contains(err.Error(), "usage:") {
		t.Fatalf("expected arg error for zero args, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"a", "b"}); err == nil {
		t.Fatal("expected arg error for two args")
	}
}

func TestWorkspacesShow_InvalidUUID(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	})

	_, err := runWorkspacesCmd(t, srv, "show", "not-a-uuid", "--project", "proj-uuid")
	if err == nil || !strings.Contains(err.Error(), "not a valid UUID") {
		t.Fatalf("expected invalid UUID error, got %v", err)
	}
}

// newMinimalProjectServer builds a minimal IPv4 test server with a working project
// endpoint and delegates everything else to extraHandler.
func newMinimalProjectServer(t *testing.T, extraHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/projects/proj-uuid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "proj-uuid", "name": "my-project"})
	})
	mux.HandleFunc("/", extraHandler)
	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: mux}}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

// --- ls error paths ---

func TestWorkspacesList_ProjectAPIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := runWorkspacesCmd(t, srv, "ls", "--project", "bad-proj")
	if err == nil {
		t.Fatal("expected error when project not found")
	}
}

func TestWorkspacesList_APIError(t *testing.T) {
	srv := newMinimalProjectServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"workspace list failed"}`, http.StatusUnprocessableEntity)
	})
	_, err := runWorkspacesCmd(t, srv, "ls", "--project", "proj-uuid")
	if err == nil {
		t.Fatal("expected error from ListWorkspaces")
	}
}

// --- create error paths ---

func TestWorkspacesCreate_ProjectAPIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := runWorkspacesCmd(t, srv, "create", "newws", "--project", "bad-proj")
	if err == nil {
		t.Fatal("expected error when project not found")
	}
}

func TestWorkspacesCreate_APIError(t *testing.T) {
	srv := newMinimalProjectServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"create failed"}`, http.StatusUnprocessableEntity)
	})
	_, err := runWorkspacesCmd(t, srv, "create", "newws", "--project", "proj-uuid")
	if err == nil {
		t.Fatal("expected error from CreateWorkspace")
	}
}

// --- update error paths ---

func TestWorkspacesUpdate_RequiresProject(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	_, err := runWorkspacesCmd(t, srv, "update", "11111111-2222-3333-4444-555555555555", "--name", "new-name")
	if err == nil || !strings.Contains(err.Error(), "--project") {
		t.Fatalf("expected missing project error, got %v", err)
	}
}

func TestWorkspacesUpdate_ProjectAPIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := runWorkspacesCmd(t, srv, "update", "11111111-2222-3333-4444-555555555555", "--project", "bad-proj", "--name", "new-name")
	if err == nil {
		t.Fatal("expected error when project not found")
	}
}

func TestWorkspacesUpdate_WorkspaceAPIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})
	_, err := runWorkspacesCmd(t, srv, "update", "22222222-2222-2222-2222-222222222222", "--project", "proj-uuid", "--name", "new-name")
	if err == nil {
		t.Fatal("expected error when workspace not found")
	}
}

func TestWorkspacesUpdate_APIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"update failed"}`, http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	_, err := runWorkspacesCmd(t, srv, "update", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid", "--name", "new-name")
	if err == nil {
		t.Fatal("expected error from UpdateWorkspace")
	}
}

// --- delete error paths ---

func TestWorkspacesDelete_ProjectAPIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := runWorkspacesCmd(t, srv, "delete", "11111111-2222-3333-4444-555555555555", "--project", "bad-proj", "--force")
	if err == nil {
		t.Fatal("expected error when project not found")
	}
}

func TestWorkspacesDelete_WorkspaceAPIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})
	_, err := runWorkspacesCmd(t, srv, "delete", "22222222-2222-2222-2222-222222222222", "--project", "proj-uuid", "--force")
	if err == nil {
		t.Fatal("expected error when workspace not found")
	}
}

func TestWorkspacesDelete_APIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, `{"error":"delete failed"}`, http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	_, err := runWorkspacesCmd(t, srv, "delete", "11111111-2222-3333-4444-555555555555", "--project", "proj-uuid", "--force")
	if err == nil {
		t.Fatal("expected error from DeleteWorkspace")
	}
}

// --- show error paths ---

func TestWorkspacesShow_ProjectAPIError(t *testing.T) {
	srv := newWorkspacesTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := runWorkspacesCmd(t, srv, "show", "11111111-2222-3333-4444-555555555555", "--project", "bad-proj")
	if err == nil {
		t.Fatal("expected error when project not found")
	}
}
