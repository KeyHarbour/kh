package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/KeyHarbour/kh/internal/khclient"
	"github.com/KeyHarbour/kh/internal/kherrors"
)

const lockTestWorkspace = "0b5c3f7e-6a1d-4c2b-9e8f-1a2b3c4d5e6f"

// fakeTFBackend emulates the lock half of KeyHarbourApi's Terraform HTTP
// backend: one lock per workspace, PUT to lock, DELETE to unlock, 409 on
// conflict (with the holder) and on unlocking an unlocked workspace.
type fakeTFBackend struct {
	mu       sync.Mutex
	holders  map[string]*khclient.LockInfo
	requests []string
}

func newFakeTFBackend(t *testing.T, mux *http.ServeMux) *fakeTFBackend {
	t.Helper()
	b := &fakeTFBackend{holders: map[string]*khclient.LockInfo{}}
	mux.HandleFunc("/statefiles/v1/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/statefiles/v1/"), "/")
		if len(parts) != 2 || parts[1] == "" {
			http.NotFound(w, r)
			return
		}
		ws := parts[0]
		b.mu.Lock()
		defer b.mu.Unlock()
		b.requests = append(b.requests, r.Method+" "+ws)
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodPut:
			if h := b.holders[ws]; h != nil {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(h)
				return
			}
			var info khclient.LockInfo
			_ = json.NewDecoder(r.Body).Decode(&info)
			b.holders[ws] = &info
			_, _ = io.WriteString(w, `{}`)
		case http.MethodDelete:
			h := b.holders[ws]
			if h == nil {
				w.WriteHeader(http.StatusConflict)
				_, _ = io.WriteString(w, `{"error":"Not Found"}`)
				return
			}
			delete(b.holders, ws)
			_ = json.NewEncoder(w).Encode(map[string]string{"ID": h.ID})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	return b
}

func (b *fakeTFBackend) holder(ws string) *khclient.LockInfo {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.holders[ws]
}

func setupLockTest(t *testing.T) (*fakeTFBackend, *http.ServeMux) {
	t.Helper()
	useTempConfigHome(t)
	t.Setenv("KH_WORKSPACE", "")
	mux := http.NewServeMux()
	b := newFakeTFBackend(t, mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")
	return b, mux
}

func runStateCmd(t *testing.T, cmd interface {
	SetOut(io.Writer)
	SetErr(io.Writer)
	SetArgs([]string)
	Execute() error
}, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func requireKHCode(t *testing.T, err error, def kherrors.ErrorDef) {
	t.Helper()
	var khErr *kherrors.KHError
	if !errors.As(err, &khErr) || khErr.Code != def.Code {
		t.Fatalf("expected %s error, got %v", def.Code, err)
	}
}

// --- lock ---

func TestLockCmd_LocksWorkspace(t *testing.T) {
	b, _ := setupLockTest(t)

	out, err := runStateCmd(t, newLockCmd(), "--workspace", lockTestWorkspace, "--environment", "prod", "--reason", "maintenance")
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}
	h := b.holder(lockTestWorkspace)
	if h == nil {
		t.Fatal("workspace was not locked")
	}
	if h.Operation != "kh-lock" || h.Info != "maintenance" || h.Who == "" || !looksLikeUUID(h.ID) {
		t.Errorf("unexpected lock info: %+v", h)
	}
	if !strings.Contains(out, h.ID) || !strings.Contains(out, "kh tf state unlock") {
		t.Errorf("output should show the lock ID and how to release it, got %q", out)
	}
}

func TestLockCmd_AlreadyLocked(t *testing.T) {
	b, _ := setupLockTest(t)
	b.holders[lockTestWorkspace] = &khclient.LockInfo{ID: "held-1", Who: "alice@ci"}

	_, err := runStateCmd(t, newLockCmd(), "--workspace", lockTestWorkspace, "--environment", "prod")
	requireKHCode(t, err, kherrors.ErrStateLocked)
	if !strings.Contains(err.Error(), "alice@ci") || !strings.Contains(err.Error(), "held-1") {
		t.Errorf("error should name the holder, got %v", err)
	}
}

func TestLockCmd_Validation(t *testing.T) {
	setupLockTest(t)
	tests := []struct {
		name string
		args []string
		def  kherrors.ErrorDef
	}{
		{"missing workspace", []string{"--environment", "prod"}, kherrors.ErrMissingFlag},
		{"workspace not a UUID", []string{"--workspace", "prod-ws", "--environment", "prod"}, kherrors.ErrInvalidValue},
		{"missing environment", []string{"--workspace", lockTestWorkspace}, kherrors.ErrMissingFlag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runStateCmd(t, newLockCmd(), tt.args...)
			requireKHCode(t, err, tt.def)
		})
	}
}

// --- unlock ---

func TestUnlockCmd_RequiresLockIDOrForce(t *testing.T) {
	setupLockTest(t)
	_, err := runStateCmd(t, newUnlockCmd(), "--workspace", lockTestWorkspace, "--environment", "prod")
	requireKHCode(t, err, kherrors.ErrMissingFlag)
}

func TestUnlockCmd_ReleasesMatchingLock(t *testing.T) {
	b, _ := setupLockTest(t)
	b.holders[lockTestWorkspace] = &khclient.LockInfo{ID: "held-1"}

	out, err := runStateCmd(t, newUnlockCmd(), "--workspace", lockTestWorkspace, "--environment", "prod", "held-1")
	if err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	if b.holder(lockTestWorkspace) != nil {
		t.Fatal("lock was not released")
	}
	if !strings.Contains(out, "Released lock held-1") {
		t.Errorf("unexpected output %q", out)
	}
}

func TestUnlockCmd_RefusesDifferentLock(t *testing.T) {
	b, _ := setupLockTest(t)
	b.holders[lockTestWorkspace] = &khclient.LockInfo{ID: "held-1", Who: "alice@ci"}

	_, err := runStateCmd(t, newUnlockCmd(), "--workspace", lockTestWorkspace, "--environment", "prod", "other-id")
	requireKHCode(t, err, kherrors.ErrStateLocked)
	if h := b.holder(lockTestWorkspace); h == nil || h.ID != "held-1" {
		t.Fatalf("another holder's lock must stay in place, got %+v", h)
	}
}

func TestUnlockCmd_ForceReleasesAnyLock(t *testing.T) {
	b, _ := setupLockTest(t)
	b.holders[lockTestWorkspace] = &khclient.LockInfo{ID: "held-1"}

	if _, err := runStateCmd(t, newUnlockCmd(), "--workspace", lockTestWorkspace, "--environment", "prod", "--force"); err != nil {
		t.Fatalf("unlock --force failed: %v", err)
	}
	if b.holder(lockTestWorkspace) != nil {
		t.Fatal("lock was not released")
	}
}

func TestUnlockCmd_NotLocked(t *testing.T) {
	b, _ := setupLockTest(t)

	_, err := runStateCmd(t, newUnlockCmd(), "--workspace", lockTestWorkspace, "--environment", "prod", "held-1")
	requireKHCode(t, err, kherrors.ErrNotFound)
	if b.holder(lockTestWorkspace) != nil {
		t.Fatal("the holder check must not leave a lock behind")
	}

	_, err = runStateCmd(t, newUnlockCmd(), "--workspace", lockTestWorkspace, "--environment", "prod", "--force")
	requireKHCode(t, err, kherrors.ErrNotFound)
}

// --- verify ---

const verifyStoredState = `{"version":4,"terraform_version":"1.9.0","serial":7,"lineage":"lin-1","outputs":{},"resources":[]}`

func setupVerifyServer(t *testing.T, content string) {
	t.Helper()
	useTempConfigHome(t)
	t.Setenv("KH_WORKSPACE", "")
	mux := http.NewServeMux()
	body := map[string]any{"uuid": "sf-1", "content": content, "published_at": "2026-10-26T00:00:00Z"}
	mux.HandleFunc("/api/v2/workspaces/"+lockTestWorkspace+"/statefiles/last", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, body)
	})
	mux.HandleFunc("/api/v2/statefiles/sf-1", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("KH_ENDPOINT", srv.URL+"/api/v2")
	t.Setenv("KH_TOKEN", "test-token")
}

func TestVerifyCmd_LatestStatefileOK(t *testing.T) {
	setupVerifyServer(t, verifyStoredState)

	out, err := runStateCmd(t, newVerifyCmd(), "--workspace", lockTestWorkspace)
	if err != nil {
		t.Fatalf("verify failed: %v\n%s", err, out)
	}
	for _, want := range []string{"sf-1", "version", "lineage", "lin-1", "serial"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "FAIL") {
		t.Errorf("no check should fail:\n%s", out)
	}
}

func TestVerifyCmd_ByStatefileUUIDAgainstLocalFile(t *testing.T) {
	setupVerifyServer(t, verifyStoredState)
	local := filepath.Join(t.TempDir(), "terraform.tfstate")
	if err := os.WriteFile(local, []byte(verifyStoredState), 0o644); err != nil {
		t.Fatal(err)
	}

	outputFormat = "json"
	defer func() { outputFormat = "" }()
	out, err := runStateCmd(t, newVerifyCmd(), "sf-1", "--against", local)
	if err != nil {
		t.Fatalf("verify failed: %v\n%s", err, out)
	}
	var res struct {
		OK     bool          `json:"ok"`
		Checks []verifyCheck `json:"checks"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if !res.OK || len(res.Checks) != 7 {
		t.Errorf("expected 7 passing checks, got %+v", res)
	}
}

func TestVerifyCmd_MismatchFails(t *testing.T) {
	setupVerifyServer(t, verifyStoredState)
	local := filepath.Join(t.TempDir(), "terraform.tfstate")
	if err := os.WriteFile(local, []byte(`{"version":4,"serial":8,"lineage":"lin-1"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runStateCmd(t, newVerifyCmd(), "--workspace", lockTestWorkspace, "--against", local)
	requireKHCode(t, err, kherrors.ErrStateVerifyFailed)
	if !strings.Contains(err.Error(), "local-serial") || !strings.Contains(err.Error(), "local-checksum") {
		t.Errorf("error should name the failed checks, got %v", err)
	}
	if !strings.Contains(out, "FAIL") {
		t.Errorf("table should show the failures:\n%s", out)
	}
}

func TestVerifyStateContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		failing []string
	}{
		{"valid", verifyStoredState, nil},
		{"not json", `not json`, []string{"json"}},
		{"wrong version", `{"version":3,"serial":1,"lineage":"x"}`, []string{"version"}},
		{"missing lineage and serial", `{"version":4}`, []string{"lineage", "serial"}},
		{"negative serial", `{"version":4,"serial":-1,"lineage":"x"}`, []string{"serial"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var failing []string
			for _, c := range verifyStateContent([]byte(tt.content), nil) {
				if !c.OK {
					failing = append(failing, c.Name)
				}
			}
			if fmt.Sprint(failing) != fmt.Sprint(tt.failing) {
				t.Errorf("failing checks = %v, want %v", failing, tt.failing)
			}
		})
	}
}

// --- sync --lock ---

func TestSyncCmd_LockHoldsDestinationWorkspaceForTheRun(t *testing.T) {
	b, mux := setupLockTest(t)
	stateFile := filepath.Join(t.TempDir(), "terraform.tfstate")
	if err := os.WriteFile(stateFile, []byte(verifyStoredState), 0o644); err != nil {
		t.Fatal(err)
	}
	projectUUID := "a1b2c3d4-a1b2-c3d4-e5f6-a1b2c3d4e5f6"
	mux.HandleFunc("/api/v2/projects/"+projectUUID, func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, khclient.Project{UUID: projectUUID, Name: "p"})
	})
	mux.HandleFunc("/api/v2/projects/"+projectUUID+"/workspaces", func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, []khclient.Workspace{{UUID: lockTestWorkspace, Name: "target"}})
	})
	mux.HandleFunc("/api/v2/workspaces/"+lockTestWorkspace, func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, khclient.Workspace{UUID: lockTestWorkspace, Name: "target"})
	})
	var lockedDuringUpload bool
	mux.HandleFunc("/api/v2/workspaces/"+lockTestWorkspace+"/statefiles", func(w http.ResponseWriter, r *http.Request) {
		lockedDuringUpload = b.holder(lockTestWorkspace) != nil
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"status":"created"}`)
	})

	err := runSyncCmd(t, "--from=local", "--local-path="+stateFile, "--to=keyharbour",
		"--kh-project="+projectUUID, "--kh-workspace="+lockTestWorkspace, "--kh-env=prod", "--lock")
	if err != nil {
		t.Fatalf("sync --lock failed: %v", err)
	}
	if !lockedDuringUpload {
		t.Error("destination workspace should be locked while the state is uploaded")
	}
	if b.holder(lockTestWorkspace) != nil {
		t.Error("lock should be released after the run")
	}
	if got := strings.Join(b.requests, ","); got != "PUT "+lockTestWorkspace+",DELETE "+lockTestWorkspace {
		t.Errorf("expected one lock and one unlock, got %s", got)
	}
}

func TestSyncCmd_LockConflictStopsBeforeWriting(t *testing.T) {
	b, mux := setupLockTest(t)
	b.holders[lockTestWorkspace] = &khclient.LockInfo{ID: "held-1", Who: "alice@ci"}
	stateFile := filepath.Join(t.TempDir(), "terraform.tfstate")
	if err := os.WriteFile(stateFile, []byte(verifyStoredState), 0o644); err != nil {
		t.Fatal(err)
	}
	projectUUID := "a1b2c3d4-a1b2-c3d4-e5f6-a1b2c3d4e5f6"
	mux.HandleFunc("/api/v2/projects/"+projectUUID, func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, khclient.Project{UUID: projectUUID, Name: "p"})
	})
	mux.HandleFunc("/api/v2/workspaces/"+lockTestWorkspace+"/statefiles", func(w http.ResponseWriter, r *http.Request) {
		t.Error("nothing should be written while another process holds the lock")
	})

	err := runSyncCmd(t, "--from=local", "--local-path="+stateFile, "--to=keyharbour",
		"--kh-project="+projectUUID, "--kh-workspace="+lockTestWorkspace, "--kh-env=prod", "--lock")
	requireKHCode(t, err, kherrors.ErrStateLocked)
	if h := b.holder(lockTestWorkspace); h == nil || h.ID != "held-1" {
		t.Fatalf("the other holder's lock must stay in place, got %+v", h)
	}
}

func TestSyncCmd_LockValidation(t *testing.T) {
	_, mux := setupLockTest(t)
	projectUUID := "a1b2c3d4-a1b2-c3d4-e5f6-a1b2c3d4e5f6"
	mux.HandleFunc("/api/v2/projects/"+projectUUID, func(w http.ResponseWriter, r *http.Request) {
		writeJSONBody(t, w, khclient.Project{UUID: projectUUID, Name: "p"})
	})
	stateFile := filepath.Join(t.TempDir(), "terraform.tfstate")
	if err := os.WriteFile(stateFile, []byte(verifyStoredState), 0o644); err != nil {
		t.Fatal(err)
	}
	toKH := []string{"--from=local", "--local-path=" + stateFile, "--kh-project=" + projectUUID}
	tests := []struct {
		name string
		args []string
		def  kherrors.ErrorDef
		msg  string
	}{
		{"no KeyHarbour side", []string{"--from=local", "--local-path=" + stateFile, "--to=local", "--local-out=" + filepath.Join(t.TempDir(), "out.tfstate"), "--lock"}, kherrors.ErrInvalidValue, "--from=keyharbour or --to=keyharbour"},
		{"destination workspace not a UUID", append(toKH, "--kh-workspace=target", "--kh-env=prod", "--lock"), kherrors.ErrMissingFlag, "--kh-workspace <workspace-uuid>"},
		{"destination without --kh-env", append(toKH, "--kh-workspace="+lockTestWorkspace, "--lock"), kherrors.ErrMissingFlag, "--kh-env"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runSyncCmd(t, tt.args...)
			requireKHCode(t, err, tt.def)
			if !strings.Contains(err.Error(), tt.msg) {
				t.Errorf("error %q should mention %q", err, tt.msg)
			}
		})
	}
}
