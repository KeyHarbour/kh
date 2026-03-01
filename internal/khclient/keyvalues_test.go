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

func TestListKeyValues(t *testing.T) {
	var called bool
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/workspaces/ws/keyvalues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"key":"FOO","value":"bar","expires_at":null,"private":false},{"key":"SECRET","value":"s3cr3t","expires_at":null,"private":true}]`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	items, err := c.ListKeyValues(context.Background(), "ws")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !called {
		t.Fatal("server not called")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Key != "FOO" || items[0].Value != "bar" || items[0].Private {
		t.Fatalf("unexpected item[0]: %+v", items[0])
	}
	if items[1].Key != "SECRET" || !items[1].Private {
		t.Fatalf("unexpected item[1]: %+v", items[1])
	}
}

func TestGetKeyValue(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/keyvalues/MY_KEY" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"value":"hello","expires_at":null,"private":false}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	kv, err := c.GetKeyValue(context.Background(), "MY_KEY")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if kv.Key != "MY_KEY" {
		t.Fatalf("expected key to be set to MY_KEY, got %q", kv.Key)
	}
	if kv.Value != "hello" {
		t.Fatalf("expected value hello, got %q", kv.Value)
	}
}

func TestGetKeyValue_RequiresKey(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	_, err := c.GetKeyValue(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestCreateKeyValue(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/workspaces/ws/keyvalues" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"accepted"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.CreateKeyValue(context.Background(), "ws", CreateKeyValueRequest{
		Key:   "NEW_KEY",
		Value: "new-value",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !strings.Contains(string(bodyBytes), `"key":"NEW_KEY"`) {
		t.Fatalf("expected key in body, got: %s", bodyBytes)
	}
}

func TestUpdateKeyValue(t *testing.T) {
	var bodyBytes []byte
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if r.URL.Path != "/keyvalues/MY_KEY" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"updated"}`)
	})

	c := New(config.Config{Endpoint: srv.URL})
	err := c.UpdateKeyValue(context.Background(), "MY_KEY", UpdateKeyValueRequest{Value: "updated"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	if m["value"] != "updated" {
		t.Fatalf("expected value=updated in body, got: %s", bodyBytes)
	}
}

func TestDeleteKeyValue(t *testing.T) {
	var hits int
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/keyvalues/MY_KEY" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})

	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteKeyValue(context.Background(), "MY_KEY"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if hits != 1 {
		t.Fatalf("expected 1 DELETE, got %d", hits)
	}
}

func TestDeleteKeyValue_RequiresKey(t *testing.T) {
	srv := newIPv4Server(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	c := New(config.Config{Endpoint: srv.URL})
	if err := c.DeleteKeyValue(context.Background(), ""); err == nil {
		t.Fatal("expected error for empty key")
	}
}
