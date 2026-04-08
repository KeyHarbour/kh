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
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{})
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
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{})
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
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{})
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

// ── users (alias for team-member, with import) ────────────────────────────────

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

	out, err := runLicenseCmd(t, srv, "users", "ls")
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

	out, err := runLicenseCmd(t, srv, "users", "import", f.Name())
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

	out, err := runLicenseCmd(t, srv, "users", "import", f.Name())
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
