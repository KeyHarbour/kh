package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithEnv_EnvVarsAppliedEvenOnLoadError(t *testing.T) {
	// Redirect HOME so UserConfigDir points into our temp dir.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "") // ensure Linux uses HOME-based fallback

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
