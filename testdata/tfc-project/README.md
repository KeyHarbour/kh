# TFC Migration Test Project

Simple Terraform project backed by Terraform Cloud for testing migration to KeyHarbour.

## Setup

1. Ensure you have TFC credentials:
   ```bash
   export TF_API_TOKEN=your-tfc-token
   ```

2. Initialize and apply:
   ```bash
   terraform init
   terraform apply -auto-approve
   ```

3. Verify state exists in TFC.

## Migration Test

Once state exists in TFC, test migration to KeyHarbour:

```bash
# Set KeyHarbour credentials
export KH_ENDPOINT=https://api.keyharbour.ca
export KH_TOKEN=your-kh-token
export KH_PROJECT=your-project-uuid

# Dry-run first
kh migrate auto --project $KH_PROJECT --dry-run

# Execute migration
kh migrate auto --project $KH_PROJECT --workspace cli-migration-test

# Switch backend
mv cloud.tf cloud.tf.bak
terraform init -reconfigure -backend-config=backend.hcl

# Verify
terraform plan
```

## Cleanup

To destroy resources:
```bash
terraform destroy -auto-approve
```
