//go:build integration

package integrationtests

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLicenseDiagnostics exercises read-only license operations against a live backend.
//
// Required env: KH_ENDPOINT, KH_TOKEN
// Mode guard:   KH_TEST_MODE=diagnostics
func TestLicenseDiagnostics(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "diagnostics" {
		t.Skip("set KH_TEST_MODE=diagnostics to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN")

	kh := khBin(t)

	t.Run("ListLicenses", func(t *testing.T) {
		out := timedRun(t, 10*time.Second, kh, "license", "ls", "-o", "json")
		var items []map[string]any
		if err := json.Unmarshal(out, &items); err != nil {
			t.Fatalf("license ls returned invalid JSON: %v\noutput: %s", err, out)
		}
		t.Logf("found %d license record(s)", len(items))
	})
}

// TestLicenseRoundTrip exercises the full create → show → update → delete lifecycle.
// Creates a uniquely named license record, verifies it, updates its status, then deletes it.
// Safe to run against a real backend as it cleans up after itself.
//
// Required env: KH_ENDPOINT, KH_TOKEN
// Mode guard:   KH_TEST_MODE=diagnostics
func TestLicenseRoundTrip(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "diagnostics" {
		t.Skip("set KH_TEST_MODE=diagnostics to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN")

	kh := khBin(t)

	// Use a timestamped unique name to avoid collisions.
	name := fmt.Sprintf("CLI Test License %d", time.Now().UnixMilli())
	shortName := fmt.Sprintf("clt%d", time.Now().UnixMilli()%100000)

	var uuid string

	t.Cleanup(func() {
		if uuid == "" {
			return
		}
		if out, err := runCmd(t, kh, "license", "delete", uuid, "--force"); err != nil {
			t.Logf("cleanup delete failed: %v\n%s", err, out)
		}
	})

	t.Run("Create", func(t *testing.T) {
		out := runOK(t, kh,
			"license", "create", name,
			"--short-name", shortName,
			"--owner", "cli-test",
			"--vendor", "TestVendor",
			"--tier", "Enterprise",
			"--renewal-date", "2027-01-01",
		)
		if !strings.Contains(string(out), name) {
			t.Fatalf("expected license name in output, got: %s", out)
		}
		t.Logf("created license %q", name)
	})

	t.Run("FindUUID", func(t *testing.T) {
		out := timedRun(t, 10*time.Second, kh, "license", "ls", "-o", "json")
		var items []map[string]any
		if err := json.Unmarshal(out, &items); err != nil {
			t.Fatalf("license ls returned invalid JSON: %v\noutput: %s", err, out)
		}
		for _, item := range items {
			if item["name"] == name {
				uuid, _ = item["uuid"].(string)
				break
			}
		}
		if uuid == "" {
			t.Fatalf("could not find UUID for license %q in ls output", name)
		}
		t.Logf("found UUID %s for license %q", uuid, name)
	})

	t.Run("Show", func(t *testing.T) {
		if uuid == "" {
			t.Skip("no UUID available — FindUUID failed")
		}
		out := timedRun(t, 10*time.Second, kh, "license", "show", uuid)
		var app map[string]any
		if err := json.Unmarshal(out, &app); err != nil {
			t.Fatalf("license show returned invalid JSON: %v\noutput: %s", err, out)
		}
		if app["name"] != name {
			t.Fatalf("expected name=%q, got: %v", name, app["name"])
		}
		t.Logf("verified license %q (uuid: %s)", name, uuid)
	})

	t.Run("Update", func(t *testing.T) {
		if uuid == "" {
			t.Skip("no UUID available — FindUUID failed")
		}
		runOK(t, kh, "license", "update", uuid, "--status", "disabled")

		out := timedRun(t, 10*time.Second, kh, "license", "show", uuid)
		var app map[string]any
		if err := json.Unmarshal(out, &app); err != nil {
			t.Fatalf("license show after update invalid JSON: %v\noutput: %s", err, out)
		}
		if app["status"] != "disabled" {
			t.Fatalf("expected status=disabled after update, got: %v", app["status"])
		}
		t.Logf("updated license %q status to disabled", name)
	})

	t.Run("Delete", func(t *testing.T) {
		if uuid == "" {
			t.Skip("no UUID available — FindUUID failed")
		}
		out := runOK(t, kh, "license", "delete", uuid, "--force")
		if !strings.Contains(string(out), uuid) {
			t.Fatalf("expected uuid in delete output, got: %s", out)
		}
		uuid = "" // prevent double-delete in cleanup
		t.Logf("deleted license %q", name)
	})
}
