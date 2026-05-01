package config

import (
	"os"
	"path/filepath"
	"testing"
)

func useTempConfigEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("KH_ENDPOINT", "")
	t.Setenv("KH_TOKEN", "")
	t.Setenv("KH_ORG", "")
	t.Setenv("KH_PROJECT", "")
	t.Setenv("KH_CONCURRENCY", "")
	t.Setenv("KH_INSECURE", "")
}

func TestLoadWithEnv_EnvVarsAppliedEvenOnLoadError(t *testing.T) {
	// Redirect HOME so UserConfigDir points into our temp dir.
	useTempConfigEnv(t)

	// Determine where the config file will be written and create it with bad JSON.
	cfgDir, err := configDir()
	if err != nil {
		t.Fatalf("configDir: %v", err)
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(cfgDir, "config")
	if err := os.WriteFile(cfgFile, []byte("{invalid json}"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("KH_ENDPOINT", "https://env.keyharbour.test/api/v2")
	t.Setenv("KH_TOKEN", "env-token")

	cfg, err := LoadWithEnv()
	if err == nil {
		t.Fatal("expected an error from malformed config file")
	}
	if cfg.Endpoint != "https://env.keyharbour.test/api/v2" {
		t.Errorf("KH_ENDPOINT not applied on load error, got %q", cfg.Endpoint)
	}
	if cfg.Token != "env-token" {
		t.Errorf("KH_TOKEN not applied on load error, got %q", cfg.Token)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	useTempConfigEnv(t)

	want := Config{
		Endpoint:    "https://app.keyharbour.test",
		Org:         "acme",
		Project:     "proj-1",
		Token:       "secret-token",
		Concurrency: 8,
	}
	if err := Save(want); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	p, err := Path()
	if err != nil {
		t.Fatalf("Path error: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got != want {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	useTempConfigEnv(t)

	got, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if got.Concurrency != 4 {
		t.Fatalf("expected default concurrency 4, got %d", got.Concurrency)
	}
}

func TestLoadWithEnv_AppliesOverridesAndClampsConcurrency(t *testing.T) {
	useTempConfigEnv(t)
	if err := Save(Config{Endpoint: "https://disk.keyharbour.test", Token: "disk-token", Concurrency: 4}); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	t.Setenv("KH_ENDPOINT", "https://env.keyharbour.test")
	t.Setenv("KH_TOKEN", "env-token")
	t.Setenv("KH_ORG", "env-org")
	t.Setenv("KH_PROJECT", "env-project")
	t.Setenv("KH_CONCURRENCY", "999")
	t.Setenv("KH_INSECURE", "yes")

	got, err := LoadWithEnv()
	if err != nil {
		t.Fatalf("LoadWithEnv error: %v", err)
	}
	if got.Endpoint != "https://env.keyharbour.test/api/v2" {
		t.Fatalf("expected normalized env endpoint, got %q", got.Endpoint)
	}
	if got.Token != "env-token" || got.Org != "env-org" || got.Project != "env-project" {
		t.Fatalf("unexpected env overrides: %+v", got)
	}
	if got.Concurrency != 64 {
		t.Fatalf("expected concurrency clamp to 64, got %d", got.Concurrency)
	}
	if !got.InsecureTLS {
		t.Fatal("expected insecure TLS override")
	}
}

func TestFromEnvOrAndFromEnvOrInt(t *testing.T) {
	useTempConfigEnv(t)
	cfg := Config{Endpoint: "cfg-endpoint", Org: "cfg-org", Project: "cfg-project", Token: "cfg-token", Concurrency: 7}

	if got := FromEnvOr(cfg, "KH_ENDPOINT", "default"); got != "cfg-endpoint" {
		t.Fatalf("expected config endpoint, got %q", got)
	}
	t.Setenv("KH_ENDPOINT", "env-endpoint")
	if got := FromEnvOr(cfg, "KH_ENDPOINT", "default"); got != "env-endpoint" {
		t.Fatalf("expected env endpoint, got %q", got)
	}
	if got := FromEnvOr(cfg, "UNKNOWN", "default"); got != "default" {
		t.Fatalf("expected default fallback, got %q", got)
	}

	if got := FromEnvOrInt(cfg, "KH_CONCURRENCY", 3); got != 7 {
		t.Fatalf("expected config concurrency, got %d", got)
	}
	t.Setenv("KH_CONCURRENCY", "11")
	if got := FromEnvOrInt(cfg, "KH_CONCURRENCY", 3); got != 11 {
		t.Fatalf("expected env concurrency, got %d", got)
	}
	t.Setenv("KH_CONCURRENCY", "not-a-number")
	if got := FromEnvOrInt(cfg, "KH_CONCURRENCY", 3); got != 7 {
		t.Fatalf("expected config fallback after invalid env value, got %d", got)
	}
}

func TestGetAndSet(t *testing.T) {
	cfg := Config{}
	if err := Set(&cfg, "endpoint", "https://example.test"); err != nil {
		t.Fatalf("Set endpoint error: %v", err)
	}
	if err := Set(&cfg, "org", "acme"); err != nil {
		t.Fatalf("Set org error: %v", err)
	}
	if err := Set(&cfg, "project", "proj-1"); err != nil {
		t.Fatalf("Set project error: %v", err)
	}
	if err := Set(&cfg, "token", "secret"); err != nil {
		t.Fatalf("Set token error: %v", err)
	}
	if err := Set(&cfg, "concurrency", "12"); err != nil {
		t.Fatalf("Set concurrency error: %v", err)
	}

	for field, want := range map[string]string{
		"endpoint":    "https://example.test",
		"org":         "acme",
		"project":     "proj-1",
		"token":       "secret",
		"concurrency": "12",
	} {
		got, err := Get(cfg, field)
		if err != nil {
			t.Fatalf("Get(%s) error: %v", field, err)
		}
		if got != want {
			t.Fatalf("Get(%s) = %q, want %q", field, got, want)
		}
	}
}

func TestGetAndSet_Errors(t *testing.T) {
	if _, err := Get(Config{}, "token"); err == nil {
		t.Fatal("expected error for missing token")
	}
	if _, err := Get(Config{}, "unknown"); err == nil {
		t.Fatal("expected error for unknown key")
	}
	if err := Set(&Config{}, "unknown", "value"); err == nil {
		t.Fatal("expected error for unknown key")
	}
	if err := Set(&Config{}, "concurrency", "bad"); err == nil {
		t.Fatal("expected error for invalid concurrency")
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://app.keyharbour.ca", "https://app.keyharbour.ca/api/v2"},
		{"https://app.keyharbour.ca/", "https://app.keyharbour.ca/api/v2"},
		{"https://app.keyharbour.ca/api/v2", "https://app.keyharbour.ca/api/v2"},
		{"https://app.keyharbour.ca/api/v2/", "https://app.keyharbour.ca/api/v2"},
		{"https://infra.acme.com/kh", "https://infra.acme.com/kh/api/v2"},
		{"https://infra.acme.com/kh/api/v2", "https://infra.acme.com/kh/api/v2"},
	}
	for _, tc := range tests {
		got := normalizeEndpoint(tc.input)
		if got != tc.want {
			t.Errorf("normalizeEndpoint(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
