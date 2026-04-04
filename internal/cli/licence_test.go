package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newLicenseTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v2/license/applications/", handler)
	mux.HandleFunc("/api/v2/license/applications", handler)

	srv := &httptest.Server{Listener: l, Config: &http.Server{Handler: mux}}
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func runLicenseCmd(t *testing.T, srv *httptest.Server, args ...string) (string, error) {
	t.Helper()
	t.Setenv("KH_ENDPOINT", srv.URL)
	t.Setenv("KH_TOKEN", "test-token")

	buf := &bytes.Buffer{}
	cmd := newLicenseCmd()
	cmd.SetOut(buf)
	cmd.SetErr(io.Discard)
	cmd.SetContext(context.Background())
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestLicenseList_TableOutput(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/applications" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "app-1", "name": "Terraform Cloud", "short_name": "tfc", "vendor": "HashiCorp", "owner": "ops", "status": "active"},
		})
	})

	out, err := runLicenseCmd(t, srv, "ls")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "Terraform Cloud") {
		t.Errorf("expected name in output, got: %s", out)
	}
	if !strings.Contains(out, "HashiCorp") {
		t.Errorf("expected vendor in output, got: %s", out)
	}
}

func TestLicenseList_JSONOutput(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "app-1", "name": "Terraform Cloud", "short_name": "tfc", "vendor": "HashiCorp", "owner": "ops"},
		})
	})

	out, err := runLicenseCmd(t, srv, "ls", "-o", "json")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	var items []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &items); err != nil {
		t.Fatalf("expected valid JSON: %v\noutput: %s", err, out)
	}
	if len(items) != 1 || items[0]["name"] != "Terraform Cloud" {
		t.Errorf("unexpected items: %v", items)
	}
}

func TestLicenseCreate_SendsCorrectPayload(t *testing.T) {
	var bodyBytes []byte
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	})

	out, err := runLicenseCmd(t, srv, "create", "Terraform Cloud",
		"--short-name", "tfc",
		"--owner", "ops",
		"--vendor", "HashiCorp",
		"--tier", "Plus",
		"--renewal-date", "2027-01-01",
	)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "Terraform Cloud") {
		t.Errorf("expected name in output, got: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	app, _ := m["application"].(map[string]any)
	if app == nil {
		t.Fatalf("expected application wrapper in body, got: %s", bodyBytes)
	}
	if app["name"] != "Terraform Cloud" {
		t.Errorf("expected name in body, got: %s", bodyBytes)
	}
	if app["vendor"] != "HashiCorp" {
		t.Errorf("expected vendor in body, got: %s", bodyBytes)
	}
}

func TestLicenseCreate_RequiresFlags(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runLicenseCmd(t, srv, "create", "Terraform Cloud")
	if err == nil {
		t.Fatal("expected error when required flags missing")
	}
}

func TestLicenseUpdate_SendsCorrectPayload(t *testing.T) {
	var bodyBytes []byte
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	})

	out, err := runLicenseCmd(t, srv, "update", "app-1", "--status", "disabled")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "updated") {
		t.Errorf("expected update confirmation in output, got: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	app, _ := m["application"].(map[string]any)
	if app["status"] != "disabled" {
		t.Errorf("expected status=disabled in body, got: %s", bodyBytes)
	}
}

func TestLicenseUpdate_RequiresAtLeastOneFlag(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})

	_, err := runLicenseCmd(t, srv, "update", "app-1")
	if err == nil {
		t.Fatal("expected error when no flags provided")
	}
}

func TestLicenseDelete_RequiresForce(t *testing.T) {
	var deleteCalled bool
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, _ = runLicenseCmd(t, srv, "delete", "app-1")
	if deleteCalled {
		t.Fatal("DELETE should not be called without --force")
	}
}

func TestLicenseDelete_WithForce(t *testing.T) {
	var deleteCalled bool
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, err := runLicenseCmd(t, srv, "delete", "app-1", "--force")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected DELETE to be called with --force")
	}
}
