package khclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"kh/internal/config"
)

func TestListLicensees(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/license/instances/inst-1/licensees" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Licensee{
			{UUID: "user-1", Status: "active"},
			{UUID: "user-2", Status: "inactive"},
		})
	})

	c := New(config.Config{Endpoint: srv.URL})
	items, err := c.ListLicensees(context.Background(), "inst-1")
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

func TestListLicensees_RequiresInstanceUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.ListLicensees(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty instance uuid")
	}
}

func TestGetLicensee(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/license/licensees/user-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Licensee{Status: "active"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	lc, err := c.GetLicensee(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if lc.UUID != "user-1" {
		t.Fatalf("expected UUID to be set from path, got %q", lc.UUID)
	}
	if lc.Status != "active" {
		t.Fatalf("unexpected status: %q", lc.Status)
	}
}

func TestGetLicensee_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.GetLicensee(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestCreateLicensee(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/license/instances/inst-1/licensees" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.CreateLicensee(context.Background(), "inst-1", CreateLicenseeRequest{UUID: "user-1"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	lc, _ := m["licensee"].(map[string]any)
	if lc == nil {
		t.Fatalf("expected licensee wrapper in body, got: %s", bodyBytes)
	}
	if lc["uuid"] != "user-1" {
		t.Fatalf("expected uuid=user-1 in body, got: %s", bodyBytes)
	}
}

func TestCreateLicensee_RequiresInstanceUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.CreateLicensee(context.Background(), "", CreateLicenseeRequest{UUID: "user-1"}); err == nil {
		t.Fatal("expected error for empty instance uuid")
	}
}

func TestUpdateLicensee(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/license/licensees/user-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.UpdateLicensee(context.Background(), "user-1", UpdateLicenseeRequest{Status: "inactive"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	lc, _ := m["licensee"].(map[string]any)
	if lc == nil {
		t.Fatalf("expected licensee wrapper in body, got: %s", bodyBytes)
	}
	if lc["status"] != "inactive" {
		t.Fatalf("expected status=inactive in body, got: %s", bodyBytes)
	}
}

func TestUpdateLicensee_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.UpdateLicensee(context.Background(), "", UpdateLicenseeRequest{}); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestDeleteLicensee(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/license/licensees/user-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteLicensee(context.Background(), "user-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 DELETE, got %d", hits)
	}
}

func TestDeleteLicensee_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteLicensee(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}
