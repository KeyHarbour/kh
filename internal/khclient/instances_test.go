package khclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"kh/internal/config"
)

func TestListInstances(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/license/applications/app-1/instances" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Instance{
			{UUID: "inst-1", Name: "Production", ShortName: "prod"},
			{UUID: "inst-2", Name: "Staging", ShortName: "stg"},
		})
	})

	c := New(config.Config{Endpoint: srv.URL})
	items, err := c.ListInstances(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].UUID != "inst-1" || items[0].Name != "Production" {
		t.Fatalf("unexpected item[0]: %+v", items[0])
	}
}

func TestListInstances_RequiresApplicationUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.ListInstances(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty application uuid")
	}
}

func TestGetInstance(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/license/instances/inst-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Instance{Name: "Production", ShortName: "prod", Status: "active"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	inst, err := c.GetInstance(context.Background(), "inst-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if inst.UUID != "inst-1" {
		t.Fatalf("expected UUID to be set from path, got %q", inst.UUID)
	}
	if inst.Name != "Production" {
		t.Fatalf("unexpected name: %q", inst.Name)
	}
}

func TestGetInstance_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.GetInstance(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestCreateInstance(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/license/applications/app-1/instances" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Instance{UUID: "inst-uuid-1", Name: "Production", ShortName: "prod"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	inst, err := c.CreateInstance(context.Background(), "app-1", CreateInstanceRequest{
		Name:      "Production",
		ShortName: "prod",
		Owner:     "ops",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if inst.UUID != "inst-uuid-1" {
		t.Fatalf("expected uuid inst-uuid-1, got %s", inst.UUID)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	wrapper, _ := m["instance"].(map[string]any)
	if wrapper == nil {
		t.Fatalf("expected instance wrapper in body, got: %s", bodyBytes)
	}
	if wrapper["name"] != "Production" {
		t.Fatalf("expected name in body, got: %s", bodyBytes)
	}
}

func TestCreateInstance_RequiresApplicationUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.CreateInstance(context.Background(), "", CreateInstanceRequest{Name: "x"}); err == nil {
		t.Fatal("expected error for empty application uuid")
	}
}

func TestUpdateInstance(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/license/instances/inst-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.UpdateInstance(context.Background(), "inst-1", UpdateInstanceRequest{Status: "disabled"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	inst, _ := m["instance"].(map[string]any)
	if inst == nil {
		t.Fatalf("expected instance wrapper in body, got: %s", bodyBytes)
	}
	if inst["status"] != "disabled" {
		t.Fatalf("expected status=disabled in body, got: %s", bodyBytes)
	}
}

func TestUpdateInstance_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.UpdateInstance(context.Background(), "", UpdateInstanceRequest{}); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestDeleteInstance(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/license/instances/inst-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteInstance(context.Background(), "inst-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 DELETE, got %d", hits)
	}
}

func TestDeleteInstance_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteInstance(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}
