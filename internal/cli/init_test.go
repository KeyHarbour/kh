package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scaffoldAndRead is a test helper that scaffolds a project and returns the
// contents of the generated files indexed by base filename.
func scaffoldAndRead(t *testing.T, dir, name, env, module, endpoint, org, khProject string, force bool, backendType, tfcOrg, tfcWs string) (target string, files map[string]string, err error) {
	t.Helper()
	target, err = scaffoldTerraformProject(dir, name, env, module, endpoint, org, khProject, force, backendType, tfcOrg, tfcWs)
	if err != nil {
		return target, nil, err
	}
	files = map[string]string{}
	entries, readErr := os.ReadDir(target)
	if readErr != nil {
		t.Fatalf("ReadDir: %v", readErr)
	}
	for _, e := range entries {
		data, readErr := os.ReadFile(filepath.Join(target, e.Name()))
		if readErr != nil {
			t.Fatalf("ReadFile %s: %v", e.Name(), readErr)
		}
		files[e.Name()] = string(data)
	}
	return target, files, nil
}

// --- Validation ---

func TestScaffold_MissingName(t *testing.T) {
	_, err := scaffoldTerraformProject(t.TempDir(), "", "dev", "infra", "", "", "", false, "http", "", "")
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected name-required error, got %v", err)
	}
}

func TestScaffold_MissingEnv(t *testing.T) {
	_, err := scaffoldTerraformProject(t.TempDir(), "myapp", "", "infra", "", "", "", false, "http", "", "")
	if err == nil || !strings.Contains(err.Error(), "env is required") {
		t.Fatalf("expected env-required error, got %v", err)
	}
}

func TestScaffold_UnsupportedBackend(t *testing.T) {
	_, err := scaffoldTerraformProject(t.TempDir(), "myapp", "dev", "infra", "", "", "", false, "s3", "", "")
	if err == nil || !strings.Contains(err.Error(), "unsupported backend") {
		t.Fatalf("expected unsupported-backend error, got %v", err)
	}
}

func TestScaffold_CloudBackend_MissingOrg(t *testing.T) {
	_, err := scaffoldTerraformProject(t.TempDir(), "myapp", "dev", "infra", "", "", "", false, "cloud", "", "")
	if err == nil || !strings.Contains(err.Error(), "--tfc-org") {
		t.Fatalf("expected tfc-org error, got %v", err)
	}
}

// --- HTTP backend ---

func TestScaffold_HTTP_CreatesExpectedFiles(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "https://api.example.com", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"backend.hcl", "backend.tf", "main.tf", "variables.tf", "outputs.tf", "versions.tf", "providers.tf", ".gitignore", "README.md"}
	for _, name := range expected {
		if _, ok := files[name]; !ok {
			t.Errorf("missing expected file: %s", name)
		}
	}
}

func TestScaffold_HTTP_DirectoryLayout(t *testing.T) {
	dir := t.TempDir()
	target, _, err := scaffoldAndRead(t, dir, "myapp", "prod", "network", "", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := filepath.Join(dir, "network", "prod")
	if target != want {
		t.Errorf("target = %q, want %q", target, want)
	}
}

func TestScaffold_HTTP_BackendHCL_ContainsEndpointAndV2Path(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "https://api.example.com", "", "proj-abc", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hcl := files["backend.hcl"]
	if !strings.Contains(hcl, "https://api.example.com") {
		t.Errorf("backend.hcl missing endpoint, got:\n%s", hcl)
	}
	if !strings.Contains(hcl, "/workspaces/") {
		t.Errorf("backend.hcl missing V2 workspace path, got:\n%s", hcl)
	}
	if !strings.Contains(hcl, "YOUR_WORKSPACE_UUID") {
		t.Errorf("backend.hcl missing UUID placeholder, got:\n%s", hcl)
	}
}

func TestScaffold_HTTP_DefaultEndpoint(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hcl := files["backend.hcl"]
	if !strings.Contains(hcl, "https://api.keyharbour.test") {
		t.Errorf("backend.hcl should use default endpoint, got:\n%s", hcl)
	}
}

func TestScaffold_HTTP_EndpointTrailingSlashNormalized(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "https://api.example.com///", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hcl := files["backend.hcl"]
	if strings.Contains(hcl, "///") {
		t.Errorf("backend.hcl contains un-normalized trailing slashes:\n%s", hcl)
	}
}

func TestScaffold_HTTP_DefaultModule(t *testing.T) {
	dir := t.TempDir()
	target, _, err := scaffoldAndRead(t, dir, "myapp", "dev", "", "", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// default module is "infra"
	want := filepath.Join(dir, "infra", "dev")
	if target != want {
		t.Errorf("target = %q, want %q", target, want)
	}
}

// --- Cloud backend ---

func TestScaffold_Cloud_CreatesCloudTF(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "", "", "", false, "cloud", "my-org", "my-workspace")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := files["cloud.tf"]; !ok {
		t.Error("missing cloud.tf for cloud backend")
	}
	if _, ok := files["backend.tf"]; ok {
		t.Error("cloud backend should not create backend.tf")
	}
	if _, ok := files["backend.hcl"]; ok {
		t.Error("cloud backend should not create backend.hcl")
	}
}

func TestScaffold_Cloud_CloudTF_ContainsOrgAndWorkspace(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "", "", "", false, "cloud", "acme-corp", "acme-prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cloud := files["cloud.tf"]
	if !strings.Contains(cloud, `"acme-corp"`) {
		t.Errorf("cloud.tf missing org, got:\n%s", cloud)
	}
	if !strings.Contains(cloud, `"acme-prod"`) {
		t.Errorf("cloud.tf missing workspace, got:\n%s", cloud)
	}
}

func TestScaffold_Cloud_DefaultWorkspaceDerived(t *testing.T) {
	dir := t.TempDir()
	// No workspace provided; it should default to name-module-env
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "", "", "", false, "cloud", "my-org", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cloud := files["cloud.tf"]
	if !strings.Contains(cloud, "myapp-infra-dev") {
		t.Errorf("cloud.tf should contain derived workspace name, got:\n%s", cloud)
	}
}

// --- File content sanity checks ---

func TestScaffold_MainTF_ContainsNameEnvModule(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "my-project", "staging", "api", "", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	main := files["main.tf"]
	for _, want := range []string{`"my-project"`, `"staging"`, `"api"`} {
		if !strings.Contains(main, want) {
			t.Errorf("main.tf missing %s, got:\n%s", want, main)
		}
	}
}

func TestScaffold_VersionsTF_HasRequiredVersion(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(files["versions.tf"], "required_version") {
		t.Error("versions.tf should contain required_version")
	}
}

func TestScaffold_GitIgnore_ExcludesTerraformDir(t *testing.T) {
	dir := t.TempDir()
	_, files, err := scaffoldAndRead(t, dir, "myapp", "dev", "infra", "", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(files[".gitignore"], ".terraform/") {
		t.Error(".gitignore should exclude .terraform/")
	}
}

// --- Force flag ---

func TestScaffold_RefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	// First scaffold
	_, err := scaffoldTerraformProject(dir, "myapp", "dev", "infra", "", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("first scaffold failed: %v", err)
	}

	// Second scaffold without --force
	_, err = scaffoldTerraformProject(dir, "myapp", "dev", "infra", "", "", "", false, "http", "", "")
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected overwrite refusal, got %v", err)
	}
}

func TestScaffold_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	// First scaffold
	_, err := scaffoldTerraformProject(dir, "myapp", "dev", "infra", "", "", "", false, "http", "", "")
	if err != nil {
		t.Fatalf("first scaffold failed: %v", err)
	}

	// Second scaffold with --force
	_, err = scaffoldTerraformProject(dir, "myapp", "dev", "infra", "", "", "", true, "http", "", "")
	if err != nil {
		t.Errorf("force overwrite failed: %v", err)
	}
}

// --- Sanitize ---

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MyProject", "myproject"},
		{"  hello world  ", "hello-world"},
		{"PROD", "prod"},
		{"already-clean", "already-clean"},
		{"Mixed CASE spaces", "mixed-case-spaces"},
	}
	for _, tt := range tests {
		got := sanitize(tt.input)
		if got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Command wiring ---

func TestNewInitProjectCmd_RequiredFlags(t *testing.T) {
	cmd := newInitProjectCmd()
	// Executing without required flags should fail
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when required flags are missing")
	}
}

func TestNewInitProjectCmd_Scaffold(t *testing.T) {
	dir := t.TempDir()
	var out strings.Builder
	cmd := newInitProjectCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--name", "testapp",
		"--env", "dev",
		"--dir", dir,
		"--backend", "http",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if !strings.Contains(out.String(), "Scaffolded Terraform project at") {
		t.Errorf("unexpected output: %s", out.String())
	}
}

func TestNewInitProjectCmd_TFCEnvVarFallback(t *testing.T) {
	t.Setenv("TF_CLOUD_ORGANIZATION", "env-org")
	t.Setenv("TF_WORKSPACE", "env-ws")

	dir := t.TempDir()
	var out strings.Builder
	cmd := newInitProjectCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{
		"--name", "testapp",
		"--env", "dev",
		"--dir", dir,
		"--backend", "cloud",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// cloud.tf should exist and contain the env-sourced org and workspace
	cloudTF, err := os.ReadFile(filepath.Join(dir, "infra", "dev", "cloud.tf"))
	if err != nil {
		t.Fatalf("cloud.tf not created: %v", err)
	}
	if !strings.Contains(string(cloudTF), `"env-org"`) {
		t.Errorf("cloud.tf missing TF_CLOUD_ORGANIZATION, got:\n%s", cloudTF)
	}
	if !strings.Contains(string(cloudTF), `"env-ws"`) {
		t.Errorf("cloud.tf missing TF_WORKSPACE, got:\n%s", cloudTF)
	}
}

func TestNewTFInitCmd_AliasesInitProject(t *testing.T) {
	cmd := newTFInitCmd()
	if cmd.Use != "init" {
		t.Errorf("Use = %q, want %q", cmd.Use, "init")
	}
}
