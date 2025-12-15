#!/bin/bash
#
# Apply Kubernetes Secret `service-layer-secrets` from the root `.env` file.
# This keeps K8s configuration in sync with local `.env` without committing secrets.
#
# Usage:
#   ./scripts/apply_k8s_secrets_from_env.sh [--namespace service-layer] [--name service-layer-secrets]
#
set -euo pipefail

NAMESPACE="service-layer"
SECRET_NAME="service-layer-secrets"
ENV_FILE=".env"
DRY_RUN=false

# Keys in `.env` that should NOT be copied into the runtime Secret.
# These are deploy-time or local-only settings (registry, Vercel, test wallets, etc.).
DENY_PREFIXES=(
  "VERCEL_"
  "VITE_"
  "SLCLI_"
  "TESTNET_"
  "NEO_TESTNET_"
  "POSTGRES_"
  "SUPABASE_DEV_"
  "SUPABASE_TEST_"
  "SUPABASE_PROD_"
  "IMAGE_"
)

DENY_KEYS=(
  "DATABASE_URL"
  "MANIFEST_PATH"
  "JWT_SECRET"
  "OAUTH_TOKENS_MASTER_KEY"
  "MARBLERUN_COORDINATOR_IMAGE"
  "SUPABASE_ANON_KEY"
  "EGO_VERSION"
  "ENVIRONMENT"
  "REGISTRY"
  "IMAGE_PREFIX"
  "IMAGE_TAG"
  "WAIT_TIMEOUT"
  "KUBECONFIG"
  "PUSH_TO_REGISTRY"
  "SKIP_BUILD"
  "SKIP_TESTS"
  "ROLLING_UPDATE"
  "DRY_RUN"
  "GATEWAY_HOSTNAME"
  "GATEWAY_TLS_SECRET"
  "INGRESS_CLASS"
  "CERT_MANAGER_CLUSTER_ISSUER"
  "FAIRY_RPC_URL"
  "TEE_PUBKEY"
)

is_denied_key() {
  local key="$1"
  for denied in "${DENY_KEYS[@]}"; do
    if [[ "$key" == "$denied" ]]; then
      return 0
    fi
  done
  for prefix in "${DENY_PREFIXES[@]}"; do
    if [[ "$key" == "${prefix}"* ]]; then
      return 0
    fi
  done
  return 1
}

usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Options:
  --namespace <ns>   Kubernetes namespace (default: service-layer)
  --name <name>      Secret name (default: service-layer-secrets)
  --env-file <path>  Path to env file (default: .env)
  --dry-run          Print generated Secret YAML without applying
  -h, --help         Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)
      NAMESPACE="$2"; shift 2;;
    --name)
      SECRET_NAME="$2"; shift 2;;
    --env-file)
      ENV_FILE="$2"; shift 2;;
    --dry-run)
      DRY_RUN=true; shift;;
    -h|--help)
      usage; exit 0;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1;;
  esac
done

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing $ENV_FILE. Create it from .env.example first." >&2
  exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl not found. Install kubectl to apply secrets." >&2
  exit 1
fi

if [[ "$DRY_RUN" == "true" ]]; then
  # Build a filtered env file first.
  tmp_env=$(mktemp)
  trap 'rm -f "$tmp_env"' EXIT

  skipped_empty_keys=()

  while IFS= read -r line || [[ -n "$line" ]]; do
    trimmed="${line#"${line%%[![:space:]]*}"}"
    [[ -z "$trimmed" || "$trimmed" == \#* ]] && continue
    if [[ "$trimmed" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
      key="${BASH_REMATCH[1]}"
      value="${BASH_REMATCH[2]}"
      if ! is_denied_key "$key"; then
        if [[ -z "$value" ]]; then
          skipped_empty_keys+=("$key")
          continue
        fi
        echo "$trimmed" >> "$tmp_env"
      fi
    fi
  done < "$ENV_FILE"

  if [[ "${#skipped_empty_keys[@]}" -gt 0 ]]; then
    echo "Skipping empty keys (not applied to Secret): ${skipped_empty_keys[*]}" >&2
  fi

  kubectl create secret generic "$SECRET_NAME" \
    --from-env-file="$tmp_env" \
    --namespace="$NAMESPACE" \
    --dry-run=client -o yaml
  exit 0
fi

# Build a filtered env file containing only runtime keys.
tmp_env=$(mktemp)
trap 'rm -f "$tmp_env"' EXIT

skipped_empty_keys=()

while IFS= read -r line || [[ -n "$line" ]]; do
  trimmed="${line#"${line%%[![:space:]]*}"}"
  [[ -z "$trimmed" || "$trimmed" == \#* ]] && continue
  if [[ "$trimmed" =~ ^([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]]; then
    key="${BASH_REMATCH[1]}"
    value="${BASH_REMATCH[2]}"
    if ! is_denied_key "$key"; then
      if [[ -z "$value" ]]; then
        skipped_empty_keys+=("$key")
        continue
      fi
      echo "$trimmed" >> "$tmp_env"
    fi
  fi
done < "$ENV_FILE"

if [[ "${#skipped_empty_keys[@]}" -gt 0 ]]; then
  echo "Skipping empty keys (not applied to Secret): ${skipped_empty_keys[*]}" >&2
fi

kubectl create secret generic "$SECRET_NAME" \
  --from-env-file="$tmp_env" \
  --namespace="$NAMESPACE" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "Applied Secret $SECRET_NAME in namespace $NAMESPACE from $ENV_FILE"
