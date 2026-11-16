package khclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/KeyHarbour/kh/internal/config"
)

func TestHostRoot(t *testing.T) {
	tests := map[string]string{
		"https://app.keyharbour.ca/api/v2":  "https://app.keyharbour.ca",
		"https://app.keyharbour.ca/api/v2/": "https://app.keyharbour.ca",
		"https://app.keyharbour.ca":         "https://app.keyharbour.ca",
		"https://kh.internal/base/api/v2":   "https://kh.internal/base",
	}
	for in, want := range tests {
		if got := HostRoot(in); got != want {
			t.Errorf("HostRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLockWorkspace_PutsLockInfoAtHostRoot(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	var gotBody LockInfo
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotAuth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	})

	c := New(config.Config{Endpoint: srv.URL + "/api/v2", Token: "tok"})
	err := c.LockWorkspace(context.Background(), "ws-uuid", "prod", LockInfo{ID: "lock-1", Operation: "kh-sync", Who: "me@host"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/statefiles/v1/ws-uuid/prod" {
		t.Errorf("path = %s, want /statefiles/v1/ws-uuid/prod", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want Bearer tok", gotAuth)
	}
	if gotBody.ID != "lock-1" || gotBody.Operation != "kh-sync" || gotBody.Who != "me@host" {
		t.Errorf("unexpected lock body: %+v", gotBody)
	}
}

func TestLockWorkspace_ConflictReturnsHolder(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		io.WriteString(w, `{"ID":"held-1","Who":"alice@ci","Operation":"OperationTypeApply","Created":"2026-10-26T10:00:00Z"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.LockWorkspace(context.Background(), "ws", "prod", LockInfo{ID: "mine"})
	var locked *LockedError
	if !errors.As(err, &locked) {
		t.Fatalf("expected *LockedError, got %v", err)
	}
	if locked.Holder.ID != "held-1" || locked.Holder.Who != "alice@ci" {
		t.Errorf("unexpected holder: %+v", locked.Holder)
	}
	for _, want := range []string{"alice@ci", "OperationTypeApply", "held-1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q should mention %q", err, want)
		}
	}
}

func TestLockWorkspace_RequiresWorkspaceAndEnvironment(t *testing.T) {
	c := New(config.Config{Endpoint: "http://unused"})
	if err := c.LockWorkspace(context.Background(), "", "prod", LockInfo{}); err == nil {
		t.Error("expected error for missing workspace")
	}
	if err := c.LockWorkspace(context.Background(), "ws", "", LockInfo{}); err == nil {
		t.Error("expected error for missing environment")
	}
}

func TestUnlockWorkspace_DeletesAndReturnsReleasedID(t *testing.T) {
	var gotMethod, gotPath string
	var gotBody LockInfo
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"ID":"lock-1"}`)
	})

	c := New(config.Config{Endpoint: srv.URL + "/api/v2"})
	id, err := c.UnlockWorkspace(context.Background(), "ws-uuid", "prod", "lock-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotMethod != http.MethodDelete || gotPath != "/statefiles/v1/ws-uuid/prod" {
		t.Errorf("request = %s %s, want DELETE /statefiles/v1/ws-uuid/prod", gotMethod, gotPath)
	}
	if gotBody.ID != "lock-1" {
		t.Errorf("unlock body ID = %q, want lock-1", gotBody.ID)
	}
	if id != "lock-1" {
		t.Errorf("released ID = %q, want lock-1", id)
	}
}

func TestUnlockWorkspace_NotLocked(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		io.WriteString(w, `{"error":"Not Found"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.UnlockWorkspace(context.Background(), "ws", "prod", ""); !errors.Is(err, ErrNotLocked) {
		t.Fatalf("expected ErrNotLocked, got %v", err)
	}
}

func TestLockHolder_Locked(t *testing.T) {
	var methods []string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.WriteHeader(http.StatusConflict)
		io.WriteString(w, `{"ID":"held-1"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	holder, err := c.LockHolder(context.Background(), "ws", "prod", LockInfo{ID: "probe"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if holder == nil || holder.ID != "held-1" {
		t.Fatalf("unexpected holder: %+v", holder)
	}
	if len(methods) != 1 || methods[0] != http.MethodPut {
		t.Errorf("a held lock must not be released by the probe, got requests %v", methods)
	}
}

func TestLockHolder_UnlockedReleasesProbe(t *testing.T) {
	var methods []string
	var releasedID string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Method == http.MethodDelete {
			var body LockInfo
			_ = json.NewDecoder(r.Body).Decode(&body)
			releasedID = body.ID
		}
		w.WriteHeader(http.StatusOK)
	})

	c := New(config.Config{Endpoint: srv.URL})
	holder, err := c.LockHolder(context.Background(), "ws", "prod", LockInfo{ID: "probe"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if holder != nil {
		t.Fatalf("expected no holder, got %+v", holder)
	}
	if strings.Join(methods, ",") != "PUT,DELETE" || releasedID != "probe" {
		t.Errorf("probe should lock then release itself, got %v releasing %q", methods, releasedID)
	}
}
