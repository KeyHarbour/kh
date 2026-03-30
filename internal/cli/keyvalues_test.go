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
	mux.HandleFunc("/projects/proj-uuid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "proj-uuid", "name": "my-project"})
	})
	// workspace resolution (list, for name-based lookup)
	mux.HandleFunc("/projects/proj-uuid/workspaces", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{{"uuid": "ws-uuid", "name": "my-workspace"}})
	})
	// workspace detail
	mux.HandleFunc("/workspaces/ws-uuid", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "ws-uuid", "name": "my-workspace"})
	})
	// kv collection endpoint (ls, set)
	mux.HandleFunc("/workspaces/ws-uuid/keyvalues", kvHandler)
	// kv individual key endpoint (get, update, delete)
	mux.HandleFunc("/keyvalues/", kvHandler)

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

	_, err := runKVCmd(t, srv, "set", "NEW_KEY", "new-value", "--project", "proj-uuid", "--workspace", "ws-uuid")
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

// testEncKey is a valid 64-char hex key for encryption tests.
const testEncKey = "a3f12e849c47b011235678abcdef01234567" +
	"89abcdef01234567" + "89abcdef0123456"

func TestKVSet_EncryptsValueWhenKeyProvided(t *testing.T) {
	var bodyBytes []byte
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	_, err := runKVCmd(t, srv, "set", "MY_KEY", "plaintext",
		"--project", "proj-uuid", "--workspace", "ws-uuid",
		"--encryption-key", strings.Repeat("ab", 32),
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
		"--project", "proj-uuid", "--workspace", "ws-uuid",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(string(bodyBytes), `"value":"plaintext"`) {
		t.Errorf("expected plaintext value in body, got: %s", bodyBytes)
	}
}

func TestKVSet_InvalidEncryptionKey(t *testing.T) {
	srv := newKVTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called with invalid key")
	})

	_, err := runKVCmd(t, srv, "set", "MY_KEY", "value",
		"--project", "proj-uuid", "--workspace", "ws-uuid",
		"--encryption-key", "tooshort",
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

	out, err := runKVCmd(t, srv, "get", "MY_KEY",
		"--encryption-key", strings.Repeat("ab", 32),
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

	_, err := runKVCmd(t, srv, "get", "MY_KEY",
		"--encryption-key", strings.Repeat("cd", 32), // different key
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

	out, err := runKVCmd(t, srv, "ls", "--project", "proj-uuid", "--workspace", "ws-uuid")
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

	out, err := runKVCmd(t, srv, "ls",
		"--project", "proj-uuid", "--workspace", "ws-uuid",
		"--encryption-key", strings.Repeat("ab", 32),
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "decrypted-value") {
		t.Errorf("expected decrypted value in output, got: %s", out)
	}
}
