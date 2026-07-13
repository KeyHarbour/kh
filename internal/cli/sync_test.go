package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kh/internal/khclient"
	"kh/internal/kherrors"
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
	mux.HandleFunc("/api/v2/projects/"+projectUUID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(khclient.Project{
			UUID: projectUUID,
			Name: "my-project",
		})
	})

	// Resolve Workspace (GET workspaces list - simulating resolve by name)
	// The client might use ListWorkspaces if reference is a name.
	mux.HandleFunc(fmt.Sprintf("/api/v2/projects/%s/workspaces", projectUUID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]khclient.Workspace{{
			UUID: workspaceUUID,
			Name: "prod",
		}})
	})

	// Create Statefile
	// POST /projects/{pid}/workspaces/{wid}/statefiles?env=default
	uploadPath := fmt.Sprintf("/api/v2/workspaces/%s/statefiles", workspaceUUID)
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
		"--local-path=" + stateFile,
		"--kh-project=" + projectUUID,
		"--kh-workspace=prod",
	}
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync command failed: %v", err)
	}
}

func TestSyncCmd_TFC_Success(t *testing.T) {
	// 1. Mock TFC API
	tfcOrg := "myorg"
	tfcWorkspace := "myws"
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
	khMux.HandleFunc("/api/v2/projects/"+projectUUID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(khclient.Project{
			UUID: projectUUID,
			Name: "my-project",
		})
	})

	// Resolve Workspace (GET workspaces list - simulating resolve by name)
	khMux.HandleFunc(fmt.Sprintf("/api/v2/projects/%s/workspaces", projectUUID), func(w http.ResponseWriter, r *http.Request) {
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
	uploadPath := fmt.Sprintf("/api/v2/workspaces/%s/statefiles", workspaceUUID)
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
		"--tfc-src-host=" + tfcSrv.URL,
		"--tfc-src-org=" + tfcOrg,
		"--tfc-src-workspace=" + tfcWorkspace, // This becomes the source object key "my-ws"
		"--tfc-src-token=dummy-tfc-token",
		"--kh-project=" + projectUUID,
		// Not specifying --kh-workspace, allowing inference from source key (my-ws) which matches mocked KH workspace name
	}
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("sync command (tfc) failed: %v", err)
	}
}

// ── Error taxonomy tests ──────────────────────────────────────────────────

func runSyncCmd(t *testing.T, args ...string) error {
	t.Helper()
	cmd := newSyncCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs(args)
	return cmd.Execute()
}

func assertKHError(t *testing.T, err error, wantCode string) *kherrors.KHError {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", wantCode)
	}
	var khErr *kherrors.KHError
	if !errors.As(err, &khErr) {
		t.Fatalf("expected *kherrors.KHError (code %q), got %T: %v", wantCode, err, err)
	}
	if khErr.Code != wantCode {
		t.Errorf("Code = %q, want %q", khErr.Code, wantCode)
	}
	return khErr
}

func TestSyncCmd_MissingFrom_ReturnsKHError(t *testing.T) {
	err := runSyncCmd(t, "--kh-project=some-uuid")
	assertKHError(t, err, "KH-VAL-001")
}

func TestSyncCmd_MissingToken_ReturnsKHError(t *testing.T) {
	// Redirect HOME so config.LoadWithEnv finds no config file and no stored token.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("KH_TOKEN", "")
	err := runSyncCmd(t, "--from=keyharbour", "--kh-src-project=p", "--kh-src-workspace=w", "--to=local", "--local-out=out.tfstate")
	assertKHError(t, err, "KH-AUTH-001")
}

func TestSyncCmd_UnsupportedFrom_ReturnsKHError(t *testing.T) {
	err := runSyncCmd(t, "--from=ftp", "--kh-project=some-uuid")
	assertKHError(t, err, "KH-VAL-002")
}

func TestSyncCmd_UnsupportedTo_ReturnsKHError(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "terraform.tfstate")
	os.WriteFile(stateFile, []byte(`{"version":4,"terraform_version":"1.0","serial":1,"lineage":"x","outputs":{}}`), 0o644)
	err := runSyncCmd(t, "--from=local", "--local-path="+stateFile, "--to=ftp")
	assertKHError(t, err, "KH-VAL-002")
}

func TestSyncCmd_MissingPath_ReturnsKHError(t *testing.T) {
	err := runSyncCmd(t, "--from=local")
	assertKHError(t, err, "KH-VAL-001")
}

func TestSyncCmd_InvalidWorkspacePattern_ReturnsKHError(t *testing.T) {
	t.Setenv("KH_TOKEN", "dummy")
	err := runSyncCmd(t, "--from=keyharbour", "--local-workspace-pattern=[invalid")
	assertKHError(t, err, "KH-VAL-002")
}

func TestSyncCmd_UsesKHWorkspaceEnvForDestination(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "terraform.tfstate")
	stateContent := `{"version": 4, "terraform_version": "1.5.0", "serial": 1, "lineage": "abc", "outputs": {}}`
	if err := os.WriteFile(stateFile, []byte(stateContent), 0o644); err != nil {
		t.Fatal(err)
	}

	projectUUID := "a1b2c3d4-a1b2-c3d4-e5f6-a1b2c3d4e5f6"
	defaultWorkspaceUUID := "aaaaaaaa-1111-2222-3333-bbbbbbbbbbbb"
	targetWorkspaceUUID := "cccccccc-1111-2222-3333-dddddddddddd"
	targetWorkspaceName := "targetws"

	var uploadedToTarget bool
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/projects/"+projectUUID, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(khclient.Project{UUID: projectUUID, Name: "my-project"})
	})
	mux.HandleFunc(fmt.Sprintf("/api/v2/projects/%s/workspaces", projectUUID), func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]khclient.Workspace{
			{UUID: defaultWorkspaceUUID, Name: "default"},
			{UUID: targetWorkspaceUUID, Name: targetWorkspaceName},
		})
	})
	mux.HandleFunc(fmt.Sprintf("/api/v2/workspaces/%s/statefiles", targetWorkspaceUUID), func(w http.ResponseWriter, r *http.Request) {
		uploadedToTarget = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	})
	mux.HandleFunc(fmt.Sprintf("/api/v2/workspaces/%s/statefiles", defaultWorkspaceUUID), func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("sync wrote to default workspace instead of KH_WORKSPACE env target")
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	t.Setenv("KH_ENDPOINT", srv.URL)
	t.Setenv("KH_TOKEN", "dummy-token")
	t.Setenv("KH_WORKSPACE", targetWorkspaceName)

	err := runSyncCmd(t,
		"--from=local",
		"--local-path="+stateFile,
		"--to=keyharbour",
		"--kh-project="+projectUUID,
	)
	if err != nil {
		t.Fatalf("sync command failed: %v", err)
	}
	if !uploadedToTarget {
		t.Fatal("expected sync to upload into workspace selected by KH_WORKSPACE env")
	}
}

func TestSyncCmd_ToLocal_MultiObjectRequiresUniqueOutputPaths(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "a.tfstate"), []byte(`{"version":4}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "b.tfstate"), []byte(`{"version":4}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runSyncCmd(t,
		"--from=local",
		"--local-path="+tmpDir,
		"--to=local",
		"--local-out="+filepath.Join(tmpDir, "out.tfstate"),
	)
	khErr := assertKHError(t, err, "KH-VAL-002")
	if !strings.Contains(khErr.Message, "unique output paths") {
		t.Fatalf("unexpected error message: %s", khErr.Message)
	}
}

func TestNormalizeConcurrency(t *testing.T) {
	tests := []struct {
		name        string
		flagValue   int
		configValue int
		want        int
	}{
		{name: "default fallback", flagValue: 0, configValue: 0, want: 4},
		{name: "use config", flagValue: 0, configValue: 8, want: 8},
		{name: "flag wins", flagValue: 3, configValue: 8, want: 3},
		{name: "clamp low from flag", flagValue: -10, configValue: 8, want: 1},
		{name: "clamp high from flag", flagValue: 100000, configValue: 8, want: 64},
		{name: "clamp high from config", flagValue: 0, configValue: 100000, want: 64},
		{name: "clamp low from config", flagValue: 0, configValue: -5, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeConcurrency(tt.flagValue, tt.configValue)
			if got != tt.want {
				t.Fatalf("normalizeConcurrency(%d, %d) = %d, want %d", tt.flagValue, tt.configValue, got, tt.want)
			}
		})
	}
}
