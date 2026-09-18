package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	internalconfig "kh/internal/config"
)

func TestLoginCommand_RequiresTokenOrDevice(t *testing.T) {
	useTempConfigHome(t)

	cmd := newLoginCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.RunE(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "provide --token") {
		t.Fatalf("expected missing token error, got %v", err)
	}
}

func TestLoginCommand_SavesTokenAndEndpoint(t *testing.T) {
	useTempConfigHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/organizations/org-123/projects" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()
	t.Setenv("KH_ORG", "org-123")

	buf := &bytes.Buffer{}
	cmd := newLoginCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--token", "pat-123", "--endpoint", srv.URL + "/api/v2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("login failed: %v", err)
	}

	cfg, err := internalconfig.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Token != "pat-123" {
		t.Fatalf("expected token to be saved, got %q", cfg.Token)
	}
	if cfg.Endpoint != srv.URL+"/api/v2" {
		t.Fatalf("expected endpoint to be saved, got %q", cfg.Endpoint)
	}
	if !strings.Contains(buf.String(), "login ok") || !strings.Contains(buf.String(), srv.URL+"/api/v2") {
		t.Fatalf("unexpected output %q", buf.String())
	}
}

func TestLoginCommand_UsesEnvFallbacksAndDeviceFlow(t *testing.T) {
	t.Run("env fallback", func(t *testing.T) {
		useTempConfigHome(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/organizations/org-123/projects" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}))
		defer srv.Close()

		t.Setenv("KH_TOKEN", "env-token")
		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_ORG", "org-123")

		buf := &bytes.Buffer{}
		cmd := newLoginCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("login failed: %v", err)
		}

		cfg, err := internalconfig.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if cfg.Token != "env-token" {
			t.Fatalf("expected env token to be saved, got %q", cfg.Token)
		}
		if cfg.Endpoint != srv.URL+"/api/v2" {
			t.Fatalf("expected env endpoint to be saved as-is, got %q", cfg.Endpoint)
		}
	})

	t.Run("device flow", func(t *testing.T) {
		useTempConfigHome(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/organizations/org-123/projects" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
		}))
		defer srv.Close()
		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_ORG", "org-123")

		oldStderr := os.Stderr
		readPipe, writePipe, err := os.Pipe()
		if err != nil {
			t.Fatalf("Pipe error: %v", err)
		}
		os.Stderr = writePipe
		defer func() { os.Stderr = oldStderr }()

		buf := &bytes.Buffer{}
		cmd := newLoginCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"--device"})

		if err := cmd.Execute(); err != nil {
			_ = writePipe.Close()
			t.Fatalf("device login failed: %v", err)
		}
		_ = writePipe.Close()

		stderrBuf := &bytes.Buffer{}
		if _, err := io.Copy(stderrBuf, readPipe); err != nil {
			t.Fatalf("Copy stderr error: %v", err)
		}
		_ = readPipe.Close()

		cfg, err := internalconfig.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if cfg.Token != "device-token-stub" {
			t.Fatalf("expected device token to be saved, got %q", cfg.Token)
		}
		if !strings.Contains(stderrBuf.String(), "Starting device flow") {
			t.Fatalf("expected device flow message, got %q", stderrBuf.String())
		}
	})
}

func TestLogoutCommand(t *testing.T) {
	t.Run("already logged out", func(t *testing.T) {
		useTempConfigHome(t)

		buf := &bytes.Buffer{}
		cmd := newLogoutCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("logout failed: %v", err)
		}
		if !strings.Contains(buf.String(), "already logged out") {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})

	t.Run("clears token", func(t *testing.T) {
		useTempConfigHome(t)
		if err := internalconfig.Save(internalconfig.Config{Token: "secret-token", Endpoint: "https://example.test"}); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		buf := &bytes.Buffer{}
		cmd := newLogoutCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)

		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("logout failed: %v", err)
		}
		cfg, err := internalconfig.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if cfg.Token != "" {
			t.Fatalf("expected token to be cleared, got %q", cfg.Token)
		}
		if !strings.Contains(buf.String(), "logged out") {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})
}

func TestProjectsCommand_HasExpectedSubcommands(t *testing.T) {
	cmd := newProjectsCmd()
	if cmd.Use != "project" {
		t.Fatalf("expected use project, got %s", cmd.Use)
	}

	seen := map[string]bool{}
	for _, sub := range cmd.Commands() {
		seen[sub.Name()] = true
	}

	for _, want := range []string{"ls", "show", "create", "update"} {
		if !seen[want] {
			t.Fatalf("expected subcommand %s to be present", want)
		}
	}
}

func TestProjectsListCommand_RequiresOrg(t *testing.T) {
	useTempConfigHome(t)
	cmd := newProjectsListCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.RunE(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "--org is required") {
		t.Fatalf("expected missing org error, got %v", err)
	}
}

func TestProjectsListCommand_TableOutput(t *testing.T) {
	useTempConfigHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/organizations/org-123/projects" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"uuid":"p-1","name":"alpha"},{"uuid":"p-2","name":"beta"}]`))
	}))
	defer srv.Close()

	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")

	buf := &bytes.Buffer{}
	cmd := newProjectsListCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--org", "org-123"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("ls failed: %v", err)
	}
	if !strings.Contains(buf.String(), "alpha") || !strings.Contains(buf.String(), "beta") {
		t.Fatalf("unexpected output %q", buf.String())
	}
}

func TestProjectsListCommand_JSONOutput(t *testing.T) {
	useTempConfigHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"uuid":"p-1","name":"alpha"}]`))
	}))
	defer srv.Close()

	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")

	buf := &bytes.Buffer{}
	cmd := newProjectsListCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--org", "org-123", "-o", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("ls failed: %v", err)
	}
	if !strings.Contains(buf.String(), `"uuid"`) || !strings.Contains(buf.String(), "alpha") {
		t.Fatalf("unexpected output %q", buf.String())
	}
}

func TestProjectsCreateCommand_RequiresOrg(t *testing.T) {
	useTempConfigHome(t)
	cmd := newProjectsCreateCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"my-project", "--environment", "production"})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--org is required") {
		t.Fatalf("expected missing org error, got %v", err)
	}
}

func TestProjectsCreateCommand_RequiresEnvironment(t *testing.T) {
	useTempConfigHome(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	}))
	defer srv.Close()

	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")

	cmd := newProjectsCreateCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"my-project", "--org", "org-123"})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--environment is required") {
		t.Fatalf("expected missing environment error, got %v", err)
	}
}

func TestProjectsCreateCommand_SendsCorrectPayload(t *testing.T) {
	useTempConfigHome(t)

	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/organizations/org-123/projects" {
			http.NotFound(w, r)
			return
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"p-new","name":"my-project"}`))
	}))
	defer srv.Close()

	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")

	buf := &bytes.Buffer{}
	cmd := newProjectsCreateCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"my-project", "--org", "org-123", "--environment", "production", "--environment", "staging"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if !strings.Contains(buf.String(), "created") {
		t.Fatalf("unexpected output %q", buf.String())
	}
	body := string(gotBody)
	if !strings.Contains(body, `"my-project"`) {
		t.Fatalf("expected project name in body, got: %s", body)
	}
	if !strings.Contains(body, "production") || !strings.Contains(body, "staging") {
		t.Fatalf("expected environments in body, got: %s", body)
	}
}

func TestProjectsUpdateCommand_RequiresFlag(t *testing.T) {
	useTempConfigHome(t)
	cmd := newProjectsUpdateCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"proj-123"})

	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "at least one of --name or --environment") {
		t.Fatalf("expected missing flag error, got %v", err)
	}
}

func TestProjectsUpdateCommand_SendsCorrectPayload(t *testing.T) {
	useTempConfigHome(t)

	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/proj-123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"uuid":"proj-123","name":"old-name","environment_names":["staging"]}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v2/projects/proj-123":
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"updated"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")

	buf := &bytes.Buffer{}
	cmd := newProjectsUpdateCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"proj-123", "--name", "new-name", "--environment", "production"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	body := string(gotBody)
	if !strings.Contains(body, `"new-name"`) {
		t.Fatalf("expected new-name in body, got: %s", body)
	}
	if !strings.Contains(body, "production") {
		t.Fatalf("expected environment in body, got: %s", body)
	}
}

func TestProjectsUpdateCommand_FillsUnchangedFieldsFromCurrentState(t *testing.T) {
	useTempConfigHome(t)

	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects/proj-123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"uuid":"proj-123","name":"existing-name","environment_names":["staging","production"]}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v2/projects/proj-123":
			gotBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"updated"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")

	cmd := newProjectsUpdateCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	// Only pass --name; environments should be preserved from current state.
	cmd.SetArgs([]string{"proj-123", "--name", "renamed"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	body := string(gotBody)
	if !strings.Contains(body, `"renamed"`) {
		t.Fatalf("expected renamed in body, got: %s", body)
	}
	if !strings.Contains(body, "staging") || !strings.Contains(body, "production") {
		t.Fatalf("expected existing environments preserved in body, got: %s", body)
	}
}

func TestProjectsShowCommand(t *testing.T) {
	t.Run("too many args", func(t *testing.T) {
		cmd := newProjectsShowCmd()
		if err := cmd.Args(cmd, []string{"a", "b"}); err == nil || !strings.Contains(err.Error(), "at most one argument") {
			t.Fatalf("expected arg error, got %v", err)
		}
	})

	t.Run("missing project reference", func(t *testing.T) {
		useTempConfigHome(t)
		cmd := newProjectsShowCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(nil)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "project uuid is required") {
			t.Fatalf("expected missing project error, got %v", err)
		}
	})

	t.Run("shows project from argument", func(t *testing.T) {
		useTempConfigHome(t)
		outputFormat = "json"
		defer func() { outputFormat = "" }()

		var getCount int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/projects/proj-123" {
				http.NotFound(w, r)
				return
			}
			getCount++
			w.Header().Set("Content-Type", "application/json")
			if getCount == 1 {
				_, _ = w.Write([]byte(`{"uuid":"proj-123","name":"demo"}`))
				return
			}
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer srv.Close()

		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_TOKEN", "test-token")

		buf := &bytes.Buffer{}
		cmd := newProjectsShowCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"proj-123"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("show failed: %v", err)
		}
		if !strings.Contains(buf.String(), `"uuid": "proj-123"`) || !strings.Contains(buf.String(), `"name": "demo"`) {
			t.Fatalf("unexpected output %q", buf.String())
		}
		if getCount < 2 {
			t.Fatalf("expected at least two GetProject calls, got %d", getCount)
		}
	})

	// Like every sibling show command, the default is a table; JSON is opt-in
	// via --output json.
	t.Run("defaults to table output", func(t *testing.T) {
		useTempConfigHome(t)
		outputFormat = "table"
		defer func() { outputFormat = "" }()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/projects/proj-123" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"uuid":"proj-123","name":"demo","environment_names":["prod","dev"]}`))
		}))
		defer srv.Close()

		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_TOKEN", "test-token")

		buf := &bytes.Buffer{}
		cmd := newProjectsShowCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"proj-123"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("show failed: %v", err)
		}
		out := buf.String()
		if strings.Contains(out, `"uuid"`) {
			t.Fatalf("expected table output, got JSON: %q", out)
		}
		for _, want := range []string{"UUID", "NAME", "ENVIRONMENTS", "proj-123", "demo", "prod, dev"} {
			if !strings.Contains(out, want) {
				t.Fatalf("expected %q in output, got %q", want, out)
			}
		}
	})

	t.Run("project api error", func(t *testing.T) {
		useTempConfigHome(t)

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer srv.Close()

		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_TOKEN", "test-token")

		cmd := newProjectsShowCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs([]string{"unknown-proj"})

		if err := cmd.Execute(); err == nil {
			t.Fatal("expected error when project not found")
		}
	})

	t.Run("uses KH_PROJECT fallback", func(t *testing.T) {
		useTempConfigHome(t)
		outputFormat = "json"
		defer func() { outputFormat = "" }()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v2/projects/proj-env" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"uuid":"proj-env","name":"from-env"}`))
		}))
		defer srv.Close()

		t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
		t.Setenv("KH_TOKEN", "test-token")
		t.Setenv("KH_PROJECT", "proj-env")

		buf := &bytes.Buffer{}
		cmd := newProjectsShowCmd()
		cmd.SetOut(buf)
		cmd.SetErr(io.Discard)

		if err := cmd.Execute(); err != nil {
			t.Fatalf("show failed: %v", err)
		}
		if !strings.Contains(buf.String(), `"uuid": "proj-env"`) {
			t.Fatalf("unexpected output %q", buf.String())
		}
	})
}

// ── login token validation (#44) ──────────────────────────────────────────

// loginStubServer answers the org-scoped project list used to verify a token,
// keyed by the org in the path so one server can stand in for every outcome.
func loginStubServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/organizations/good/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
		case strings.Contains(r.URL.Path, "/organizations/unauth/"):
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"token expired"}`))
		case strings.Contains(r.URL.Path, "/organizations/forbidden/"):
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"no access"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A token nothing has checked must never be reported as "login ok". Before this
// fix, login with no org configured skipped validation entirely and printed
// "login ok" for any string, against any endpoint — including one that does not
// resolve.
func TestLoginCommand_UnverifiedWithoutOrg(t *testing.T) {
	useTempConfigHome(t)
	t.Setenv("KH_ORG", "")

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := newLoginCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--token", "pat-123", "--endpoint", "https://example.test/api/v2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("login should still save the token, got %v", err)
	}
	if strings.Contains(stdout.String(), "login ok") {
		t.Errorf("must not claim success for an unverified token, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "NOT verified") {
		t.Errorf("expected the unverified notice on stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "could not be checked") {
		t.Errorf("expected a warning on stderr, got %q", stderr.String())
	}

	// The token is still saved — the command did its job, it just says so honestly.
	cfg, err := internalconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Token != "pat-123" {
		t.Errorf("token should be saved, got %q", cfg.Token)
	}
}

// --org verifies the token without needing KH_ORG, and is persisted for later
// commands.
func TestLoginCommand_OrgFlagVerifies(t *testing.T) {
	useTempConfigHome(t)
	t.Setenv("KH_ORG", "")
	srv := loginStubServer(t)

	stdout := &bytes.Buffer{}
	cmd := newLoginCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--token", "pat-123", "--endpoint", srv.URL + "/api/v2", "--org", "good"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("login failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "login ok") {
		t.Errorf("expected login ok, got %q", stdout.String())
	}
	cfg, err := internalconfig.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Org != "good" {
		t.Errorf("expected --org to be saved, got %q", cfg.Org)
	}
}

// A failure to verify is not automatically an invalid token. Reporting a 403,
// a wrong org or an unreachable API as "token validation failed" sends users
// off to regenerate a token that was never the problem.
func TestLoginCommand_ValidationErrorsAreDistinguished(t *testing.T) {
	srv := loginStubServer(t)

	cases := []struct {
		name     string
		org      string
		endpoint string
		wantCode string
		wantMsg  string
	}{
		{"401 is an invalid token", "unauth", srv.URL + "/api/v2", "KH-AUTH-002", "rejected this token"},
		{"403 is a permission problem", "forbidden", srv.URL + "/api/v2", "KH-PERM-001", "not permitted to read organization"},
		{"404 is a wrong org", "missing", srv.URL + "/api/v2", "KH-NF-001", "does not exist"},
		{"unreachable API is not a token problem", "good", "http://127.0.0.1:1/api/v2", "KH-NET-002", "could not reach the API"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useTempConfigHome(t)
			t.Setenv("KH_ORG", "")

			cmd := newLoginCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"--token", "pat", "--endpoint", tc.endpoint, "--org", tc.org})

			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected an error")
			}
			khErr := classifyError(err)
			if khErr.Code != tc.wantCode {
				t.Errorf("code = %q, want %q (err: %v)", khErr.Code, tc.wantCode, err)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tc.wantMsg)
			}

			// A token the API did not accept must not be left in the config.
			cfg, loadErr := internalconfig.Load()
			if loadErr == nil && cfg.Token == "pat" {
				t.Error("an unverified token must not be saved after a failed check")
			}
		})
	}
}
