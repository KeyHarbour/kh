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

// TestWorkspaceDiagnostics exercises read-only workspace operations against a live backend.
//
// Required env: KH_ENDPOINT, KH_TOKEN, KH_PROJECT
// Mode guard:   KH_TEST_MODE=diagnostics
func TestWorkspaceDiagnostics(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "diagnostics" {
		t.Skip("set KH_TEST_MODE=diagnostics to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN", "KH_PROJECT")

	kh := khBin(t)
	project := os.Getenv("KH_PROJECT")

	t.Run("ListWorkspaces", func(t *testing.T) {
		out := timedRun(t, 10*time.Second, kh,
			"workspaces", "ls",
			"--project", project,
			"-o", "json",
		)
		var items []map[string]any
		if err := json.Unmarshal(out, &items); err != nil {
			t.Fatalf("workspaces ls returned invalid JSON: %v\noutput: %s", err, out)
		}
		t.Logf("found %d workspace(s)", len(items))
	})
}

// TestWorkspaceRoundTrip exercises the full create → show → update → delete
// lifecycle for workspaces. It creates a uniquely named workspace, verifies it,
// updates its description, then deletes it. Safe to run against a real backend
// as it cleans up after itself.
//
// Required env: KH_ENDPOINT, KH_TOKEN, KH_PROJECT
// Mode guard:   KH_TEST_MODE=diagnostics
func TestWorkspaceRoundTrip(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "diagnostics" {
		t.Skip("set KH_TEST_MODE=diagnostics to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN", "KH_PROJECT")

	kh := khBin(t)
	project := os.Getenv("KH_PROJECT")

	// Use a timestamped unique name (alphanumeric only — CLI requirement).
	name := fmt.Sprintf("khtest%d", time.Now().UnixMilli())

	// Always clean up, even on test failure.
	t.Cleanup(func() {
		if out, err := runCmd(t, kh,
			"workspaces", "delete", name,
			"--project", project,
			"--force",
		); err != nil {
			t.Logf("cleanup delete failed (may already be deleted): %v\n%s", err, out)
		}
	})

	t.Run("Create", func(t *testing.T) {
		out := runOK(t, kh,
			"workspaces", "create", name,
			"--project", project,
			"--description", "cli integration test",
		)
		if !strings.Contains(string(out), name) {
			t.Fatalf("expected workspace name in output, got: %s", out)
		}
		t.Logf("created workspace %q", name)
	})

	t.Run("Show", func(t *testing.T) {
		out := timedRun(t, 10*time.Second, kh,
			"workspaces", "show", name,
			"--project", project,
		)
		var payload map[string]any
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("workspaces show returned invalid JSON: %v\noutput: %s", err, out)
		}
		ws, _ := payload["workspace"].(map[string]any)
		if ws == nil {
			t.Fatalf("expected workspace key in JSON, got: %s", out)
		}
		if ws["name"] != name {
			t.Fatalf("expected name=%q, got: %v", name, ws["name"])
		}
		t.Logf("verified workspace %q exists (uuid: %v)", name, ws["uuid"])
	})

	t.Run("Update", func(t *testing.T) {
		runOK(t, kh,
			"workspaces", "update", name,
			"--project", project,
			"--description", "updated description",
		)

		out := timedRun(t, 10*time.Second, kh,
			"workspaces", "show", name,
			"--project", project,
		)
		var payload map[string]any
		if err := json.Unmarshal(out, &payload); err != nil {
			t.Fatalf("workspaces show after update invalid JSON: %v\noutput: %s", err, out)
		}
		ws, _ := payload["workspace"].(map[string]any)
		if ws == nil {
			t.Fatalf("expected workspace key in JSON, got: %s", out)
		}
		if ws["description"] != "updated description" {
			t.Fatalf("expected description to be updated, got: %v", ws["description"])
		}
		t.Logf("updated workspace %q description", name)
	})

	t.Run("Delete", func(t *testing.T) {
		out := runOK(t, kh,
			"workspaces", "delete", name,
			"--project", project,
			"--force",
		)
		if !strings.Contains(string(out), name) {
			t.Fatalf("expected workspace name in delete output, got: %s", out)
		}
		t.Logf("deleted workspace %q", name)
	})
}
