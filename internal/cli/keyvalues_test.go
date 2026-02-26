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

// newKVTestServer starts a test server pre-wired with project and workspace resolution
// endpoints and a custom handler for /v1/projects/proj-uuid/workspaces/ws-uuid/keyvalues...
func newKVTestServer(t *testing.T, kvHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()

	// project resolution
	mux.HandleFunc("/v1/projects/proj-uuid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "proj-uuid", "name": "my-project"})
	})
	// workspace resolution (list, for name-based lookup)
	mux.HandleFunc("/v1/projects/proj-uuid/workspaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{{"uuid": "ws-uuid", "name": "my-workspace"}})
	})
	// workspace detail
	mux.HandleFunc("/v1/projects/proj-uuid/workspaces/ws-uuid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "ws-uuid", "name": "my-workspace"})
	})
	// kv endpoints
	mux.HandleFunc("/v1/projects/proj-uuid/workspaces/ws-uuid/keyvalues", kvHandler)
	mux.HandleFunc("/v1/projects/proj-uuid/workspaces/ws-uuid/keyvalues/", kvHandler)

	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: mux}}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func runKVCmd(t *testing.T, srv *httptest.Server, args ...string) (string, error) {
	t.Helper()
	// Inject endpoint via env so config.LoadWithEnv picks it up
	t.Setenv("KH_ENDPOINT", srv.URL)
	t.Setenv("KH_TOKEN", "test-token")

	buf := &bytes.Buffer{}
	cmd := newKVCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestKVList_TableOutput(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "FOO", "value": "bar", "expires_at": nil, "private": false},
			{"key": "SECRET", "value": "hidden", "expires_at": nil, "private": true},
		})
	})

	out, err := runKVCmd(t, srv, "ls", "--project", "proj-uuid", "--workspace", "ws-uuid")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "FOO") {
		t.Errorf("expected FOO in output, got: %s", out)
	}
	// Private value should be masked
	if strings.Contains(out, "hidden") {
		t.Errorf("expected private value to be masked, got: %s", out)
	}
	if !strings.Contains(out, "***") {
		t.Errorf("expected *** mask for private value, got: %s", out)
	}
}

func TestKVList_JSONOutput(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "FOO", "value": "bar", "expires_at": nil, "private": false},
		})
	})

	out, err := runKVCmd(t, srv, "ls", "--project", "proj-uuid", "--workspace", "ws-uuid", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &items); err != nil {
		t.Fatalf("expected valid JSON output: %v\noutput: %s", err, out)
	}
	if len(items) != 1 || items[0]["key"] != "FOO" {
		t.Errorf("unexpected items: %v", items)
	}
}

func TestKVGet_MasksPrivateByDefault(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"value": "s3cr3t", "expires_at": nil, "private": true})
	})

	out, err := runKVCmd(t, srv, "get", "MY_KEY", "--project", "proj-uuid", "--workspace", "ws-uuid")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.Contains(out, "s3cr3t") {
		t.Errorf("expected private value to be masked, got: %s", out)
	}
	if !strings.Contains(out, "--reveal") {
		t.Errorf("expected hint about --reveal, got: %s", out)
	}
}

func TestKVGet_RevealFlag(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"value": "s3cr3t", "expires_at": nil, "private": true})
	})

	out, err := runKVCmd(t, srv, "get", "MY_KEY", "--project", "proj-uuid", "--workspace", "ws-uuid", "--reveal")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "s3cr3t") {
		t.Errorf("expected plain value with --reveal, got: %s", out)
	}
}

func TestKVSet_SendsCorrectPayload(t *testing.T) {
	var bodyBytes []byte
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.URL.Query().Get("environment"); got != "production" {
			t.Fatalf("expected environment=production, got %q", got)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	_, err := runKVCmd(t, srv, "set", "NEW_KEY", "new-value", "--project", "proj-uuid", "--workspace", "ws-uuid", "--env", "production")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(string(bodyBytes), `"key":"NEW_KEY"`) {
		t.Errorf("expected key in body, got: %s", bodyBytes)
	}
	if !strings.Contains(string(bodyBytes), `"value":"new-value"`) {
		t.Errorf("expected value in body, got: %s", bodyBytes)
	}
}

func TestKVSet_RequiresEnv(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runKVCmd(t, srv, "set", "KEY", "val", "--project", "proj-uuid", "--workspace", "ws-uuid")
	if err == nil {
		t.Fatal("expected error when --env is missing")
	}
}

func TestKVDelete_RequiresForce(t *testing.T) {
	var deleteCalled bool
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	// Without --force, should not call DELETE
	_, _ = runKVCmd(t, srv, "delete", "MY_KEY", "--project", "proj-uuid", "--workspace", "ws-uuid")
	if deleteCalled {
		t.Fatal("DELETE should not be called without --force")
	}
}

func TestKVDelete_WithForce(t *testing.T) {
	var deleteCalled bool
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, err := runKVCmd(t, srv, "delete", "MY_KEY", "--project", "proj-uuid", "--workspace", "ws-uuid", "--force")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected DELETE to be called with --force")
	}
}
