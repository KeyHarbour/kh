//go:build integration

package integrationtests

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

	t.Run("LicenseListNoDeletions", func(t *testing.T) {
		snapPath := filepath.Join(snapDir, "licenses.json")
		if _, err := os.Stat(snapPath); os.IsNotExist(err) {
			t.Skip("no licenses.json in snapshot — re-run TestSnapshot to capture licenses")
		}
		snapLicenses := loadJSON[[]map[string]any](t, snapPath)

		liveOut := runOK(t, kh, "license", "ls", "-o", "json")
		var liveLicenses []map[string]any
		if err := json.Unmarshal(liveOut, &liveLicenses); err != nil {
			t.Fatalf("license ls returned invalid JSON: %v\noutput: %s", err, liveOut)
		}

		liveByUUID := make(map[string]bool, len(liveLicenses))
		for _, l := range liveLicenses {
			if uuid, _ := l["uuid"].(string); uuid != "" {
				liveByUUID[uuid] = true
			}
		}
		for _, l := range snapLicenses {
			uuid, _ := l["uuid"].(string)
			name, _ := l["name"].(string)
			if uuid != "" && !liveByUUID[uuid] {
				t.Errorf("license %q (%s) was present at snapshot time but is missing from live system", name, uuid)
			}
		}
		t.Logf("snapshot: %d licenses, live: %d licenses", len(snapLicenses), len(liveLicenses))
	})

	t.Run("ProjectDetail", func(t *testing.T) {
		snapPath := filepath.Join(snapDir, "project.json")
		if _, err := os.Stat(snapPath); os.IsNotExist(err) {
			t.Skip("no project.json in snapshot — re-run TestSnapshot to capture project detail")
		}
		snap := loadJSON[map[string]any](t, snapPath)

		liveOut := runOK(t, kh, "projects", "show", project)
		var live map[string]any
		if err := json.Unmarshal(liveOut, &live); err != nil {
			t.Fatalf("projects show returned invalid JSON: %v\noutput: %s", err, liveOut)
		}

		assertField(t, snap, live, "name")

		snapEnvs, _ := snap["environment_names"].([]any)
		liveEnvs, _ := live["environment_names"].([]any)
		if len(liveEnvs) < len(snapEnvs) {
			t.Errorf("environment_names shrank: snapshot=%d live=%d", len(snapEnvs), len(liveEnvs))
		}
		t.Logf("project %q: name=%q environments=%d", project, live["name"], len(liveEnvs))
	})

	t.Run("WorkspaceDetailStability", func(t *testing.T) {
		wsDetailsDir := filepath.Join(snapDir, "workspace_details")
		if _, err := os.Stat(wsDetailsDir); os.IsNotExist(err) {
			t.Skip("no workspace_details snapshots found — re-run TestSnapshot to capture workspace details")
		}

		entries, err := os.ReadDir(wsDetailsDir)
		if err != nil {
			t.Fatalf("read workspace_details dir: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			wsUUID := strings.TrimSuffix(e.Name(), ".json")
			t.Run(wsUUID, func(t *testing.T) {
				snapPayload := loadJSON[map[string]any](t, filepath.Join(wsDetailsDir, e.Name()))
				snapWS, _ := snapPayload["workspace"].(map[string]any)
				if snapWS == nil {
					t.Skip("no workspace key in snapshot file")
				}

				liveOut := runOK(t, kh, "workspaces", "show", wsUUID, "--project", project)
				var livePayload map[string]any
				if err := json.Unmarshal(liveOut, &livePayload); err != nil {
					t.Fatalf("workspaces show returned invalid JSON: %v\noutput: %s", err, liveOut)
				}
				liveWS, _ := livePayload["workspace"].(map[string]any)
				if liveWS == nil {
					t.Fatalf("no workspace key in live response: %s", liveOut)
				}

				assertField(t, snapWS, liveWS, "name")
				assertField(t, snapWS, liveWS, "description")
				t.Logf("workspace %s: name=%q description=%q", wsUUID, liveWS["name"], liveWS["description"])
			})
		}
	})

	t.Run("KeyValueContentIntegrity", func(t *testing.T) {
		kvSnapDir := filepath.Join(snapDir, "keyvalues")
		if _, err := os.Stat(kvSnapDir); os.IsNotExist(err) {
			t.Skip("no keyvalues snapshots found — re-run TestSnapshot to capture key/values")
		}

		entries, err := os.ReadDir(kvSnapDir)
		if err != nil {
			t.Fatalf("read keyvalues dir: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			wsUUID := strings.TrimSuffix(e.Name(), ".json")
			t.Run(wsUUID, func(t *testing.T) {
				snapKVs := loadJSON[[]map[string]any](t, filepath.Join(kvSnapDir, e.Name()))

				liveOut := runOK(t, kh, "kv", "ls",
					"--project", project,
					"--workspace", wsUUID,
					"-o", "json",
				)
				var liveKVs []map[string]any
				if err := json.Unmarshal(liveOut, &liveKVs); err != nil {
					t.Fatalf("kv ls returned invalid JSON: %v\noutput: %s", err, liveOut)
				}

				liveByKey := make(map[string]map[string]any, len(liveKVs))
				for _, kv := range liveKVs {
					if k, _ := kv["key"].(string); k != "" {
						liveByKey[k] = kv
					}
				}
				for _, snapKV := range snapKVs {
					k, _ := snapKV["key"].(string)
					if k == "" {
						continue
					}
					liveKV, ok := liveByKey[k]
					if !ok {
						t.Errorf("key %q was present at snapshot time but is missing from live system", k)
						continue
					}
					// Skip value comparison for private or encrypted values.
					private, _ := snapKV["private"].(bool)
					snapVal, _ := snapKV["value"].(string)
					if !private && !strings.HasPrefix(snapVal, "enc:v1:") {
						assertField(t, snapKV, liveKV, "value")
					}
				}
				t.Logf("workspace %s: %d keys checked (snapshot had %d)", wsUUID, len(liveKVs), len(snapKVs))
			})
		}
	})

	t.Run("KeyValueCountPerWorkspace", func(t *testing.T) {
		kvSnapDir := filepath.Join(snapDir, "keyvalues")
		if _, err := os.Stat(kvSnapDir); os.IsNotExist(err) {
			t.Skip("no keyvalues snapshots found — re-run TestSnapshot to capture key/values")
		}

		entries, err := os.ReadDir(kvSnapDir)
		if err != nil {
			t.Fatalf("read keyvalues dir: %v", err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			wsUUID := strings.TrimSuffix(e.Name(), ".json")
			t.Run(wsUUID, func(t *testing.T) {
				snapKVs := loadJSON[[]map[string]any](t, filepath.Join(kvSnapDir, e.Name()))

				liveOut := runOK(t, kh, "kv", "ls",
					"--project", project,
					"--workspace", wsUUID,
					"-o", "json",
				)
				var liveKVs []map[string]any
				if err := json.Unmarshal(liveOut, &liveKVs); err != nil {
					t.Fatalf("kv ls returned invalid JSON: %v\noutput: %s", err, liveOut)
				}

				// Build a set of live keys for deletion detection.
				liveKeys := make(map[string]bool, len(liveKVs))
				for _, kv := range liveKVs {
					if k, _ := kv["key"].(string); k != "" {
						liveKeys[k] = true
					}
				}
				for _, kv := range snapKVs {
					k, _ := kv["key"].(string)
					if k != "" && !liveKeys[k] {
						t.Errorf("workspace %s: key %q was present at snapshot time but is missing from live system", wsUUID, k)
					}
				}
				t.Logf("workspace %s: %d keys (snapshot had %d)", wsUUID, len(liveKVs), len(snapKVs))
			})
		}
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

	t.Run("KVValueFileRoundTripAppKeyHarbour", func(t *testing.T) {
		endpoint := os.Getenv("KH_ENDPOINT")
		if !strings.Contains(endpoint, "app.keyharbour.ca") {
			t.Skipf("KH_ENDPOINT is %q; this regression runs only against app.keyharbour.ca", endpoint)
		}
		requireEnv(t, "KH_WORKSPACE")

		workspace := os.Getenv("KH_WORKSPACE")
		env := os.Getenv("KH_ENV")
		key := fmt.Sprintf("KH_REGRESSION_FILE_%d", time.Now().UnixMilli())

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

		tmp := t.TempDir()
		setFile := filepath.Join(tmp, "set.txt")
		updateFile := filepath.Join(tmp, "update.txt")
		getFile := filepath.Join(tmp, "get.txt")

		setValue := []byte("value-from-file-regression-v1\n")
		updateValue := []byte("value-from-file-regression-v2\n")
		if err := os.WriteFile(setFile, setValue, 0o600); err != nil {
			t.Fatalf("write set file: %v", err)
		}
		if err := os.WriteFile(updateFile, updateValue, 0o600); err != nil {
			t.Fatalf("write update file: %v", err)
		}

		setArgs := []string{"kv", "set", key, "--value-file", setFile, "--project", project, "--workspace", workspace}
		if env != "" {
			setArgs = append(setArgs, "--env", env)
		}
		runOK(t, kh, setArgs...)

		runOK(t, kh,
			"kv", "get", key,
			"--project", project,
			"--workspace", workspace,
			"--output-file", getFile,
		)
		gotSetValue, err := os.ReadFile(getFile)
		if err != nil {
			t.Fatalf("read get output after set: %v", err)
		}
		if string(gotSetValue) != string(setValue) {
			t.Fatalf("value mismatch after set via --value-file: got=%q want=%q", string(gotSetValue), string(setValue))
		}

		runOK(t, kh,
			"kv", "update", key,
			"--value-file", updateFile,
			"--project", project,
			"--workspace", workspace,
		)

		runOK(t, kh,
			"kv", "get", key,
			"--project", project,
			"--workspace", workspace,
			"--output-file", getFile,
		)
		gotUpdatedValue, err := os.ReadFile(getFile)
		if err != nil {
			t.Fatalf("read get output after update: %v", err)
		}
		if string(gotUpdatedValue) != string(updateValue) {
			t.Fatalf("value mismatch after update via --value-file: got=%q want=%q", string(gotUpdatedValue), string(updateValue))
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
