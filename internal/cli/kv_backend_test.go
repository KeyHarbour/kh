package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"kh/internal/config"
)

func runKVLocalCmd(t *testing.T, args ...string) (stdout string, stderr string, err error) {
	t.Helper()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd := newKVCmd()
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetIn(strings.NewReader(""))
	cmd.SetContext(context.Background())
	cmd.SetArgs(args)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestKVBackend_FileMode_WorksEndToEnd(t *testing.T) {
	kvFile := filepath.Join(t.TempDir(), "kv.json")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KH_TOKEN", "")
	t.Setenv("KH_WORKSPACE", "11111111-2222-3333-4444-555555555555")
	t.Setenv("KH_KV_FILE", kvFile)

	if _, _, err := runKVLocalCmd(t, "set", "API_KEY", "abc123", "--kv-backend", "file"); err != nil {
		t.Fatalf("set failed: %v", err)
	}

	stdout, _, err := runKVLocalCmd(t, "get", "API_KEY", "--reveal", "--kv-backend", "file")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if strings.TrimSpace(stdout) != "abc123" {
		t.Fatalf("expected abc123, got: %q", stdout)
	}
}

func TestKVBackend_FileModeRequiresWorkspace(t *testing.T) {
	kvFile := filepath.Join(t.TempDir(), "kv.json")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KH_TOKEN", "")
	t.Setenv("KH_KV_FILE", kvFile)
	t.Setenv("KH_WORKSPACE", "")

	_, _, err := runKVLocalCmd(t, "get", "MISSING", "--kv-backend", "file")
	if err == nil {
		t.Fatal("expected workspace required error in file mode")
	}
	if !strings.Contains(err.Error(), "--workspace is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKVBackend_RemoteModeWithoutEndpointFails(t *testing.T) {
	opts := &kvCmdOpts{backend: kvBackendRemote}
	var errBuf bytes.Buffer
	_, err := opts.resolveBackendMode(config.Config{}, &errBuf)
	if err == nil {
		t.Fatal("expected error when remote backend is selected without endpoint")
	}
	if !strings.Contains(err.Error(), "endpoint is missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKVBackend_FileStore_OneTimeOnlyConsumedByGet(t *testing.T) {
	kvFile := filepath.Join(t.TempDir(), "kv.json")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("KH_TOKEN", "")
	t.Setenv("KH_WORKSPACE", "11111111-2222-3333-4444-555555555555")
	t.Setenv("KH_KV_FILE", kvFile)

	if _, _, err := runKVLocalCmd(t, "set", "OTP", "secret", "--one-time-only", "--kv-backend", "file"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if _, _, err := runKVLocalCmd(t, "get", "OTP", "--reveal", "--kv-backend", "file"); err != nil {
		t.Fatalf("first get failed: %v", err)
	}
	if _, _, err := runKVLocalCmd(t, "get", "OTP", "--reveal", "--kv-backend", "file"); err == nil {
		t.Fatal("expected second get to fail after one-time-only consumption")
	}
}

func TestKVBackend_ResolveStoreFilePathPreference(t *testing.T) {
	t.Setenv("KH_KV_FILE", filepath.Join(t.TempDir(), "from-env.json"))
	opts := &kvCmdOpts{kvFile: filepath.Join(t.TempDir(), "from-flag.json")}
	if got := opts.resolveKVFilePath(); !strings.HasSuffix(got, "from-flag.json") {
		t.Fatalf("expected flag path to win, got: %s", got)
	}

	opts = &kvCmdOpts{}
	if got := opts.resolveKVFilePath(); !strings.HasSuffix(got, "from-env.json") {
		t.Fatalf("expected env path to be used, got: %s", got)
	}
}

func TestKVBackend_ResolveBackendModeAuto(t *testing.T) {
	opts := &kvCmdOpts{}
	var errBuf bytes.Buffer

	mode, err := opts.resolveBackendMode(config.Config{Endpoint: "https://example.test/api/v2"}, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != kvBackendRemote {
		t.Fatalf("expected remote mode, got %s", mode)
	}

	errBuf.Reset()
	mode, err = opts.resolveBackendMode(config.Config{}, &errBuf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != kvBackendFile {
		t.Fatalf("expected file mode, got %s", mode)
	}
	if !strings.Contains(errBuf.String(), "using file KV backend") {
		t.Fatalf("expected fallback warning, got: %s", errBuf.String())
	}
}
