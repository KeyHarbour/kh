package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"kh/internal/kvencrypt"
)

// newKVTestServer starts a test server pre-wired with project and workspace resolution
// endpoints plus V2 key/value routes.
func newKVTestServer(t *testing.T, kvHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()

	// project resolution
	mux.HandleFunc("/api/v2/projects/proj-uuid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "proj-uuid", "name": "my-project"})
	})
	// workspace resolution (list, for name-based lookup)
	mux.HandleFunc("/api/v2/projects/proj-uuid/workspaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{{"uuid": "11111111-2222-3333-4444-555555555555", "name": "my-workspace"}})
	})
	// workspace detail
	mux.HandleFunc("/api/v2/workspaces/11111111-2222-3333-4444-555555555555", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "11111111-2222-3333-4444-555555555555", "name": "my-workspace"})
	})
	// kv collection endpoint (ls, set)
	mux.HandleFunc("/api/v2/workspaces/11111111-2222-3333-4444-555555555555/keyvalues", kvHandler)
	// kv individual key endpoint (get, update, delete)
	mux.HandleFunc("/api/v2/keyvalues/", kvHandler)

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

	out, err := runKVCmd(t, srv, "ls", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
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

func TestKVList_WorkspaceUUIDNoProjectRequired(t *testing.T) {
	const wsUUID = "11111111-2222-3333-4444-555555555555"
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/workspaces/"+wsUUID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": wsUUID, "name": "my-workspace"})
	})
	mux.HandleFunc("/api/v2/workspaces/"+wsUUID+"/keyvalues", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "FOO", "value": "bar", "expires_at": nil, "private": false},
		})
	})
	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: mux}}
	srv.Start()
	t.Cleanup(srv.Close)

	// No --project flag — should succeed because workspace is a UUID
	out, err := runKVCmd(t, srv, "ls", "--workspace", wsUUID)
	if err != nil {
		t.Fatalf("command failed without --project: %v", err)
	}
	if !strings.Contains(out, "FOO") {
		t.Errorf("expected FOO in output, got: %s", out)
	}
}

func TestKVList_JSONOutput(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "FOO", "value": "bar", "expires_at": nil, "private": false},
		})
	})

	out, err := runKVCmd(t, srv, "ls", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "-o", "json")
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

func TestKVGet_PrintsRawValue(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"key": "MY_KEY", "value": "hello-world", "expires_at": nil, "private": false})
	})

	out, err := runKVCmd(t, srv, "get", "MY_KEY")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.TrimSpace(out) != "hello-world" {
		t.Errorf("expected raw value 'hello-world', got: %q", out)
	}
}

func TestKVShow_PrintsTable(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"key": "MY_KEY", "value": "hello-world", "expires_at": nil, "private": false, "environment": "prod"})
	})

	out, err := runKVCmd(t, srv, "show", "MY_KEY")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "MY_KEY") {
		t.Errorf("expected KEY column in output, got: %s", out)
	}
	if !strings.Contains(out, "hello-world") {
		t.Errorf("expected VALUE column in output, got: %s", out)
	}
	if !strings.Contains(out, "prod") {
		t.Errorf("expected ENVIRONMENT column in output, got: %s", out)
	}
}

func TestKVShow_JSONOutput(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"key": "MY_KEY", "value": "hello-world", "expires_at": nil, "private": false})
	})

	out, err := runKVCmd(t, srv, "show", "MY_KEY", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, `"key"`) || !strings.Contains(out, `"value"`) {
		t.Errorf("expected JSON object with key/value fields, got: %s", out)
	}
}

func TestKVGet_MasksPrivateByDefault(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"value": "s3cr3t", "expires_at": nil, "private": true})
	})

	out, err := runKVCmd(t, srv, "get", "MY_KEY")
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

	out, err := runKVCmd(t, srv, "get", "MY_KEY", "--reveal")
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
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	_, err := runKVCmd(t, srv, "set", "NEW_KEY", "new-value", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
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

func TestKVSet_WorkspaceUUIDNoProjectRequired(t *testing.T) {
	const wsUUID = "11111111-2222-3333-4444-555555555555"
	var bodyBytes []byte
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/workspaces/"+wsUUID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": wsUUID, "name": "my-workspace"})
	})
	mux.HandleFunc("/api/v2/workspaces/"+wsUUID+"/keyvalues", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})
	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: mux}}
	srv.Start()
	t.Cleanup(srv.Close)

	// No --project flag — should succeed because workspace is a UUID
	_, err = runKVCmd(t, srv, "set", "NEW_KEY", "new-value", "--workspace", wsUUID)
	if err != nil {
		t.Fatalf("command failed without --project: %v", err)
	}
	if !strings.Contains(string(bodyBytes), `"key":"NEW_KEY"`) {
		t.Errorf("expected key in body, got: %s", bodyBytes)
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
	_, _ = runKVCmd(t, srv, "delete", "MY_KEY")
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

	_, err := runKVCmd(t, srv, "delete", "MY_KEY", "--force")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected DELETE to be called with --force")
	}
}

// writeKeyFile writes a hex key to a temp file and returns its path.
func writeKeyFile(t *testing.T, hex string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "kh-enc-key-*")
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	if _, err := f.WriteString(hex); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestKVSet_EncryptsValueWhenKeyProvided(t *testing.T) {
	var bodyBytes []byte
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	keyFile := writeKeyFile(t, strings.Repeat("ab", 32))
	_, err := runKVCmd(t, srv, "set", "MY_KEY", "plaintext",
		"--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555",
		"--encryption-key-file", keyFile,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.Contains(string(bodyBytes), "plaintext") {
		t.Errorf("plaintext must not be sent to API, got: %s", bodyBytes)
	}
	if !strings.Contains(string(bodyBytes), `enc:v1:`) {
		t.Errorf("expected enc:v1: ciphertext in body, got: %s", bodyBytes)
	}
}

func TestKVSet_NoEncryptionWithoutKey(t *testing.T) {
	var bodyBytes []byte
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	_, err := runKVCmd(t, srv, "set", "MY_KEY", "plaintext",
		"--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(string(bodyBytes), `"value":"plaintext"`) {
		t.Errorf("expected plaintext value in body, got: %s", bodyBytes)
	}
}

func TestKVSet_InvalidEncryptionKeyFile(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called with invalid key")
	})

	keyFile := writeKeyFile(t, "tooshort")
	_, err := runKVCmd(t, srv, "set", "MY_KEY", "value",
		"--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555",
		"--encryption-key-file", keyFile,
	)
	if err == nil {
		t.Fatal("expected error for invalid encryption key")
	}
}

func TestKVGet_DecryptsWithMatchingKey(t *testing.T) {
	encKey := [32]byte{}
	for i := range encKey {
		encKey[i] = 0xab
	}
	ciphertext, _ := kvencrypt.Encrypt(encKey, "secret-value")

	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"value": ciphertext, "expires_at": nil, "private": false})
	})

	keyFile := writeKeyFile(t, strings.Repeat("ab", 32))
	out, err := runKVCmd(t, srv, "get", "MY_KEY",
		"--encryption-key-file", keyFile,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "secret-value") {
		t.Errorf("expected decrypted value in output, got: %s", out)
	}
}

func TestKVGet_WarnsWhenEncryptedButNoKey(t *testing.T) {
	encKey := [32]byte{}
	ciphertext, _ := kvencrypt.Encrypt(encKey, "secret-value")

	var stderrBuf bytes.Buffer
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"value": ciphertext, "expires_at": nil, "private": false})
	})

	t.Setenv("KH_ENDPOINT", srv.URL)
	t.Setenv("KH_TOKEN", "test-token")
	cmd := newKVCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(&stderrBuf)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"get", "MY_KEY"})
	_ = cmd.Execute()

	if !strings.Contains(stderrBuf.String(), "encrypted") {
		t.Errorf("expected encryption warning on stderr, got: %s", stderrBuf.String())
	}
}

func TestKVGet_ErrorsWithWrongKey(t *testing.T) {
	encKey := [32]byte{}
	ciphertext, _ := kvencrypt.Encrypt(encKey, "secret-value")

	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"value": ciphertext, "expires_at": nil, "private": false})
	})

	keyFile := writeKeyFile(t, strings.Repeat("cd", 32)) // different key
	_, err := runKVCmd(t, srv, "get", "MY_KEY",
		"--encryption-key-file", keyFile,
	)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
	if !strings.Contains(err.Error(), "decryption failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKVList_ShowsEncryptedLabelWithoutKey(t *testing.T) {
	encKey := [32]byte{}
	ciphertext, _ := kvencrypt.Encrypt(encKey, "secret-value")

	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "MY_KEY", "value": ciphertext, "expires_at": nil, "private": false},
		})
	})

	out, err := runKVCmd(t, srv, "ls", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "[encrypted]") {
		t.Errorf("expected [encrypted] label in output, got: %s", out)
	}
}

func TestKVList_DecryptsWithKey(t *testing.T) {
	encKey := [32]byte{}
	for i := range encKey {
		encKey[i] = 0xab
	}
	ciphertext, _ := kvencrypt.Encrypt(encKey, "decrypted-value")

	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "MY_KEY", "value": ciphertext, "expires_at": nil, "private": false},
		})
	})

	keyFile := writeKeyFile(t, strings.Repeat("ab", 32))
	out, err := runKVCmd(t, srv, "ls",
		"--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555",
		"--encryption-key-file", keyFile,
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "decrypted-value") {
		t.Errorf("expected decrypted value in output, got: %s", out)
	}
}

// writeValueFile writes content to a temp file and returns its path.
func writeValueFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "kh-value-*")
	if err != nil {
		t.Fatalf("create value file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write value file: %v", err)
	}
	f.Close()
	return f.Name()
}

func TestKVSet_ValueFile(t *testing.T) {
	var bodyBytes []byte
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	vf := writeValueFile(t, "value-from-file")
	_, err := runKVCmd(t, srv, "set", "FILE_KEY", "--value-file", vf,
		"--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(string(bodyBytes), `"key":"FILE_KEY"`) {
		t.Errorf("expected key in body, got: %s", bodyBytes)
	}
	if !strings.Contains(string(bodyBytes), `"value":"value-from-file"`) {
		t.Errorf("expected file content as value in body, got: %s", bodyBytes)
	}
}

func TestKVSet_ValueFile_AndArgMutuallyExclusive(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	vf := writeValueFile(t, "from-file")
	_, err := runKVCmd(t, srv, "set", "MY_KEY", "direct-value", "--value-file", vf,
		"--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
	if err == nil {
		t.Fatal("expected error when both value arg and --value-file are provided")
	}
}

func TestKVSet_NoValueReturnsError(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runKVCmd(t, srv, "set", "MY_KEY",
		"--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
	if err == nil {
		t.Fatal("expected error when neither value arg nor --value-file are provided")
	}
}

func TestKVSet_KHWorkspaceEnvVar(t *testing.T) {
	// Regression: KH_WORKSPACE env var must be respected when --workspace flag is omitted.
	const wsUUID = "11111111-2222-3333-4444-555555555555"
	var called bool
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/workspaces/"+wsUUID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": wsUUID, "name": "my-workspace"})
	})
	mux.HandleFunc("/api/v2/workspaces/"+wsUUID+"/keyvalues", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})
	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: mux}}
	srv.Start()
	t.Cleanup(srv.Close)

	t.Setenv("KH_WORKSPACE", wsUUID) // no --workspace flag; should use env var
	_, err = runKVCmd(t, srv, "set", "MY_KEY", "my-value")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !called {
		t.Fatal("expected keyvalues endpoint to be called")
	}
}

func TestKVUpdate_ValueFile(t *testing.T) {
	var bodyBytes []byte
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
	})

	vf := writeValueFile(t, "updated-from-file")
	_, err := runKVCmd(t, srv, "update", "MY_KEY", "--value-file", vf)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(string(bodyBytes), `"value":"updated-from-file"`) {
		t.Errorf("expected file content as value in body, got: %s", bodyBytes)
	}
}

func TestKVUpdate_ValueFile_AndFlagMutuallyExclusive(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	vf := writeValueFile(t, "from-file")
	_, err := runKVCmd(t, srv, "update", "MY_KEY", "--value", "direct", "--value-file", vf)
	if err == nil {
		t.Fatal("expected error when both --value and --value-file are provided")
	}
}

// ── env ───────────────────────────────────────────────────────────────────────

func TestKVEnv_ExportFormat(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "FOO", "value": "bar", "private": false},
			{"key": "SECRET", "value": "s3cr3t", "private": true},
		})
	})

	out, err := runKVCmd(t, srv, "env", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "export FOO='bar'") {
		t.Errorf("expected export FOO='bar', got: %s", out)
	}
	if !strings.Contains(out, "export SECRET='s3cr3t'") {
		t.Errorf("expected export SECRET='s3cr3t', got: %s", out)
	}
}

func TestKVEnv_DotenvFormat(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "FOO", "value": "bar", "private": false},
		})
	})

	out, err := runKVCmd(t, srv, "env", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--format", "dotenv")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.Contains(out, "export ") {
		t.Errorf("dotenv format should not contain 'export', got: %s", out)
	}
	if !strings.Contains(out, "FOO='bar'") {
		t.Errorf("expected FOO='bar', got: %s", out)
	}
}

func TestKVEnv_FilterByEnvironment(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "PROD_KEY", "value": "pval", "private": false, "environment": "prod"},
			{"key": "STG_KEY", "value": "sval", "private": false, "environment": "staging"},
		})
	})

	out, err := runKVCmd(t, srv, "env", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--environment", "prod")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "PROD_KEY") {
		t.Errorf("expected PROD_KEY in output, got: %s", out)
	}
	if strings.Contains(out, "STG_KEY") {
		t.Errorf("staging key should be filtered out, got: %s", out)
	}
}

func TestKVEnv_EscapesSingleQuotes(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "TRICKY", "value": "it's a 'test'", "private": false},
		})
	})

	out, err := runKVCmd(t, srv, "env", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	// Single quotes inside value must be escaped as '\''
	if !strings.Contains(out, `it'\''s a '\''test'\''`) {
		t.Errorf("expected escaped single quotes, got: %s", out)
	}
}

func TestKVEnv_SkipsEncryptedWithoutKey(t *testing.T) {
	var rawKey [32]byte
	for i := range rawKey {
		rawKey[i] = byte(i)
	}
	ciphertext, _ := kvencrypt.Encrypt(rawKey, "secret-value")

	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "ENC_VAR", "value": ciphertext, "private": false},
			{"key": "PLAIN_VAR", "value": "plain", "private": false},
		})
	})

	// No encryption key provided — encrypted key should be skipped with a warning
	errBuf := &bytes.Buffer{}
	t.Setenv("KH_ENDPOINT", srv.URL)
	t.Setenv("KH_TOKEN", "test-token")
	cmd := newKVCmd()
	outBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)
	cmd.SetContext(context.Background())
	cmd.SetArgs([]string{"env", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if strings.Contains(outBuf.String(), "ENC_VAR") {
		t.Errorf("encrypted key should be skipped without decryption key, got: %s", outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "PLAIN_VAR") {
		t.Errorf("plain key should still be printed, got: %s", outBuf.String())
	}
	if !strings.Contains(errBuf.String(), "ENC_VAR") {
		t.Errorf("expected warning about ENC_VAR on stderr, got: %s", errBuf.String())
	}
}

// ── run ───────────────────────────────────────────────────────────────────────

func TestKVRun_InjectsEnvVars(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "MY_TEST_VAR", "value": "hello-from-kh", "private": false},
		})
	})

	t.Setenv("KH_ENDPOINT", srv.URL)
	t.Setenv("KH_TOKEN", "test-token")

	outBuf := &bytes.Buffer{}
	cmd := newKVCmd()
	cmd.SetOut(outBuf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	// Use `sh -c 'echo $MY_TEST_VAR'` to verify the var is in the subprocess env.
	// We can't use syscall.Exec in tests (it would replace the process), so
	// this test verifies the command reaches exec by checking for "command not found"
	// when the binary doesn't exist — the real injection is tested via integration.
	cmd.SetArgs([]string{"run", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--", "nonexistent-binary-xyz"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent binary")
	}
	if !strings.Contains(err.Error(), "nonexistent-binary-xyz") {
		t.Errorf("expected command-not-found error, got: %v", err)
	}
}

func TestKVRun_RequiresCommand(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := runKVCmd(t, srv, "run", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
	if err == nil {
		t.Fatal("expected error when no command provided")
	}
}

func TestKVEnv_PrefixFiltersAndStrips(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "KH_ENV_DATABASE_URL", "value": "postgres://localhost/db", "private": false},
			{"key": "KH_ENV_API_KEY", "value": "secret123", "private": true},
			{"key": "INTERNAL_ONLY", "value": "should-be-excluded", "private": false},
		})
	})

	out, err := runKVCmd(t, srv, "env", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555", "--prefix", "KH_ENV_")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	// Prefix should be stripped
	if !strings.Contains(out, "export DATABASE_URL='postgres://localhost/db'") {
		t.Errorf("expected DATABASE_URL without prefix, got: %s", out)
	}
	if !strings.Contains(out, "export API_KEY='secret123'") {
		t.Errorf("expected API_KEY without prefix, got: %s", out)
	}
	// KH_ENV_ prefixed key name must not appear
	if strings.Contains(out, "KH_ENV_") {
		t.Errorf("prefix should be stripped from output, got: %s", out)
	}
	// Non-prefixed key must be excluded
	if strings.Contains(out, "INTERNAL_ONLY") {
		t.Errorf("non-prefixed key should be excluded, got: %s", out)
	}
}

func TestKVEnv_NoPrefixIncludesAll(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"key": "KH_ENV_FOO", "value": "v1", "private": false},
			{"key": "OTHER", "value": "v2", "private": false},
		})
	})

	out, err := runKVCmd(t, srv, "env", "--project", "proj-uuid", "--workspace", "11111111-2222-3333-4444-555555555555")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "export KH_ENV_FOO='v1'") {
		t.Errorf("expected KH_ENV_FOO unchanged, got: %s", out)
	}
	if !strings.Contains(out, "export OTHER='v2'") {
		t.Errorf("expected OTHER in output, got: %s", out)
	}
}
