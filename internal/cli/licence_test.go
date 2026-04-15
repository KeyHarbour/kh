package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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

func newLicenseFullTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"uuid": "app-uuid-1", "name": "Terraform Cloud", "short_name": "tfc"})
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

func TestLicenseShow(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/applications/app-1" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"uuid": "app-1", "name": "Terraform Cloud", "vendor": "HashiCorp",
		})
	})

	out, err := runLicenseCmd(t, srv, "show", "app-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "Terraform Cloud") {
		t.Errorf("expected name in output, got: %s", out)
	}
}

func TestLicenseImport_CreatesApplications(t *testing.T) {
	var names []string
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		app, _ := body["application"].(map[string]any)
		if app != nil {
			names = append(names, app["name"].(string))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"uuid": "app-uuid-x", "name": ""})
	})

	f, err := os.CreateTemp(t.TempDir(), "apps-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("name,short_name,owner,vendor\n")
	f.WriteString("Terraform Cloud,tfc,ops,HashiCorp\n")
	f.WriteString("GitHub,gh,eng,GitHub\n")
	f.Close()

	out, err := runLicenseCmd(t, srv, "import", f.Name())
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 POSTs, got %d", len(names))
	}
	if !strings.Contains(out, "2 created") {
		t.Errorf("expected summary in output, got: %s", out)
	}
}

func TestLicenseImport_SkipsInvalidRows(t *testing.T) {
	var postCount int
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		postCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"uuid": "app-uuid-x", "name": ""})
	})

	f, err := os.CreateTemp(t.TempDir(), "apps-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("name,short_name,owner,vendor\n")
	f.WriteString("Terraform Cloud,tfc,ops,HashiCorp\n")
	f.WriteString(",missing-name,ops,Vendor\n") // should be skipped
	f.Close()

	out, err := runLicenseCmd(t, srv, "import", f.Name())
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if postCount != 1 {
		t.Errorf("expected 1 POST, got %d", postCount)
	}
	if !strings.Contains(out, "1 created, 1 skipped") {
		t.Errorf("expected summary in output, got: %s", out)
	}
}

// ── instance ──────────────────────────────────────────────────────────────────

func TestLicenseInstanceList(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/applications/app-1/instances" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "inst-1", "name": "Production", "short_name": "prod", "status": "active"},
		})
	})

	out, err := runLicenseCmd(t, srv, "instance", "ls", "app-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "Production") {
		t.Errorf("expected instance name in output, got: %s", out)
	}
}

func TestLicenseInstanceShow(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/instances/inst-1" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "inst-1", "name": "Production"})
	})

	out, err := runLicenseCmd(t, srv, "instance", "show", "inst-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "inst-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
}

func TestLicenseInstanceCreate(t *testing.T) {
	var bodyBytes []byte
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/license/applications/app-1/instances" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"uuid": "inst-uuid-1", "name": "Production", "short_name": "prod"})
	})

	out, err := runLicenseCmd(t, srv, "instance", "create", "app-1", "Production", "--short-name", "prod")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "Production") {
		t.Errorf("expected name in output, got: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	inst, _ := m["instance"].(map[string]any)
	if inst["name"] != "Production" {
		t.Errorf("expected name in body, got: %s", bodyBytes)
	}
}

func TestLicenseInstanceUpdate(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v2/license/instances/inst-1" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{})
	})

	out, err := runLicenseCmd(t, srv, "instance", "update", "inst-1", "--status", "disabled")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "inst-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
}

func TestLicenseInstanceDelete_RequiresForce(t *testing.T) {
	var deleteCalled bool
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, _ = runLicenseCmd(t, srv, "instance", "delete", "inst-1")
	if deleteCalled {
		t.Fatal("DELETE should not be called without --force")
	}
}

func TestLicenseInstanceDelete_WithForce(t *testing.T) {
	var deleteCalled bool
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, err := runLicenseCmd(t, srv, "instance", "delete", "inst-1", "--force")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected DELETE to be called with --force")
	}
}

// ── licensee ──────────────────────────────────────────────────────────────────

func TestLicenseLicenseeList(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/instances/inst-1/licensees" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "member-1", "status": "active"},
		})
	})

	out, err := runLicenseCmd(t, srv, "licensee", "ls", "inst-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "member-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
}

func TestLicenseLicenseeShow(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/licensees/member-1" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "member-1", "status": "active"})
	})

	out, err := runLicenseCmd(t, srv, "licensee", "show", "member-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "member-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
}

func TestLicenseLicenseeAdd(t *testing.T) {
	var bodyBytes []byte
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/license/instances/inst-1/licensees" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{})
	})

	out, err := runLicenseCmd(t, srv, "licensee", "add", "inst-1", "member-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "member-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	l, _ := m["licensee"].(map[string]any)
	if l["uuid"] != "member-1" {
		t.Errorf("expected uuid in body, got: %s", bodyBytes)
	}
}

func TestLicenseLicenseeUpdate(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v2/license/licensees/member-1" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{})
	})

	out, err := runLicenseCmd(t, srv, "licensee", "update", "member-1", "--status", "disabled")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "member-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
}

func TestLicenseLicenseeDelete_RequiresForce(t *testing.T) {
	var deleteCalled bool
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, _ = runLicenseCmd(t, srv, "licensee", "delete", "member-1")
	if deleteCalled {
		t.Fatal("DELETE should not be called without --force")
	}
}

func TestLicenseLicenseeDelete_WithForce(t *testing.T) {
	var deleteCalled bool
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, err := runLicenseCmd(t, srv, "licensee", "delete", "member-1", "--force")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected DELETE to be called with --force")
	}
}

// ── team-member ───────────────────────────────────────────────────────────────

func TestLicenseTeamMemberList(t *testing.T) {
	mgr := "mgr-1"
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/team_members" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "tm-1", "manager_uuid": mgr},
		})
	})

	out, err := runLicenseCmd(t, srv, "team-member", "ls")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "tm-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
	if !strings.Contains(out, "mgr-1") {
		t.Errorf("expected manager uuid in output, got: %s", out)
	}
}

func TestLicenseTeamMemberShow(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/team_members/tm-1" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "tm-1"})
	})

	out, err := runLicenseCmd(t, srv, "team-member", "show", "tm-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "tm-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
}

func TestLicenseTeamMemberAdd(t *testing.T) {
	var bodyBytes []byte
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/license/team_members" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{})
	})

	out, err := runLicenseCmd(t, srv, "team-member", "add", "tm-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "tm-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	tm, _ := m["team_member"].(map[string]any)
	if tm["uuid"] != "tm-1" {
		t.Errorf("expected uuid in body, got: %s", bodyBytes)
	}
}

func TestLicenseTeamMemberUpdate(t *testing.T) {
	var bodyBytes []byte
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/v2/license/team_members/tm-1" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		bodyBytes, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{})
	})

	out, err := runLicenseCmd(t, srv, "team-member", "update", "tm-1", "--manager-uuid", "mgr-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "tm-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
	var m map[string]any
	if err := json.Unmarshal(bodyBytes, &m); err != nil {
		t.Fatalf("invalid body JSON: %v", err)
	}
	tm, _ := m["team_member"].(map[string]any)
	if tm["manager_uuid"] != "mgr-1" {
		t.Errorf("expected manager_uuid in body, got: %s", bodyBytes)
	}
}

func TestLicenseTeamMemberDelete_RequiresForce(t *testing.T) {
	var deleteCalled bool
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, _ = runLicenseCmd(t, srv, "team-member", "delete", "tm-1")
	if deleteCalled {
		t.Fatal("DELETE should not be called without --force")
	}
}

func TestLicenseTeamMemberDelete_WithForce(t *testing.T) {
	var deleteCalled bool
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			deleteCalled = true
		}
		w.WriteHeader(http.StatusNoContent)
	})

	_, err := runLicenseCmd(t, srv, "team-member", "delete", "tm-1", "--force")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected DELETE to be called with --force")
	}
}

// ── team-member import ────────────────────────────────────────────────────────

func TestLicenseUsersList(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/team_members" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "tm-1"},
		})
	})

	out, err := runLicenseCmd(t, srv, "team-member", "ls")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "tm-1") {
		t.Errorf("expected uuid in output, got: %s", out)
	}
}

func TestLicenseUsersImport_CreatesMembers(t *testing.T) {
	var requests []map[string]any
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/license/team_members":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			requests = append(requests, body)
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]string{})
		case r.Method == http.MethodPatch:
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]string{})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	})

	f, err := os.CreateTemp(t.TempDir(), "members-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("uuid,manager_uuid\n")
	f.WriteString("tm-1,mgr-1\n")
	f.WriteString("tm-2,\n")
	f.Close()

	out, err := runLicenseCmd(t, srv, "team-member", "import", f.Name())
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if len(requests) != 2 {
		t.Errorf("expected 2 POSTs, got %d", len(requests))
	}
	if !strings.Contains(out, "2 created") {
		t.Errorf("expected summary in output, got: %s", out)
	}
}

func TestLicenseUsersImport_SkipsMissingUUID(t *testing.T) {
	var postCount int
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{})
	})

	f, err := os.CreateTemp(t.TempDir(), "members-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("uuid,manager_uuid\n")
	f.WriteString("tm-1,\n")
	f.WriteString(",mgr-1\n") // missing uuid — should be skipped
	f.Close()

	out, err := runLicenseCmd(t, srv, "team-member", "import", f.Name())
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if postCount != 1 {
		t.Errorf("expected 1 POST, got %d", postCount)
	}
	if !strings.Contains(out, "1 created, 1 skipped") {
		t.Errorf("expected summary in output, got: %s", out)
	}
}

// ── license ls filters ────────────────────────────────────────────────────────

func TestLicenseList_FilterByVendor(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "a1", "name": "App A", "short_name": "aa", "vendor": "HashiCorp", "owner": "ops"},
			{"uuid": "a2", "name": "App B", "short_name": "ab", "vendor": "GitHub", "owner": "eng"},
		})
	})
	out, err := runLicenseCmd(t, srv, "ls", "--vendor", "HashiCorp")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "App A") {
		t.Errorf("expected App A in output, got: %s", out)
	}
	if strings.Contains(out, "App B") {
		t.Errorf("expected App B to be filtered out, got: %s", out)
	}
}

func TestLicenseList_FilterByStatus(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "a1", "name": "Active App", "short_name": "aa", "vendor": "Acme", "owner": "ops", "status": "active"},
			{"uuid": "a2", "name": "Archived App", "short_name": "ab", "vendor": "Acme", "owner": "ops", "status": "archived"},
		})
	})
	out, err := runLicenseCmd(t, srv, "ls", "--status", "active")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "Active App") {
		t.Errorf("expected Active App in output, got: %s", out)
	}
	if strings.Contains(out, "Archived App") {
		t.Errorf("expected Archived App to be filtered out, got: %s", out)
	}
}

func TestLicenseList_FilterByRenewalBefore(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "a1", "name": "Expiring Soon", "short_name": "es", "vendor": "Acme", "owner": "ops", "renewal_date": "2026-06-01"},
			{"uuid": "a2", "name": "Far Future", "short_name": "ff", "vendor": "Acme", "owner": "ops", "renewal_date": "2028-01-01"},
			{"uuid": "a3", "name": "No Date", "short_name": "nd", "vendor": "Acme", "owner": "ops"},
		})
	})
	out, err := runLicenseCmd(t, srv, "ls", "--renewal-before", "2027-01-01")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "Expiring Soon") {
		t.Errorf("expected Expiring Soon in output, got: %s", out)
	}
	if strings.Contains(out, "Far Future") {
		t.Errorf("expected Far Future to be filtered out, got: %s", out)
	}
	if strings.Contains(out, "No Date") {
		t.Errorf("expected No Date (empty renewal) to be filtered out, got: %s", out)
	}
}

func TestLicenseList_FilterByRenewalBefore_InvalidDate(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{})
	})
	_, err := runLicenseCmd(t, srv, "ls", "--renewal-before", "not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid date format")
	}
}

// ── license export ────────────────────────────────────────────────────────────

func TestLicenseExport_WritesCSV(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "a1", "name": "Terraform Cloud", "short_name": "tfc", "owner": "ops", "vendor": "HashiCorp", "renewal_date": "2027-01-01", "status": "active"},
		})
	})
	out, err := runLicenseCmd(t, srv, "export")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if !strings.Contains(out, "name,short_name") {
		t.Errorf("expected CSV header in output, got: %s", out)
	}
	if !strings.Contains(out, "Terraform Cloud") {
		t.Errorf("expected app name in output, got: %s", out)
	}
	if !strings.Contains(out, "HashiCorp") {
		t.Errorf("expected vendor in output, got: %s", out)
	}
}

func TestLicenseExport_ToFile(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "a1", "name": "GitHub", "short_name": "gh", "owner": "eng", "vendor": "GitHub"},
		})
	})
	outFile := t.TempDir() + "/export.csv"
	_, err := runLicenseCmd(t, srv, "export", "--out", outFile)
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("could not read output file: %v", err)
	}
	if !strings.Contains(string(data), "GitHub") {
		t.Errorf("expected GitHub in file, got: %s", data)
	}
}

// ── license import --dry-run ──────────────────────────────────────────────────

func TestLicenseImport_DryRun(t *testing.T) {
	var postCount int
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"uuid": "x"})
	})
	f, err := os.CreateTemp(t.TempDir(), "apps-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("name,short_name,owner,vendor\n")
	f.WriteString("Terraform Cloud,tfc,ops,HashiCorp\n")
	f.Close()
	out, err := runLicenseCmd(t, srv, "import", f.Name(), "--dry-run")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if postCount != 0 {
		t.Errorf("expected no POSTs in dry-run, got %d", postCount)
	}
	if !strings.Contains(out, "would create") {
		t.Errorf("expected 'would create' in output, got: %s", out)
	}
}

// ── license instance import ───────────────────────────────────────────────────

func TestLicenseInstanceImport_CreatesInstances(t *testing.T) {
	var names []string
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v2/license/applications/app-1/instances" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if inst, ok := body["instance"].(map[string]any); ok {
			names = append(names, inst["name"].(string))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"uuid": "inst-x", "name": ""})
	})
	f, err := os.CreateTemp(t.TempDir(), "instances-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("name,short_name,owner\n")
	f.WriteString("Production,prod,ops\n")
	f.WriteString("Staging,stg,ops\n")
	f.Close()
	out, err := runLicenseCmd(t, srv, "instance", "import", "app-1", f.Name())
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 POSTs, got %d", len(names))
	}
	if !strings.Contains(out, "2 created") {
		t.Errorf("expected summary in output, got: %s", out)
	}
}

func TestLicenseInstanceImport_DryRun(t *testing.T) {
	var postCount int
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"uuid": "x"})
	})
	f, err := os.CreateTemp(t.TempDir(), "instances-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("name,short_name\n")
	f.WriteString("Production,prod\n")
	f.Close()
	out, err := runLicenseCmd(t, srv, "instance", "import", "app-1", f.Name(), "--dry-run")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if postCount != 0 {
		t.Errorf("expected no POSTs in dry-run, got %d", postCount)
	}
	if !strings.Contains(out, "would create") {
		t.Errorf("expected 'would create' in output, got: %s", out)
	}
}

func TestLicenseInstanceImport_SkipsInvalidRows(t *testing.T) {
	var postCount int
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"uuid": "x"})
	})
	f, err := os.CreateTemp(t.TempDir(), "instances-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("name,short_name\n")
	f.WriteString("Production,prod\n")
	f.WriteString(",missing-name\n")
	f.Close()
	out, err := runLicenseCmd(t, srv, "instance", "import", "app-1", f.Name())
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if postCount != 1 {
		t.Errorf("expected 1 POST, got %d", postCount)
	}
	if !strings.Contains(out, "1 created, 1 skipped") {
		t.Errorf("expected summary in output, got: %s", out)
	}
}

// ── team-member import --dry-run ──────────────────────────────────────────────

func TestLicenseUsersImport_DryRun(t *testing.T) {
	var postCount int
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			postCount++
		}
		w.WriteHeader(http.StatusCreated)
	})
	f, err := os.CreateTemp(t.TempDir(), "members-*.csv")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("uuid\n")
	f.WriteString("tm-1\n")
	f.Close()
	out, err := runLicenseCmd(t, srv, "team-member", "import", f.Name(), "--dry-run")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	if postCount != 0 {
		t.Errorf("expected no POSTs in dry-run, got %d", postCount)
	}
	if !strings.Contains(out, "would create") {
		t.Errorf("expected 'would create' in output, got: %s", out)
	}
}

// ── show table output ─────────────────────────────────────────────────────────

func TestLicenseShow_TableOutput(t *testing.T) {
	srv := newLicenseTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"uuid": "app-1", "name": "Terraform Cloud", "short_name": "tfc",
			"vendor": "HashiCorp", "owner": "ops", "tier": "Plus",
			"renewal_date": "2027-01-01", "status": "active",
		})
	})

	out, err := runLicenseCmd(t, srv, "show", "app-1", "-o", "table")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, want := range []string{"Terraform Cloud", "HashiCorp", "ops", "Plus", "2027-01-01", "active"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in table output, got: %s", want, out)
		}
	}
}

func TestLicenseInstanceShow_TableOutput(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/license/instances/inst-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"uuid": "inst-1", "name": "Production", "short_name": "prod",
			"owner": "ops", "renewal_date": "2027-06-01", "status": "active",
		})
	})

	out, err := runLicenseCmd(t, srv, "instance", "show", "inst-1", "-o", "table")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, want := range []string{"inst-1", "Production", "prod", "ops", "2027-06-01", "active"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in table output, got: %s", want, out)
		}
	}
}

func TestLicenseLicenseeShow_TableOutput(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/license/licensees/member-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"uuid": "member-1", "name": "Alice", "email": "alice@example.com", "status": "active",
		})
	})

	out, err := runLicenseCmd(t, srv, "licensee", "show", "member-1", "-o", "table")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, want := range []string{"member-1", "Alice", "alice@example.com", "active"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in table output, got: %s", want, out)
		}
	}
}

func TestLicenseTeamMemberShow_TableOutput(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/license/team_members/tm-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uuid": "tm-1", "manager_uuid": "mgr-1"})
	})

	out, err := runLicenseCmd(t, srv, "team-member", "show", "tm-1", "-o", "table")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, want := range []string{"tm-1", "mgr-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in table output, got: %s", want, out)
		}
	}
}

// ── licensee ls name/email columns ───────────────────────────────────────────

func TestLicenseLicenseeList_ShowsNameEmail(t *testing.T) {
	srv := newLicenseFullTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v2/license/instances/inst-1/licensees" {
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]any{
			{"uuid": "m-1", "name": "Alice", "email": "alice@example.com", "status": "active"},
			{"uuid": "m-2", "status": "disabled"},
		})
	})

	out, err := runLicenseCmd(t, srv, "licensee", "ls", "inst-1")
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}
	for _, want := range []string{"Alice", "alice@example.com", "active", "m-2", "disabled"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %s", want, out)
		}
	}
	// Row with no name/email should display dash placeholders.
	if !strings.Contains(out, "-") {
		t.Errorf("expected dash placeholder for missing name/email, got: %s", out)
	}
}
