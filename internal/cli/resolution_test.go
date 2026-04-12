package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"kh/internal/config"
	"kh/internal/kherrors"
	"kh/internal/khclient"
	"kh/internal/kherrors"
)

func newIPv4Server(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on ipv4: %v", err)
	}
	srv := &httptest.Server{
		Listener: l,
		Config:   &http.Server{Handler: handler},
	}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func TestResolveProjectRefRequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	client := khclient.New(config.Config{Endpoint: srv.URL})
	_, err := resolveProjectRef(context.Background(), client, "demo")
	if err == nil {
		t.Fatal("expected error when project not found")
	}
	var khErr *kherrors.KHError
	if !errors.As(err, &khErr) {
		t.Fatalf("expected *kherrors.KHError, got %T: %v", err, err)
	}
	if khErr.Code != "KH-NF-001" {
		t.Errorf("Code = %q, want KH-NF-001", khErr.Code)
	}
}

func TestResolveProjectRefEmptyRef(t *testing.T) {
	client := khclient.New(config.Config{})
	_, err := resolveProjectRef(context.Background(), client, "")
	if err == nil {
		t.Fatal("expected error for empty ref")
	}
	var khErr *kherrors.KHError
	if !errors.As(err, &khErr) {
		t.Fatalf("expected *kherrors.KHError, got %T", err)
	}
	if khErr.Code != "KH-VAL-001" {
		t.Errorf("Code = %q, want KH-VAL-001", khErr.Code)
	}
}

func TestResolveWorkspaceRefEmptyRef(t *testing.T) {
	client := khclient.New(config.Config{})
	_, err := resolveWorkspaceRef(context.Background(), client, "p-1", "")
	if err == nil {
		t.Fatal("expected error for empty workspace ref")
	}
	var khErr *kherrors.KHError
	if !errors.As(err, &khErr) {
		t.Fatalf("expected *kherrors.KHError, got %T", err)
	}
	if khErr.Code != "KH-VAL-001" {
		t.Errorf("Code = %q, want KH-VAL-001", khErr.Code)
	}
}

func TestResolveWorkspaceRefNonUUID(t *testing.T) {
	client := khclient.New(config.Config{})
	_, err := resolveWorkspaceRef(context.Background(), client, "p-1", "my-workspace-name")
	if err == nil {
		t.Fatal("expected error for non-UUID workspace ref")
	}
	var khErr *kherrors.KHError
	if !errors.As(err, &khErr) {
		t.Fatalf("expected *kherrors.KHError, got %T", err)
	}
	if khErr.Code != "KH-VAL-002" {
		t.Errorf("Code = %q, want KH-VAL-002", khErr.Code)
	}
}

func TestResolveProjectRefByUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/projects/p-1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(khclient.Project{UUID: "p-1", Name: "demo"})
		default:
			http.NotFound(w, r)
		}
	})

	client := khclient.New(config.Config{Endpoint: srv.URL})
	proj, err := resolveProjectRef(context.Background(), client, "p-1")
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if proj.UUID != "p-1" || proj.Name != "demo" {
		t.Fatalf("unexpected project: %+v", proj)
	}
}

func TestResolveWorkspaceRefByUUID(t *testing.T) {
	const wsUUID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/workspaces/" + wsUUID:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(khclient.Workspace{UUID: wsUUID, Name: "my-workspace"})
		default:
			http.NotFound(w, r)
		}
	})

	client := khclient.New(config.Config{Endpoint: srv.URL})
	ws, err := resolveWorkspaceRef(context.Background(), client, "p-1", wsUUID)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if ws.UUID != wsUUID {
		t.Fatalf("unexpected workspace: %+v", ws)
	}
}
