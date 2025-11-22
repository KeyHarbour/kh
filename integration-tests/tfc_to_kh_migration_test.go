package integrationtests
package integration_tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestTFCToKeyHarbourMigration exercises a full migration flow from
// Terraform Cloud into KeyHarbour using the tmp/app/dev project.
//
// This test is intentionally environment-driven and will be skipped unless
// the required environment variables and binaries are available:
//
//   - TF_API_TOKEN (or TFC_TOKEN or TF_TOKEN_app_terraform_io)
//   - TF_CLOUD_ORGANIZATION
//   - TF_WORKSPACE
//   - KH_ENDPOINT
//   - KH_TOKEN
//   - KH_PROJECT



































































































}	}		t.Fatalf("expected .kh-migrate-backup directory to be created: %v", err)	if _, err := os.Stat(filepath.Join(projectDir, ".kh-migrate-backup")); err != nil {	}		t.Fatalf("expected backend.hcl to be created: %v", err)	if _, err := os.Stat(filepath.Join(projectDir, "backend.hcl")); err != nil {	}		t.Fatalf("expected backend.tf to be created: %v", err)	if _, err := os.Stat(filepath.Join(projectDir, "backend.tf")); err != nil {	// 4) Assert that backend.tf, backend.hcl, and the backup directory exist.	run(t, khPath, migrateArgs...)	}		"-o", "json",		"--dir", ".",		"--project", os.Getenv("KH_PROJECT"),		"migrate", "auto",	migrateArgs := []string{	// generate backend.tf/backend.hcl and backups.	// 3) Run kh migrate auto to push local state into KeyHarbour and	run(t, khPath, importArgs...)	}		"-o", "json",		"--overwrite",		"--out", "terraform.tfstate",		"--tfc-workspace", os.Getenv("TF_WORKSPACE"),		"--tfc-org", os.Getenv("TF_CLOUD_ORGANIZATION"),		"--from=tfc",		"import", "tfstate",	importArgs := []string{	// test idempotent.	// We deliberately overwrite any existing local tfstate to keep the	// 2) Import state from TFC into a local terraform.tfstate using kh.	run(t, "terraform", "apply", "-auto-approve")	run(t, "terraform", "init")	// 1) Ensure Terraform is initialized and has produced a state in TFC.	}		}			t.Fatalf("command %s %v failed: %v\nOutput:\n%s", name, args, err, string(out))		if err != nil {		out, err := cmd.CombinedOutput()		cmd.Env = os.Environ()		cmd.Dir = projectDir		cmd := exec.Command(name, args...)		t.Helper()	run := func(t *testing.T, name string, args ...string) {	// Run commands in that directory.	}		t.Fatalf("tmp/app/dev not found: %v", err)	if _, err := os.Stat(projectDir); err != nil {	projectDir := filepath.Join(repoRoot, "tmp", "app", "dev")	// Work in tmp/app/dev.	}		t.Skip("kh binary not found at ./bin/kh; run `make build` first")	if _, err := os.Stat(khPath); err != nil {	khPath := filepath.Join(repoRoot, "bin", "kh")	}		t.Fatalf("failed to get working directory: %v", err)	if err != nil {	repoRoot, err := os.Getwd()	// Ensure kh binary has been built in ./bin/kh relative to repo root.	}		t.Skip("terraform binary not found on PATH; skipping integration migration test")	if _, err := exec.LookPath("terraform"); err != nil {	// Ensure terraform binary is available.	}		t.Skip("One of TF_API_TOKEN, TFC_TOKEN, TF_TOKEN_app_terraform_io must be set for integration migration test")	if os.Getenv("TF_API_TOKEN") == "" && os.Getenv("TFC_TOKEN") == "" && os.Getenv("TF_TOKEN_app_terraform_io") == "" {	}		t.Skip("TF_CLOUD_ORGANIZATION and TF_WORKSPACE must be set for integration migration test")	if os.Getenv("TF_CLOUD_ORGANIZATION") == "" || os.Getenv("TF_WORKSPACE") == "" {	}		t.Skip("KH_ENDPOINT, KH_TOKEN, KH_PROJECT must be set for integration migration test")	if os.Getenv("KH_ENDPOINT") == "" || os.Getenv("KH_TOKEN") == "" || os.Getenv("KH_PROJECT") == "" {	// Check for required env; skip if not configured.func TestTFCToKeyHarbourMigration(t *testing.T) {//   5. Assert that backend.tf/backend.hcl and backup files are created.//   4. Run `kh migrate auto` to push that state into KeyHarbour//   3. Use `kh import tfstate --from=tfc` to pull state into terraform.tfstate//   2. Ensure terraform init/apply succeed (so TFC has state)//   1. Change directory to tmp/app/dev// When enabled, the test will:////   - A KeyHarbour backend is reachable at KH_ENDPOINT//   - kh binary has been built at ./bin/kh from the repo root//   - terraform is available on PATH// It also expects that://