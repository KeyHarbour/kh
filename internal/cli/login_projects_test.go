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

	buf := &bytes.Buffer{}
	cmd := newLoginCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--token", "pat-123", "--endpoint", "https://example.test/api/v2"})

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
	if cfg.Endpoint != "https://example.test/api/v2" {
		t.Fatalf("expected endpoint to be saved, got %q", cfg.Endpoint)
	}
	if !strings.Contains(buf.String(), "login ok") || !strings.Contains(buf.String(), "https://example.test/api/v2") {
		t.Fatalf("unexpected output %q", buf.String())
	}
}

func TestLoginCommand_UsesEnvFallbacksAndDeviceFlow(t *testing.T) {
	t.Run("env fallback", func(t *testing.T) {
		useTempConfigHome(t)
		t.Setenv("KH_TOKEN", "env-token")
		t.Setenv("KH_ENDPOINT", "https://env.keyharbour.test")

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
		if cfg.Endpoint != "https://env.keyharbour.test" {
			t.Fatalf("expected env endpoint to be saved as-is, got %q", cfg.Endpoint)
		}
	})

	t.Run("device flow", func(t *testing.T) {
		useTempConfigHome(t)

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

	for _, want := range []string{"ls", "show"} {
		if !seen[want] {
			t.Fatalf("expected subcommand %s to be present", want)
		}
	}
}

func TestProjectsListCommand_ReturnsGuidanceError(t *testing.T) {
	cmd := newProjectsListCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	err := cmd.RunE(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "projects listing is not supported") {
		t.Fatalf("expected guidance error, got %v", err)
	}
	if !strings.Contains(err.Error(), "workspace ls --project") {
		t.Fatalf("expected guidance in error, got %v", err)
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
