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
