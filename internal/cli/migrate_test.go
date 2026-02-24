package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectBackend_HTTP(t *testing.T) {
	tmpDir := t.TempDir()
	
	backendTF := `terraform {
  backend "http" {
    address = "https://api.example.com/state"
    lock_address = "https://api.example.com/state/lock"
  }
}`
	
	if err := os.WriteFile(filepath.Join(tmpDir, "backend.tf"), []byte(backendTF), 0o644); err != nil {
		t.Fatal(err)
	}
	
	cfg, err := detectBackend(tmpDir)
	if err != nil {
		t.Fatalf("detectBackend failed: %v", err)
	}
	
	if cfg.Type != "http" {
		t.Errorf("expected backend type 'http', got '%s'", cfg.Type)
	}
	
	if cfg.Config["address"] != "https://api.example.com/state" {
		t.Errorf("expected address 'https://api.example.com/state', got '%s'", cfg.Config["address"])
	}
}

func TestDetectBackend_TerraformCloud(t *testing.T) {
	tmpDir := t.TempDir()
	
	cloudTF := `terraform {
  cloud {
    organization = "my-org"
    workspaces {
      name = "my-workspace"
    }
  }
}`
	
	if err := os.WriteFile(filepath.Join(tmpDir, "cloud.tf"), []byte(cloudTF), 0o644); err != nil {
		t.Fatal(err)
	}
	
	cfg, err := detectBackend(tmpDir)
	if err != nil {
		t.Fatalf("detectBackend failed: %v", err)
	}
	
	if cfg.Type != "tfc" {
		t.Errorf("expected backend type 'tfc', got '%s'", cfg.Type)
	}
	
	if cfg.Config["organization"] != "my-org" {
		t.Errorf("expected organization 'my-org', got '%s'", cfg.Config["organization"])
	}
}

func TestDetectBackend_Local(t *testing.T) {
	tmpDir := t.TempDir()
	
	// No backend config files - should default to local
	cfg, err := detectBackend(tmpDir)
	if err != nil {
		t.Fatalf("detectBackend failed: %v", err)
	}
	
	if cfg.Type != "local" {
		t.Errorf("expected backend type 'local', got '%s'", cfg.Type)
	}
}

func TestDetectBackend_S3(t *testing.T) {
	tmpDir := t.TempDir()
	
	backendTF := `terraform {
  backend "s3" {
    bucket = "my-terraform-state"
    key    = "prod/terraform.tfstate"
    region = "us-east-1"
  }
}`
	
	if err := os.WriteFile(filepath.Join(tmpDir, "backend.tf"), []byte(backendTF), 0o644); err != nil {
		t.Fatal(err)
	}
	
	cfg, err := detectBackend(tmpDir)
	if err != nil {
		t.Fatalf("detectBackend failed: %v", err)
	}
	
	if cfg.Type != "s3" {
		t.Errorf("expected backend type 's3', got '%s'", cfg.Type)
	}
	
	if cfg.Config["bucket"] != "my-terraform-state" {
		t.Errorf("expected bucket 'my-terraform-state', got '%s'", cfg.Config["bucket"])
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"MyProject", "myproject"},
		{"my-project", "my-project"},
		{"my_project", "my-project"},
		{"My Project", "my-project"},
		{"  MyProject  ", "myproject"},
		{"My-Cool_Project 2024", "my-cool-project-2024"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeID(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBackupBackendConfig(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, ".kh-migrate-backup")
	
	// Create a backend file
	backendPath := filepath.Join(tmpDir, "backend.tf")
	backendContent := `terraform {
  backend "http" {
    address = "https://example.com/state"
  }
}`
	if err := os.WriteFile(backendPath, []byte(backendContent), 0o644); err != nil {
		t.Fatal(err)
	}
	
	cfg := &BackendConfig{
		Type:     "http",
		FilePath: backendPath,
	}
	
	backupPath, err := backupBackendConfig(cfg, backupDir)
	if err != nil {
		t.Fatalf("backupBackendConfig failed: %v", err)
	}
	
	// Verify backup file exists
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Fatalf("backup file not created at %s", backupPath)
	}
	
	// Verify backup content matches original
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	
	if string(backupContent) != backendContent {
		t.Errorf("backup content doesn't match original")
	}
	
	// Verify BackupPath is set
	if cfg.BackupPath != backupPath {
		t.Errorf("BackupPath not set correctly")
	}
}

func TestParseBackendBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name: "simple key-value",
			input: `
				address = "https://example.com/state"
				lock_address = "https://example.com/lock"
			`,
			expected: map[string]string{
				"address":      "https://example.com/state",
				"lock_address": "https://example.com/lock",
			},
		},
		{
			name: "terraform cloud",
			input: `
				organization = "my-org"
				name = "my-workspace"
			`,
			expected: map[string]string{
				"organization": "my-org",
				"name":         "my-workspace",
			},
		},
		{
			name: "s3 backend",
			input: `
				bucket = "my-bucket"
				key = "path/to/state.tfstate"
				region = "us-east-1"
			`,
			expected: map[string]string{
				"bucket": "my-bucket",
				"key":    "path/to/state.tfstate",
				"region": "us-east-1",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseBackendBlock(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("result has %d keys, want %d: got %v", len(result), len(tt.expected), result)
			}
			for key, expectedValue := range tt.expected {
				if result[key] != expectedValue {
					t.Errorf("key %q: got %q, want %q", key, result[key], expectedValue)
				}
			}
		})
	}
}
