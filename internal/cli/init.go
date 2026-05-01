package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kh/internal/config"
	"kh/internal/kherrors"

	"github.com/spf13/cobra"
)

// Suggested helper name for reuse and tests
// scaffoldTerraformProject creates a minimal Terraform project scaffold
// under dir/<module>/<env> following best practices and using an HTTP backend
// pointing to the KeyHarbour service.
func scaffoldTerraformProject(dir, name, env, module, endpoint, _ string, _ string, force bool, backendType string, tfcOrg string, tfcWorkspace string) (string, error) {
	if name == "" {
		return "", kherrors.ErrMissingFlag.New("name is required")
	}
	if env == "" {
		return "", kherrors.ErrMissingFlag.New("env is required")
	}
	if module == "" {
		module = "infra"
	}
	if backendType == "" {
		backendType = "http"
	}
	if backendType == "http" {
		if endpoint == "" {
			endpoint = "https://api.keyharbour.test"
		}
		// Normalize endpoint (no trailing slash)
		endpoint = strings.TrimRight(endpoint, "/")
	}

	// Target directory layout: <dir>/<module>/<env>
	target := filepath.Join(dir, module, env)
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
		files[filepath.Join(target, "backend.hcl")] = terraformBackendHCL(endpoint)
	case "cloud":
		if tfcOrg == "" {
			return "", kherrors.ErrMissingFlag.New("--tfc-org (or TF_CLOUD_ORGANIZATION) is required for --backend=cloud")
		}
		if tfcWorkspace == "" {
			// default workspace: name-module-env
			tfcWorkspace = fmt.Sprintf("%s-%s-%s", sanitize(name), sanitize(module), sanitize(env))
		}
		files[filepath.Join(target, "cloud.tf")] = terraformCloudBlock(tfcOrg, tfcWorkspace)
	default:
		return "", kherrors.ErrInvalidValue.Newf("unsupported backend: %s (use http|cloud)", backendType)
	}

	for path, content := range files {
		if _, err := os.Stat(path); err == nil && !force {
			return "", kherrors.ErrInvalidValue.Newf("refusing to overwrite existing file without --force: %s", path)
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

// terraformBackendHCL generates backend.hcl using the V2 workspace-based path.
// Replace YOUR_WORKSPACE_UUID with the actual workspace UUID after creating the workspace.
func terraformBackendHCL(endpoint string) string {
	const placeholder = "YOUR_WORKSPACE_UUID"
	return fmt.Sprintf(`# Replace %s with your workspace UUID.
# Run 'kh tf version ls' after creating the workspace to find it.
address        = "%s/workspaces/%s/state"
lock_address   = "%s/workspaces/%s/state/lock"
unlock_address = "%s/workspaces/%s/state/lock"
lock_method    = "POST"
unlock_method  = "DELETE"
username       = "kh"
# password is read from TF_HTTP_PASSWORD environment variable
retry_max      = 2
`, placeholder, endpoint, placeholder, endpoint, placeholder, endpoint, placeholder)
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

1. (HTTP backend) Replace YOUR_WORKSPACE_UUID in backend.hcl with your workspace UUID.
   Create the workspace first, then run 'kh tf version ls' to find the UUID.

2. Set your API token:

   export TF_HTTP_PASSWORD=<your-kh-token>

3. Initialize backend (HTTP backend uses backend.hcl):

   terraform init -backend-config=backend.hcl

4. Optional: set variables (or edit locals in main.tf):

   terraform plan -var="project=%s" -var="environment=%s" -var="module=%s"

Notes:
- For HTTP backend: backend.tf uses partial configuration; backend.hcl carries the KeyHarbour addresses.
- For Terraform Cloud backend: cloud.tf defines the cloud block (no backend.hcl required).
- Commit .terraform.lock.hcl after the first init.
`, name, module, env, name, env, module)
}

// newTFInitCmd returns the `kh tf init` command (scaffold a Terraform project).
func newTFInitCmd() *cobra.Command {
	cmd := newInitProjectCmd()
	cmd.Use = "init"
	cmd.Short = "Scaffold a Terraform project configured for KeyHarbour"
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
			cfg, _ := config.LoadWithEnv()
			if endpoint == "" {
				endpoint = cfg.Endpoint
				if endpoint == "" {
					endpoint = "https://api.keyharbour.test"
				}
			}
			if org == "" {
				org = cfg.Org
			}
			if khProj == "" {
				khProj = cfg.Project
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
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "KeyHarbour API endpoint (defaults to KH_ENDPOINT or https://api.keyharbour.test)")
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
