package khclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"kh/internal/config"
)

func TestListApplications(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/license/applications" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]Application{
			{UUID: "app-1", Name: "Terraform Cloud", ShortName: "tfc", Vendor: "HashiCorp", Owner: "ops"},
			{UUID: "app-2", Name: "Datadog", ShortName: "dd", Vendor: "Datadog", Owner: "sre"},
		})
	})

	c := New(config.Config{Endpoint: srv.URL})
	items, err := c.ListApplications(context.Background())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].UUID != "app-1" || items[0].Name != "Terraform Cloud" {
		t.Fatalf("unexpected item[0]: %+v", items[0])
	}
}

func TestGetApplication(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/license/applications/app-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Application{
			Name: "Terraform Cloud", ShortName: "tfc", Vendor: "HashiCorp", Owner: "ops", Status: "active",
		})
	})

	c := New(config.Config{Endpoint: srv.URL})
	app, err := c.GetApplication(context.Background(), "app-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if app.UUID != "app-1" {
		t.Fatalf("expected UUID to be set from path, got %q", app.UUID)
	}
	if app.Name != "Terraform Cloud" {
		t.Fatalf("unexpected name: %q", app.Name)
	}
}

func TestGetApplication_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.GetApplication(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestCreateApplication(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/license/applications" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(Application{UUID: "app-uuid-1", Name: "Terraform Cloud", ShortName: "tfc"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	app, err := c.CreateApplication(context.Background(), CreateApplicationRequest{
		Name:      "Terraform Cloud",
		ShortName: "tfc",
		Owner:     "ops",
		Vendor:    "HashiCorp",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if app.UUID != "app-uuid-1" {
		t.Fatalf("expected uuid app-uuid-1, got %s", app.UUID)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	wrapper, _ := m["application"].(map[string]any)
	if wrapper == nil {
		t.Fatalf("expected application wrapper in body, got: %s", bodyBytes)
	}
	if wrapper["name"] != "Terraform Cloud" {
		t.Fatalf("expected name in body, got: %s", bodyBytes)
	}
}

func TestUpdateApplication(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/license/applications/app-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.UpdateApplication(context.Background(), "app-1", UpdateApplicationRequest{
		Status: "disabled",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	app, _ := m["application"].(map[string]any)
	if app == nil {
		t.Fatalf("expected application wrapper in body, got: %s", bodyBytes)
	}
	if app["status"] != "disabled" {
		t.Fatalf("expected status=disabled in body, got: %s", bodyBytes)
	}
}

func TestUpdateApplication_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.UpdateApplication(context.Background(), "", UpdateApplicationRequest{}); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}

func TestDeleteApplication(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/license/applications/app-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteApplication(context.Background(), "app-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 DELETE, got %d", hits)
	}
}

func TestDeleteApplication_RequiresUUID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteApplication(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty uuid")
	}
}
