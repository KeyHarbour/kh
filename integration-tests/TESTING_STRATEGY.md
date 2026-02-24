# KeyHarbour CLI — Integration & Non-Regression Testing Strategy

## Overview

Three complementary test modes cover the full lifecycle of KeyHarbour backend operations:

| Mode | When to run | Goal |
|------|-------------|------|
| **Snapshot** | Before a backend update | Record the exact observable state of the system |
| **Regression** | After a backend update | Diff the live system against the snapshot; surface any breakage |
| **Diagnostics** | On demand / scheduled | Verify connectivity, API health, and core operations without side effects |

All three modes share the same Go test infrastructure in `integration-tests/` and are driven by environment variables and build tags so they never execute accidentally in regular `go test ./...` runs.

---

## 1. Shared Conventions

### 1.1 Build Tags and Skip Guards

Every integration test file starts with a build constraint so it is excluded from the default test run:

```go
//go:build integration

package integrationtests
```

Run with:

```bash
go test -tags integration ./integration-tests/... -v
```

Each test also has an explicit skip guard for missing env:

```go
func requireEnv(t *testing.T, keys ...string) {
    t.Helper()
    for _, k := range keys {
        if os.Getenv(k) == "" {
            t.Skipf("skipping: %s not set", k)
        }
    }
}
```

### 1.2 Required Environment Variables

| Variable | Description |
|----------|-------------|
| `KH_ENDPOINT` | Backend URL, e.g. `https://api.keyharbour.test` |
| `KH_TOKEN` | Personal Access Token |
| `KH_ORG` | Organisation slug (optional, inferred from token) |
| `KH_PROJECT` | Project UUID for the test workspace |
| `KH_TEST_MODE` | `snapshot`, `regression`, or `diagnostics` |
| `KH_SNAPSHOT_DIR` | Directory where snapshots are written / read (default: `./testdata/snapshots`) |

### 1.3 The `kh` Binary

Tests always invoke the pre-built binary at `bin/kh` (resolved relative to the repo root) via `exec.Command`. This ensures tests exercise the real compiled binary, not individual internal packages.

```go
func khBin(t *testing.T) string {
    t.Helper()
    _, file, _, _ := runtime.Caller(0)
    bin := filepath.Join(filepath.Dir(file), "..", "bin", "kh")
    if _, err := os.Stat(bin); err != nil {
        t.Fatal("bin/kh not found — run `make build` first")
    }
    return bin
}
```

### 1.4 JSON Output for Assertions

Every command that produces data is called with `-o json`. This makes assertions deterministic and independent of table-formatting changes.

```bash
kh state ls -o json
kh whoami -o json
kh statefiles last --project <p> --workspace <w> -o json
```

---

## 2. Mode 1 — Snapshot (Pre-Update)

### Purpose

Capture a machine-readable picture of the system **before** any backend change (deployment, migration, config update). The snapshot becomes the ground truth for regression testing.

### What to snapshot

| Artifact | Command | Stored as |
|----------|---------|-----------|
| Auth identity | `kh whoami -o json` | `whoami.json` |
| State list | `kh state ls -o json` | `states.json` |
| Per-state metadata | `kh state show <id> --raw` | `states/<id>.json` |
| Statefiles last | `kh statefiles last -o json` for each workspace | `statefiles/<workspace>.json` |
| Lock status | inferred from state metadata | included in `states/<id>.json` |
| API version / health | `GET /v1/health` (if available) | `health.json` |

### Snapshot directory layout

```
testdata/snapshots/
  2026-02-20T14-05-00Z/        ← timestamped run
    manifest.json              ← run metadata (version, timestamp, env hash)
    whoami.json
    health.json
    states.json
    states/
      <uuid-1>.json
      <uuid-2>.json
    statefiles/
      <workspace-1>.json
      <workspace-2>.json
  latest -> 2026-02-20T14-05-00Z/   ← symlink updated after each snapshot
```

### Running

```bash
KH_ENDPOINT=https://api.keyharbour.test \
KH_TOKEN=<pat> \
KH_PROJECT=<uuid> \
KH_TEST_MODE=snapshot \
go test -tags integration ./integration-tests/... -run TestSnapshot -v
```

Or via Make (recommended):

```bash
make snapshot
```

### Key test: `TestSnapshot`

```go
// integration-tests/snapshot_test.go
//go:build integration

func TestSnapshot(t *testing.T) {
    if os.Getenv("KH_TEST_MODE") != "snapshot" {
        t.Skip("set KH_TEST_MODE=snapshot to run")
    }
    requireEnv(t, "KH_ENDPOINT", "KH_TOKEN", "KH_PROJECT")

    kh := khBin(t)
    dir := snapshotDir(t) // creates timestamped dir, writes manifest, updates symlink

    // 1. Auth check
    captureJSON(t, kh, dir, "whoami.json", "whoami", "-o", "json")

    // 2. State list
    captureJSON(t, kh, dir, "states.json", "state", "ls", "-o", "json")

    // 3. Per-state details
    var states []khclient.StateItem
    mustUnmarshal(t, filepath.Join(dir, "states.json"), &states)
    for _, s := range states {
        captureJSON(t, kh, dir, filepath.Join("states", s.ID+".json"),
            "state", "show", s.ID, "--raw")
    }

    t.Logf("Snapshot written to %s", dir)
}
```

`captureJSON` runs the command, asserts exit code 0, and writes stdout to the path.

---

## 3. Mode 2 — Regression (Post-Update)

### Purpose

After a backend change, run the same queries and diff the results against the stored snapshot. Fails if anything structurally changed that wasn't expected.

### What is compared

| Check | Pass condition |
|-------|---------------|
| `whoami` identity | Same `login` / `org` |
| State IDs | No IDs disappeared from `state ls` |
| State content | SHA-256 of each `state show --raw` matches snapshot |
| Statefiles last | Same `serial`, `lineage`, `terraform_version` |
| Lock status | Same locked/unlocked state per workspace |
| Exit codes | All commands exit 0 |
| HTTP status codes | No unexpected 4xx / 5xx in `-debug` output |

### Regression levels

Use an env variable `KH_REGRESSION_LEVEL` to control strictness:

| Level | Behaviour |
|-------|-----------|
| `strict` (default) | Any diff fails the test |
| `warn` | Diffs logged as warnings, test passes |
| `partial` | Only critical fields (IDs, serials, lineage) are compared |

### Running

```bash
KH_ENDPOINT=https://api.keyharbour.test \
KH_TOKEN=<pat> \
KH_PROJECT=<uuid> \
KH_TEST_MODE=regression \
KH_SNAPSHOT_DIR=./testdata/snapshots/latest \
go test -tags integration ./integration-tests/... -run TestRegression -v
```

### Key test: `TestRegression`

```go
// integration-tests/regression_test.go
//go:build integration

func TestRegression(t *testing.T) {
    if os.Getenv("KH_TEST_MODE") != "regression" {
        t.Skip("set KH_TEST_MODE=regression to run")
    }
    requireEnv(t, "KH_ENDPOINT", "KH_TOKEN", "KH_PROJECT", "KH_SNAPSHOT_DIR")

    kh := khBin(t)
    snapDir := os.Getenv("KH_SNAPSHOT_DIR")

    t.Run("WhoAmI", func(t *testing.T) {
        got := runJSON[map[string]any](t, kh, "whoami", "-o", "json")
        snap := loadJSON[map[string]any](t, filepath.Join(snapDir, "whoami.json"))
        assertField(t, snap, got, "login")
        assertField(t, snap, got, "organization")
    })

    t.Run("StateListNoDeletions", func(t *testing.T) {
        got := runJSON[[]khclient.StateItem](t, kh, "state", "ls", "-o", "json")
        snap := loadJSON[[]khclient.StateItem](t, filepath.Join(snapDir, "states.json"))
        gotIDs := idSet(got)
        for _, s := range snap {
            if !gotIDs[s.ID] {
                t.Errorf("state %s disappeared after update", s.ID)
            }
        }
    })

    t.Run("StateContentIntegrity", func(t *testing.T) {
        snap := loadJSON[[]khclient.StateItem](t, filepath.Join(snapDir, "states.json"))
        for _, s := range snap {
            t.Run(s.ID, func(t *testing.T) {
                snapFile := filepath.Join(snapDir, "states", s.ID+".json")
                if _, err := os.Stat(snapFile); err != nil {
                    t.Skipf("no snapshot for state %s", s.ID)
                }
                snapHash := sha256File(t, snapFile)
                live := runRaw(t, kh, "state", "show", s.ID, "--raw")
                liveHash := sha256Bytes(live)
                if snapHash != liveHash {
                    t.Errorf("state %s content changed: snapshot=%s live=%s", s.ID, snapHash, liveHash)
                }
            })
        }
    })
}
```

### Tips

- Commit the `testdata/snapshots/latest/` symlink but optionally gitignore the per-run timestamped directories.
- For secrets, store only non-sensitive fields (IDs, serials, lineage) and exclude raw token values from JSON.
- Consider a `--update-snapshot` flag (similar to Jest's `--updateSnapshot`) that rewrites the baseline with the current live data when a deliberate change is expected.

---

## 4. Mode 3 — Diagnostics

### Purpose

Run non-destructive health checks at any time — after an incident, before a release, or on a schedule — to verify that the backend and CLI are operating correctly. Diagnostics **do not mutate state**.

### Diagnostic checks

| Check | Description |
|-------|-------------|
| **Connectivity** | TCP reachability + TLS handshake to `KH_ENDPOINT` |
| **Authentication** | `kh whoami` returns 200 with correct identity |
| **State listing** | `kh state ls` succeeds and returns valid JSON |
| **State retrieval** | `kh state show <id>` for a known test fixture |
| **Lock round-trip** | Lock → unlock a test state (if `KH_DIAG_STATE_ID` is set) |
| **Latency** | Each command measured; warn if >2 s |
| **Exit codes** | All commands exit 0 without `KH_DEBUG` errors |
| **Backend version** | Compare `GET /v1/health` version against a minimum expected version |

### Running

```bash
KH_ENDPOINT=https://api.keyharbour.test \
KH_TOKEN=<pat> \
KH_TEST_MODE=diagnostics \
go test -tags integration ./integration-tests/... -run TestDiagnostics -v
```

Or via Make:

```bash
make diagnostics
```

### Key test: `TestDiagnostics`

```go
// integration-tests/diagnostics_test.go
//go:build integration

func TestDiagnostics(t *testing.T) {
    if os.Getenv("KH_TEST_MODE") != "diagnostics" {
        t.Skip("set KH_TEST_MODE=diagnostics to run")
    }
    requireEnv(t, "KH_ENDPOINT", "KH_TOKEN")

    kh := khBin(t)

    t.Run("Connectivity", func(t *testing.T) {
        endpoint := os.Getenv("KH_ENDPOINT")
        conn, err := tls.DialWithDialer(
            &net.Dialer{Timeout: 5 * time.Second},
            "tcp", hostPort(endpoint), nil,
        )
        if err != nil {
            t.Fatalf("TLS connectivity failed: %v", err)
        }
        conn.Close()
    })

    t.Run("Authentication", func(t *testing.T) {
        out := timedRun(t, 2*time.Second, kh, "whoami", "-o", "json")
        var identity map[string]any
        if err := json.Unmarshal(out, &identity); err != nil {
            t.Fatalf("whoami returned invalid JSON: %v", err)
        }
        if identity["login"] == nil {
            t.Error("whoami response missing 'login' field")
        }
    })

    t.Run("StateListing", func(t *testing.T) {
        out := timedRun(t, 5*time.Second, kh, "state", "ls", "-o", "json")
        var states []map[string]any
        if err := json.Unmarshal(out, &states); err != nil {
            t.Fatalf("state ls returned invalid JSON: %v", err)
        }
        t.Logf("Found %d states", len(states))
    })

    t.Run("LockRoundTrip", func(t *testing.T) {
        stateID := os.Getenv("KH_DIAG_STATE_ID")
        if stateID == "" {
            t.Skip("set KH_DIAG_STATE_ID to test lock/unlock")
        }
        runOK(t, kh, "lock", stateID)
        t.Cleanup(func() { runOK(t, kh, "unlock", stateID) }) // always unlock
        runOK(t, kh, "unlock", stateID)
    })
}
```

---

## 5. File Structure

```
integration-tests/
  README.md                        ← existing overview
  TESTING_STRATEGY.md              ← this document
  tfc_to_kh_migration_test.go      ← existing migration test
  snapshot_test.go                 ← Mode 1: snapshot collection
  regression_test.go               ← Mode 2: regression comparison
  diagnostics_test.go              ← Mode 3: health checks
  helpers_test.go                  ← shared helpers (khBin, captureJSON, runJSON, …)
testdata/
  snapshots/
    .gitkeep
    latest -> ...                  ← symlink to most recent snapshot
```

---

## 6. Makefile Targets

Add these targets to the root `Makefile`:

```makefile
# Integration test modes
.PHONY: snapshot regression diagnostics

snapshot: build
	KH_TEST_MODE=snapshot \
	go test -tags integration ./integration-tests/... -run TestSnapshot -v -count=1

regression: build
	KH_TEST_MODE=regression \
	KH_SNAPSHOT_DIR=./testdata/snapshots/latest \
	go test -tags integration ./integration-tests/... -run TestRegression -v -count=1

diagnostics: build
	KH_TEST_MODE=diagnostics \
	go test -tags integration ./integration-tests/... -run TestDiagnostics -v -count=1
```

---

## 7. CI/CD Integration

### Pre-deployment pipeline step (snapshot)

```yaml
# .github/workflows/pre-deploy.yml
- name: Collect pre-deploy snapshot
  env:
    KH_ENDPOINT: ${{ vars.KH_ENDPOINT }}
    KH_TOKEN: ${{ secrets.KH_TOKEN }}
    KH_PROJECT: ${{ vars.KH_PROJECT }}
    KH_TEST_MODE: snapshot
  run: make snapshot

- name: Upload snapshot artifact
  uses: actions/upload-artifact@v4
  with:
    name: kh-snapshot-${{ github.run_id }}
    path: integration-tests/testdata/snapshots/
```

### Post-deployment pipeline step (regression)

```yaml
# .github/workflows/post-deploy.yml
- name: Download pre-deploy snapshot
  uses: actions/download-artifact@v4
  with:
    name: kh-snapshot-${{ github.run_id }}
    path: integration-tests/testdata/snapshots/

- name: Run regression tests
  env:
    KH_ENDPOINT: ${{ vars.KH_ENDPOINT }}
    KH_TOKEN: ${{ secrets.KH_TOKEN }}
    KH_PROJECT: ${{ vars.KH_PROJECT }}
    KH_TEST_MODE: regression
    KH_SNAPSHOT_DIR: ./testdata/snapshots/latest
  run: make regression
```

### Scheduled diagnostics job

```yaml
# .github/workflows/scheduled-diagnostics.yml
on:
  schedule:
    - cron: '0 * * * *'   # every hour

jobs:
  diagnostics:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: make build
      - name: Run diagnostics
        env:
          KH_ENDPOINT: ${{ vars.KH_ENDPOINT }}
          KH_TOKEN: ${{ secrets.KH_TOKEN }}
          KH_TEST_MODE: diagnostics
        run: make diagnostics
```

---

## 8. Design Decisions and Rationale

### Why invoke the binary instead of testing packages directly?

Integration tests exercise the full stack — compilation, flag parsing, config loading, HTTP client, serialisation. Testing individual packages separately (unit tests) cannot catch regressions that only surface at the boundary between these layers.

### Why JSON output for assertions?

- Immune to table-formatting changes.
- Easily diffed and stored as snapshot artifacts.
- Allows targeted field-level assertions rather than brittle string matching.

### Why timestamped snapshot directories?

- Immutable history: you can compare across multiple releases.
- Easy rollback: point `KH_SNAPSHOT_DIR` at any past run.
- Small storage overhead: store only JSON text, not binary state blobs (unless you need content integrity checks).

### Why a separate `KH_TEST_MODE` variable instead of separate binaries?

A single test binary with guarded subtests is simpler to maintain. The mode variable makes the intent explicit in CI logs and prevents accidental mutations (regression/diagnostics never write to the backend).

### Snapshot sensitivity

State content snapshots (SHA-256 comparison) are the strictest check. Reserve them for states that are known to be stable (e.g., test fixtures, locked states). For actively evolving workspaces, snapshot only metadata fields (`serial`, `lineage`, `resource_count`) rather than the full blob.

---

## 9. Quick Reference

```bash
# Build the binary first
make build

# Collect a snapshot (before backend update)
KH_ENDPOINT=https://api.keyharbour.test KH_TOKEN=pat KH_PROJECT=uuid make snapshot

# Verify after the update
KH_ENDPOINT=https://api.keyharbour.test KH_TOKEN=pat KH_PROJECT=uuid make regression

# Run diagnostics any time
KH_ENDPOINT=https://api.keyharbour.test KH_TOKEN=pat make diagnostics
```
