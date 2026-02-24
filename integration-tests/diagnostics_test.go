//go:build integration

package integrationtests

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// TestDiagnostics runs non-destructive health checks against a live KeyHarbour
// backend. It verifies connectivity, authentication, and core read operations.
// Set KH_DIAG_STATE_ID to also exercise lock/unlock.
//
// Required env: KH_ENDPOINT, KH_TOKEN
// Mode guard:   KH_TEST_MODE=diagnostics
func TestDiagnostics(t *testing.T) {
	if os.Getenv("KH_TEST_MODE") != "diagnostics" {
		t.Skip("set KH_TEST_MODE=diagnostics to run")
	}
	requireEnv(t, "KH_ENDPOINT", "KH_TOKEN")

	kh := khBin(t)
	endpoint := os.Getenv("KH_ENDPOINT")

	t.Run("Connectivity", func(t *testing.T) {
		host := hostPort(endpoint)
		conn, err := tls.DialWithDialer(
			&net.Dialer{Timeout: 5 * time.Second},
			"tcp", host, nil,
		)
		if err != nil {
			t.Fatalf("TLS connectivity to %s failed: %v", host, err)
		}
		conn.Close()
		t.Logf("connected to %s", host)
	})

	t.Run("Authentication", func(t *testing.T) {
		out := timedRun(t, 5*time.Second, kh, "whoami")
		if len(strings.TrimSpace(string(out))) == 0 {
			t.Fatal("whoami returned empty output")
		}
		t.Logf("identity:\n%s", strings.TrimSpace(string(out)))
	})

	t.Run("StateListing", func(t *testing.T) {
		out := timedRun(t, 10*time.Second, kh, "state", "ls", "-o", "json")
		var states []map[string]any
		if err := json.Unmarshal(out, &states); err != nil {
			t.Fatalf("state ls returned invalid JSON: %v\noutput: %s", err, out)
		}
		t.Logf("found %d states", len(states))
	})

	t.Run("LockRoundTrip", func(t *testing.T) {
		stateID := os.Getenv("KH_DIAG_STATE_ID")
		if stateID == "" {
			t.Skip("set KH_DIAG_STATE_ID to test lock/unlock round-trip")
		}
		runOK(t, kh, "lock", stateID)
		t.Cleanup(func() {
			// Force-unlock on cleanup so a test failure never leaves state locked.
			if out, err := runCmd(t, kh, "unlock", "--force", stateID); err != nil {
				t.Logf("cleanup unlock failed: %v\n%s", err, out)
			}
		})
		runOK(t, kh, "unlock", stateID)
	})
}

// hostPort extracts host:port from an endpoint URL, defaulting to port 443.
func hostPort(rawURL string) string {
	rawURL = strings.TrimPrefix(rawURL, "https://")
	rawURL = strings.TrimPrefix(rawURL, "http://")
	rawURL = strings.SplitN(rawURL, "/", 2)[0]
	if !strings.Contains(rawURL, ":") {
		rawURL += ":443"
	}
	return rawURL
}
