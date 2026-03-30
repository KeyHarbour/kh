//go:build integration

package integrationtests

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSnapshot captures the observable state of a live KeyHarbour backend
// before a planned change (deployment, migration, config update). The
// snapshot is stored as timestamped JSON files and serves as the ground
// truth for TestRegression.
//
// Required env: KH_ENDPOINT, KH_TOKEN, KH_PROJECT
// Mode guard:   KH_TEST_MODE=snapshot
// Optional env: KH_SNAPSHOT_DIR (default: ./testdata/snapshots)
func TestSnapshot(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "snapshot" {
		t.Skip("set KH_TEST_MODE=snapshot to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN", "KH_PROJECT")

	kh := khBin(t)
	project := os.Getenv("KH_PROJECT")
	dir := snapshotDir(t)

	// 1. Auth identity (text output — whoami has no JSON flag).
	whoamiOut := runOK(t, kh, "whoami")
	if err := os.WriteFile(filepath.Join(dir, "whoami.txt"), whoamiOut, 0o644); err != nil {
		t.Fatalf("write whoami.txt: %v", err)
	}

	// 2. HTTP-backend states list (populated when Terraform pushes via HTTP backend).
	captureJSON(t, kh, dir, "states.json", "state", "ls", "-o", "json")

	// 3. Per-state raw content for integrity hashing during regression.
	var states []map[string]any
	stateData := loadJSON[[]map[string]any](t, filepath.Join(dir, "states.json"))
	states = stateData

	statesDir := filepath.Join(dir, "states")
	if err := os.MkdirAll(statesDir, 0o755); err != nil {
		t.Fatalf("mkdir states: %v", err)
	}
	for _, s := range states {
		id, ok := s["id"].(string)
		if !ok || id == "" {
			continue
		}
		t.Run("state/"+id, func(t *testing.T) {
			out := runOK(t, kh, "state", "show", id, "--raw")
			if err := os.WriteFile(filepath.Join(statesDir, id+".json"), out, 0o644); err != nil {
				t.Errorf("write state %s: %v", id, err)
			}
		})
	}

	// 4. Workspaces and their statefiles (populated by kh sync / statefiles push).
	captureJSON(t, kh, dir, "workspaces.json", "workspaces", "ls", "--project", project, "-o", "json")

	var workspaces []map[string]any
	wsData := loadJSON[[]map[string]any](t, filepath.Join(dir, "workspaces.json"))
	workspaces = wsData

	sfDir := filepath.Join(dir, "statefiles")
	if err := os.MkdirAll(sfDir, 0o755); err != nil {
		t.Fatalf("mkdir statefiles: %v", err)
	}
	for _, ws := range workspaces {
		wsUUID, ok := ws["uuid"].(string)
		if !ok || wsUUID == "" {
			continue
		}
		wsName, _ := ws["name"].(string)
		t.Run("statefiles/"+wsName, func(t *testing.T) {
			// Capture statefile list for this workspace.
			captureJSON(t, kh, sfDir, wsUUID+".json",
				"statefiles", "ls",
				"--project", project,
				"--workspace", wsUUID,
				"-o", "json",
			)
		})
	}

	// 5. Key/value pairs per workspace.
	kvDir := filepath.Join(dir, "keyvalues")
	if err := os.MkdirAll(kvDir, 0o755); err != nil {
		t.Fatalf("mkdir keyvalues: %v", err)
	}
	for _, ws := range workspaces {
		wsUUID, ok := ws["uuid"].(string)
		if !ok || wsUUID == "" {
			continue
		}
		wsName, _ := ws["name"].(string)
		t.Run("keyvalues/"+wsName, func(t *testing.T) {
			captureJSON(t, kh, kvDir, wsUUID+".json",
				"kv", "ls",
				"--project", project,
				"--workspace", wsUUID,
				"-o", "json",
			)
		})
	}

	// 6. Project detail.
	captureJSON(t, kh, dir, "project.json", "projects", "show", project)

	// 7. Per-workspace details (name, description).
	wsDetailsDir := filepath.Join(dir, "workspace_details")
	if err := os.MkdirAll(wsDetailsDir, 0o755); err != nil {
		t.Fatalf("mkdir workspace_details: %v", err)
	}
	for _, ws := range workspaces {
		wsUUID, ok := ws["uuid"].(string)
		if !ok || wsUUID == "" {
			continue
		}
		wsName, _ := ws["name"].(string)
		t.Run("workspace_details/"+wsName, func(t *testing.T) {
			captureJSON(t, kh, wsDetailsDir, wsUUID+".json",
				"workspaces", "show", wsUUID,
				"--project", project,
			)
		})
	}

	// 8. License records (org-level, no project/workspace scope).
	captureJSON(t, kh, dir, "licenses.json", "license", "ls", "-o", "json")

	t.Logf("snapshot written to %s (%d states, %d workspaces)", dir, len(states), len(workspaces))
}
