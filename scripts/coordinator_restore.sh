#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: ./scripts/coordinator_restore.sh [options] <backup.tar.gz | s3://bucket/key.tar.gz>

Restores a MarbleRun Coordinator PVC backup created by coordinator_backup.sh.

Options:
  --namespace NAME        Kubernetes namespace (default: marblerun)
  --deployment NAME       Coordinator Deployment name (default: coordinator)
  --pvc NAME              PVC name (default: coordinator-pvc)
  --image IMAGE           Helper image used to mount the PVC (default: alpine:3.20)
  --timeout SECONDS       kubectl wait/rollout timeout (default: 600)
  --s3-uri URI            S3 object to restore from (alternative to positional arg)
  --no-scale              Do not scale the coordinator Deployment down/up (not recommended with RWO PVCs)
  --skip-checksum         Skip verifying *.sha256 checksum if present
  -h, --help              Show this help

Environment variables (optional):
  KUBECTL_CONTEXT         kubectl context to use
  AWS_PROFILE             AWS profile for S3 download

Examples:
  ./scripts/coordinator_restore.sh ./backups/coordinator-pvc-20250101T000000Z.tar.gz
  ./scripts/coordinator_restore.sh --s3-uri s3://my-bucket/marblerun/coordinator-pvc-...tar.gz
EOF
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"
}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

NAMESPACE="marblerun"
DEPLOYMENT="coordinator"
PVC="coordinator-pvc"
IMAGE="alpine:3.20"
TIMEOUT_SECONDS="600"
S3_URI=""
NO_SCALE="false"
SKIP_CHECKSUM="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)
      NAMESPACE="${2:-}"; shift 2 ;;
    --deployment)
      DEPLOYMENT="${2:-}"; shift 2 ;;
    --pvc)
      PVC="${2:-}"; shift 2 ;;
    --image)
      IMAGE="${2:-}"; shift 2 ;;
    --timeout)
      TIMEOUT_SECONDS="${2:-}"; shift 2 ;;
    --s3-uri)
      S3_URI="${2:-}"; shift 2 ;;
    --no-scale)
      NO_SCALE="true"; shift ;;
    --skip-checksum)
      SKIP_CHECKSUM="true"; shift ;;
    -h|--help)
      usage; exit 0 ;;
    -*)
      die "Unknown argument: $1 (run --help)" ;;
    *)
      break ;;
  esac
done

backup_source="${1:-}"
if [[ -n "$S3_URI" && -n "$backup_source" ]]; then
  die "Provide either --s3-uri or a positional backup file, not both"
fi
if [[ -z "$S3_URI" && -z "$backup_source" ]]; then
  usage
  exit 2
fi

require_cmd kubectl
require_cmd date

KUBECTL_ARGS=()
if [[ -n "${KUBECTL_CONTEXT:-}" ]]; then
  KUBECTL_ARGS+=(--context "$KUBECTL_CONTEXT")
fi

restore_pod=""
previous_replicas=""
scaled_down="false"
tmp_dir=""
local_backup_file=""

cleanup() {
  set +e
  if [[ -n "$restore_pod" ]]; then
    kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" delete pod "$restore_pod" --ignore-not-found >/dev/null 2>&1 || true
  fi
  if [[ -n "$tmp_dir" ]]; then
    rm -rf "$tmp_dir" >/dev/null 2>&1 || true
  fi
  if [[ "$scaled_down" == "true" && "$NO_SCALE" != "true" && -n "$previous_replicas" ]]; then
    kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" scale deployment "$DEPLOYMENT" --replicas="$previous_replicas" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

start_epoch="$(date +%s)"

echo "==> Validating Kubernetes resources..."
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" get pvc "$PVC" >/dev/null
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" get deployment "$DEPLOYMENT" >/dev/null

tmp_dir="$(mktemp -d "${PROJECT_ROOT}/.coordinator-restore.XXXXXX")"

if [[ -n "$S3_URI" || "$backup_source" == s3://* ]]; then
  require_cmd aws
  s3_object="${S3_URI:-$backup_source}"
  local_backup_file="${tmp_dir}/$(basename "$s3_object")"
  echo "==> Downloading ${s3_object} to ${local_backup_file}..."
  aws s3 cp "$s3_object" "$local_backup_file"
else
  local_backup_file="$backup_source"
fi

[[ -f "$local_backup_file" ]] || die "Backup file not found: $local_backup_file"

if [[ "$SKIP_CHECKSUM" != "true" ]]; then
  checksum_candidate="${local_backup_file}.sha256"
  if [[ -f "$checksum_candidate" ]]; then
    echo "==> Verifying checksum ${checksum_candidate}..."
    if command -v sha256sum >/dev/null 2>&1; then
      (cd "$(dirname "$checksum_candidate")" && sha256sum -c "$(basename "$checksum_candidate")")
    else
      expected="$(awk '{print $1}' "$checksum_candidate")"
      actual="$(shasum -a 256 "$local_backup_file" | awk '{print $1}')"
      [[ "$expected" == "$actual" ]] || die "Checksum mismatch for $(basename "$local_backup_file")"
    fi
  else
    echo "==> No checksum file found (${checksum_candidate}); continuing."
  fi
fi

if [[ "$NO_SCALE" != "true" ]]; then
  echo "==> Scaling down deployment/${DEPLOYMENT} to avoid RWO multi-attach..."
  previous_replicas="$(kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" get deployment "$DEPLOYMENT" -o jsonpath='{.spec.replicas}')"
  kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" scale deployment "$DEPLOYMENT" --replicas=0
  scaled_down="true"
  kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" wait --for=delete pod -l app=coordinator --timeout="${TIMEOUT_SECONDS}s" >/dev/null 2>&1 || true
fi

timestamp="$(date -u +%Y%m%dt%H%M%Sz)"
restore_pod="coordinator-restore-${timestamp}"
restore_pod="${restore_pod//[^a-z0-9-]/-}"
restore_pod="${restore_pod:0:63}"

echo "==> Creating helper pod ${restore_pod}..."
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" apply -f - >/dev/null <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: ${restore_pod}
  labels:
    app: coordinator-restore
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  containers:
    - name: restore
      image: ${IMAGE}
      command: ["sh", "-c", "sleep 3600"]
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: ${PVC}
EOF

kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" wait --for=condition=Ready "pod/${restore_pod}" --timeout="${TIMEOUT_SECONDS}s" >/dev/null

echo "==> Clearing existing PVC data (if any)..."
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" exec "$restore_pod" -- sh -c 'rm -rf /data/* /data/.[!.]* /data/..?* 2>/dev/null || true'

echo "==> Restoring from ${local_backup_file}..."
cat "$local_backup_file" | kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" exec -i "$restore_pod" -- tar -xzf - -C /data

echo "==> Verifying restored data is non-empty..."
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" exec "$restore_pod" -- sh -c 'test -n "$(ls -A /data 2>/dev/null)"'

echo "==> Removing helper pod to release PVC..."
kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" delete pod "$restore_pod" --wait >/dev/null
restore_pod=""

if [[ "$NO_SCALE" != "true" ]]; then
  echo "==> Scaling deployment/${DEPLOYMENT} back to ${previous_replicas}..."
  kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" scale deployment "$DEPLOYMENT" --replicas="$previous_replicas"
  kubectl "${KUBECTL_ARGS[@]}" -n "$NAMESPACE" rollout status deployment "$DEPLOYMENT" --timeout="${TIMEOUT_SECONDS}s"
  scaled_down="false"
fi

end_epoch="$(date +%s)"
elapsed="$((end_epoch - start_epoch))"
echo "==> Restore complete in ${elapsed}s."
