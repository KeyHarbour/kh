//go:build integration

package integrationtests

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// khBin returns the path to the pre-built kh binary, failing the test if not found.
func khBin(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	bin := filepath.Join(filepath.Dir(file), "..", "bin", "kh")
	if _, err := os.Stat(bin); err != nil {
		t.Fatal("bin/kh not found — run `make build` first")
	}
	return bin
}

// requireEnv skips the test if any of the named environment variables are unset.
func requireEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if os.Getenv(k) == "" {
			t.Skipf("skipping: %s not set", k)
		}
	}
}

// runCmd runs the kh binary with args and returns combined stdout+stderr.
func runCmd(t *testing.T, bin string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}

// runOK runs a kh command and fails the test immediately if it exits non-zero.
func runOK(t *testing.T, bin string, args ...string) []byte {
	t.Helper()
	out, err := runCmd(t, bin, args...)
	if err != nil {
		t.Fatalf("kh %v failed: %v\noutput:\n%s", args, err, out)
	}
	return out
}

// timedRun runs a kh command, logs a warning if it exceeds maxDuration, and
// fails if the command itself fails.
func timedRun(t *testing.T, maxDuration time.Duration, bin string, args ...string) []byte {
	t.Helper()
	start := time.Now()
	out := runOK(t, bin, args...)
	if elapsed := time.Since(start); elapsed > maxDuration {
		t.Logf("WARNING: kh %v took %v (threshold: %v)", args, elapsed.Round(time.Millisecond), maxDuration)
	}
	return out
}

// captureJSON runs kh with the supplied args, expects success, and writes
// stdout to dir/filename, creating intermediate directories as needed.
func captureJSON(t *testing.T, bin, dir, filename string, args ...string) {
	t.Helper()
	out := runOK(t, bin, args...)
	path := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// loadJSON reads path and unmarshals its contents into a value of type T.
func loadJSON[T any](t *testing.T, path string) T {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var v T
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return v
}

// snapshotDir creates a timestamped directory under KH_SNAPSHOT_DIR (or the
// default testdata/snapshots path), writes a manifest.json, and updates the
// "latest" symlink. Returns the directory path.
func snapshotDir(t *testing.T) string {
	t.Helper()
	base := os.Getenv("KH_SNAPSHOT_DIR")
	if base == "" {
		_, file, _, _ := runtime.Caller(0)
		base = filepath.Join(filepath.Dir(file), "..", "testdata", "snapshots")
	}
	ts := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	dir := filepath.Join(base, ts)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir snapshot dir %s: %v", dir, err)
	}
	manifest := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"endpoint":  os.Getenv("KH_ENDPOINT"),
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0o644)

	// Symlink target must be relative to the symlink's own directory so the
	// OS resolves it correctly. filepath.Base(dir) is just the timestamp name.
	latest := filepath.Join(base, "latest")
	_ = os.Remove(latest)
	if err := os.Symlink(filepath.Base(dir), latest); err != nil {
		t.Logf("warning: could not update latest symlink: %v", err)
	}
	return dir
}

// sha256File returns the hex-encoded SHA-256 digest of the file at path.
func sha256File(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return sha256Hex(b)
}

// sha256Hex returns the hex-encoded SHA-256 digest of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// idSet converts a slice of state maps into a set of their "id" values.
func idSet(states []map[string]any) map[string]bool {
	m := make(map[string]bool, len(states))
	for _, s := range states {
		if id, ok := s["id"].(string); ok && id != "" {
			m[id] = true
		}
	}
	return m
}

// assertField fails the test if snap[field] != got[field].
func assertField(t *testing.T, snap, got map[string]any, field string) {
	t.Helper()
	sv := fmt.Sprint(snap[field])
	gv := fmt.Sprint(got[field])
	if sv != gv {
		t.Errorf("field %q: snapshot=%q live=%q", field, sv, gv)
	}
}
