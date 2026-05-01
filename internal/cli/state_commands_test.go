package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestStateCommand_HasExpectedSubcommands(t *testing.T) {
	cmd := newStateCmd()
	seen := map[string]bool{}
	for _, sub := range cmd.Commands() {
		seen[sub.Name()] = true
	}
	for _, want := range []string{"ls", "show", "lock", "unlock", "verify"} {
		if !seen[want] {
			t.Fatalf("expected subcommand %s", want)
		}
	}
}

func TestStateLsCommand(t *testing.T) {
	t.Run("json output", func(t *testing.T) {
		useTempConfigHome(t)
		outputFormat = "json"
		defer func() { outputFormat = "" }()
		srv := newStateCLITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/v1/states" {
				http.NotFound(w, r)
				return
			}
			if r.URL.Query().Get("project") != "proj" || r.URL.Query().Get("module") != "mod" || r.URL.Query().Get("workspace") != "ws" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"state-1","project":"proj","module":"mod","workspace":"ws","size":42}]`))
		})
		defer srv.Close()

		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_TOKEN", "test-token")

		buf := &bytes.Buffer{}
		cmd := newStateLsCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--project", "proj", "--module", "mod", "--workspace", "ws"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("state ls failed: %v", err)
		}
		if !strings.Contains(buf.String(), `"id": "state-1"`) {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})

	t.Run("table output", func(t *testing.T) {
		useTempConfigHome(t)
		srv := newStateCLITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"state-1","project":"proj","module":"mod","workspace":"ws","size":42}]`))
		})
		defer srv.Close()
		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_TOKEN", "test-token")

		buf := &bytes.Buffer{}
		cmd := newStateLsCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("state ls failed: %v", err)
		}
		if !strings.Contains(buf.String(), "PROJECT") || !strings.Contains(buf.String(), "state-1") {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})
}

func TestStateShowCommand(t *testing.T) {
	t.Run("requires one arg", func(t *testing.T) {
		cmd := newStateShowCmd()
		if err := cmd.Args(cmd, nil); err == nil || !strings.Contains(err.Error(), "requires 1 argument") {
			t.Fatalf("expected arg error, got %v", err)
		}
	})

	t.Run("json output", func(t *testing.T) {
		useTempConfigHome(t)
		srv := newStateCLITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/v1/states/state-1" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("X-State-Meta", `{"id":"state-1","project":"proj"}`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":4}`))
		})
		defer srv.Close()
		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_TOKEN", "test-token")

		buf := &bytes.Buffer{}
		cmd := newStateShowCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"state-1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("state show failed: %v", err)
		}
		if !strings.Contains(buf.String(), `"version": 4`) {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})

	t.Run("meta output", func(t *testing.T) {
		useTempConfigHome(t)
		srv := newStateCLITestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-State-Meta", `{"id":"state-1","project":"proj","module":"mod","workspace":"ws","size":11}`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":4}`))
		})
		defer srv.Close()
		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_TOKEN", "test-token")

		buf := &bytes.Buffer{}
		cmd := newStateShowCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"state-1", "--meta"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("state show failed: %v", err)
		}
		if !strings.Contains(buf.String(), `"id": "state-1"`) || !strings.Contains(buf.String(), `"project": "proj"`) {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})
}

func TestStatefilesCommand_HasExpectedSubcommands(t *testing.T) {
	cmd := newStatefilesCmd()
	seen := map[string]bool{}
	for _, sub := range cmd.Commands() {
		seen[sub.Name()] = true
	}
	for _, want := range []string{"ls", "last", "get", "push", "rm", "rm-all"} {
		if !seen[want] {
			t.Fatalf("expected subcommand %s", want)
		}
	}
}

func TestStatefilesCommands(t *testing.T) {
	published := "2024-01-01T00:00:00Z"

	t.Run("list json output", func(t *testing.T) {
		useTempConfigHome(t)
		outputFormat = "json"
		defer func() { outputFormat = "" }()
		srv := newStatefilesCLITestServer(t, published)
		defer srv.Close()
		setStatefilesEnv(t, srv)

		buf := &bytes.Buffer{}
		cmd := newStatefilesCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"ls", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--environment", "prod"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("statefiles ls failed: %v", err)
		}
		if !strings.Contains(buf.String(), `"uuid": "sf-1"`) {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})

	t.Run("last raw output", func(t *testing.T) {
		useTempConfigHome(t)
		srv := newStatefilesCLITestServer(t, published)
		defer srv.Close()
		setStatefilesEnv(t, srv)

		buf := &bytes.Buffer{}
		cmd := newStatefilesCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"last", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--raw"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("statefiles last failed: %v", err)
		}
		if strings.TrimSpace(buf.String()) != `{"version":4}` {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})

	t.Run("get arg validation and table output", func(t *testing.T) {
		cmd := newStatefilesGetCmd(&statefileTarget{})
		if err := cmd.Args(cmd, nil); err == nil {
			t.Fatal("expected missing arg error")
		}

		useTempConfigHome(t)
		srv := newStatefilesCLITestServer(t, published)
		defer srv.Close()
		setStatefilesEnv(t, srv)

		buf := &bytes.Buffer{}
		cmd = newStatefilesGetCmd(&statefileTarget{})
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"sf-1"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("statefiles get failed: %v", err)
		}
		if !strings.Contains(buf.String(), "sf-1") || !strings.Contains(buf.String(), published) {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})

	t.Run("push validation and stdin success", func(t *testing.T) {
		cmd := newStatefilesPushCmd(&statefileTarget{})
		if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), "provide --file or --stdin") {
			t.Fatalf("expected missing input error, got %v", err)
		}

		cmd = newStatefilesPushCmd(&statefileTarget{})
		cmd.SetArgs([]string{"--file", "a", "--stdin"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("expected conflicting flags error, got %v", err)
		}

		useTempConfigHome(t)
		outputFormat = "json"
		defer func() { outputFormat = "" }()
		srv := newStatefilesCLITestServer(t, published)
		defer srv.Close()
		setStatefilesEnv(t, srv)

		buf := &bytes.Buffer{}
		cmd = newStatefilesCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetIn(strings.NewReader(`{"version":4}`))
		cmd.SetArgs([]string{"push", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--stdin"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("statefiles push failed: %v", err)
		}
		if !strings.Contains(buf.String(), `"action": "push"`) || !strings.Contains(buf.String(), `"status": "accepted"`) {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})

	t.Run("delete and delete all", func(t *testing.T) {
		useTempConfigHome(t)
		srv := newStatefilesCLITestServer(t, published)
		defer srv.Close()
		setStatefilesEnv(t, srv)

		deleteCmd := newStatefilesDeleteCmd(&statefileTarget{})
		if err := deleteCmd.Args(deleteCmd, nil); err == nil {
			t.Fatal("expected missing arg error")
		}

		buf := &bytes.Buffer{}
		deleteCmd = newStatefilesDeleteCmd(&statefileTarget{})
		deleteCmd.SetOut(buf)
		deleteCmd.SetErr(io.Discard)
		deleteCmd.SetArgs([]string{"sf-1"})
		if err := deleteCmd.Execute(); err != nil {
			t.Fatalf("statefiles rm failed: %v", err)
		}
		if !strings.Contains(buf.String(), "Statefile sf-1 deleted") {
			t.Fatalf("unexpected output %q", buf.String())
		}

		rmAll := newStatefilesCmd()
		rmAll.SetOut(io.Discard)
		rmAll.SetErr(io.Discard)
		rmAll.SetArgs([]string{"rm-all", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555"})
		if err := rmAll.Execute(); err == nil || !strings.Contains(err.Error(), "without --force") {
			t.Fatalf("expected force error, got %v", err)
		}

		buf = &bytes.Buffer{}
		rmAll = newStatefilesCmd()
		rmAll.SetOut(buf)
		rmAll.SetErr(io.Discard)
		rmAll.SetArgs([]string{"rm-all", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--force"})
		if err := rmAll.Execute(); err != nil {
			t.Fatalf("statefiles rm-all failed: %v", err)
		}
		if !strings.Contains(buf.String(), "All statefiles deleted") {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})

	t.Run("push file and empty stdin errors", func(t *testing.T) {
		useTempConfigHome(t)
		srv := newStatefilesCLITestServer(t, published)
		defer srv.Close()
		setStatefilesEnv(t, srv)

		file := t.TempDir() + "/terraform.tfstate"
		if err := os.WriteFile(file, []byte(`{"serial":1}`), 0o600); err != nil {
			t.Fatalf("WriteFile error: %v", err)
		}

		buf := &bytes.Buffer{}
		cmd := newStatefilesCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"push", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--file", file})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("statefiles push file failed: %v", err)
		}
		if !strings.Contains(buf.String(), "Statefile pushed") {
			t.Fatalf("unexpected output %q", buf.String())
		}

		cmd = newStatefilesCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetIn(strings.NewReader(""))
		cmd.SetArgs([]string{"push", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--stdin"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "content is empty") {
			t.Fatalf("expected empty content error, got %v", err)
		}
	})
}

func newStateCLITestServer(t *testing.T, statesHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/v1/states", statesHandler)
	mux.HandleFunc("/api/v2/v1/states/", statesHandler)
	return httptest.NewServer(mux)
}

func newStatefilesCLITestServer(t *testing.T, published string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/projects/proj-uuid", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, map[string]any{"uuid": "proj-uuid", "name": "demo-project"})
	})
	mux.HandleFunc("/api/v2/workspaces/11111111-2222-3333-4444-555555555555", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "name": "demo-workspace"})
	})
	mux.HandleFunc("/api/v2/workspaces/11111111-2222-3333-4444-555555555555/statefiles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if env := r.URL.Query().Get("environment"); env != "" && env != "prod" {
				t.Fatalf("unexpected environment query %q", env)
			}
			writeJSONBody(t, w, []map[string]any{{"uuid": "sf-1", "content": `{"version":4}`, "published_at": published}})
		case http.MethodPost:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if body["content"] == "" {
				t.Fatal("expected non-empty content")
			}
			writeJSONStatusBody(t, w, http.StatusCreated, map[string]any{"status": "accepted"})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/api/v2/workspaces/11111111-2222-3333-4444-555555555555/statefiles/last", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, map[string]any{"uuid": "sf-last", "content": `{"version":4}`, "published_at": published})
	})
	mux.HandleFunc("/api/v2/statefiles/sf-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSONBody(t, w, map[string]any{"uuid": "sf-1", "content": `{"version":4}`, "published_at": published})
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	})
	return httptest.NewServer(mux)
}

func setStatefilesEnv(t *testing.T, srv *httptest.Server) {
	t.Helper()
	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")
	t.Setenv("KH_PROJECT", "proj-uuid")
	t.Setenv("KH_WORKSPACE", "11111111-2222-3333-4444-555555555555")
}

func writeJSONBody(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
}

func writeJSONStatusBody(t *testing.T, w http.ResponseWriter, status int, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode JSON: %v", err)
	}
}

// errorBody returns a JSON error response handler with the given status code.
func errorBody(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"operation failed"}`, status)
	}
}

// newStatefilesOpErrorServer returns a server where project and workspace resolve
// successfully but the given operationPath returns an error response.
func newStatefilesOpErrorServer(t *testing.T, operationPath string, status int) *httptest.Server {
	t.Helper()
	published := "2024-01-01T00:00:00Z"
	base := newStatefilesCLITestServer(t, published)
	base.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/projects/proj-uuid", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, map[string]any{"uuid": "proj-uuid", "name": "demo-project"})
	})
	mux.HandleFunc("/api/v2/workspaces/11111111-2222-3333-4444-555555555555", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "name": "demo-workspace"})
	})
	mux.HandleFunc(operationPath, errorBody(status))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// --- state ls error ---

func TestStateLsAPIError(t *testing.T) {
	useTempConfigHome(t)
	srv := newStateCLITestServer(t, errorBody(http.StatusUnprocessableEntity))
	defer srv.Close()
	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")

	cmd := newStateLsCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from ListStates")
	}
}

// --- state show error ---

func TestStateShowAPIError(t *testing.T) {
	useTempConfigHome(t)
	srv := newStateCLITestServer(t, errorBody(http.StatusUnprocessableEntity))
	defer srv.Close()
	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")

	cmd := newStateShowCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"state-1"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from GetStateRaw")
	}
}

// --- statefiles ls ---

func TestStatefilesListTableOutput(t *testing.T) {
	useTempConfigHome(t)
	published := "2024-01-01T00:00:00Z"
	srv := newStatefilesCLITestServer(t, published)
	defer srv.Close()
	setStatefilesEnv(t, srv)

	buf := &bytes.Buffer{}
	cmd := newStatefilesCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"ls", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("statefiles ls table failed: %v", err)
	}
	if !strings.Contains(buf.String(), "sf-1") || !strings.Contains(buf.String(), "UUID") {
		t.Errorf("expected table output with UUID header and sf-1, got: %s", buf.String())
	}
}

func TestStatefilesListResolveError(t *testing.T) {
	useTempConfigHome(t)
	t.Setenv("KH_PROJECT", "")
	t.Setenv("KH_WORKSPACE", "")
	t.Setenv("KH_TOKEN", "test-token")

	cmd := newStatefilesCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"ls"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected resolve error")
	}
}

func TestStatefilesListAPIError(t *testing.T) {
	useTempConfigHome(t)
	srv := newStatefilesOpErrorServer(t, "/api/v2/workspaces/11111111-2222-3333-4444-555555555555/statefiles", http.StatusUnprocessableEntity)
	setStatefilesEnv(t, srv)

	cmd := newStatefilesCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"ls", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from ListStatefiles")
	}
}

// --- statefiles last ---

func TestStatefilesLastResolveError(t *testing.T) {
	useTempConfigHome(t)
	t.Setenv("KH_PROJECT", "")
	t.Setenv("KH_WORKSPACE", "")
	t.Setenv("KH_TOKEN", "test-token")

	cmd := newStatefilesCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"last"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected resolve error")
	}
}

func TestStatefilesLastAPIError(t *testing.T) {
	useTempConfigHome(t)
	srv := newStatefilesOpErrorServer(t, "/api/v2/workspaces/11111111-2222-3333-4444-555555555555/statefiles/last", http.StatusUnprocessableEntity)
	setStatefilesEnv(t, srv)

	cmd := newStatefilesCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"last", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from GetLastStatefile")
	}
}

func TestStatefilesLastJSONOutput(t *testing.T) {
	useTempConfigHome(t)
	outputFormat = "json"
	defer func() { outputFormat = "" }()

	published := "2024-01-01T00:00:00Z"
	srv := newStatefilesCLITestServer(t, published)
	defer srv.Close()
	setStatefilesEnv(t, srv)

	buf := &bytes.Buffer{}
	cmd := newStatefilesCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"last", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("statefiles last JSON failed: %v", err)
	}
	if !strings.Contains(buf.String(), `"uuid"`) {
		t.Errorf("expected JSON output, got: %s", buf.String())
	}
}

func TestStatefilesLastTableOutput(t *testing.T) {
	useTempConfigHome(t)
	published := "2024-01-01T00:00:00Z"
	srv := newStatefilesCLITestServer(t, published)
	defer srv.Close()
	setStatefilesEnv(t, srv)

	buf := &bytes.Buffer{}
	cmd := newStatefilesCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"last", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("statefiles last table failed: %v", err)
	}
	if !strings.Contains(buf.String(), "sf-last") || !strings.Contains(buf.String(), "UUID") {
		t.Errorf("expected table with sf-last, got: %s", buf.String())
	}
}

// --- statefiles get ---

func TestStatefilesGetAPIError(t *testing.T) {
	useTempConfigHome(t)
	srv := newStatefilesOpErrorServer(t, "/api/v2/statefiles/", http.StatusUnprocessableEntity)
	setStatefilesEnv(t, srv)

	cmd := newStatefilesGetCmd(&statefileTarget{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"sf-1"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from GetStatefile")
	}
}

func TestStatefilesGetRawOutput(t *testing.T) {
	useTempConfigHome(t)
	published := "2024-01-01T00:00:00Z"
	srv := newStatefilesCLITestServer(t, published)
	defer srv.Close()
	setStatefilesEnv(t, srv)

	buf := &bytes.Buffer{}
	cmd := newStatefilesGetCmd(&statefileTarget{})
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"sf-1", "--raw"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("statefiles get --raw failed: %v", err)
	}
	out := strings.TrimSpace(buf.String())
	if out != `{"version":4}` {
		t.Errorf("expected raw content, got: %s", out)
	}
}

func TestStatefilesGetJSONOutput(t *testing.T) {
	useTempConfigHome(t)
	outputFormat = "json"
	defer func() { outputFormat = "" }()

	published := "2024-01-01T00:00:00Z"
	srv := newStatefilesCLITestServer(t, published)
	defer srv.Close()
	setStatefilesEnv(t, srv)

	buf := &bytes.Buffer{}
	cmd := newStatefilesGetCmd(&statefileTarget{})
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"sf-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("statefiles get JSON failed: %v", err)
	}
	if !strings.Contains(buf.String(), `"uuid"`) {
		t.Errorf("expected JSON output, got: %s", buf.String())
	}
}

// --- statefiles push ---

func TestStatefilesPushFileReadError(t *testing.T) {
	cmd := newStatefilesPushCmd(&statefileTarget{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--file", "/nonexistent/path/terraform.tfstate"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected file read error")
	}
}

func TestStatefilesPushResolveError(t *testing.T) {
	useTempConfigHome(t)
	t.Setenv("KH_PROJECT", "")
	t.Setenv("KH_WORKSPACE", "")
	t.Setenv("KH_TOKEN", "test-token")

	cmd := newStatefilesCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(`{"version":4}`))
	cmd.SetArgs([]string{"push", "--stdin"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected resolve error for push")
	}
}

func TestStatefilesPushAPIError(t *testing.T) {
	useTempConfigHome(t)
	srv := newStatefilesOpErrorServer(t, "/api/v2/workspaces/11111111-2222-3333-4444-555555555555/statefiles", http.StatusUnprocessableEntity)
	setStatefilesEnv(t, srv)

	cmd := newStatefilesCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetIn(strings.NewReader(`{"version":4}`))
	cmd.SetArgs([]string{"push", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--stdin"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from CreateStatefile")
	}
}

// --- statefiles rm ---

func TestStatefilesDeleteAPIError(t *testing.T) {
	useTempConfigHome(t)
	srv := newStatefilesOpErrorServer(t, "/api/v2/statefiles/", http.StatusUnprocessableEntity)
	setStatefilesEnv(t, srv)

	cmd := newStatefilesDeleteCmd(&statefileTarget{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"sf-1"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from DeleteStatefile")
	}
}

// --- statefiles rm-all ---

func TestStatefilesDeleteAllResolveError(t *testing.T) {
	useTempConfigHome(t)
	t.Setenv("KH_PROJECT", "")
	t.Setenv("KH_WORKSPACE", "")
	t.Setenv("KH_TOKEN", "test-token")

	cmd := newStatefilesCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"rm-all", "--force"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected resolve error for rm-all")
	}
}

func TestStatefilesDeleteAllAPIError(t *testing.T) {
	useTempConfigHome(t)
	srv := newStatefilesOpErrorServer(t, "/api/v2/workspaces/11111111-2222-3333-4444-555555555555/statefiles", http.StatusUnprocessableEntity)
	setStatefilesEnv(t, srv)

	cmd := newStatefilesCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"rm-all", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--force"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error from DeleteStatefiles")
	}
}

// --- lock ---

func TestLockCmd_RequiresOneArg(t *testing.T) {
	cmd := newLockCmd()
	if err := cmd.Args(cmd, nil); err == nil || !strings.Contains(err.Error(), "lock requires 1 argument") {
		t.Fatalf("expected arg error, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"id"}); err != nil {
		t.Fatalf("expected no error for one arg, got %v", err)
	}
}

func TestLockCmd_NotImplemented(t *testing.T) {
	cmd := newLockCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"state-1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not-implemented error, got %v", err)
	}
}

// --- unlock ---

func TestUnlockCmd_RequiresOneArg(t *testing.T) {
	cmd := newUnlockCmd()
	if err := cmd.Args(cmd, nil); err == nil || !strings.Contains(err.Error(), "unlock requires 1 argument") {
		t.Fatalf("expected arg error, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"id"}); err != nil {
		t.Fatalf("expected no error for one arg, got %v", err)
	}
}

func TestUnlockCmd_NotImplemented(t *testing.T) {
	cmd := newUnlockCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"state-1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not-implemented error, got %v", err)
	}
}

// --- verify ---

func TestVerifyCmd_RequiresOneArg(t *testing.T) {
	cmd := newVerifyCmd()
	if err := cmd.Args(cmd, nil); err == nil || !strings.Contains(err.Error(), "verify requires 1 argument") {
		t.Fatalf("expected arg error, got %v", err)
	}
	if err := cmd.Args(cmd, []string{"id"}); err != nil {
		t.Fatalf("expected no error for one arg, got %v", err)
	}
}

func TestVerifyCmd_NotImplementedWithoutFull(t *testing.T) {
	cmd := newVerifyCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"state-1"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Fatalf("expected not-implemented error, got %v", err)
	}
}

func TestVerifyCmd_FullTableOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	cmd := newVerifyCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"state-1", "--full"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify --full failed: %v", err)
	}
	if !strings.Contains(buf.String(), "state-1") || !strings.Contains(buf.String(), "ok") {
		t.Errorf("expected table output with state-1 and ok, got: %s", buf.String())
	}
}

func TestVerifyCmd_FullJSONOutput(t *testing.T) {
	outputFormat = "json"
	defer func() { outputFormat = "" }()

	buf := &bytes.Buffer{}
	cmd := newVerifyCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"state-1", "--full"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("verify --full json failed: %v", err)
	}
	if !strings.Contains(buf.String(), `"state_id"`) || !strings.Contains(buf.String(), `"ok": true`) {
		t.Errorf("expected JSON output, got: %s", buf.String())
	}
}
