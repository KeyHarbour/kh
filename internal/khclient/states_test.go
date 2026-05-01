package khclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"kh/internal/config"
)

func TestListStates_404ReturnsEmpty(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/states" {
			http.NotFound(w, r)
			return
		}
		http.NotFound(w, r)
	})

	cfg := config.Config{Endpoint: srv.URL}
	c := New(cfg)
	out, err := c.ListStates(context.Background(), ListStatesRequest{})
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty list, got %d", len(out))
	}
}

func TestListStates_Success(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[
			{"id":"state-1","project":"proj-a","module":"infra","workspace":"default","lineage":"abc","serial":3,"size":1024,"checksum":"deadbeef"},
			{"id":"state-2","project":"proj-a","module":"db","workspace":"prod","lineage":"def","serial":1,"size":512,"checksum":"cafebabe"}
		]`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	out, err := c.ListStates(context.Background(), ListStatesRequest{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 items, got %d", len(out))
	}
	if out[0].ID != "state-1" || out[0].Module != "infra" || out[0].Serial != 3 {
		t.Fatalf("unexpected item[0]: %+v", out[0])
	}
	if out[1].ID != "state-2" || out[1].Workspace != "prod" {
		t.Fatalf("unexpected item[1]: %+v", out[1])
	}
}

func TestListStates_FiltersForwardedAsQueryParams(t *testing.T) {
	var gotQuery string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	_, err := c.ListStates(context.Background(), ListStatesRequest{
		Project:   "proj-a",
		Module:    "infra",
		Workspace: "prod",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	for _, want := range []string{"project=proj-a", "module=infra", "workspace=prod"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("expected query to contain %q, got %q", want, gotQuery)
		}
	}
}

func TestListStates_EmptyFiltersOmittedFromQuery(t *testing.T) {
	var gotQuery string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[]`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	_, err := c.ListStates(context.Background(), ListStatesRequest{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotQuery != "" {
		t.Errorf("expected empty query string, got %q", gotQuery)
	}
}

func TestListStates_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	c := New(config.Config{Endpoint: srv.URL})
	_, err := c.ListStates(context.Background(), ListStatesRequest{})
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

func TestGetStateRaw_RequiresID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	_, _, err := c.GetStateRaw(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestGetStateRaw_Success(t *testing.T) {
	meta := StateMeta{ID: "state-1", Project: "proj-a", Module: "infra", Workspace: "default", Lineage: "abc", Serial: 5}
	metaJSON, _ := json.Marshal(meta)

	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/states/state-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Accept") != "application/vnd.terraform.state+json;version=4" {
			t.Errorf("unexpected Accept header: %s", r.Header.Get("Accept"))
		}
		w.Header().Set("X-State-Meta", string(metaJSON))
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"format_version":"1.0","terraform_version":"1.6.0"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	body, got, err := c.GetStateRaw(context.Background(), "state-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if string(body) != `{"format_version":"1.0","terraform_version":"1.6.0"}` {
		t.Fatalf("unexpected body: %s", body)
	}
	if got.ID != "state-1" || got.Serial != 5 || got.Module != "infra" {
		t.Fatalf("unexpected meta: %+v", got)
	}
}

func TestGetStateRaw_NoMetaHeader(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"format_version":"1.0"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	body, got, err := c.GetStateRaw(context.Background(), "state-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty body")
	}
	if got.ID != "" {
		t.Fatalf("expected zero-value meta when header absent, got %+v", got)
	}
}

func TestGetStateRaw_IDEscapedInPath(t *testing.T) {
	var gotPath string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	c.GetStateRaw(context.Background(), "proj/module/workspace")
	if gotPath != "/v1/states/proj%2Fmodule%2Fworkspace" {
		t.Fatalf("expected path-escaped id, got %q", gotPath)
	}
}

func TestGetStateRaw_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	c := New(config.Config{Endpoint: srv.URL})
	_, _, err := c.GetStateRaw(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestAcquireLock_RequiresID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.AcquireLock(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestAcquireLock_Success200(t *testing.T) {
	var called bool
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/states/state-1/lock" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.AcquireLock(context.Background(), "state-1"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !called {
		t.Fatal("server not called")
	}
}

func TestAcquireLock_Success204(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.AcquireLock(context.Background(), "state-1"); err != nil {
		t.Fatalf("unexpected err on 204: %v", err)
	}
}

func TestAcquireLock_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.AcquireLock(context.Background(), "state-1"); err == nil {
		t.Fatal("expected error on 409, got nil")
	}
}

func TestReleaseLock_RequiresID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.ReleaseLock(context.Background(), "", false); err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestReleaseLock_Success(t *testing.T) {
	var called bool
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/states/state-1/unlock" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("force") != "" {
			t.Errorf("expected no force param, got %q", r.URL.Query().Get("force"))
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.ReleaseLock(context.Background(), "state-1", false); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !called {
		t.Fatal("server not called")
	}
}

func TestReleaseLock_WithForce(t *testing.T) {
	var gotForce string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		gotForce = r.URL.Query().Get("force")
		w.WriteHeader(http.StatusOK)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.ReleaseLock(context.Background(), "state-1", true); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotForce != "true" {
		t.Fatalf("expected force=true in query, got %q", gotForce)
	}
}

func TestReleaseLock_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.ReleaseLock(context.Background(), "state-1", false); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

func TestPutState_RequiresID(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	_, err := c.PutState(context.Background(), "", []byte("{}"), false)
	if err == nil {
		t.Fatal("expected error for empty id")
	}
}

func TestPutState_Success201(t *testing.T) {
	meta := StateMeta{ID: "state-1", Serial: 7, Checksum: "abc123"}
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/states/state-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/vnd.terraform.state+json;version=4" {
			t.Errorf("unexpected Content-Type: %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"version":4}` {
			t.Errorf("unexpected body: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(meta)
	})

	c := New(config.Config{Endpoint: srv.URL})
	got, err := c.PutState(context.Background(), "state-1", []byte(`{"version":4}`), false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.ID != "state-1" || got.Serial != 7 || got.Checksum != "abc123" {
		t.Fatalf("unexpected meta: %+v", got)
	}
}

func TestPutState_Success200(t *testing.T) {
	meta := StateMeta{ID: "state-1", Serial: 8}
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(meta)
	})

	c := New(config.Config{Endpoint: srv.URL})
	got, err := c.PutState(context.Background(), "state-1", []byte(`{}`), false)
	if err != nil {
		t.Fatalf("unexpected err on 200: %v", err)
	}
	if got.Serial != 8 {
		t.Fatalf("expected serial 8, got %d", got.Serial)
	}
}

func TestPutState_OverwriteQueryParam(t *testing.T) {
	var gotOverwrite string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		gotOverwrite = r.URL.Query().Get("overwrite")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.PutState(context.Background(), "state-1", []byte(`{}`), true); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotOverwrite != "true" {
		t.Fatalf("expected overwrite=true in query, got %q", gotOverwrite)
	}
}

func TestPutState_NoOverwriteQueryParam(t *testing.T) {
	var gotOverwrite string
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		gotOverwrite = r.URL.Query().Get("overwrite")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if _, err := c.PutState(context.Background(), "state-1", []byte(`{}`), false); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if gotOverwrite != "" {
		t.Fatalf("expected no overwrite param, got %q", gotOverwrite)
	}
}

func TestPutState_ServerError(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})

	c := New(config.Config{Endpoint: srv.URL})
	_, err := c.PutState(context.Background(), "state-1", []byte(`{}`), false)
	if err == nil {
		t.Fatal("expected error on 409, got nil")
	}
}
