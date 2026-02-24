//go:build integration

package integrationtests

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestKVDiagnostics exercises read-only key/value operations against a live backend.
//
// Required env: KH_ENDPOINT, KH_TOKEN, KH_PROJECT, KH_WORKSPACE
// Mode guard:   KH_TEST_MODE=diagnostics
func TestKVDiagnostics(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "diagnostics" {
		t.Skip("set KH_TEST_MODE=diagnostics to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN", "KH_PROJECT", "KH_WORKSPACE")

	kh := khBin(t)
	project := os.Getenv("KH_PROJECT")
	workspace := os.Getenv("KH_WORKSPACE")

	t.Run("ListKeyValues", func(t *testing.T) {
		out := timedRun(t, 10*time.Second, kh,
			"kv", "ls",
			"--project", project,
			"--workspace", workspace,
			"-o", "json",
		)
		var items []map[string]any
		if err := json.Unmarshal(out, &items); err != nil {
			t.Fatalf("kv ls returned invalid JSON: %v\noutput: %s", err, out)
		}
		t.Logf("found %d key/value(s)", len(items))
	})

	t.Run("ListKeyValues_WithEnv", func(t *testing.T) {
		env := os.Getenv("KH_ENV")
		if env == "" {
			t.Skip("set KH_ENV to test environment-filtered listing")
		}
		out := timedRun(t, 10*time.Second, kh,
			"kv", "ls",
			"--project", project,
			"--workspace", workspace,
			"--env", env,
			"-o", "json",
		)
		var items []map[string]any
		if err := json.Unmarshal(out, &items); err != nil {
			t.Fatalf("kv ls --env returned invalid JSON: %v\noutput: %s", err, out)
		}
		t.Logf("found %d key/value(s) in env %q", len(items), env)
	})
}

// TestKVRoundTrip exercises the full CRUD lifecycle for key/value pairs.
// It creates a unique key, reads it, updates it, then deletes it. Safe to run
// against a real backend as it cleans up after itself.
//
// Required env: KH_ENDPOINT, KH_TOKEN, KH_PROJECT, KH_WORKSPACE, KH_ENV
// Mode guard:   KH_TEST_MODE=diagnostics
func TestKVRoundTrip(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "diagnostics" {
		t.Skip("set KH_TEST_MODE=diagnostics to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN", "KH_PROJECT", "KH_WORKSPACE", "KH_ENV")

	kh := khBin(t)
	project := os.Getenv("KH_PROJECT")
	workspace := os.Getenv("KH_WORKSPACE")
	env := os.Getenv("KH_ENV")

	// Use a timestamped unique key to avoid collisions between test runs.
	key := fmt.Sprintf("KH_CLI_TEST_%d", time.Now().UnixMilli())

	// Always clean up, even on test failure.
	t.Cleanup(func() {
		if out, err := runCmd(t, kh,
			"kv", "delete", key,
			"--project", project,
			"--workspace", workspace,
			"--force",
		); err != nil {
			t.Logf("cleanup delete failed (may already be deleted): %v\n%s", err, out)
		}
	})

	t.Run("Set", func(t *testing.T) {
		out := runOK(t, kh,
			"kv", "set", key, "initial-value",
			"--project", project,
			"--workspace", workspace,
			"--env", env,
		)
		if !strings.Contains(string(out), key) {
			t.Fatalf("expected key name in output, got: %s", out)
		}
		t.Logf("created key %q", key)
	})

	t.Run("Get", func(t *testing.T) {
		out := timedRun(t, 10*time.Second, kh,
			"kv", "get", key,
			"--project", project,
			"--workspace", workspace,
			"-o", "json",
		)
		var kv map[string]any
		if err := json.Unmarshal(out, &kv); err != nil {
			t.Fatalf("kv get returned invalid JSON: %v\noutput: %s", err, out)
		}
		if kv["value"] != "initial-value" {
			t.Fatalf("expected value=initial-value, got: %v", kv["value"])
		}
		t.Logf("read back key %q: value=%v", key, kv["value"])
	})

	t.Run("Update", func(t *testing.T) {
		runOK(t, kh,
			"kv", "update", key,
			"--value", "updated-value",
			"--project", project,
			"--workspace", workspace,
		)

		out := timedRun(t, 10*time.Second, kh,
			"kv", "get", key,
			"--project", project,
			"--workspace", workspace,
			"-o", "json",
		)
		var kv map[string]any
		if err := json.Unmarshal(out, &kv); err != nil {
			t.Fatalf("kv get after update invalid JSON: %v\noutput: %s", err, out)
		}
		if kv["value"] != "updated-value" {
			t.Fatalf("expected value=updated-value after update, got: %v", kv["value"])
		}
		t.Logf("updated key %q: value=%v", key, kv["value"])
	})

	t.Run("ListContainsKey", func(t *testing.T) {
		out := timedRun(t, 10*time.Second, kh,
			"kv", "ls",
			"--project", project,
			"--workspace", workspace,
			"--env", env,
			"-o", "json",
		)
		if !strings.Contains(string(out), key) {
			t.Fatalf("expected key %q in ls output, got: %s", key, out)
		}
		t.Logf("key %q visible in ls output", key)
	})

	t.Run("Delete", func(t *testing.T) {
		out := runOK(t, kh,
			"kv", "delete", key,
			"--project", project,
			"--workspace", workspace,
			"--force",
		)
		if !strings.Contains(string(out), key) {
			t.Fatalf("expected key name in delete output, got: %s", out)
		}
		t.Logf("deleted key %q", key)
	})
}
