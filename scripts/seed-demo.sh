#!/usr/bin/env bash
# seed-demo.sh — Populate KeyHarbour with realistic demo data
#
# Usage:
#   source scripts/.env
#   KH_BIN=./bin/kh ./scripts/seed-demo.sh
#
# Required env vars:
#   KH_ENDPOINT        API base URL
#   KH_TOKEN_DEV       Access token scoped to the dev environment
#   KH_TOKEN_PROD      Access token scoped to the prod environment
#   KH_PROJECT_INFRA   UUID of the "Platform Infrastructure" project
#   KH_PROJECT_DATA    UUID of the "Data Platform" project
#
# Optional:
#   KH_ENCRYPTION_KEY  Hex-encoded 32-byte AES key for client-side encryption
#                      Generate: openssl rand -hex 32

set -euo pipefail

KH="${KH_BIN:-kh}"
CLEANUP=false
for arg in "$@"; do
  [ "$arg" = "--cleanup" ] && CLEANUP=true
done

require_var() {
  local var="$1"
  if [ -z "${!var:-}" ]; then
    echo "Error: $var is required." >&2
    exit 1
  fi
}

require_var KH_ENDPOINT
require_var KH_TOKEN_DEV
require_var KH_TOKEN_PROD
require_var KH_TOKEN_APP
require_var KH_PROJECT_INFRA
require_var KH_PROJECT_DATA

INFRA="$KH_PROJECT_INFRA"
DATA="$KH_PROJECT_DATA"

ENCRYPT_FLAGS=""
if [ -n "${KH_ENCRYPTION_KEY:-}" ]; then
  ENCRYPT_FLAGS="--encryption-key $KH_ENCRYPTION_KEY"
fi

log() { echo "[seed] $*"; }

# ── Helpers ───────────────────────────────────────────────────────────────────

# Run kh with the dev token
kh_dev()  { KH_TOKEN="$KH_TOKEN_DEV"  "$KH" "$@"; }

# Run kh with the prod token
kh_prod() { KH_TOKEN="$KH_TOKEN_PROD" "$KH" "$@"; }

# Run kh with the app token (licenses, environment-agnostic resources)
kh_app()  { KH_TOKEN="$KH_TOKEN_APP"  "$KH" "$@"; }

# Workspace and statefile operations use the dev token
kh_any()  { KH_TOKEN="$KH_TOKEN_DEV"  "$KH" "$@"; }

# ── Existence caches (loaded per workspace section) ───────────────────────────

_ws_list_infra=""
_ws_list_data=""
_kv_list=""
_license_list=""

load_workspace_lists() {
  _ws_list_infra=$(kh_any workspaces ls --project "$INFRA" -o json 2>/dev/null || echo "[]")
  _ws_list_data=$(kh_any  workspaces ls --project "$DATA"  -o json 2>/dev/null || echo "[]")
}

# load_kv_list <token> <project> <workspace>
load_kv_list() {
  local token="$1" project="$2" workspace="$3"
  _kv_list=$(KH_TOKEN="$token" "$KH" kv ls --project "$project" --workspace "$workspace" -o json 2>/dev/null || echo "[]")
}

load_license_list() {
  _license_list=$(kh_app license ls -o json 2>/dev/null || echo "[]")
}

# json_has_name <json> <field> <value> — exits 0 if found, 1 if not
json_has_name() {
  echo "$1" | python3 -c "
import json, sys
data = json.load(sys.stdin)
sys.exit(0 if any(item.get('$2') == '$3' for item in data) else 1)
" 2>/dev/null
}

create_workspace() {
  local project="$1" name="$2" description="$3"
  local list="$_ws_list_infra"
  [ "$project" = "$DATA" ] && list="$_ws_list_data"
  if json_has_name "$list" "name" "$name"; then
    log "  workspace '$name' already exists, skipping"
    return
  fi
  log "  workspace '$name'"
  kh_any workspaces create "$name" --project "$project" --description "$description"
}

push_state() {
  local project="$1" workspace="$2" state_json="$3"
  echo "$state_json" | kh_any statefiles push --stdin \
    --project "$project" --workspace "$workspace"
}

# kv_dev  <project> <workspace> <key> <value> [extra flags...]
kv_dev()  {
  local project="$1" workspace="$2" key="$3" value="$4"; shift 4
  if json_has_name "$_kv_list" "key" "$key"; then
    log "  skip KV '$key' (exists)"
    return
  fi
  kh_dev kv set "$key" "$value" --project "$project" --workspace "$workspace" "$@"
}

# kv_prod <project> <workspace> <key> <value> [extra flags...]
kv_prod() {
  local project="$1" workspace="$2" key="$3" value="$4"; shift 4
  if json_has_name "$_kv_list" "key" "$key"; then
    log "  skip KV '$key' (exists)"
    return
  fi
  kh_prod kv set "$key" "$value" --project "$project" --workspace "$workspace" "$@"
}

create_license() {
  local name="$1"; shift
  if json_has_name "$_license_list" "name" "$name"; then
    log "  skip license '$name' (exists)"
    return
  fi
  create_license "$name" "$@"
}

# ── Project 1: Platform Infrastructure ────────────────────────────────────────

load_workspace_lists
load_license_list

# ── Cleanup ───────────────────────────────────────────────────────────────────

if $CLEANUP; then
  log "=== Cleanup: deleting all workspaces and licenses ==="

  for project in "$INFRA" "$DATA"; do
    kh_any workspaces ls --project "$project" -o json 2>/dev/null \
      | python3 -c "import json,sys; [print(w['uuid']) for w in json.load(sys.stdin)]" \
      | while read -r uuid; do
          log "  deleting workspace $uuid"
          kh_any workspaces delete "$uuid" --project "$project" --force
        done
  done

  kh_app license ls -o json 2>/dev/null \
    | python3 -c "import json,sys; [print(a['uuid']) for a in json.load(sys.stdin)]" \
    | while read -r uuid; do
        log "  deleting license $uuid"
        kh_app license delete "$uuid" --force
      done

  load_workspace_lists
  load_license_list
  log "Cleanup complete."
fi

log "=== Platform Infrastructure — workspaces ==="

create_workspace "$INFRA" "ekscluster"  "Amazon EKS cluster — Kubernetes control plane and node groups"
create_workspace "$INFRA" "rdspostgres" "Amazon RDS PostgreSQL — primary application database"
create_workspace "$INFRA" "networking"  "VPC, subnets, route tables, NAT gateways and security groups"
create_workspace "$INFRA" "apigw"       "AWS API Gateway — public-facing REST APIs and WAF rules"
create_workspace "$INFRA" "monitoring"  "Datadog agent, dashboards and alerting rules"
create_workspace "$INFRA" "vault"       "HashiCorp Vault cluster — secrets management"

# ── KV: ekscluster ────────────────────────────────────────────────────────────

log "KV — ekscluster/dev"
load_kv_list "$KH_TOKEN_DEV" "$INFRA" ekscluster
kv_dev  "$INFRA" ekscluster AWS_REGION             "us-east-1"
kv_dev  "$INFRA" ekscluster EKS_CLUSTER_NAME       "keyharbour-dev"
kv_dev  "$INFRA" ekscluster EKS_CLUSTER_VERSION    "1.29"
kv_dev  "$INFRA" ekscluster EKS_NODE_INSTANCE_TYPE "t3.large"
kv_dev  "$INFRA" ekscluster EKS_MIN_NODES          "2"
kv_dev  "$INFRA" ekscluster EKS_MAX_NODES          "8"
kv_dev  "$INFRA" ekscluster AWS_ACCESS_KEY_ID      "AKIAIOSFODNN7DEVEXMP" --private
kv_dev  "$INFRA" ekscluster AWS_SECRET_ACCESS_KEY  "wJalrXUtnFEMI/K7MDENG/bPxRfiCYDEVKEY" --private $ENCRYPT_FLAGS

log "KV — ekscluster/prod"
load_kv_list "$KH_TOKEN_PROD" "$INFRA" ekscluster
kv_prod "$INFRA" ekscluster AWS_REGION             "ca-central-1"
kv_prod "$INFRA" ekscluster EKS_CLUSTER_NAME       "keyharbour-prod"
kv_prod "$INFRA" ekscluster EKS_CLUSTER_VERSION    "1.29"
kv_prod "$INFRA" ekscluster EKS_NODE_INSTANCE_TYPE "m6i.xlarge"
kv_prod "$INFRA" ekscluster EKS_MIN_NODES          "3"
kv_prod "$INFRA" ekscluster EKS_MAX_NODES          "20"
kv_prod "$INFRA" ekscluster AWS_ACCESS_KEY_ID      "AKIAIOSFODNN7PRODXMP" --private
kv_prod "$INFRA" ekscluster AWS_SECRET_ACCESS_KEY  "wJalrXUtnFEMI/K7MDENG/bPxRfiCYPRODKEY" --private $ENCRYPT_FLAGS

# ── KV: rdspostgres ───────────────────────────────────────────────────────────

log "KV — rdspostgres/dev"
load_kv_list "$KH_TOKEN_DEV" "$INFRA" rdspostgres
kv_dev  "$INFRA" rdspostgres DB_INSTANCE_CLASS    "db.t3.medium"
kv_dev  "$INFRA" rdspostgres DB_ENGINE_VERSION    "15.4"
kv_dev  "$INFRA" rdspostgres DB_ALLOCATED_STORAGE "50"
kv_dev  "$INFRA" rdspostgres DB_MULTI_AZ          "false"
kv_dev  "$INFRA" rdspostgres DB_BACKUP_RETENTION  "3"
kv_dev  "$INFRA" rdspostgres DATABASE_URL \
  "postgresql://admin@keyharbour-dev-db.us-east-1.rds.amazonaws.com:5432/keyharbour" \
  --private $ENCRYPT_FLAGS
kv_dev  "$INFRA" rdspostgres DB_ROOT_PASSWORD "DevDbR00tP@ss!" --private $ENCRYPT_FLAGS

log "KV — rdspostgres/prod"
load_kv_list "$KH_TOKEN_PROD" "$INFRA" rdspostgres
kv_prod "$INFRA" rdspostgres DB_INSTANCE_CLASS    "db.r6g.large"
kv_prod "$INFRA" rdspostgres DB_ENGINE_VERSION    "15.4"
kv_prod "$INFRA" rdspostgres DB_ALLOCATED_STORAGE "500"
kv_prod "$INFRA" rdspostgres DB_MULTI_AZ          "true"
kv_prod "$INFRA" rdspostgres DB_BACKUP_RETENTION  "14"
kv_prod "$INFRA" rdspostgres DATABASE_URL \
  "postgresql://admin@keyharbour-prod-db.cluster.ca-central-1.rds.amazonaws.com:5432/keyharbour" \
  --private $ENCRYPT_FLAGS
kv_prod "$INFRA" rdspostgres DB_ROOT_PASSWORD "Pr0dDbRootP@ss2024" --private $ENCRYPT_FLAGS

# ── KV: monitoring ────────────────────────────────────────────────────────────

log "KV — monitoring/dev"
load_kv_list "$KH_TOKEN_DEV" "$INFRA" monitoring
kv_dev  "$INFRA" monitoring DATADOG_SITE    "datadoghq.com"
kv_dev  "$INFRA" monitoring DATADOG_API_KEY "dd_api_dev_EXAMPLE000000" --private $ENCRYPT_FLAGS
kv_dev  "$INFRA" monitoring DATADOG_APP_KEY "dd_app_dev_EXAMPLE000000" --private $ENCRYPT_FLAGS
kv_dev  "$INFRA" monitoring LOG_LEVEL       "debug"
kv_dev  "$INFRA" monitoring ALERT_EMAIL     "dev-alerts@example.com"

log "KV — monitoring/prod"
load_kv_list "$KH_TOKEN_PROD" "$INFRA" monitoring
kv_prod "$INFRA" monitoring DATADOG_SITE          "datadoghq.com"
kv_prod "$INFRA" monitoring DATADOG_API_KEY       "dd_api_prod_EXAMPLE000000" --private $ENCRYPT_FLAGS
kv_prod "$INFRA" monitoring DATADOG_APP_KEY       "dd_app_prod_EXAMPLE000000" --private $ENCRYPT_FLAGS
kv_prod "$INFRA" monitoring PAGERDUTY_ROUTING_KEY "R0000000000000000000001"   --private $ENCRYPT_FLAGS
kv_prod "$INFRA" monitoring LOG_LEVEL             "warn"
kv_prod "$INFRA" monitoring ALERT_EMAIL           "oncall@example.com"

# ── KV: vault ─────────────────────────────────────────────────────────────────

log "KV — vault/prod"
load_kv_list "$KH_TOKEN_PROD" "$INFRA" vault
kv_prod "$INFRA" vault VAULT_ADDR         "https://vault.internal:8200"
kv_prod "$INFRA" vault VAULT_CLUSTER_SIZE "3"
kv_prod "$INFRA" vault VAULT_KMS_KEY_ID \
  "arn:aws:kms:ca-central-1:123456789012:key/mrk-abcd1234" --private
kv_prod "$INFRA" vault VAULT_ROOT_TOKEN "hvs.EXAMPLETOKEN000000000000" --private $ENCRYPT_FLAGS

# ── State history: ekscluster ─────────────────────────────────────────────────

log "State history — ekscluster (3 versions)"

push_state "$INFRA" "ekscluster" '{
  "version": 4,
  "terraform_version": "1.6.6",
  "serial": 1,
  "lineage": "a3f8d2c1-1111-4a2b-8e6f-000000000001",
  "outputs": {},
  "resources": [
    {
      "module": "module.eks",
      "mode": "managed",
      "type": "aws_eks_cluster",
      "name": "main",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "keyharbour-prod",
          "name": "keyharbour-prod",
          "version": "1.28",
          "status": "ACTIVE",
          "endpoint": "https://AAAA0000000000000000.gr7.ca-central-1.eks.amazonaws.com",
          "role_arn": "arn:aws:iam::123456789012:role/eks-cluster-role"
        }
      }]
    },
    {
      "module": "module.eks",
      "mode": "managed",
      "type": "aws_eks_node_group",
      "name": "app",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "keyharbour-prod:app-nodes",
          "cluster_name": "keyharbour-prod",
          "node_group_name": "app-nodes",
          "instance_types": ["t3.medium"],
          "scaling_config": {"desired_size": 2, "max_size": 5, "min_size": 1},
          "status": "ACTIVE"
        }
      }]
    }
  ]
}'

push_state "$INFRA" "ekscluster" '{
  "version": 4,
  "terraform_version": "1.7.5",
  "serial": 12,
  "lineage": "a3f8d2c1-1111-4a2b-8e6f-000000000001",
  "outputs": {
    "cluster_endpoint": {
      "value": "https://AAAA0000000000000000.gr7.ca-central-1.eks.amazonaws.com",
      "type": "string"
    },
    "cluster_version": {"value": "1.29", "type": "string"}
  },
  "resources": [
    {
      "module": "module.eks",
      "mode": "managed",
      "type": "aws_eks_cluster",
      "name": "main",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "keyharbour-prod",
          "name": "keyharbour-prod",
          "version": "1.29",
          "status": "ACTIVE",
          "endpoint": "https://AAAA0000000000000000.gr7.ca-central-1.eks.amazonaws.com",
          "role_arn": "arn:aws:iam::123456789012:role/eks-cluster-role"
        }
      }]
    },
    {
      "module": "module.eks",
      "mode": "managed",
      "type": "aws_eks_node_group",
      "name": "app",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "keyharbour-prod:app-nodes",
          "cluster_name": "keyharbour-prod",
          "node_group_name": "app-nodes",
          "instance_types": ["m6i.xlarge"],
          "scaling_config": {"desired_size": 4, "max_size": 20, "min_size": 3},
          "status": "ACTIVE"
        }
      }]
    },
    {
      "module": "module.eks",
      "mode": "managed",
      "type": "helm_release",
      "name": "cluster_autoscaler",
      "provider": "provider[\"registry.terraform.io/hashicorp/helm\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "cluster-autoscaler",
          "name": "cluster-autoscaler",
          "namespace": "kube-system",
          "chart": "cluster-autoscaler",
          "version": "9.36.0",
          "status": "deployed"
        }
      }]
    }
  ]
}'

push_state "$INFRA" "ekscluster" '{
  "version": 4,
  "terraform_version": "1.8.5",
  "serial": 29,
  "lineage": "a3f8d2c1-1111-4a2b-8e6f-000000000001",
  "outputs": {
    "cluster_endpoint": {
      "value": "https://AAAA0000000000000000.gr7.ca-central-1.eks.amazonaws.com",
      "type": "string"
    },
    "cluster_version": {"value": "1.29", "type": "string"},
    "karpenter_role_arn": {
      "value": "arn:aws:iam::123456789012:role/karpenter-controller",
      "type": "string"
    }
  },
  "resources": [
    {
      "module": "module.eks",
      "mode": "managed",
      "type": "aws_eks_cluster",
      "name": "main",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "keyharbour-prod",
          "name": "keyharbour-prod",
          "version": "1.29",
          "status": "ACTIVE",
          "endpoint": "https://AAAA0000000000000000.gr7.ca-central-1.eks.amazonaws.com",
          "role_arn": "arn:aws:iam::123456789012:role/eks-cluster-role"
        }
      }]
    },
    {
      "module": "module.karpenter",
      "mode": "managed",
      "type": "helm_release",
      "name": "karpenter",
      "provider": "provider[\"registry.terraform.io/hashicorp/helm\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "karpenter",
          "name": "karpenter",
          "namespace": "kube-system",
          "chart": "karpenter",
          "version": "0.37.0",
          "status": "deployed"
        }
      }]
    }
  ]
}'

# ── State history: rdspostgres ────────────────────────────────────────────────

log "State history — rdspostgres (2 versions)"

push_state "$INFRA" "rdspostgres" '{
  "version": 4,
  "terraform_version": "1.7.2",
  "serial": 1,
  "lineage": "b9e1c3d5-2222-4f3c-9a7e-000000000002",
  "outputs": {
    "db_endpoint": {
      "value": "keyharbour-prod-db.cluster.ca-central-1.rds.amazonaws.com",
      "type": "string"
    }
  },
  "resources": [
    {
      "mode": "managed",
      "type": "aws_db_instance",
      "name": "main",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "keyharbour-prod-db",
          "identifier": "keyharbour-prod-db",
          "engine": "postgres",
          "engine_version": "15.4",
          "instance_class": "db.t3.medium",
          "allocated_storage": 100,
          "multi_az": false,
          "backup_retention_period": 7,
          "status": "available"
        }
      }]
    }
  ]
}'

push_state "$INFRA" "rdspostgres" '{
  "version": 4,
  "terraform_version": "1.8.5",
  "serial": 18,
  "lineage": "b9e1c3d5-2222-4f3c-9a7e-000000000002",
  "outputs": {
    "db_endpoint": {
      "value": "keyharbour-prod-db.cluster.ca-central-1.rds.amazonaws.com",
      "type": "string"
    },
    "db_replica_endpoint": {
      "value": "keyharbour-prod-db-ro.cluster-ro.ca-central-1.rds.amazonaws.com",
      "type": "string"
    }
  },
  "resources": [
    {
      "mode": "managed",
      "type": "aws_db_instance",
      "name": "main",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "keyharbour-prod-db",
          "identifier": "keyharbour-prod-db",
          "engine": "postgres",
          "engine_version": "15.4",
          "instance_class": "db.r6g.large",
          "allocated_storage": 500,
          "multi_az": true,
          "backup_retention_period": 14,
          "deletion_protection": true,
          "performance_insights_enabled": true,
          "status": "available"
        }
      }]
    },
    {
      "mode": "managed",
      "type": "aws_db_instance",
      "name": "read_replica",
      "provider": "provider[\"registry.terraform.io/hashicorp/aws\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "keyharbour-prod-db-ro",
          "identifier": "keyharbour-prod-db-ro",
          "engine": "postgres",
          "instance_class": "db.r6g.large",
          "replicate_source_db": "keyharbour-prod-db",
          "status": "available"
        }
      }]
    }
  ]
}'

# ── Project 2: Data Platform ───────────────────────────────────────────────────

log "=== Data Platform — workspaces ==="

create_workspace "$DATA" "snowflake"  "Snowflake databases, warehouses and RBAC roles"
create_workspace "$DATA" "fivetran"   "Fivetran connectors and destination configuration"
create_workspace "$DATA" "dbtcloud"   "dbt Cloud environments, jobs and webhooks"
create_workspace "$DATA" "gcsbuckets" "GCS buckets — raw, staging and curated data zones"
create_workspace "$DATA" "montecarlo" "Monte Carlo data observability — monitors and alerts"

# ── KV: snowflake ─────────────────────────────────────────────────────────────

log "KV — snowflake/dev"
load_kv_list "$KH_TOKEN_DEV" "$DATA" snowflake
kv_dev  "$DATA" snowflake SNOWFLAKE_ACCOUNT   "abc12345.us-east-1"
kv_dev  "$DATA" snowflake SNOWFLAKE_DATABASE  "ANALYTICS_DEV"
kv_dev  "$DATA" snowflake SNOWFLAKE_WAREHOUSE "TRANSFORM_WH_XS"
kv_dev  "$DATA" snowflake SNOWFLAKE_USER      "svc_terraform_dev"
kv_dev  "$DATA" snowflake SNOWFLAKE_ROLE      "SYSADMIN"
kv_dev  "$DATA" snowflake SNOWFLAKE_PASSWORD  "DevSnowflakePass2024" --private $ENCRYPT_FLAGS

log "KV — snowflake/prod"
load_kv_list "$KH_TOKEN_PROD" "$DATA" snowflake
kv_prod "$DATA" snowflake SNOWFLAKE_ACCOUNT   "abc12345.ca-central-1"
kv_prod "$DATA" snowflake SNOWFLAKE_DATABASE  "ANALYTICS_PROD"
kv_prod "$DATA" snowflake SNOWFLAKE_WAREHOUSE "TRANSFORM_WH_L"
kv_prod "$DATA" snowflake SNOWFLAKE_USER      "svc_terraform_prod"
kv_prod "$DATA" snowflake SNOWFLAKE_ROLE      "SYSADMIN"
kv_prod "$DATA" snowflake SNOWFLAKE_PASSWORD  "ProdSnowflakePassS3cr3t" --private $ENCRYPT_FLAGS

# ── KV: dbtcloud ──────────────────────────────────────────────────────────────

log "KV — dbtcloud/dev"
load_kv_list "$KH_TOKEN_DEV" "$DATA" dbtcloud
kv_dev  "$DATA" dbtcloud DBT_CLOUD_ACCOUNT_ID "12345"
kv_dev  "$DATA" dbtcloud DBT_CLOUD_TOKEN      "dbtc_DevToken0000000000000000000" --private $ENCRYPT_FLAGS
kv_dev  "$DATA" dbtcloud DBT_TARGET           "dev"
kv_dev  "$DATA" dbtcloud DBT_TARGET_SCHEMA    "dev_analytics"

log "KV — dbtcloud/prod"
load_kv_list "$KH_TOKEN_PROD" "$DATA" dbtcloud
kv_prod "$DATA" dbtcloud DBT_CLOUD_ACCOUNT_ID "12345"
kv_prod "$DATA" dbtcloud DBT_CLOUD_TOKEN      "dbtc_ProdToken000000000000000000" --private $ENCRYPT_FLAGS
kv_prod "$DATA" dbtcloud DBT_TARGET           "prod"
kv_prod "$DATA" dbtcloud DBT_TARGET_SCHEMA    "analytics"
kv_prod "$DATA" dbtcloud DBT_GIT_REPO         "git@github.com:example-org/dbt-analytics.git"

# ── KV: fivetran (prod only — single destination account) ─────────────────────

log "KV — fivetran/prod"
load_kv_list "$KH_TOKEN_PROD" "$DATA" fivetran
kv_prod "$DATA" fivetran FIVETRAN_API_KEY    "prod_ft_api_key_EXAMPLE" --private $ENCRYPT_FLAGS
kv_prod "$DATA" fivetran FIVETRAN_API_SECRET "prod_ft_secret_EXAMPLE"  --private $ENCRYPT_FLAGS
kv_prod "$DATA" fivetran FIVETRAN_GROUP_ID   "projected_warehouse_group"

# ── KV: montecarlo (prod only) ────────────────────────────────────────────────

log "KV — montecarlo/prod"
load_kv_list "$KH_TOKEN_PROD" "$DATA" montecarlo
kv_prod "$DATA" montecarlo MONTE_CARLO_API_KEY_ID "mc_key_id_EXAMPLE"     --private
kv_prod "$DATA" montecarlo MONTE_CARLO_API_KEY    "mc_prod_key_EXAMPLE00" --private $ENCRYPT_FLAGS

# ── State history: snowflake ──────────────────────────────────────────────────

log "State history — snowflake (3 versions)"

push_state "$DATA" "snowflake" '{
  "version": 4,
  "terraform_version": "1.7.4",
  "serial": 1,
  "lineage": "c4d2e8f0-3333-4b5d-a1c9-000000000003",
  "outputs": {
    "analytics_db": {"value": "ANALYTICS", "type": "string"}
  },
  "resources": [
    {
      "mode": "managed",
      "type": "snowflake_database",
      "name": "analytics",
      "provider": "provider[\"registry.terraform.io/Snowflake-Labs/snowflake\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "ANALYTICS",
          "name": "ANALYTICS",
          "comment": "Primary analytics database",
          "data_retention_time_in_days": 14
        }
      }]
    },
    {
      "mode": "managed",
      "type": "snowflake_warehouse",
      "name": "transform",
      "provider": "provider[\"registry.terraform.io/Snowflake-Labs/snowflake\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "TRANSFORM_WH",
          "name": "TRANSFORM_WH",
          "warehouse_size": "Medium",
          "auto_suspend": 120,
          "auto_resume": true
        }
      }]
    }
  ]
}'

push_state "$DATA" "snowflake" '{
  "version": 4,
  "terraform_version": "1.8.3",
  "serial": 15,
  "lineage": "c4d2e8f0-3333-4b5d-a1c9-000000000003",
  "outputs": {
    "analytics_db": {"value": "ANALYTICS", "type": "string"},
    "transformer_role": {"value": "TRANSFORMER", "type": "string"}
  },
  "resources": [
    {
      "mode": "managed",
      "type": "snowflake_database",
      "name": "analytics",
      "provider": "provider[\"registry.terraform.io/Snowflake-Labs/snowflake\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "ANALYTICS",
          "name": "ANALYTICS",
          "data_retention_time_in_days": 14
        }
      }]
    },
    {
      "mode": "managed",
      "type": "snowflake_role",
      "name": "transformer",
      "provider": "provider[\"registry.terraform.io/Snowflake-Labs/snowflake\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {"id": "TRANSFORMER", "name": "TRANSFORMER", "comment": "dbt transformation role"}
      }]
    },
    {
      "mode": "managed",
      "type": "snowflake_role",
      "name": "analyst",
      "provider": "provider[\"registry.terraform.io/Snowflake-Labs/snowflake\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {"id": "ANALYST", "name": "ANALYST", "comment": "Read-only BI analyst role"}
      }]
    },
    {
      "mode": "managed",
      "type": "snowflake_row_access_policy",
      "name": "pii_filter",
      "provider": "provider[\"registry.terraform.io/Snowflake-Labs/snowflake\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "ANALYTICS|PUBLIC|PII_FILTER_POLICY",
          "name": "PII_FILTER_POLICY",
          "database": "ANALYTICS",
          "schema": "PUBLIC"
        }
      }]
    }
  ]
}'

push_state "$DATA" "snowflake" '{
  "version": 4,
  "terraform_version": "1.8.5",
  "serial": 27,
  "lineage": "c4d2e8f0-3333-4b5d-a1c9-000000000003",
  "outputs": {
    "analytics_db": {"value": "ANALYTICS", "type": "string"},
    "snowpipe_sqs_arn": {
      "value": "arn:aws:sqs:ca-central-1:123456789012:snowflake-events-pipe",
      "type": "string"
    }
  },
  "resources": [
    {
      "mode": "managed",
      "type": "snowflake_database",
      "name": "analytics",
      "provider": "provider[\"registry.terraform.io/Snowflake-Labs/snowflake\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "ANALYTICS",
          "name": "ANALYTICS",
          "data_retention_time_in_days": 14
        }
      }]
    },
    {
      "mode": "managed",
      "type": "snowflake_pipe",
      "name": "events_ingest",
      "provider": "provider[\"registry.terraform.io/Snowflake-Labs/snowflake\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "ANALYTICS|INGEST|EVENTS_PIPE",
          "name": "EVENTS_PIPE",
          "database": "ANALYTICS",
          "schema": "INGEST",
          "auto_ingest": true,
          "notification_channel": "arn:aws:sqs:ca-central-1:123456789012:snowflake-events-pipe"
        }
      }]
    }
  ]
}'

# ── State history: gcsbuckets ─────────────────────────────────────────────────

log "State history — gcsbuckets (2 versions)"

push_state "$DATA" "gcsbuckets" '{
  "version": 4,
  "terraform_version": "1.7.5",
  "serial": 1,
  "lineage": "d7f3a9b2-4444-4c6e-b2da-000000000004",
  "outputs": {
    "raw_bucket": {"value": "data-platform-raw-zone", "type": "string"}
  },
  "resources": [
    {
      "mode": "managed",
      "type": "google_storage_bucket",
      "name": "raw_zone",
      "provider": "provider[\"registry.terraform.io/hashicorp/google\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "data-platform-raw-zone",
          "name": "data-platform-raw-zone",
          "location": "NORTHAMERICA-NORTHEAST1",
          "storage_class": "STANDARD",
          "versioning": [{"enabled": true}]
        }
      }]
    }
  ]
}'

push_state "$DATA" "gcsbuckets" '{
  "version": 4,
  "terraform_version": "1.8.5",
  "serial": 11,
  "lineage": "d7f3a9b2-4444-4c6e-b2da-000000000004",
  "outputs": {
    "raw_bucket":     {"value": "data-platform-raw-zone",     "type": "string"},
    "staging_bucket": {"value": "data-platform-staging-zone", "type": "string"},
    "curated_bucket": {"value": "data-platform-curated-zone", "type": "string"}
  },
  "resources": [
    {
      "mode": "managed",
      "type": "google_storage_bucket",
      "name": "raw_zone",
      "provider": "provider[\"registry.terraform.io/hashicorp/google\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "data-platform-raw-zone",
          "name": "data-platform-raw-zone",
          "location": "NORTHAMERICA-NORTHEAST1",
          "storage_class": "STANDARD",
          "versioning": [{"enabled": true}],
          "lifecycle_rule": [{"action": [{"type": "SetStorageClass", "storage_class": "NEARLINE"}], "condition": [{"age": 30}]}]
        }
      }]
    },
    {
      "mode": "managed",
      "type": "google_storage_bucket",
      "name": "staging_zone",
      "provider": "provider[\"registry.terraform.io/hashicorp/google\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "data-platform-staging-zone",
          "name": "data-platform-staging-zone",
          "location": "NORTHAMERICA-NORTHEAST1",
          "storage_class": "STANDARD",
          "versioning": [{"enabled": false}]
        }
      }]
    },
    {
      "mode": "managed",
      "type": "google_storage_bucket",
      "name": "curated_zone",
      "provider": "provider[\"registry.terraform.io/hashicorp/google\"]",
      "instances": [{
        "schema_version": 0,
        "attributes": {
          "id": "data-platform-curated-zone",
          "name": "data-platform-curated-zone",
          "location": "NORTHAMERICA-NORTHEAST1",
          "storage_class": "STANDARD",
          "versioning": [{"enabled": true}]
        }
      }]
    }
  ]
}'

# ── Software licenses ──────────────────────────────────────────────────────────

log "=== Software licenses ==="

create_license "GitHub Enterprise Cloud" \
  --short-name "GitHub Enterprise" \
  --vendor "GitHub (Microsoft)" \
  --owner "platform-team@example.com" \
  --tier "Enterprise" \
  --seats 200 \
  --renewal-date "2026-03-01"

create_license "Datadog Pro" \
  --short-name "Datadog" \
  --vendor "Datadog Inc." \
  --owner "ops-lead@example.com" \
  --tier "Pro" \
  --seats 25 \
  --renewal-date "2026-01-31"

create_license "PagerDuty Business" \
  --short-name "PagerDuty" \
  --vendor "PagerDuty Inc." \
  --owner "sre-lead@example.com" \
  --tier "Business" \
  --seats 30 \
  --renewal-date "2025-12-15"

create_license "HashiCorp Vault Enterprise" \
  --short-name "Vault" \
  --vendor "HashiCorp (IBM)" \
  --owner "security-team@example.com" \
  --tier "Enterprise" \
  --renewal-date "2026-06-01"

create_license "Snowflake Enterprise" \
  --short-name "Snowflake" \
  --vendor "Snowflake Inc." \
  --owner "data-lead@example.com" \
  --tier "Enterprise" \
  --renewal-date "2026-09-30"

create_license "Fivetran Business Critical" \
  --short-name "Fivetran" \
  --vendor "Fivetran Inc." \
  --owner "data-lead@example.com" \
  --tier "Business Critical" \
  --renewal-date "2026-08-01"

create_license "dbt Cloud Team" \
  --short-name "dbt Cloud" \
  --vendor "dbt Labs" \
  --owner "analytics-eng@example.com" \
  --tier "Team" \
  --seats 12 \
  --renewal-date "2025-11-01"

create_license "Grafana Cloud Pro" \
  --short-name "Grafana" \
  --vendor "Grafana Labs" \
  --owner "platform-team@example.com" \
  --tier "Pro" \
  --renewal-date "2026-02-28"

create_license "JetBrains All Products Pack" \
  --short-name "JetBrains" \
  --vendor "JetBrains s.r.o." \
  --owner "engineering@example.com" \
  --tier "All Products Pack" \
  --seats 40 \
  --renewal-date "2026-04-15"

create_license "Monte Carlo Data" \
  --short-name "Monte Carlo" \
  --vendor "Monte Carlo Data Inc." \
  --owner "data-lead@example.com" \
  --tier "Enterprise" \
  --renewal-date "2026-07-01"

log "=== Done ==="
log "Platform Infrastructure : ekscluster, rdspostgres, networking, apigw, monitoring, vault"
log "Data Platform            : snowflake, fivetran, dbtcloud, gcsbuckets, montecarlo"
log "State history            : ekscluster (3), rdspostgres (2), snowflake (3), gcsbuckets (2)"
log "Licenses                 : 10 records"
