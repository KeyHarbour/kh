package khclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"kh/internal/config"
)

func TestListTeamMembers(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/license/team_members" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]TeamMember{
			{UUID: "user-1"},
			{UUID: "user-2"},
		})
	})

	c := New(config.Config{Endpoint: srv.URL})
	items, err := c.ListTeamMembers(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].UUID != "user-1" {
		t.Fatalf("unexpected item[0]: %+v", items[0])
	}
}

func TestGetTeamMember(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/license/team_members/user-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TeamMember{})
	})

	c := New(config.Config{Endpoint: srv.URL})
	tm, err := c.GetTeamMember(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tm.UUID != "user-1" {
		t.Fatalf("expected UUID to be set from path, got %q", tm.UUID)
	}
}

func TestGetTeamMember_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.GetTeamMember(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestCreateTeamMember(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/license/team_members" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.CreateTeamMember(context.Background(), CreateTeamMemberRequest{UUID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	tm, _ := m["team_member"].(map[string]any)
	if tm == nil {
		t.Fatalf("expected team_member wrapper in body, got: %s", bodyBytes)
	}
	if tm["uuid"] != "user-1" {
		t.Fatalf("expected uuid=user-1 in body, got: %s", bodyBytes)
	}
}

func TestUpdateTeamMember(t *testing.T) {
	var bodyBytes []byte
	managerUUID := "mgr-1"
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/license/team_members/user-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.UpdateTeamMember(context.Background(), "user-1", UpdateTeamMemberRequest{ManagerUUID: managerUUID})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	tm, _ := m["team_member"].(map[string]any)
	if tm == nil {
		t.Fatalf("expected team_member wrapper in body, got: %s", bodyBytes)
	}
	if tm["manager_uuid"] != managerUUID {
		t.Fatalf("expected manager_uuid=%s in body, got: %s", managerUUID, bodyBytes)
	}
}

func TestUpdateTeamMember_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.UpdateTeamMember(context.Background(), "", UpdateTeamMemberRequest{}); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestDeleteTeamMember(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/license/team_members/user-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteTeamMember(context.Background(), "user-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 DELETE, got %d", hits)
	}
}

func TestDeleteTeamMember_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteTeamMember(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}
