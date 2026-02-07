package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"kh/internal/khclient"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSyncCmd_Local_Success(t *testing.T) {
	// 1. Create a dummy terraform state file
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "terraform.tfstate")
	stateContent := `{"version": 4, "terraform_version": "1.5.0", "serial": 1, "lineage": "abc", "outputs": {}}`
	if err := os.WriteFile(stateFile, []byte(stateContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// 2. Mock KeyHarbour API
	projectUUID := "a1b2c3d4-a1b2-c3d4-e5f6-a1b2c3d4e5f6"
	workspaceUUID := "f1e2d3c4-b5a6-7890-1234-567890abcdef"

	mux := http.NewServeMux()

	// GET Project
	mux.HandleFunc("/v1/projects/"+projectUUID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(khclient.Project{
			UUID: projectUUID,
			Name: "my-project",
		})
	})

	// Resolve Workspace (GET workspaces list - simulating resolve by name)
	// The client might use ListWorkspaces if reference is a name.
	mux.HandleFunc(fmt.Sprintf("/v1/projects/%s/workspaces", projectUUID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]khclient.Workspace{{
			UUID: workspaceUUID,
			Name: "prod",
		}})
	})

	// Create Statefile
	// POST /v1/projects/{pid}/workspaces/{wid}/statefiles?env=default
	uploadPath := fmt.Sprintf("/v1/projects/%s/workspaces/%s/statefiles", projectUUID, workspaceUUID)
	mux.HandleFunc(uploadPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}

		var req khclient.CreateStatefileRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("failed to unmarshal upload request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if req.Content != stateContent {
			t.Errorf("expected content %q, got %q", stateContent, req.Content)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status": "created"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 3. Set Environment Variables to point to mock server
	t.Setenv("KH_ENDPOINT", srv.URL)
	t.Setenv("KH_TOKEN", "dummy-token")

	// 4. Run Command
	cmd := newSyncCmd()
	cmd.SetOut(io.Discard)
	cmd.SetContext(context.Background())

	// Flags
	args := []string{
		"--from=local",
		"--path=" + stateFile,
		"--project=" + projectUUID,
		"--workspace=prod",
	}
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync command failed: %v", err)
	}
}

func TestSyncCmd_TFC_Success(t *testing.T) {
	// 1. Mock TFC API
	tfcOrg := "my-org"
	tfcWorkspace := "my-ws"
	tfcWorkspaceID := "ws-12345"
	stateContent := `{"version": 4, "terraform_version": "1.5.0", "serial": 5, "lineage": "xyz", "outputs": {}}`

	tfcMux := http.NewServeMux()

	// 1a. Get Workspace ID
	// GET /api/v2/organizations/{org}/workspaces/{workspace}
	tfcMux.HandleFunc(fmt.Sprintf("/api/v2/organizations/%s/workspaces/%s", tfcOrg, tfcWorkspace), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id": tfcWorkspaceID,
			},
		})
	})

	// 1b. Get Current State Version Download URL
	// GET /api/v2/workspaces/{id}/current-state-version
	tfcMux.HandleFunc(fmt.Sprintf("/api/v2/workspaces/%s/current-state-version", tfcWorkspaceID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		// The download URL will be on the same server for simplicity
		downloadURL := fmt.Sprintf("http://%s/download/state", r.Host)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"attributes": map[string]any{
					"hosted-state-download-url": downloadURL,
				},
			},
		})
	})

	// 1c. Download State
	tfcMux.HandleFunc("/download/state", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(stateContent))
	})

	tfcSrv := httptest.NewServer(tfcMux)
	defer tfcSrv.Close()

	// 2. Mock KeyHarbour API
	projectUUID := "a1b2c3d4-a1b2-c3d4-e5f6-a1b2c3d4e5f6"
	workspaceUUID := "f1e2d3c4-b5a6-7890-1234-567890abcdef"

	khMux := http.NewServeMux()

	// GET Project
	khMux.HandleFunc("/v1/projects/"+projectUUID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(khclient.Project{
			UUID: projectUUID,
			Name: "my-project",
		})
	})

	// Resolve Workspace (GET workspaces list - simulating resolve by name)
	khMux.HandleFunc(fmt.Sprintf("/v1/projects/%s/workspaces", projectUUID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// TFC reader sets the "key" to the workspace name "my-ws"
		// The sync command uses that key as the default target workspace name if --workspace is not provided
		// So we mock that "my-ws" exists in KeyHarbour or we provide mapped name
		json.NewEncoder(w).Encode([]khclient.Workspace{{
			UUID: workspaceUUID,
			Name: tfcWorkspace, // Match the source workspace name
		}})
	})

	// Create Statefile
	uploadPath := fmt.Sprintf("/v1/projects/%s/workspaces/%s/statefiles", projectUUID, workspaceUUID)
	khMux.HandleFunc(uploadPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "invalid method", http.StatusMethodNotAllowed)
			return
		}

		var req khclient.CreateStatefileRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("failed to unmarshal upload request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if req.Content != stateContent {
			t.Errorf("expected content %q, got %q", stateContent, req.Content)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status": "created"}`))
	})

	khSrv := httptest.NewServer(khMux)
	defer khSrv.Close()

	// 3. Set Environment Variables
	t.Setenv("KH_ENDPOINT", khSrv.URL)
	t.Setenv("KH_TOKEN", "dummy-token")

	// 4. Run Command
	cmd := newSyncCmd()
	cmd.SetOut(io.Discard)
	cmd.SetContext(context.Background())

	args := []string{
		"--from=tfc",
		"--tfc-host=" + tfcSrv.URL,
		"--tfc-org=" + tfcOrg,
		"--tfc-workspace=" + tfcWorkspace, // This becomes the source object key "my-ws"
		"--tfc-token=dummy-tfc-token",
		"--project=" + projectUUID,
		// Not specifying --workspace, allowing inference from source key (my-ws) which matches mocked KH workspace name
	}
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync command (tfc) failed: %v", err)
	}
}
