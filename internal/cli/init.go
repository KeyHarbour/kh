package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kh/internal/config"

	"github.com/spf13/cobra"
)

// Suggested helper name for reuse and tests
// scaffoldTerraformProject creates a minimal Terraform project scaffold
// under dir/<module>/<env> following best practices and using an HTTP backend
// pointing to the KeyHarbour service.
func scaffoldTerraformProject(dir, name, env, module, endpoint, org, khProject string, force bool, backendType string, tfcOrg string, tfcWorkspace string) (string, error) {
	if name == "" {
		return "", errors.New("name is required")
	}
	if env == "" {
		return "", errors.New("env is required")
	}
	if module == "" {
		module = "infra"
	}
	if backendType == "" {
		backendType = "http"
	}
	if backendType == "http" {
		if endpoint == "" {
			endpoint = "https://api.keyharbour.ca"
		}
		// Normalize endpoint (no trailing slash)
		endpoint = strings.TrimRight(endpoint, "/")
	}

	// Derive a stable state ID; this can be revisited later to match server conventions.
	// Format: <khProject>:<module>:<env> or fallback to name if khProject empty.
	base := khProject
	if base == "" {
		base = name
	}
	stateID := fmt.Sprintf("%s-%s-%s", sanitize(base), sanitize(module), sanitize(env))

	// Target directory layout: <dir>/<module>/<env>
	root := filepath.Join(dir)
	target := filepath.Join(root, module, env)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", err
	}

	// Files to create
	files := map[string]string{
		filepath.Join(target, "versions.tf"):  terraformVersionsTF(),
		filepath.Join(target, "providers.tf"): terraformProvidersTF(),
		filepath.Join(target, "variables.tf"): terraformVariablesTF(),
		filepath.Join(target, "outputs.tf"):   terraformOutputsTF(),
		filepath.Join(target, "main.tf"):      terraformMainTF(name, env, module),
		filepath.Join(target, "README.md"):    terraformReadme(name, env, module),
		filepath.Join(target, ".gitignore"):   terraformGitIgnore(),
	}
	switch backendType {
	case "http":
		files[filepath.Join(target, "backend.tf")] = terraformBackendTF()
		files[filepath.Join(target, "backend.hcl")] = terraformBackendHCL(endpoint, stateID)
	case "cloud":
		if tfcOrg == "" {
			return "", errors.New("--tfc-org (or TF_CLOUD_ORGANIZATION) is required for --backend=cloud")
		}
		if tfcWorkspace == "" {
			// default workspace: name-module-env
			tfcWorkspace = fmt.Sprintf("%s-%s-%s", sanitize(name), sanitize(module), sanitize(env))
		}
		files[filepath.Join(target, "cloud.tf")] = terraformCloudBlock(tfcOrg, tfcWorkspace)
	default:
		return "", fmt.Errorf("unsupported backend: %s (use http|cloud)", backendType)
	}

	for path, content := range files {
		if _, err := os.Stat(path); err == nil && !force {
			return "", fmt.Errorf("refusing to overwrite existing file without --force: %s", path)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	return target, nil
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func terraformBackendTF() string {
	return `terraform {
  backend "http" {}
}`
}

func terraformCloudBlock(org, workspace string) string {
	return fmt.Sprintf(`terraform {
	cloud {
		organization = "%s"
		workspaces {
			name = "%s"
		}
	}
}`, org, workspace)
}

func terraformBackendHCL(endpoint, stateID string) string {
	// HTTP backend with explicit lock/unlock routes
	return fmt.Sprintf(`address        = "%s/api/v1/states/%s"
lock_address   = "%s/api/v1/states/%s/lock"
unlock_address = "%s/api/v1/states/%s/unlock"
lock_method    = "POST"
unlock_method  = "POST"
retry_max      = 2
`, endpoint, stateID, endpoint, stateID, endpoint, stateID)
}

func terraformVersionsTF() string {
	return `terraform {
  required_version = ">= 1.6.0"
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = ">= 3.2.2"
    }
  }
}`
}

func terraformProvidersTF() string {
	return `provider "null" {}`
}

func terraformVariablesTF() string {
	return `variable "project" {
  description = "Project name"
  type        = string
}

variable "environment" {
  description = "Deployment environment (e.g., dev, staging, prod)"
  type        = string
}

variable "module" {
  description = "Module/component name"
  type        = string
  default     = "infra"
}`
}

func terraformOutputsTF() string {
	return `output "project" {
  value = var.project
}

output "environment" {
  value = var.environment
}`
}

func terraformMainTF(name, env, module string) string {
	return fmt.Sprintf(`locals {
  project     = "%s"
  environment = "%s"
  module      = "%s"
}

resource "null_resource" "placeholder" {
  triggers = {
    project     = local.project
    environment = local.environment
    module      = local.module
  }
}
`, name, env, module)
}

func terraformReadme(name, env, module string) string {
	return fmt.Sprintf(`# %s / %s / %s

This folder was scaffolded by kh to bootstrap a Terraform project with a backend managed by KeyHarbour or Terraform Cloud.

How to use:

1. Initialize backend (HTTP backend uses backend.hcl):

   terraform init -backend-config=backend.hcl

2. Optional: set variables (or edit locals in main.tf):

   terraform plan -var="project=%s" -var="environment=%s" -var="module=%s"

Notes:
- For HTTP backend: backend.tf uses partial configuration; backend.hcl carries the KeyHarbour addresses.
- For Terraform Cloud backend: cloud.tf defines the cloud block (no backend.hcl required).
- Commit .terraform.lock.hcl after the first init.
`, name, module, env, name, env, module)
}

// Cobra wiring
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Project initialization helpers",
	}
	cmd.AddCommand(newInitProjectCmd())
	return cmd
}

func terraformGitIgnore() string {
	return `# Terraform
.terraform/
*.tfstate
*.tfstate.*
crash.log
`
}

func newInitProjectCmd() *cobra.Command {
	var (
		name     string
		env      string
		module   string
		dir      string
		endpoint string
		org      string
		khProj   string
		force    bool
		backend  string
		tfcOrg   string
		tfcWs    string
	)
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Scaffold a minimal Terraform project configured for KeyHarbour backend",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load()
			if endpoint == "" {
				endpoint = config.FromEnvOr(cfg, "KH_ENDPOINT", "https://api.keyharbour.ca")
			}
			if org == "" {
				org = config.FromEnvOr(cfg, "KH_ORG", "")
			}
			if khProj == "" {
				khProj = config.FromEnvOr(cfg, "KH_PROJECT", "")
			}
			// Defaults for TFC flags from environment when not provided
			if tfcOrg == "" {
				if v := os.Getenv("TF_CLOUD_ORGANIZATION"); v != "" {
					tfcOrg = v
				}
			}
			if tfcWs == "" {
				if v := os.Getenv("TF_WORKSPACE"); v != "" {
					tfcWs = v
				}
			}
			target, err := scaffoldTerraformProject(dir, name, env, module, endpoint, org, khProj, force, backend, tfcOrg, tfcWs)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Scaffolded Terraform project at %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Human-friendly project name (required)")
	cmd.Flags().StringVarP(&env, "env", "e", "", "Environment (e.g., dev, staging, prod) (required)")
	cmd.Flags().StringVarP(&module, "module", "m", "infra", "Module/component name")
	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Base directory to scaffold into")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "KeyHarbour API endpoint (defaults to KH_ENDPOINT or https://api.keyharbour.ca)")
	cmd.Flags().StringVar(&org, "org", "", "KeyHarbour organization (defaults to KH_ORG)")
	cmd.Flags().StringVar(&khProj, "kh-project", "", "KeyHarbour project (defaults to KH_PROJECT)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing files")
	cmd.Flags().StringVar(&backend, "backend", "http", "Backend type: http|cloud")
	cmd.Flags().StringVar(&tfcOrg, "tfc-org", "", "Terraform Cloud organization (defaults to TF_CLOUD_ORGANIZATION)")
	cmd.Flags().StringVar(&tfcWs, "tfc-workspace", "", "Terraform Cloud workspace name (defaults to <name>-<module>-<env> or TF_WORKSPACE)")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("env")
	return cmd
}
