//go:build integration

package integrationtests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegression compares the live system against a stored snapshot produced
// by TestSnapshot. It verifies that no states or workspaces have disappeared
// and that the statefile inventory per workspace is stable.
//
// Required env: KH_ENDPOINT, KH_TOKEN, KH_PROJECT, KH_SNAPSHOT_DIR
// Mode guard:   KH_TEST_MODE=regression
func TestRegression(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "regression" {
		t.Skip("set KH_TEST_MODE=regression to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN", "KH_PROJECT")

	snapDir := os.Getenv("KH_SNAPSHOT_DIR")
	if snapDir == "" {
		t.Skip("set KH_SNAPSHOT_DIR to the snapshot directory to compare against")
	}

	kh := khBin(t)
	project := os.Getenv("KH_PROJECT")

	t.Run("Authentication", func(t *testing.T) {
		snap, err := os.ReadFile(filepath.Join(snapDir, "whoami.txt"))
		if err != nil {
			t.Skipf("no whoami.txt in snapshot: %v", err)
		}
		live := runOK(t, kh, "whoami")
		snapOrg := extractLine(string(snap), "org:")
		liveOrg := extractLine(string(live), "org:")
		if snapOrg != liveOrg {
			t.Errorf("org changed: snapshot=%q live=%q", snapOrg, liveOrg)
		}
	})

	t.Run("StateListNoDeletions", func(t *testing.T) {
		snapStates := loadJSON[[]map[string]any](t, filepath.Join(snapDir, "states.json"))

		liveOut := runOK(t, kh, "state", "ls", "-o", "json")
		var liveStates []map[string]any
		if err := json.Unmarshal(liveOut, &liveStates); err != nil {
			t.Fatalf("live state ls returned invalid JSON: %v\noutput: %s", err, liveOut)
		}

		liveIDs := idSet(liveStates)
		for _, s := range snapStates {
			id, _ := s["id"].(string)
			if id != "" && !liveIDs[id] {
				t.Errorf("state %s was present at snapshot time but is missing from live system", id)
			}
		}
		t.Logf("snapshot: %d states, live: %d states", len(snapStates), len(liveStates))
	})

	t.Run("StateContentIntegrity", func(t *testing.T) {
		statesDir := filepath.Join(snapDir, "states")
		entries, err := os.ReadDir(statesDir)
		if os.IsNotExist(err) {
			t.Skip("no per-state snapshots found")
		}
		if err != nil {
			t.Fatalf("read states dir: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			stateID := strings.TrimSuffix(e.Name(), ".json")
			t.Run(stateID, func(t *testing.T) {
				snapHash := sha256File(t, filepath.Join(statesDir, e.Name()))
				liveOut := runOK(t, kh, "state", "show", stateID, "--raw")
				liveHash := sha256Hex(liveOut)
				if snapHash != liveHash {
					t.Errorf("content changed: snapshot=%s live=%s", snapHash, liveHash)
				}
			})
		}
	})

	t.Run("WorkspaceListNoDeletions", func(t *testing.T) {
		snapPath := filepath.Join(snapDir, "workspaces.json")
		if _, err := os.Stat(snapPath); os.IsNotExist(err) {
			t.Skip("no workspaces.json in snapshot — re-run TestSnapshot to capture workspaces")
		}
		snapWorkspaces := loadJSON[[]map[string]any](t, snapPath)

		liveOut := runOK(t, kh, "workspaces", "ls", "--project", project, "-o", "json")
		var liveWorkspaces []map[string]any
		if err := json.Unmarshal(liveOut, &liveWorkspaces); err != nil {
			t.Fatalf("live workspaces ls returned invalid JSON: %v\noutput: %s", err, liveOut)
		}

		liveByUUID := make(map[string]bool, len(liveWorkspaces))
		for _, ws := range liveWorkspaces {
			if uuid, _ := ws["uuid"].(string); uuid != "" {
				liveByUUID[uuid] = true
			}
		}
		for _, ws := range snapWorkspaces {
			uuid, _ := ws["uuid"].(string)
			name, _ := ws["name"].(string)
			if uuid != "" && !liveByUUID[uuid] {
				t.Errorf("workspace %q (%s) was present at snapshot time but is missing from live system", name, uuid)
			}
		}
		t.Logf("snapshot: %d workspaces, live: %d workspaces", len(snapWorkspaces), len(liveWorkspaces))
	})

	t.Run("StatfileCountPerWorkspace", func(t *testing.T) {
		sfSnapDir := filepath.Join(snapDir, "statefiles")
		if _, err := os.Stat(sfSnapDir); os.IsNotExist(err) {
			t.Skip("no statefiles snapshots found — re-run TestSnapshot to capture statefiles")
		}

		entries, err := os.ReadDir(sfSnapDir)
		if err != nil {
			t.Fatalf("read statefiles dir: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			wsUUID := strings.TrimSuffix(e.Name(), ".json")
			t.Run(wsUUID, func(t *testing.T) {
				snapFiles := loadJSON[[]map[string]any](t, filepath.Join(sfSnapDir, e.Name()))

				liveOut := runOK(t, kh, "statefiles", "ls",
					"--project", project,
					"--workspace", wsUUID,
					"-o", "json",
				)
				var liveFiles []map[string]any
				if err := json.Unmarshal(liveOut, &liveFiles); err != nil {
					t.Fatalf("statefiles ls returned invalid JSON: %v\noutput: %s", err, liveOut)
				}

				if len(liveFiles) < len(snapFiles) {
					t.Errorf("workspace %s: statefile count decreased from %d to %d",
						wsUUID, len(snapFiles), len(liveFiles))
				} else {
					t.Logf("workspace %s: %d statefiles (snapshot had %d)",
						wsUUID, len(liveFiles), len(snapFiles))
				}
			})
		}
	})
}

// extractLine returns the first line in text that starts with prefix.
func extractLine(text, prefix string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}
